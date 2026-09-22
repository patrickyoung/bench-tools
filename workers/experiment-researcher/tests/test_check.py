"""Public checker regressions with synthetic reader processes; no inference."""
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

CHECK = Path(__file__).resolve().parents[1] / 'expert/bin/check'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


class CheckTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.work = self.root / 'work'
        self.work.mkdir()
        self.archive = self.root / 'archive'
        self.archive.mkdir()
        self.session = self.archive / 'sample.jsonl'
        self.session.write_text(json.dumps({'seq': 5, 'type': 'done', 'data': {'reason': 'error', 'error': 'answer was cut off at the output limit'}}))
        self.report = self.root / 'scores.txt'
        self.report.write_text('Independent score: candidate failed.\n')
        self.log = self.root / 'calls.jsonl'
        program = '''import json, pathlib, sys
p=pathlib.Path(__file__)
with (p.parent/'calls.jsonl').open('a') as out: out.write(json.dumps([p.name,*sys.argv[1:]])+'\\n')
if p.name=='ask':
    assert sys.argv[1:3]==['replay','-check']
    if (p.parent/'bad-replay').exists(): sys.exit(1)
elif p.name=='trail':
    assert sys.argv[1:6]==['window','-before','0','-after','0']
    if (p.parent/'warning').exists():
        print(json.dumps({'kind':'warning','message':'torn line'})); sys.exit(0)
    event=json.loads(pathlib.Path(sys.argv[6]).read_text())
    print(json.dumps({'kind':'event','event':event}))
elif p.name=='improve':
    assert sys.argv[1:]==['-n']
    spec=json.load(sys.stdin)
    if (p.parent/'bad-plan').exists(): sys.exit(2)
    print(json.dumps({'valid':True}))
else: sys.exit(99)
'''
        tools = {}
        for name in ('ask', 'trail', 'improve'):
            p = self.root / name
            p.write_text('#!' + sys.executable + '\n' + program)
            p.chmod(0o755)
            tools[name] = str(p)
        source = self.root / 'source'
        source.mkdir()
        (source / 'AGENTS.md').write_text('Return JSON.\n')
        cases = []
        for name in ('dev', 'held'):
            p = self.root / (name + '.json')
            p.write_text(json.dumps({'case': name}))
            cases.append({'id': name, 'family': name, 'file': str(p)})
        self.request = {
            'version': 1, 'question': 'Find a bounded improvement.',
            'sources': [{'id': 'runs', 'kind': 'archive', 'path': str(self.archive)},
                        {'id': 'scores', 'kind': 'text', 'path': str(self.report)}],
            'tools': tools, 'fresh_holdout': True,
            'template': {'version': 1, 'source': str(source), 'mutable': ['AGENTS.md'],
                         'development': [cases[0]], 'holdout': [cases[1]],
                         'commands': {n: {'argv': ['/unexecuted-adapter', n]} for n in ('propose', 'trial', 'judge')},
                         'dependencies': [], 'settings': {'runner_model': 'caller/model', 'gate': 'perfect'},
                         'repeats': 3, 'command_seconds': 30, 'max_seconds': 300}}
        self.request_path = self.root / 'request.json'
        self.bind_request()
        self.research = {
            'version': 1, 'request_sha256': sha(self.request_path), 'status': 'ready',
            'summary': 'A failed completion suggests a bounded test.',
            'hypothesis': 'Clarify the final-answer requirement.',
            'change': {'path': 'AGENTS.md', 'instruction': 'State one concise final-answer requirement.'},
            'citations': [{'source': 'runs', 'file': self.session.name, 'sha256': sha(self.session),
                           'seq': 5, 'quote': 'answer was cut off at the output limit'},
                          {'source': 'scores', 'sha256': sha(self.report), 'quote': 'candidate failed'}],
            'limitations': ['A quote does not establish causality.'], 'next_inputs': []}
        self.write_outputs()

    def bind_request(self):
        self.request_path.write_text(json.dumps(self.request))

    def write_outputs(self):
        (self.work / 'research.json').write_text(json.dumps(self.research))
        if self.research['status'] == 'ready':
            spec = copy.deepcopy(self.request['template'])
            spec['mutable'] = [self.research['change']['path']]
            spec['settings']['research'] = {k: self.research[k] for k in ('hypothesis', 'change', 'summary', 'citations')}
            (self.work / 'experiment.json').write_text(json.dumps(spec))

    def check(self, accepted, reason=None, assemble=False):
        env = dict(os.environ, EXPERIMENT_RESEARCH_REQUEST=str(self.request_path), PYTHONDONTWRITEBYTECODE='1')
        p = subprocess.run([sys.executable, str(CHECK)] + (['--assemble'] if assemble else []), cwd=self.work, env=env, capture_output=True, text=True, timeout=10)
        self.assertEqual(p.returncode, 0 if accepted else 1, p.stdout + p.stderr)
        result = json.loads(p.stdout)
        self.assertEqual(result['accepted'], accepted)
        self.assertNotIn('Traceback', p.stderr)
        if reason:
            self.assertIn(reason, result['reason'])
        return result

    def test_ready_uses_only_public_readers_and_plan(self):
        self.check(True)
        calls = [json.loads(line) for line in self.log.read_text().splitlines()]
        self.assertEqual([v[0] for v in calls], ['ask', 'trail', 'improve'])
        self.assertEqual(calls[-1], ['improve', '-n'])

    def test_missing_holdout_and_no_experiment_are_completed_research(self):
        for status in ('needs_input', 'no_experiment'):
            with self.subTest(status=status):
                self.request['template'] = None
                self.request['fresh_holdout'] = False
                self.bind_request()
                self.research.update(request_sha256=sha(self.request_path), status=status, hypothesis=None, change=None,
                                     next_inputs=['Supply fresh cases.'] if status == 'needs_input' else [])
                (self.work / 'experiment.json').unlink(missing_ok=True)
                self.write_outputs()
                self.check(True)

    def test_ready_rejects_missing_fresh_case_attestation(self):
        self.request['fresh_holdout'] = False
        self.bind_request()
        self.research['request_sha256'] = sha(self.request_path)
        self.write_outputs()
        self.check(False, 'fresh reserved cases')

    def test_stale_request_and_citation(self):
        self.request_path.write_text(self.request_path.read_text() + '\n')
        self.check(False, 'stale request')
        self.research['request_sha256'] = sha(self.request_path)
        self.write_outputs()
        self.session.write_text(self.session.read_text() + '\n')
        self.check(False, 'stale citation')

    def test_invented_or_unselected_citations(self):
        original = copy.deepcopy(self.research)
        for field, value, reason in [('quote', 'this never happened', 'quote absent'), ('seq', 42, 'wrong event'),
                                     ('seq', True, 'invalid event'), ('file', '../elsewhere.jsonl', 'basename'),
                                     ('source', 'unselected', 'unselected')]:
            with self.subTest(field=field):
                self.research = copy.deepcopy(original)
                self.research['citations'][0][field] = value
                self.write_outputs()
                self.check(False, reason)

    def test_bad_replay_warning_and_invalid_plan_fail(self):
        for flag in ('bad-replay', 'warning', 'bad-plan'):
            with self.subTest(flag=flag):
                marker = self.root / flag
                marker.touch()
                self.check(False)
                marker.unlink()

    def test_frozen_protocol_cannot_change(self):
        path = self.work / 'experiment.json'
        base = json.loads(path.read_text())
        changes = [lambda s: s['settings'].update(gate='always pass'),
                   lambda s: s['settings'].update(runner_model='another/model'),
                   lambda s: s.update(repeats=9),
                   lambda s: s['holdout'][0].update(file=self.request['template']['development'][0]['file']),
                   lambda s: s['commands']['judge'].update(argv=['/bin/true']),
                   lambda s: s.update(version=True),
                   lambda s: s['settings']['research'].update(hypothesis='Different hypothesis')]
        for change in changes:
            spec = copy.deepcopy(base)
            change(spec)
            path.write_text(json.dumps(spec))
            self.check(False, 'frozen template')

    def test_nonready_refuses_stale_experiment(self):
        self.research.update(status='no_experiment', hypothesis=None, change=None)
        self.write_outputs()
        self.check(False, 'experiment file')

    def test_schema_feedback_names_missing_and_extra_fields(self):
        del self.research['request_sha256']
        self.research['findings'] = ['Plausible prose in the wrong contract.']
        self.write_outputs()
        result = self.check(False, 'missing fields request_sha256')
        self.assertIn('unexpected fields findings', result['reason'])

    def test_quote_feedback_identifies_the_reference_to_repair(self):
        self.research['citations'][0]['quote'] = 'unrecorded statement'
        self.write_outputs()
        self.check(False, 'source=runs file=sample.jsonl seq=5')

    def test_ready_requires_archive_evidence(self):
        self.research['citations'] = self.research['citations'][1:]
        self.write_outputs()
        self.check(False, 'archive evidence')

    def test_duplicate_json_wrong_types_and_oversized_output(self):
        path = self.work / 'research.json'
        for data in ('{"version":1,"version":1}', '[]', 'null', '{"x":NaN}', ' ' * (2 * 1024 * 1024 + 1)):
            with self.subTest(length=len(data)):
                path.write_text(data)
                self.check(False)

    def test_symlinked_output_and_citation_fail(self):
        path = self.work / 'research.json'
        elsewhere = self.root / 'report.json'
        shutil.move(path, elsewhere)
        path.symlink_to(elsewhere)
        self.check(False, 'symlink')
        path.unlink()
        shutil.move(elsewhere, path)
        elsewhere = self.root / 'real-session.jsonl'
        shutil.move(self.session, elsewhere)
        self.session.symlink_to(elsewhere)
        self.check(False, 'symlink')

    def test_workspace_request_is_not_trusted(self):
        local = self.work / 'request.json'
        shutil.copyfile(self.request_path, local)
        self.request_path = local
        self.check(False, 'outside workspace')

    def test_mutable_surface_must_be_admitted(self):
        self.research['change']['path'] = 'bin/check'
        self.write_outputs()
        self.check(False, 'unadmitted')

    def test_assembly_creates_only_the_validated_trusted_plan(self):
        path = self.work / 'experiment.json'
        expected = json.loads(path.read_text())
        path.unlink()
        self.check(False)  # ordinary verification cannot create it
        self.assertFalse(path.exists())
        self.check(True, assemble=True)
        self.assertEqual(json.loads(path.read_text()), expected)
        before = path.read_bytes()
        self.check(True, assemble=True)
        self.assertEqual(path.read_bytes(), before)
        self.check(True)

    def test_assembly_never_writes_an_invalid_or_replaces_a_stale_plan(self):
        path = self.work / 'experiment.json'
        path.write_text('{}')
        self.check(False, 'frozen template', assemble=True)
        self.assertEqual(path.read_text(), '{}')
        path.unlink()
        (self.root / 'bad-plan').touch()
        self.check(False, 'validation failed', assemble=True)
        self.assertFalse(path.exists())

    def test_nonready_assembly_needs_no_plan(self):
        (self.work / 'experiment.json').unlink()
        self.research.update(status='needs_input', hypothesis=None, change=None, next_inputs=['Fresh cases.'])
        self.write_outputs()
        self.check(True, assemble=True)
        self.assertFalse((self.work / 'experiment.json').exists())


if __name__ == '__main__':
    unittest.main()
