import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent


class CreatorTests(unittest.TestCase):
    def test_creator_requires_and_binds_policy_before_execution(self):
        with tempfile.TemporaryDirectory() as temporary:
            folder = Path(temporary)
            policy_path, draft = folder / 'policy.json', folder / 'draft.json'
            policy = json.loads((ROOT / 'fixture-policy.json').read_text())
            spec = {'version': 1, 'settings': {'runner_model': 'explicit/model'},
                    'source': '/selected/source', 'mutable': ['AGENTS.md'], 'record': '/runtime/record',
                    'repeats': 2, 'command_seconds': 90, 'max_seconds': 900,
                    'development': [{'id': 'dev', 'family': 'a', 'file': '/inputs/dev.json'}],
                    'holdout': [{'id': 'held', 'family': 'b', 'file': '/inputs/held.json'}],
                    'dependencies': ['/adapters/trial.py'],
                    'commands': {name: {'argv': ['/adapters/' + name]} for name in ('propose', 'trial', 'judge')}}
            draft.write_text(json.dumps(spec))
            policy_path.write_text(json.dumps(policy))
            argv = [sys.executable, str(ROOT / 'spec.py'), '--experiment', str(draft)]
            def call(extra):
                return subprocess.run(argv + extra, capture_output=True, text=True, timeout=5)
            missing = call([])
            self.assertEqual(missing.returncode, 2)
            self.assertEqual(missing.stdout, '')
            result = call(['--policy', str(policy_path)])
            self.assertEqual(result.returncode, 0, result.stderr)
            bound = json.loads(result.stdout)
            self.assertEqual(bound['settings']['acceptance'], policy)
            self.assertEqual(bound['settings']['runner_model'], 'explicit/model')
            self.assertEqual(bound['commands']['trial'], spec['commands']['trial'])
            self.assertEqual(bound['commands']['propose'], spec['commands']['propose'])
            for key in ('source', 'mutable', 'development', 'holdout', 'record', 'repeats', 'command_seconds', 'max_seconds'):
                self.assertEqual(bound[key], spec[key])
            self.assertIn(str(policy_path.resolve()), bound['dependencies'])
            self.assertIn(str(ROOT / 'judge.py'), bound['dependencies'])
            self.assertEqual(json.loads(draft.read_text()), spec)
            policy['coverage']['holdout']['repeats'] = 3
            policy_path.write_text(json.dumps(policy))
            self.assertEqual(call(['--policy', str(policy_path)]).returncode, 2)
            policy['coverage']['holdout']['repeats'] = 2
            policy['coverage']['holdout']['cases'] = {'different': 'category'}
            policy_path.write_text(json.dumps(policy))
            self.assertEqual(call(['--policy', str(policy_path)]).returncode, 2)
            unresolved = call(['--policy', str(ROOT / 'policy.template.json')])
            self.assertEqual(unresolved.returncode, 2)
            self.assertEqual(unresolved.stdout, '')


if __name__ == '__main__':
    unittest.main()
