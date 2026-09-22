"""Offline contract tests. No model calls or ambient provider credentials."""
import copy
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

from compare import compare
from score import strict
from trial import fingerprint


def scores():
    def make(side, repeat):
        return {'version': 1, 'trial': f'{side}-{repeat}', 'expert_sha256': side,
                'input_sha256': 'input', 'labels_sha256': 'labels', 'scorer_sha256': 'scorer',
                'settings': {'model': 'fixture'}, 'reported_models': ['fixture'],
                'status': 'ok', 'total': 2, 'correct': 2,
                'rows': [{'id': 'a', 'actual': 'billing', 'expected': 'billing', 'correct': True},
                         {'id': 'b', 'actual': 'general', 'expected': 'general', 'correct': True}],
                'input_tokens': 100 if side == 'old' else 90,
                'instruction_bytes': 200 if side == 'old' else 150,
                'model_calls': 1, 'seconds': 5, 'cost': None}
    return {'baseline': [make('old', i) for i in range(2)],
            'candidate': [make('new', i) for i in range(2)]}


class ComparisonTests(unittest.TestCase):
    def test_supported_efficiency(self):
        self.assertEqual(compare(scores())['decision'], 'supported')

    def test_duplicate_is_not_improvement(self):
        data = scores()
        data['candidate'] = copy.deepcopy(data['baseline'])
        self.assertEqual(compare(data)['decision'], 'keep_baseline')

    def test_correcting_baseline_errors_is_not_a_regression(self):
        data = scores()
        data['baseline'][0]['rows'][0].update(actual='general', correct=False)
        data['baseline'][0]['correct'] = 1
        self.assertEqual(compare(data)['decision'], 'supported')

    def test_missing_metric_and_execution_failure_block(self):
        for key, value in [('input_tokens', None), ('status', 'failed'),
                           ('model_calls', 2), ('instruction_bytes', 201)]:
            with self.subTest(key=key):
                data = scores()
                data['candidate'][0][key] = value
                self.assertEqual(compare(data)['decision'], 'keep_baseline')

    def test_one_regression_blocks(self):
        data = scores()
        bad = data['candidate'][1]
        bad['rows'][0].update(actual='general', correct=False)
        bad['correct'] = 1
        self.assertEqual(compare(data)['decision'], 'keep_baseline')

    def test_mismatched_labels_or_model_refused(self):
        for key in ('labels_sha256', 'settings', 'scorer_sha256', 'input_sha256', 'reported_models'):
            with self.subTest(key=key):
                data = scores()
                data['candidate'][0][key] = 'different'
                with self.assertRaises(ValueError):
                    compare(data)

    def test_reused_trial_is_not_repeat(self):
        data = scores()
        data['baseline'][1] = copy.deepcopy(data['baseline'][0])
        with self.assertRaises(ValueError):
            compare(data)

    def test_duplicate_keys_refused(self):
        with self.assertRaises(ValueError):
            strict('{"correct":1,"correct":2}')

    def test_nonfinite_numbers_refused(self):
        with self.assertRaises(ValueError):
            strict('{"cost":NaN}')

    def test_false_count_refused(self):
        data = scores()
        data['candidate'][0]['correct'] = 3
        with self.assertRaises(ValueError):
            compare(data)


class DefinitionTests(unittest.TestCase):
    def test_content_and_mode_change_fingerprint(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'bin').mkdir()
            (root / 'AGENTS.md').write_text('Do the job.')
            check = root / 'bin/check'
            check.write_text('#!/bin/sh\nexit 1\n')
            before = fingerprint(root)
            check.chmod(0o755)
            executable = fingerprint(root)
            self.assertNotEqual(before, executable)
            (root / 'AGENTS.md').write_text('Do the job well.')
            self.assertNotEqual(executable, fingerprint(root))

    def test_symlinks_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'AGENTS.md').symlink_to('/dev/null')
            with self.assertRaises(ValueError):
                fingerprint(root)

    def test_structural_checker(self):
        check = Path(__file__).parent / 'naive/bin/check'
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'request.json').write_text('{"notes":[{"id":"a"}]}')
            cases = [(b'', 1), (b'[{"id":"a","queue":"general"}]', 0),
                     (b'[{"id":"stale","queue":"general"}]', 1),
                     (b'[{"id":"a","queue":"sales"}]', 1),
                     (b'[{"id":"a","queue":"billing","queue":"general"}]', 1)]
            for raw, expected in cases:
                self.assertEqual(subprocess.run([str(check.resolve())], input=raw, cwd=root,
                                               capture_output=True).returncode, expected)
            (root / 'request.json').unlink()
            self.assertEqual(subprocess.run([str(check.resolve())], input=b'[]', cwd=root,
                                           capture_output=True).returncode, 2)


if __name__ == '__main__':
    unittest.main()
