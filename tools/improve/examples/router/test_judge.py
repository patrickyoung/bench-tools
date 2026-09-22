import copy
import unittest
from judge import judge, savings_percent
from bench import score


class GateTests(unittest.TestCase):
    def data(self):
        def row(repeat, candidate):
            return {'case': 'one', 'repeat': repeat, 'source_sha256': 'new' if candidate else 'old',
                'input_sha256': 'fixed', 'observation': {'cost': .8 if candidate else 1.,
                'score': {'accepted': True, 'correct': 1, 'total': 1, 'model_calls': 1,
                          'rows': [{'id': 'a', 'correct': True}]}}}
        return {'baseline': [row(i, False) for i in range(3)], 'candidate': [row(i, True) for i in range(3)]}

    def test_accepts_quality_and_measured_saving(self):
        self.assertEqual(judge(self.data())['decision'], 'keep')

    def test_refuses_unknown_or_insufficient_cost(self):
        for value in (None, 0, -.1, .99):
            with self.subTest(value=value):
                data = self.data()
                for row in data['candidate']:
                    row['observation']['cost'] = value
                self.assertEqual(judge(data)['decision'], 'discard')

    def test_refuses_regression_and_more_calls(self):
        for change in ({'accepted': False}, {'correct': 0}, {'model_calls': 2}, {'rows': [{'id': 'a', 'correct': False}]}):
            data = self.data()
            data['candidate'][0]['observation']['score'].update(change)
            self.assertEqual(judge(data)['decision'], 'discard')

    def test_refuses_mismatched_or_duplicate_evidence(self):
        for field, value in (('case', 'other'), ('input_sha256', 'changed'), ('source_sha256', 'old')):
            data = self.data()
            data['candidate'][0][field] = value
            with self.assertRaises(ValueError):
                judge(data)
        data = self.data()
        data['baseline'][1] = copy.deepcopy(data['baseline'][0])
        data['candidate'][1] = copy.deepcopy(data['candidate'][0])
        with self.assertRaises(ValueError):
            judge(data)

    def test_requires_majority_of_pairs(self):
        data = self.data()
        for row, value in zip(data['candidate'], (.1, 1.1, 1.1)):
            row['observation']['cost'] = value
        self.assertEqual(judge(data)['decision'], 'discard')

    def test_early_failure_does_not_hide_later_costs_or_validation(self):
        reasons = []
        for index in (0, 2):
            data = self.data()
            data['candidate'][index]['observation']['score']['accepted'] = False
            result = judge(data)
            self.assertEqual(result['decision'], 'discard')
            self.assertNotIn('cost', result['reason'])
            reasons.append(result['reason'])
        self.assertEqual(*reasons)
        data = self.data()
        data['candidate'][0]['observation']['cost'] = None
        data['candidate'][2]['observation']['score']['rows'][0]['id'] = 'wrong'
        with self.assertRaisesRegex(ValueError, 'mismatched labels'):
            judge(data)

    def test_exact_savings_boundary_is_independent_of_repeat_count(self):
        for count in (2, 3, 4):
            for candidate_cost, decision in ((.93, 'keep'), (.93000001, 'discard')):
                data = self.data()
                for key in ('baseline', 'candidate'):
                    data[key] = [copy.deepcopy(data[key][0]) for _ in range(count)]
                    for i, row in enumerate(data[key]):
                        row['repeat'] = i
                        row['observation']['cost'] = candidate_cost if key == 'candidate' else 1
                data['settings'] = {'min_savings_percent': 7}
                self.assertEqual(judge(data)['decision'], decision)

    def test_one_validated_savings_setting(self):
        data = self.data()
        for row in data['candidate']:
            row['observation']['cost'] = .925
        self.assertEqual(judge(data)['decision'], 'discard')
        data['settings'] = {'min_savings_percent': 7}
        self.assertEqual(judge(data)['decision'], 'keep')
        for value in (None, True, '7', -1, 100, float('nan'), float('inf')):
            with self.subTest(value=value), self.assertRaises(ValueError):
                savings_percent({'min_savings_percent': value})


class ScoreTests(unittest.TestCase):
    labels = [{'id': 'a', 'queue': 'technical'}, {'id': 'b', 'queue': 'billing'}]

    def test_order_failure_preserves_semantic_observation(self):
        import json
        s = score(json.dumps(self.labels[::-1]), self.labels, 0)
        self.assertFalse(s['accepted'])
        self.assertTrue(s['json_valid'] and s['shape_valid'] and s['ids_valid'])
        self.assertFalse(s['order_valid'])
        self.assertEqual(s['correct'], 0)
        self.assertEqual(s['semantic_correct'], 2)

    def test_wrong_labels_are_not_accepted(self):
        s = score('[{"id":"a","queue":"general"},{"id":"b","queue":"security"}]', self.labels, 0)
        self.assertTrue(s['order_valid'])
        self.assertFalse(s['accepted'])
        self.assertEqual(s['semantic_correct'], 0)

    def test_unknown_semantics_for_unusable_output(self):
        for raw in ('not json', '{}', '[{"id":"a","queue":"technical"},{"id":"a","queue":"billing"}]'):
            s = score(raw, self.labels, 0)
            self.assertFalse(s['accepted'])
            self.assertIsNone(s['semantic_correct'])
        s = score('[{"id":"a","queue":"technical"},{"id":"b","queue":"billing"}]', self.labels, 125)
        self.assertFalse(s['accepted'])
        self.assertEqual(s['semantic_correct'], 2)


if __name__ == '__main__':
    unittest.main()
