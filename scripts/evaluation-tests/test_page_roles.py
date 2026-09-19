"""Positive, malformed and near-miss proposals/requests through public checks."""
import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class PageRoles(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.work = Path(self.tmp.name)
        (self.work/'output').mkdir()

    def run_check(self, role, candidate=None, expected=0):
        result = subprocess.run([str(ROOT/'workers'/role/'expert/bin/check')], cwd=self.work,
                                input=json.dumps(candidate) if candidate is not None else '',
                                capture_output=True, text=True, timeout=10)
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        return result

    def planner(self):
        packet = {'snapshot_sha256': 'sha256:'+'a'*64,
                  'snapshot': {'workers': ['frontend'], 'criteria': ['page']}}
        (self.work/'snapshot.json').write_text(json.dumps(packet))
        return dict(snapshot_sha256=packet['snapshot_sha256'], action='plan', reason='Build the admitted page',
                    tasks=[dict(id='page', needs=[], input=dict(goal='One accessible page', criteria=['page'],
                        worker='frontend', kind='integrate', output='index.html', data={}, turns=4, seconds=120))],
                    supersede=[], result='')

    def test_planner_positive_plan_and_finish(self):
        p = self.planner()
        self.run_check('page-planner', p)
        p.update(action='finish', tasks=[], result='Accepted page')
        self.run_check('page-planner', p)

    def test_planner_rejects_unadmitted_work_and_bounds(self):
        original = self.planner()
        self.run_check('page-planner', original)
        for mutation in [lambda p: p.update(snapshot_sha256='stale'),
                         lambda p: p.update(action='finish'),
                         lambda p: p.update(reason=' '),
                         lambda p: p.update(extra='unexpected'),
                         lambda p: p['tasks'][0]['input'].update(worker='unavailable'),
                         lambda p: p['tasks'][0]['input'].update(criteria=['invented']),
                         lambda p: p['tasks'][0]['input'].update(turns=13),
                         lambda p: p['tasks'][0]['input'].update(turns=0),
                         lambda p: p['tasks'][0]['input'].update(kind='deploy')]:
            p = copy.deepcopy(original)
            mutation(p)
            with self.subTest(proposal=p):
                self.run_check('page-planner', p, 1)

    def test_image_request_missing_malformed_and_bounds(self):
        request = self.work/'output/request.json'
        notes = self.work/'output/image-notes.md'
        notes.write_text('This is a synthetic composition request; pixels have not been generated.')
        good = {'prompt': 'A blue geometric landscape', 'references': []}
        request.write_text(json.dumps(good))
        result = self.run_check('image-concept')
        self.assertIn('image not yet generated', result.stdout)
        for bad in [{}, [], dict(good, prompt=' '), dict(good, prompt='x'*32001),
                    dict(good, references=['a']*4), dict(good, references='a'),
                    dict(good, generated=True)]:
            with self.subTest(request=type(bad).__name__):
                request.write_text(json.dumps(bad))
                self.run_check('image-concept', expected=1)
        request.write_text('{bad json')
        self.run_check('image-concept', expected=1)
        request.unlink()
        self.run_check('image-concept', expected=1)
        request.write_text(json.dumps(good))
        notes.unlink()
        self.run_check('image-concept', expected=1)


if __name__ == '__main__':
    unittest.main()
