import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from judge import Invalid, assess, envelope, validate_policy

ROOT = Path(__file__).resolve().parent


class AcceptanceTests(unittest.TestCase):
    def data(self, old=20, new=24, old_cost=.01, new_cost=.014):
        policy = json.loads((ROOT / 'fixture-policy.json').read_text())
        policy['coverage'] = {'holdout': {'cases': {f'case-{i}': 'research' for i in range(12)}, 'repeats': 2}}
        data = {'version': 1, 'split': 'holdout', 'settings': {'acceptance': policy}}
        for arm, successes, cost in (('baseline', old, old_cost), ('candidate', new, new_cost)):
            data[arm] = [{'case': f'case-{i // 2}', 'repeat': i % 2,
                          'source_sha256': ('a' if arm == 'baseline' else 'b') * 64,
                          'input_sha256': f'{i // 2:064x}',
                          'observation': {'version': 1, 'cost': cost,
                                          'score': {'qualified': i < successes,
                                                    'critical_failure': False, 'elapsed_seconds': 1}}}
                         for i in range(24)]
        return data

    def test_quality_gain_can_cost_more(self):
        result = assess(self.data())
        self.assertEqual(result['outcome'], 'accepted')
        self.assertEqual(result['paths_met'], ['quality'])
        self.assertAlmostEqual(result['cost_change_percent'], 40)
        self.assertAlmostEqual(result['cost_per_added_qualified_usd'], .024)
        self.assertEqual(envelope(result)['decision'], 'keep')

    def test_equal_quality_at_ceiling_accepts_meaningful_saving(self):
        result = assess(self.data(old=24, new=24, new_cost=.008))
        self.assertEqual(result['outcome'], 'accepted')
        self.assertEqual(result['paths_met'], ['efficiency'])

    def test_no_material_benefit_and_dominated_candidate_reject(self):
        for old, new, cost in ((24, 24, .01), (24, 24, .009), (24, 23, .02)):
            with self.subTest(old=old, new=new, cost=cost):
                self.assertEqual(assess(self.data(old=old, new=new, new_cost=cost))['outcome'], 'rejected')

    def test_explicit_use_case_changes_decision(self):
        data = self.data()
        result = assess(data)
        self.assertEqual(result['outcome'], 'accepted')
        data['settings']['acceptance']['quality']['max_cost_increase_percent'] = 25
        stricter = assess(data)
        self.assertEqual(stricter['outcome'], 'rejected')
        self.assertNotEqual(result['policy_sha256'], stricter['policy_sha256'])

    def test_marginal_value_budget_and_small_gain(self):
        data = self.data()
        data['settings']['acceptance']['quality']['max_cost_per_added_qualified_usd'] = .02
        self.assertEqual(assess(data)['outcome'], 'rejected')
        self.assertEqual(assess(self.data(old=23, new=24))['outcome'], 'rejected')

    def test_quality_floor_and_critical_failures_cannot_be_bought_off(self):
        result = assess(self.data(old=18, new=22, new_cost=.001))
        self.assertEqual(result['outcome'], 'rejected')
        self.assertIn('candidate is below the quality floor', result['reasons'])
        data = self.data(old=20, new=23, new_cost=.001)
        data['candidate'][-1]['observation']['score']['critical_failure'] = True
        result = assess(data)
        self.assertEqual(result['outcome'], 'rejected')
        self.assertEqual(result['candidate_critical_failures'], 1)

    def test_category_regression_not_hidden_by_global_gain(self):
        data = self.data(old=20, new=23, new_cost=.001)
        data['settings']['acceptance']['coverage']['holdout']['cases']['case-11'] = 'must-preserve'
        data['baseline'][-2]['observation']['score']['qualified'] = True
        data['baseline'][-1]['observation']['score']['qualified'] = True
        result = assess(data)
        self.assertEqual(result['outcome'], 'rejected')
        self.assertTrue(any('must-preserve' in reason for reason in result['reasons']))

    def test_lower_quality_needs_explicit_allowances_and_floor(self):
        data = self.data(old=24, new=23, new_cost=.005)
        self.assertEqual(assess(data)['outcome'], 'rejected')
        policy = data['settings']['acceptance']
        policy['efficiency']['max_quality_loss_pp'] = 5
        policy['max_category_regression_pp'] = 5
        self.assertEqual(assess(data)['outcome'], 'accepted')

    def test_absolute_operating_limits(self):
        for name, value in (('max_mean_cost_usd', .013), ('max_job_seconds', .9)):
            data = self.data()
            data['settings']['acceptance']['limits'][name] = value
            self.assertEqual(assess(data)['outcome'], 'rejected')

    def test_unreported_cost_is_not_zero_or_a_quality_rejection(self):
        data = self.data()
        data['candidate'][0]['observation']['cost'] = None
        result = assess(data)
        self.assertEqual(result['outcome'], 'insufficient_evidence')
        self.assertIsNone(result['arms']['candidate']['total_cost_usd'])
        with self.assertRaisesRegex(Invalid, 'insufficient evidence'):
            envelope(result)

    def test_unused_time_measurement_may_remain_unknown(self):
        data = self.data()
        data['candidate'][0]['observation']['score']['elapsed_seconds'] = None
        self.assertEqual(assess(data)['outcome'], 'insufficient_evidence')
        data['settings']['acceptance']['limits']['max_job_seconds'] = None
        result = assess(data)
        self.assertEqual(result['outcome'], 'accepted')
        self.assertIsNone(result['arms']['candidate']['max_job_seconds'])

    def test_zero_observed_cost_has_defined_behavior(self):
        self.assertEqual(assess(self.data(old_cost=0, new_cost=0))['outcome'], 'accepted')
        self.assertEqual(assess(self.data(old_cost=0, new_cost=.01))['outcome'], 'rejected')

    def test_exact_percentage_and_marginal_budget_boundaries(self):
        for cost, expected in ((.015, 'accepted'), (.015000001, 'rejected')):
            self.assertEqual(assess(self.data(new_cost=cost))['outcome'], expected)
        for cost, expected in ((.008, 'accepted'), (.008000001, 'rejected')):
            self.assertEqual(assess(self.data(old=24, new_cost=cost))['outcome'], expected)
        data = self.data()
        data['settings']['acceptance']['quality']['max_cost_per_added_qualified_usd'] = .024
        self.assertEqual(assess(data)['outcome'], 'accepted')

    def test_full_grid_and_immutable_source_input_bindings(self):
        changes = [lambda d: d['candidate'].pop(),
                   lambda d: d['candidate'].__setitem__(1, copy.deepcopy(d['candidate'][0])),
                   lambda d: d['candidate'][0].update(case='absent'),
                   lambda d: d['candidate'][0].update(repeat=True),
                   lambda d: d['candidate'][0].update(input_sha256='f' * 64),
                   lambda d: d['candidate'][0].update(source_sha256='c' * 64),
                   lambda d: [r.update(source_sha256='a' * 64) for r in d['candidate']]]
        for change in changes:
            data = self.data()
            change(data)
            with self.subTest(change=change), self.assertRaises(Invalid):
                assess(data)
        data = self.data()
        data['candidate'].reverse()
        self.assertEqual(assess(data)['outcome'], 'accepted')

    def test_validation_is_not_short_circuited_by_unknown_or_failed_job(self):
        data = self.data()
        data['candidate'][0]['observation']['cost'] = None
        data['candidate'][-1]['observation']['score']['qualified'] = 'yes'
        with self.assertRaises(Invalid):
            assess(data)

    def test_no_business_defaults_or_silent_policy_typos(self):
        template = json.loads((ROOT / 'policy.template.json').read_text())
        with self.assertRaises(Invalid):
            validate_policy(template)
        for change in (lambda p: p.pop('quality_floor'),
                       lambda p: p.update(min_savings_percent=20),
                       lambda p: p.update(quality_floor=True),
                       lambda p: p.update(quality_floor=float('nan')),
                       lambda p: p['quality'].update(min_gain_pp=0),
                       lambda p: p['coverage']['holdout'].update(repeats=1000000000),
                       lambda p: p.update(quality=None, efficiency=None),
                       lambda p: p['use_case'].update(policy_basis='')):
            policy = self.data()['settings']['acceptance']
            change(policy)
            with self.subTest(change=change), self.assertRaises(Invalid):
                validate_policy(policy)

    def test_cli_protocol_no_overwrite_and_missing_evidence(self):
        with tempfile.TemporaryDirectory() as folder:
            report = Path(folder) / 'assessment.json'
            argv = [sys.executable, str(ROOT / 'judge.py'), '--report', str(report)]
            def call(data, arguments=argv):
                return subprocess.run(arguments, input=json.dumps(data), capture_output=True, text=True, timeout=5)
            result = call(self.data())
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads(result.stdout)['decision'], 'keep')
            saved = report.read_bytes()
            self.assertEqual(call(self.data()).returncode, 2)
            self.assertEqual(report.read_bytes(), saved)
            base = argv[:2]
            self.assertEqual(call(self.data(new_cost=.02), base).returncode, 1)
            self.assertEqual(call(self.data(new_cost=None), base).returncode, 2)
            policy = self.data()['settings']['acceptance']
            checked = call(policy, base + ['--check-policy'])
            self.assertEqual(checked.returncode, 0, checked.stderr)
            self.assertFalse(json.loads(checked.stdout)['executes_commands'])
            for raw in ('{"version":1,"version":1}', '{"version":NaN}', '[]', 'null'):
                invalid = subprocess.run(base, input=raw, capture_output=True, text=True, timeout=5)
                self.assertEqual(invalid.returncode, 2)
                self.assertNotIn('Traceback', invalid.stderr)


if __name__ == '__main__':
    unittest.main()
