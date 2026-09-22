#!/usr/bin/env python3
"""Actual Improve/Record/Ask boundary with synthetic scores and no inference."""
import argparse
import json
import os
from pathlib import Path
import shutil
import runpy
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bin-dir', required=True, type=Path)
    parser.add_argument('--bench-adapters', action='store_true', help='also test real Hire/Agent adapters against a local model fixture')
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    required = ('improve', 'record', 'ask', 'hire', 'agent', 'ply', 'brief', 'cage') if args.bench_adapters else ('improve', 'record', 'ask')
    for name in required:
        if not (bins / name).is_file():
            raise ValueError('missing executable: ' + name)
    with tempfile.TemporaryDirectory(prefix='bench-improve-') as temporary:
        root = Path(temporary).resolve()
        home = root / 'home'
        home.mkdir()
        env = {'PATH': str(bins) + os.pathsep + '/usr/bin:/bin', 'HOME': str(home),
               'TMPDIR': str(root), 'PYTHONDONTWRITEBYTECODE': '1', 'LANG': 'C', 'BENCH_WEIGH': '0'}
        example = root / 'example'
        shutil.copytree(ROOT / 'tools/improve/examples/router', example,
                        ignore=shutil.ignore_patterns('__pycache__', '*.pyc'))
        def call(argv, raw=b'', code=0):
            p = subprocess.run(list(map(str, argv)), input=raw, cwd=root, env=env,
                               capture_output=True, timeout=90)
            if p.returncode != code:
                raise ValueError(f'{argv[0]} returned {p.returncode}: {p.stderr.decode()} {p.stdout.decode()}')
            return p.stdout
        spec = json.loads(call([sys.executable, example / 'spec.py', '--offline']))
        spec['repeats'] = 2
        original = (example / 'source/AGENTS.md').read_bytes()
        plan = json.loads(call([bins / 'improve', '-n'], json.dumps(spec).encode()))
        assert plan['executes_commands'] is False and plan['max_worker_trials'] == 10
        for reject in (False, True):
            spec['settings']['reject_holdout'] = reject
            out = root / ('reject' if reject else 'accept')
            result = json.loads(call([bins / 'improve', '-o', out], json.dumps(spec).encode(), code=int(reject)))
            assert result['decision'] == ('keep_baseline' if reject else 'supported')
            assert result['trials'] == 10 and len(result['calls']) == 13
            assert (out / 'proposal/source').exists() != reject
            assert (example / 'source/AGENTS.md').read_bytes() == original
            verified = json.loads(call([bins / 'improve', 'verify', '-record', bins / 'record', out]))
            assert verified['verified'] and verified['inference_calls'] == 0
            (out / 'candidate/AGENTS.md').write_text('tampered')
            call([bins / 'improve', 'verify', '-record', bins / 'record', out], code=2)
        # Real Record must stop the selected adapter at the live stream cap.
        burst = root / 'burst.py'
        burst.write_text("import os, time\nfrom pathlib import Path\nfor _ in range(64): os.write(1, b'x' * 65536)\ntime.sleep(5)\nPath('continued').touch()\n")
        bounded = json.loads(json.dumps(spec))
        bounded['commands']['trial']['argv'] = [sys.executable, str(burst)]
        bounded['dependencies'].append(str(burst))
        out = root / 'overflow'
        result = json.loads(call([bins / 'improve', '-o', out], json.dumps(bounded).encode(), code=2))
        assert result['decision'] == 'invalid_evidence' and len(result['calls']) == 1
        assert not result['calls'][0]['complete'] and result['total_cost'] is None
        assert (out / 'diagnostic-0-0/stdout').stat().st_size == 2 * 1024 * 1024
        assert not (out / 'diagnostic-0-0/continued').exists()
        assert not (out / 'proposal').exists()
        call([bins / 'improve', 'verify', '-record', bins / 'record', out], code=2)
        print('Improve public-process contract passed: acceptance, held-out rejection, exact export, replay, tamper refusal, runtime stream cancellation; zero inference calls.')
        acceptance = root / 'acceptance-example'
        shutil.copytree(ROOT / 'tools/improve/examples/acceptance', acceptance,
                        ignore=shutil.ignore_patterns('__pycache__', '*.pyc'))
        policy = (acceptance / 'fixture-policy.json').read_bytes()
        checked = json.loads(call([sys.executable, acceptance / 'judge.py', '--check-policy'], policy))
        assert checked['valid'] and not checked['executes_commands']
        call([sys.executable, acceptance / 'judge.py', '--check-policy'],
             (acceptance / 'policy.template.json').read_bytes(), code=2)
        original_acceptance = (acceptance / 'source/AGENTS.md').read_bytes()
        for scenario, decision, exit_code, trials in (
                ('efficiency', 'supported', 0, 10), ('quality', 'supported', 0, 10),
                ('over-budget', 'keep_baseline', 1, 6),
                ('critical-holdout', 'keep_baseline', 1, 10),
                ('unknown-cost', 'invalid_evidence', 2, 6)):
            raw = call([sys.executable, acceptance / 'spec.py', '--scenario', scenario])
            draft = root / (scenario + '-draft.json')
            draft.write_bytes(raw)
            raw = call([sys.executable, acceptance / 'spec.py', '--experiment', draft,
                        '--policy', acceptance / 'fixture-policy.json'])
            out = root / ('policy-' + scenario)
            result = json.loads(call([bins / 'improve', '-o', out], raw, code=exit_code))
            assert result['decision'] == decision and result['trials'] == trials
            assert (out / 'proposal/source').exists() == (decision == 'supported')
            assert (acceptance / 'source/AGENTS.md').read_bytes() == original_acceptance
            split = 'development' if trials == 6 else 'holdout'
            assessment = json.loads((out / (split + '-judge') / 'assessment.json').read_text())
            assert assessment['outcome'] == ('accepted' if decision == 'supported' else
                                              'insufficient_evidence' if exit_code == 2 else 'rejected')
            if decision == 'supported':
                assert scenario in assessment['paths_met']
                assert (out / 'proposal/source/AGENTS.md').read_bytes() == (out / 'candidate/AGENTS.md').read_bytes()
                if scenario == 'quality':
                    assert assessment['arms']['candidate']['total_cost_usd'] > assessment['arms']['baseline']['total_cost_usd']
            if exit_code != 2:
                verified = json.loads(call([bins / 'improve', 'verify', '-record', bins / 'record', out]))
                assert verified['verified'] and verified['inference_calls'] == 0
        print('Use-case policy contract passed: preflight, cheaper equal quality, more expensive higher quality, budget rejection, critical holdout rejection, unknown cost, exact export; zero inference calls.')
        if args.bench_adapters:
            support = runpy.run_path(str(ROOT / 'scripts/check-integration.py'))
            tools = {name: str((bins / name).resolve()) for name in ('record', 'ask', 'hire', 'agent', 'ply', 'brief', 'cage')}
            settings = {'tools': tools, 'runner_model': 'openai/fixture', 'proposer_model': 'openai/fixture', 'effort': 'off', 'min_savings_percent': 7}
            author = root / 'author'
            author.mkdir()
            for name in ('work', 'evidence'):
                (author / name).mkdir()
            request = {'version': 1, 'source': str(example / 'source'), 'files': {'AGENTS.md': original.decode()},
                'observations': [], 'development': spec['development'], 'settings': settings,
                'work': str(author / 'work'), 'evidence': str(author / 'evidence')}
            replies = iter(["```ply\nprintf 'Route according to the policy. Return raw JSON only.\\n' > expert/AGENTS.md\nprintf 'Explicit output format.\\n' > HYPOTHESIS.md\n```", 'Revision complete.'])
            with support['model_fixture'](env, lambda _: next(replies)) as (fixture_env, calls):
                p = subprocess.run([sys.executable, example / 'bench.py', 'propose'], input=json.dumps(request).encode(),
                                   cwd=author, env=fixture_env, capture_output=True, timeout=45)
            assert p.returncode == 0, (p.stdout, p.stderr, (author / "evidence/runner.stderr").read_text())
            assert 'at least 7%' in (author / 'work/JOB.md').read_text()
            proposal = json.loads(p.stdout)
            assert proposal['changes'][0]['path'] == 'AGENTS.md' and 'raw JSON only' in proposal['changes'][0]['content']
            assert len(calls) == 2 and (example / 'source/AGENTS.md').read_bytes() == original
            trial = root / 'trial'
            trial.mkdir()
            for name in ('work', 'evidence'):
                (trial / name).mkdir()
            request = {'version': 1, 'source': str(example / 'source'), 'case': spec['development'][0],
                'settings': settings, 'work': str(trial / 'work'), 'evidence': str(trial / 'evidence')}
            labels = json.loads((example / 'development.json').read_text())['labels']
            with support['model_fixture'](env, lambda _: json.dumps(labels)) as (fixture_env, calls):
                p = subprocess.run([sys.executable, example / 'bench.py', 'trial'], input=json.dumps(request).encode(),
                                   cwd=trial, env=fixture_env, capture_output=True, timeout=45)
            assert p.returncode == 0, (p.stdout, p.stderr)
            scored = json.loads(p.stdout)
            assert scored['score']['accepted'] and scored['score']['correct'] == 2 and scored['score']['model_calls'] == 1
            assert scored['cost'] is None and len(calls) == 1
            print('Hire/Agent adapters passed: exact instruction authoring, independent labels, stop/usage evidence; three loopback model requests, zero paid calls.')



if __name__ == '__main__':
    main()
