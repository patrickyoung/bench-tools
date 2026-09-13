"""Copied roles must check their own outputs without a parent team's files.

These are synthetic format and rejection cases, not native-art or model evals.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class IndependentRoles(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='bench-role-')
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name).resolve()
        self.work = self.base / 'work'
        self.work.mkdir()
        (self.work / 'output').mkdir()

    def select(self, worker):
        self.expert = self.base / 'standalone'
        shutil.copytree(ROOT / 'workers' / worker / 'expert', self.expert)
        self.env = dict(os.environ, AGENT_HOME=str(self.expert),
                        PAGE_TEAM_ROOT=str(self.base / 'missing-parent'),
                        PYTHONDONTWRITEBYTECODE='1')

    def write(self, name, data):
        path = self.work / 'output' / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data.encode() if isinstance(data, str) else data)
        return path

    def run_command(self, argv, *, data=None, accept=True):
        result = subprocess.run([str(arg) for arg in argv], cwd=self.work,
                                env=self.env, input=data, capture_output=True, timeout=20)
        message = result.stdout.decode(errors='replace') + result.stderr.decode(errors='replace')
        if accept:
            self.assertEqual(result.returncode, 0, message)
        else:
            self.assertNotEqual(result.returncode, 0, message)
        return result

    def handoff(self, role):
        files = sorted(str(p.relative_to(self.work)) for p in (self.work / 'output').rglob('*') if p.is_file())
        self.run_command([self.expert / 'tools/make-handoff', role, 'Synthetic portable role case', *files])

    def check(self, *, data=None, accept=True):
        return self.run_command([self.expert / 'bin/check'], data=data, accept=accept)

    def tamper(self, name):
        path = self.work / 'output' / name
        path.write_bytes(path.read_bytes() + b' changed after handoff')
        self.check(accept=False)

    def test_frontend_is_independent_and_binds_the_delivered_bytes(self):
        self.select('frontend')
        page = (ROOT / 'teams/page-team/tests/browser-fixture.html').read_bytes()
        self.write('index.html', page)
        self.handoff('frontend')
        self.check()
        self.tamper('index.html')
        self.write('index.html', '<html>incomplete</html>')
        self.handoff('frontend')
        self.check(accept=False)

    def test_canvas_is_independent_and_binds_its_sources(self):
        self.select('canvas-artist')
        self.write('visual.js', 'globalThis.fixture = true;\n')
        self.write('visual.css', ':root { color: black; }')
        self.write('fallback.svg', '<svg xmlns="http://www.w3.org/2000/svg"/>')
        self.write('visual-notes.md', 'Synthetic source contract fixture.')
        self.handoff('visual-artist')
        self.check()
        self.tamper('visual.css')

    def test_blender_is_independent_and_rejects_changed_master(self):
        self.select('blender-artist')
        # Signatures exercise the existing bounded check, not Blender validity.
        self.write('asset.blend', b'BLENDER' + b'0' * 40)
        self.write('preview.png', b'\x89PNG\r\n\x1a\n' + b'0' * 40)
        self.write('asset.glb', b'glTF' + b'0' * 40)
        self.write('build_asset.py', '# synthetic checker fixture only\n' * 2)
        self.handoff('blender')
        self.check()
        self.tamper('asset.blend')

    def test_image_editor_is_independent_and_preserves_bound_originals(self):
        self.select('image-editor')
        self.write('composite.xcf', b'gimp xcf ' + b'0' * 40)
        self.write('composite.png', b'\x89PNG\r\n\x1a\n' + b'0' * 40)
        self.write('finish.py', '# synthetic checker fixture only\n')
        self.write('finish-notes.md', 'Synthetic signatures; not a native art evaluation.')
        self.write('originals/input.txt', 'Explicit fixture input')
        self.handoff('gimp')
        self.check()
        self.tamper('originals/input.txt')

    def test_reviewer_is_independent_and_rejects_changed_observations(self):
        self.select('page-reviewer')
        review = {key: [] for key in ('structural', 'functional', 'visual', 'responsive',
                                     'accessibility', 'provenance', 'limitations')}
        review.update(verdict='revise', limitations=['No browser observations supplied.'])
        self.write('review.json', json.dumps(review))
        self.write('review.md', 'Revision needed; observations are missing.')
        self.handoff('review')
        self.check()
        self.tamper('review.md')
        self.write('review.json', '{"verdict":"pass","findings":{}}')
        self.handoff('review')
        self.check(accept=False)

    def test_image_concept_checks_a_request_without_claiming_generation(self):
        self.select('image-concept')
        self.write('request.json', json.dumps({'prompt': 'Synthetic composition request', 'references': []}))
        self.write('image-notes.md', 'This is a request validation fixture, not generated artwork.')
        result = self.check()
        self.assertIn(b'image not yet generated', result.stdout)
        self.assertFalse((self.work / 'handoff.json').exists())
        self.write('request.json', '{"prompt":""}')
        self.check(accept=False)

    def test_page_planner_checks_stdout_candidate_against_supplied_snapshot(self):
        self.select('page-planner')
        packet = {'snapshot_sha256': 'sha256:' + 'a' * 64,
                  'snapshot': {'workers': ['frontend'], 'criteria': ['page']}}
        (self.work / 'snapshot.json').write_text(json.dumps(packet))
        proposal = {'snapshot_sha256': packet['snapshot_sha256'], 'action': 'blocked',
                    'reason': 'Fixture has no admitted work', 'tasks': [], 'supersede': [], 'result': ''}
        self.check(data=json.dumps(proposal).encode())
        proposal['snapshot_sha256'] = 'sha256:' + 'b' * 64
        self.check(data=json.dumps(proposal).encode(), accept=False)
        self.assertFalse((self.work / 'handoff.json').exists())


if __name__ == '__main__':
    unittest.main()
