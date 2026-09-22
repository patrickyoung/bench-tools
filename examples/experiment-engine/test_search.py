"""Offline search gates and real executable integration (explicit bin directory)."""
import copy
import contextlib
import io
import json
import os
from pathlib import Path
import runpy
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest
from unittest.mock import patch

from judge import judge
from search import load_spec, Search, BudgetExhausted
from test_experiment import scores
from trial import fingerprint

HERE = Path(__file__).resolve().parent


def pairs():
    data = scores()
    for arm in data.values():
        for i, score in enumerate(arm):
            score.update(repeat=i, case='one')
    return data


class GateTests(unittest.TestCase):
    def test_simplification(self):
        self.assertEqual(judge(pairs())['basis'], 'simplicity')

    def test_quality_can_keep_longer_instructions(self):
        data = pairs()
        for score in data['baseline']:
            score['correct'] = 1
            score['rows'][0].update(actual='general', correct=False)
        for score in data['candidate']:
            score.update(instruction_bytes=300, input_tokens=150)
        self.assertEqual(judge(data)['basis'], 'quality')

    def test_lucky_repeat_is_insufficient(self):
        data = pairs()
        data['baseline'][0]['correct'] = 1
        data['baseline'][0]['rows'][0].update(actual='general', correct=False)
        self.assertEqual(judge(data)['decision'], 'keep_baseline')

    def test_failed_candidate_and_case_regression_reject(self):
        for failure in ('status', 'row'):
            data = pairs()
            if failure == 'status':
                data['candidate'][0]['status'] = 'failed'
            else:
                data['candidate'][0]['correct'] = 1
                data['candidate'][0]['rows'][0].update(actual='general', correct=False)
            self.assertEqual(judge(data)['decision'], 'keep_baseline')

    def test_failed_work_counts_as_zero_usable(self):
        data = pairs()
        for score in data['baseline']:
            score['status'] = 'failed'
        self.assertEqual(judge(data)['basis'], 'quality')

    def test_reused_evidence_and_incomplete_repeats_rejected(self):
        for mutate in (lambda d: d['candidate'][0].update(trial=d['baseline'][0]['trial']),
                       lambda d: d['candidate'][0].update(repeat=1),
                       lambda d: d['candidate'][0].update(scorer_sha256='changed'),
                       lambda d: d['candidate'][0].update(reported_models=['other'])):
            data = pairs()
            mutate(data)
            with self.assertRaises(ValueError):
                judge(data)


def write_cases(root):
    cases = {}
    for split, ident, family, text in [('development', 'dev', 'payment', 'I was charged twice.'),
                                       ('holdout', 'held', 'refund', 'Please reimburse the duplicate debit.')]:
        request = {'policy': 'Payment and refund requests go to billing.', 'notes': [{'id': ident, 'text': text}]}
        (root / f'{split}.json').write_text(json.dumps(request))
        (root / f'{split}-labels.json').write_text(json.dumps([{'id': ident, 'queue': 'billing'}]))
        cases[split] = [{'id': ident, 'family': family, 'input': f'{split}.json', 'labels': f'{split}-labels.json'}]
    (root / 'cases.json').write_text(json.dumps(cases))
    return cases


def spec_for(root, bins):
    expert = root / 'source'
    shutil.copytree(HERE / 'naive', expert)
    write_cases(root)
    proposer = root / 'fixture-proposer.py'
    proposer.write_text('''import json,sys
r=json.load(sys.stdin)
assert 'held' not in json.dumps({k:v for k,v in r.items() if k!='expert'})
text = 'FIXTURE_GOOD: Follow the supplied policy and return the required JSON.' if r['iteration']==1 else 'FIXTURE_BAD: Always choose general.'
print(json.dumps({'instructions':text,'hypothesis':'Synthetic fixture proposal','usage':{'cost':0,'model_calls':0}}))
''')
    spec = {'version': 1, 'expert': str(expert), 'cases': str(root / 'cases.json'), 'model': 'openai/fixture',
            'effort': 'off', 'iterations': 2, 'repeats': 2, 'max_trials': 14, 'max_seconds': 120,
            'proposer_argv': [sys.executable, str(proposer)],
            **{k: str(bins / k) for k in ('agent', 'record', 'ask')}}
    (root / 'spec.json').write_text(json.dumps(spec))
    return spec


class PreparationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name).resolve()
        self.spec = spec_for(self.root, Path('/bin'))
        for key in ('agent', 'record', 'ask'):
            self.spec[key] = '/bin/echo'
        (self.root / 'spec.json').write_text(json.dumps(self.spec))

    def tearDown(self):
        self.temp.cleanup()

    def test_plan_does_not_invoke_or_create_output(self):
        output = self.root / 'out'
        result = subprocess.run([sys.executable, str(HERE / 'search.py'), '--spec', str(self.root / 'spec.json'), '--out', str(output)], capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)['mode'], 'plan')
        self.assertFalse(output.exists())

    def test_split_leakage_rejected(self):
        cases = json.loads((self.root / 'cases.json').read_text())
        cases['holdout'][0]['family'] = 'payment'
        (self.root / 'cases.json').write_text(json.dumps(cases))
        with self.assertRaises(ValueError):
            load_spec(self.root / 'spec.json')

    def test_frozen_scorer_and_source_drift_rejected(self):
        spec, cases = load_spec(self.root / 'spec.json')
        search = Search(spec, cases, self.root / 'out')
        search.prepare()
        for path in (search.protocol / 'judge.py', self.root / 'source/AGENTS.md'):
            previous = path.read_bytes()
            path.write_bytes(previous + b'\nchanged\n')
            with self.assertRaises(ValueError):
                search.guard()
            path.write_bytes(previous)
        search.guard()

    def test_trial_budget_stops_before_command(self):
        spec, cases = load_spec(self.root / 'spec.json')
        search = Search(spec, cases, self.root / 'out')
        search.trials = spec['max_trials']
        with self.assertRaises(BudgetExhausted):
            search.trial(Path(spec['expert']), cases['development'][0], 0, self.root / 'not-created')
        self.assertFalse((self.root / 'not-created').exists())

    def test_missing_model_evidence_stops_before_search(self):
        spec, cases = load_spec(self.root / 'spec.json')
        search = Search(spec, cases, self.root / 'out')
        search.prepare()
        replies = [(0, b'{}'), (0, b'{"reported_models":[],"model_calls":0}')]
        with patch.object(search, 'command', side_effect=replies):
            with self.assertRaisesRegex(ValueError, 'fix execution before searching'):
                search.execute()
        self.assertEqual(search.proposals, 0)

    def test_timeout_terminates_child_group(self):
        spec, cases = load_spec(self.root / 'spec.json')
        search = Search(spec, cases, self.root / 'out')
        search.prepare()
        with self.assertRaises(BudgetExhausted):
            search.command([sys.executable, '-c', 'import time;time.sleep(10)'], search.out, 'sleep', limit=.05)
        self.assertTrue(json.loads((search.out / 'sleep.exit.json').read_text())['interrupted'])


@unittest.skipUnless(os.environ.get('BENCH_TEST_BIN_DIR'), 'set BENCH_TEST_BIN_DIR for offline public executable integration')
class ExecutableTests(unittest.TestCase):
    def test_deadline_stops_nested_model_process(self):
        bins = Path(os.environ['BENCH_TEST_BIN_DIR']).resolve()
        support = runpy.run_path(str(HERE.parents[1] / 'scripts/check-integration.py'))
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            spec_for(root, bins)
            wrapper = root / 'ask-with-pid.py'
            pid_file = root / 'model.pid'
            wrapper.write_text('#!' + sys.executable + '\nimport os,sys\nfrom pathlib import Path\n'
                               + f"if '-m' in sys.argv: Path({str(pid_file)!r}).write_text(str(os.getpid()))\n"
                               + f'os.execv({str(bins / "ask")!r},[{str(bins / "ask")!r},*sys.argv[1:]])\n')
            wrapper.chmod(0o700)
            env = {k: os.environ[k] for k in ('PATH', 'TMPDIR') if k in os.environ}
            env.update(HOME=str(root), PATH=str(bins) + os.pathsep + env.get('PATH', os.defpath),
                       **{'AGENT_' + k.upper(): str(bins / k) for k in ('ask', 'brief', 'ply', 'cage', 'record')})
            env['AGENT_ASK'] = str(wrapper)
            release = threading.Event()
            def stalled(_):
                release.wait(15)
                return '[{"id":"dev","queue":"billing"}]'
            # The server can observe a broken pipe after the client is cancelled.
            with contextlib.redirect_stderr(io.StringIO()):
                with support['model_fixture'](env, stalled) as (fixture_env, calls):
                    try:
                        spec, cases = load_spec(root / 'spec.json')
                        search = Search(spec, cases, root / 'run')
                        search.prepare()
                        with patch.dict(os.environ, fixture_env, clear=True):
                            with self.assertRaises(BudgetExhausted):
                                search.command([sys.executable, str(search.protocol / 'trial.py'),
                                                '--expert', str(search.original), '--input', cases['development'][0]['input'],
                                                '--out', str(root / 'trial'), '--model', 'openai/fixture',
                                                '--agent', str(bins / 'agent'), '--record', str(bins / 'record')],
                                               search.out, 'deadline', limit=3)
                        self.assertEqual(len(calls), 1)
                        pid = int(pid_file.read_text())
                        with self.assertRaises(ProcessLookupError):
                            os.kill(pid, 0)
                        checked = subprocess.run([str(bins / 'record'), 'check', '-f', str(root / 'trial/process.jsonl')], capture_output=True)
                        self.assertEqual(checked.returncode, 0, checked.stderr)
                    finally:
                        release.set()

    def test_hire_adapter_authors_only_instructions(self):
        bins = Path(os.environ['BENCH_TEST_BIN_DIR']).resolve()
        support = runpy.run_path(str(HERE.parents[1] / 'scripts/check-integration.py'))
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            env = {k: os.environ[k] for k in ('PATH', 'TMPDIR') if k in os.environ}
            env.update(HOME=str(root), PATH=str(bins) + os.pathsep + env.get('PATH', os.defpath),
                       HIRE_AGENT=str(bins / 'agent'),
                       **{'AGENT_' + k.upper(): str(bins / k) for k in ('ask', 'brief', 'ply', 'cage', 'record')})
            replies = iter(["```ply\nprintf 'Follow the supplied policy. Return ordered JSON.\\n' > expert/AGENTS.md\nprintf 'Use policy instead of keywords.\\n' > HYPOTHESIS.md\n```", 'Revision completed.'])
            with support['model_fixture'](env, lambda _: next(replies)) as (fixture_env, calls):
                result = subprocess.run([sys.executable, str(HERE / 'propose.py'), '--model', 'openai/fixture',
                                         '--hire', str(bins / 'hire'), '--ask', str(bins / 'ask')],
                                        input=json.dumps({'expert': str(HERE / 'naive'), 'instructions': 'fixture',
                                                          'development': [], 'scores': [], 'history': []}).encode(),
                                        cwd=root, env=fixture_env, capture_output=True, timeout=30)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            proposal = json.loads(result.stdout)
            self.assertEqual(proposal['status'], 'proposed')
            self.assertEqual(proposal['instructions'], 'Follow the supplied policy. Return ordered JSON.\n')
            self.assertEqual(proposal['usage']['model_calls'], 2)
            self.assertEqual(len(calls), 2)

    def test_keep_discard_holdout_and_no_source_write(self):
        self.run_study(False)

    def test_holdout_failure_blocks_export_and_stops_search(self):
        self.run_study(True)

    def test_failed_proposals_are_inconclusive_not_a_quality_result(self):
        self.run_study(False, True)

    def run_study(self, holdout_failure, proposer_failure=False):
        bins = Path(os.environ['BENCH_TEST_BIN_DIR']).resolve()
        support = runpy.run_path(str(HERE.parents[1] / 'scripts/check-integration.py'))
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            spec_for(root, bins)
            if proposer_failure:
                (root / 'fixture-proposer.py').write_text('import json,sys\njson.load(sys.stdin)\nprint(json.dumps({"status":"failed","usage":{"cost":0,"model_calls":0}}))\nsys.exit(1)\n')
            before = fingerprint(root / 'source')
            env = {k: os.environ[k] for k in ('PATH', 'TMPDIR') if k in os.environ}
            env.update(HOME=str(root), PATH=str(bins) + os.pathsep + env.get('PATH', os.defpath),
                       **{'AGENT_' + k.upper(): str(bins / k) for k in ('ask', 'brief', 'ply', 'cage', 'record')})
            def respond(request):
                raw = json.dumps(request)
                queue = 'billing' if 'FIXTURE_GOOD' in raw else 'general'
                ident = 'held' if 'reimburse' in raw else 'dev'
                if holdout_failure and ident == 'held':
                    queue = 'general'
                return json.dumps([{'id': ident, 'queue': queue}])
            with support['model_fixture'](env, respond) as (fixture_env, calls):
                process = subprocess.run([sys.executable, str(HERE / 'search.py'), '--spec', str(root / 'spec.json'),
                                          '--out', str(root / 'run'), '--live'], env=fixture_env, capture_output=True, timeout=150)
            self.assertEqual(process.returncode, 1 if holdout_failure or proposer_failure else 0, process.stderr.decode() + process.stdout.decode())
            result = json.loads(process.stdout)
            self.assertEqual(result['decision'], 'inconclusive' if proposer_failure else 'keep_baseline' if holdout_failure else 'supported')
            self.assertEqual([h['decision'] for h in result['history']], ['proposal_failed'] * 2 if proposer_failure else ['retain', 'discard'])
            self.assertEqual(result['worker_trials'], 2 if proposer_failure else 14)
            self.assertEqual(len(calls), 2 if proposer_failure else 14)
            self.assertIsNone(result['total_provider_cost'])
            self.assertEqual(fingerprint(root / 'source'), before)
            if holdout_failure or proposer_failure:
                self.assertFalse((root / 'run/proposal').exists())
            else:
                self.assertIn('FIXTURE_GOOD', (root / 'run/proposal/expert/AGENTS.md').read_text())
            self.assertFalse((root / 'run/iteration-03').exists())
            # Receipt verification is offline after shutting down the provider.
            verified = subprocess.run([str(bins / 'record'), 'check', '-f', str(root / 'run/iteration-01/process.jsonl')], capture_output=True)
            self.assertEqual(verified.returncode, 0, verified.stderr)


if __name__ == '__main__':
    unittest.main()
