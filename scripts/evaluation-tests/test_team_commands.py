"""Real shell entry commands with explicit local process fixtures.

Fixtures test argv, ordering and failure propagation, not model output or the
substituted IO contracts (covered separately by team contract suites).
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


def executable(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    path.chmod(0o755)


class TeamCommands(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name).resolve()
        self.team = self.base/'team'
        self.events = self.base/'events.jsonl'
        self.env = dict(os.environ, EVENTS=str(self.events), ASK_MODEL='offline-fixture-only')

    def fixture_io(self):
        executable(self.team/'tools/team_io.py', '''#!/usr/bin/env python3
import json,os,pathlib,sys
args=sys.argv[1:]
with open(os.environ['EVENTS'],'a') as f:f.write(json.dumps(['io',*args])+'\\n')
if args[0]=='prepare':
 r=pathlib.Path(args[2]);r.mkdir()
 for role in ['manager','analyst','comparison','synthesis','reviewer']:
  (r/'stages'/role/'inputs/datasets').mkdir(parents=True)
 (r/'control').mkdir();(r/'control/analyst-request.json').write_text('{}')
 (r/'control/comparison-admission.json').write_text('{"packet_sha256":"fixture"}')
if args[0]=='failure':
 (pathlib.Path(args[1])/'status.json').write_text(json.dumps({'status':'unfinished','stage':args[2],'exit_code':int(args[3])}))
''')
        executable(self.base/'agent-fixture', '''#!/usr/bin/env python3
import json,os,pathlib,sys
args=sys.argv[1:];role=pathlib.Path(args[args.index('-C')+1]).name
with open(os.environ['EVENTS'],'a') as f:f.write(json.dumps(['agent',role,args])+'\\n')
assert args[0]=='run' and '-evidence' in args and '-record-input' in args and '-record-output' in args
assert args[args.index('-m')+1]=='offline-fixture-only'
assert args[args.index('--')-1].endswith('/agents/'+('manager' if role=='synthesis' else role))
if role==os.environ.get('FAIL_ROLE'):sys.exit(17)
''')
        self.env['COMPARISON_AGENT'] = str(self.base/'agent-fixture')

    def comparison(self, source, fail=None):
        self.team.mkdir()
        (self.team/'bin').mkdir()
        shutil.copy2(ROOT/'teams'/source/'expert/bin/compare-team', self.team/'bin/compare-team')
        self.fixture_io()
        if fail:
            self.env['FAIL_ROLE'] = fail
        result = subprocess.run([str(self.team/'bin/compare-team'), 'job with spaces.json', str(self.base/'run with spaces'), '--offline'],
                                env=self.env, capture_output=True, text=True, timeout=30)
        events = [json.loads(line) for line in self.events.read_text().splitlines()]
        self.assertEqual(result.returncode, 17 if fail else 0, result.stderr)
        roles = [e[1] for e in events if e[0]=='agent']
        expected = ['manager','analyst','comparison','synthesis','reviewer']
        self.assertEqual(roles, expected[:expected.index(fail)+1] if fail else expected)
        self.assertEqual(events[0][-1], '--offline')
        if fail:
            status = json.loads((self.base/'run with spaces/status.json').read_text())
            self.assertEqual(status, {'status':'unfinished','stage':fail,'exit_code':17})
            self.assertEqual(events[-1][1], 'failure')
        else:
            self.assertEqual([e[1] for e in events if e[0]=='io'],
                             ['prepare','handoff','comparison-inputs','synthesis-inputs','review-inputs','finish'])

    def test_comparison_entry_order_and_literal_paths(self):
        self.comparison('vendor-comparison-team')

    def test_comparison_stops_and_retains_failed_stage(self):
        self.comparison('vendor-comparison-team', 'analyst')

    def test_studio_analytical_copy_preserves_failure(self):
        self.comparison('vendor-decision-studio', 'reviewer')

    def test_studio_does_not_publish_after_analysis_failure(self):
        shutil.copytree(ROOT/'teams/vendor-decision-studio/expert', self.team)
        executable(self.team/'bin/compare-team', '#!/bin/sh\nexit 17\n')
        executable(self.team/'bin/publish-decision', '#!/bin/sh\necho forbidden > "$EVENTS"\nexit 0\n')
        run = self.base/'studio'
        p = subprocess.run([str(self.team/'bin/compare-studio'), 'job.json', str(run), '--offline'],
                           env=self.env, capture_output=True, timeout=20)
        self.assertEqual(p.returncode, 17)
        self.assertFalse(self.events.exists())
        marker = run/'keep'
        marker.write_text('retained')
        p = subprocess.run([str(self.team/'bin/compare-studio'), 'job.json', str(run)],
                           env=self.env, capture_output=True, timeout=20)
        self.assertEqual(p.returncode, 1)
        self.assertEqual(marker.read_text(), 'retained')


if __name__ == '__main__':
    unittest.main()
