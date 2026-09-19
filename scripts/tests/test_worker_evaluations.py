"""Regression tests for catalog completeness, isolation and runner outcomes."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('worker_evaluations', ROOT/'scripts/check-worker-evaluations.py')
eval_runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(eval_runner)


class WorkerEvaluations(unittest.TestCase):
    def test_current_catalog_covers_every_entry_and_test(self):
        data = eval_runner.catalog()
        self.assertGreaterEqual(len(data['quality_cases']), 24)

    def test_new_entry_and_unmapped_test_fail_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root/'scripts').mkdir()
            (root/'workers/new/tests').mkdir(parents=True)
            (root/'workers/new/worker.json').write_text('{}')
            catalog = {'suites': [], 'quality_cases': {}}
            p = root/'scripts/worker-evaluations.json'
            p.write_text(json.dumps(catalog))
            with self.assertRaisesRegex(ValueError, 'without evaluation'):
                eval_runner.catalog(root)
            test = 'workers/new/tests/test_new.py'
            (root/test).write_text('')
            catalog['suites'] = [dict(id='new', profile='core', covers=['workers/new'],
                                      tests=[], commands=[['python3','test.py']])]
            catalog['quality_cases'] = {'workers/new': {k:'case' for k in ['ordinary','missing','near_miss','transfer','rubric']}}
            p.write_text(json.dumps(catalog))
            with self.assertRaisesRegex(ValueError, 'unmapped test'):
                eval_runner.catalog(root)

    def test_environment_drops_ambient_provider_and_optional_reviewer(self):
        from unittest.mock import patch
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            with patch.dict(os.environ, OPENAI_API_KEY='must-not-inherit', ASK_MODEL='paid',
                            PUBLICATION_AUTHORING_ONLY='1', WORKBOOK_VISUAL_ASK='/bad',
                            WORKBOOK_VISUAL_MODEL='paid', POLARS_REQUEST_SHA256='stale'):
                env = eval_runner.environment(root/'source', root, Path(sys.executable))
            for key in ('OPENAI_API_KEY','ASK_MODEL','PUBLICATION_AUTHORING_ONLY','WORKBOOK_VISUAL_ASK',
                        'WORKBOOK_VISUAL_MODEL','POLARS_REQUEST_SHA256'):
                self.assertNotIn(key, env)
            self.assertEqual(env['HOME'], str(root/'home'))
            self.assertTrue((root/'bin/python3').is_file())

    def test_process_failure_timeout_and_missing_command_are_not_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for args, timeout, expected in [([sys.executable,'-c','raise SystemExit(17)'], 5, 'failed'),
                                           ([sys.executable,'-c','import time;time.sleep(10)'], 1, 'timeout'),
                                           ([str(root/'missing')], 5, 'error')]:
                result = eval_runner.execute(args, root, dict(os.environ), root/'log', timeout)
                self.assertEqual(result['status'], expected)
            self.assertEqual(eval_runner.execute([sys.executable,'-c','print("ok")'], root,
                                                dict(os.environ), root/'pass.log',5)['status'], 'passed')

    def test_list_does_not_execute_or_create_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp)/'unused'
            p = subprocess.run([sys.executable, str(ROOT/'scripts/check-worker-evaluations.py'),
                                '--list','--profile','all','--entry','polars-analyst','--out',str(dest)],
                               capture_output=True,text=True,timeout=10)
            self.assertEqual(p.returncode,0,p.stderr)
            self.assertIn('statistics [analysis]',p.stdout)
            self.assertFalse(dest.exists())

    def test_missing_prerequisites_are_explicit(self):
        suite = dict(profile='core', prerequisites={'commands':['no-such-bench-eval-command'],
                                                   'selectors':['ABSENT_EVAL_RUNTIME']})
        missing = eval_runner.prerequisites(suite, {'PATH':'/usr/bin:/bin'}, sys.executable, ROOT)
        self.assertEqual(len(missing), 2)

    def test_quality_plan_is_unrun_and_refuses_overwrite(self):
        with tempfile.TemporaryDirectory() as tmp:
            dest = Path(tmp)/'quality'
            data = eval_runner.catalog()
            eval_runner.quality_plan(data, {'workers/product-owner'}, dest)
            self.assertIn('NOT RUN', (dest/'EVALUATION.md').read_text())
            self.assertEqual(set(json.loads((dest/'cases.json').read_text())), {'workers/product-owner'})
            with self.assertRaises(FileExistsError):
                eval_runner.quality_plan(data, {'workers/product-owner'}, dest)


if __name__ == '__main__':
    unittest.main()
