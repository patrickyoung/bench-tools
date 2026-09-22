#!/usr/bin/env python3
"""Run bounded instruction research through public Bench commands and files."""
import argparse
import difflib
import json
import math
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time

from score import strict
from trial import digest, fingerprint, terminate_group

HERE = Path(__file__).resolve().parent
HELPERS = ('search.py', 'trial.py', 'score.py', 'judge.py', 'propose.py')


class BudgetExhausted(Exception):
    pass


def write(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True, allow_nan=False) + '\n')


def number(value, low, high):
    return type(value) is int and low <= value <= high


def load_spec(path):
    spec = strict(path.read_bytes())
    required = {'version', 'expert', 'cases', 'model', 'effort', 'iterations', 'repeats',
                'max_trials', 'max_seconds', 'proposer_argv'}
    if not isinstance(spec, dict) or not required <= set(spec) or set(spec) - required - {'agent', 'record', 'ask'}:
        raise ValueError('spec has missing or unknown fields')
    if spec['version'] != 1 or not all(isinstance(spec[k], str) and spec[k] for k in ('model', 'effort')):
        raise ValueError('invalid spec version/model/effort')
    for key, low, high in [('iterations', 1, 20), ('repeats', 2, 10), ('max_trials', 1, 1000), ('max_seconds', 1, 86400)]:
        if not number(spec[key], low, high):
            raise ValueError(f'invalid {key}')
    argv = spec['proposer_argv']
    if not isinstance(argv, list) or not argv or any(not isinstance(v, str) or not v or '\0' in v for v in argv):
        raise ValueError('proposer_argv must be literal nonempty argv')
    for key in ('expert', 'cases'):
        spec[key] = str((path.parent / spec[key]).resolve(strict=True))
    fingerprint(Path(spec['expert']))
    cases = strict(Path(spec['cases']).read_bytes())
    if not isinstance(cases, dict) or set(cases) != {'development', 'holdout'}:
        raise ValueError('cases requires development and holdout arrays')
    seen_ids, families, texts = set(), {}, {}
    for split, entries in cases.items():
        if not isinstance(entries, list) or not 1 <= len(entries) <= 20:
            raise ValueError('each split needs 1-20 cases')
        families[split], texts[split] = set(), set()
        for row in entries:
            if not isinstance(row, dict) or set(row) != {'id', 'family', 'input', 'labels'}:
                raise ValueError('case requires id, family, input, labels')
            if any(not isinstance(row[k], str) or not row[k] for k in row):
                raise ValueError('case fields must be nonempty strings')
            if row['id'] in seen_ids:
                raise ValueError('case IDs must be unique')
            seen_ids.add(row['id'])
            families[split].add(row['family'])
            for key in ('input', 'labels'):
                row[key] = str((Path(spec['cases']).parent / row[key]).resolve(strict=True))
            request = strict(Path(row['input']).read_bytes())
            labels = strict(Path(row['labels']).read_bytes())
            if not isinstance(request, dict) or not isinstance(request.get('policy'), str) or not request['policy']:
                raise ValueError('case needs a policy')
            notes = request.get('notes')
            if not isinstance(notes, list) or not notes or any(not isinstance(n, dict) or set(n) != {'id', 'text'} or any(not isinstance(n[k], str) or not n[k] for k in n) for n in notes):
                raise ValueError('invalid notes')
            ids = [n['id'] for n in notes]
            if len(set(ids)) != len(ids) or not isinstance(labels, list) or len(labels) != len(ids):
                raise ValueError('invalid label coverage')
            if any(not isinstance(r, dict) or set(r) != {'id', 'queue'} or r['id'] != ident or r['queue'] not in ('security', 'billing', 'technical', 'general') for r, ident in zip(labels, ids)):
                raise ValueError('labels must match ordered notes')
            texts[split].update(' '.join(n['text'].casefold().split()) for n in notes)
    if families['development'] & families['holdout'] or texts['development'] & texts['holdout']:
        raise ValueError('development/holdout overlap; select distinct families and notes')
    for key in ('agent', 'record', 'ask'):
        executable = shutil.which(spec.get(key, key))
        if executable is None:
            raise ValueError(f'missing {key} executable')
        spec[key] = str(Path(executable).resolve())
    executable = shutil.which(argv[0])
    if executable is None:
        raise ValueError('missing proposer executable')
    argv[0] = str(Path(executable).resolve())
    for i, arg in enumerate(argv[1:], 1):
        possible = path.parent / arg
        if not arg.startswith('-') and possible.is_file():
            argv[i] = str(possible.resolve())
    return spec, cases


class Search:
    def __init__(self, spec, cases, out):
        self.spec, self.cases, self.out = spec, cases, out
        self.started = time.monotonic()
        self.trials, self.proposals = 0, 0
        self.costs, self.history = [], []
        self.pins, self.definitions = {}, {}
        self.best = None

    def pin(self, path):
        self.pins[str(path)] = {'sha256': digest(path.read_bytes()), 'mode': path.stat().st_mode & 0o777}

    def guard(self):
        for name, value in self.pins.items():
            path = Path(name)
            if path.is_symlink() or digest(path.read_bytes()) != value['sha256'] or path.stat().st_mode & 0o777 != value['mode']:
                raise ValueError('frozen evaluation or executable changed')
        for name, expected in self.definitions.items():
            if fingerprint(Path(name)) != expected:
                raise ValueError('frozen definition changed')

    def command(self, argv, directory, name, raw=b'', limit=240):
        self.guard()
        remaining = self.spec['max_seconds'] - (time.monotonic() - self.started)
        if remaining <= 0:
            raise BudgetExhausted('elapsed budget exhausted')
        write(directory / (name + '.command.json'), {'argv': argv, 'stdin_sha256': digest(raw)})
        env = dict(os.environ, BENCH_WEIGH='0')
        with (directory / (name + '.stdout')).open('wb') as stdout, (directory / (name + '.stderr')).open('wb') as stderr:
            process = subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=stdout, stderr=stderr,
                                       cwd=directory, env=env, start_new_session=True)
            try:
                process.communicate(raw, timeout=min(limit, remaining))
            except (subprocess.TimeoutExpired, KeyboardInterrupt):
                # The trial helper allows seven seconds for nested cancellation;
                # this outer boundary leaves room for its receipt/manifest too.
                terminate_group(process, grace=10)
                write(directory / (name + '.exit.json'), {'exit': process.returncode, 'interrupted': True})
                raise BudgetExhausted('command timed out or was interrupted; no retry')
        write(directory / (name + '.exit.json'), {'exit': process.returncode, 'interrupted': False})
        self.guard()
        return process.returncode, (directory / (name + '.stdout')).read_bytes()

    def prepare(self):
        self.out.mkdir(parents=True, exist_ok=False)
        self.protocol = self.out / 'protocol'
        self.protocol.mkdir()
        for name in HELPERS:
            shutil.copy2(HERE / name, self.protocol / name)
            self.pin(self.protocol / name)
        # Supplied adapters in this recipe use the frozen copies too.
        self.spec['proposer_argv'] = [str(self.protocol / Path(a).name)
                                     if a in [str(HERE / n) for n in HELPERS] else a
                                     for a in self.spec['proposer_argv']]
        for name in [self.spec[k] for k in ('agent', 'record', 'ask')] + self.spec['proposer_argv']:
            path = Path(name)
            if path.is_file():
                self.pin(path)
        self.original = self.out / 'baseline'
        shutil.copytree(self.spec['expert'], self.original)
        self.definitions[str(self.original)] = fingerprint(self.original)
        self.definitions[self.spec['expert']] = fingerprint(Path(self.spec['expert']))
        if self.definitions[self.spec['expert']] != self.definitions[str(self.original)]:
            raise ValueError('source changed during snapshot')
        self.best = self.original
        case_root = self.out / 'cases'
        case_root.mkdir()
        for split, entries in self.cases.items():
            for index, row in enumerate(entries):
                for key in ('input', 'labels'):
                    dest = case_root / f'{split}-{index}-{key}.json'
                    shutil.copyfile(row[key], dest)
                    row[key] = str(dest)
                    self.pin(dest)
        write(self.out / 'spec.json', self.spec)
        write(self.out / 'cases.json', self.cases)
        self.pin(self.out / 'spec.json')
        self.pin(self.out / 'cases.json')
        write(self.out / 'frozen.json', {'files': self.pins, 'definitions': self.definitions})
        self.pin(self.out / 'frozen.json')

    def trial(self, expert, case, repeat, directory):
        if self.trials >= self.spec['max_trials']:
            raise BudgetExhausted('worker invocation budget exhausted')
        self.trials += 1
        index = len(self.costs)
        self.costs.append({'stage': 'worker', 'cost': None})
        directory.mkdir()
        argv = [sys.executable, str(self.protocol / 'trial.py'), '--expert', str(expert),
                '--input', case['input'], '--out', str(directory / 'trial'), '--model', self.spec['model'],
                '--effort', self.spec['effort'], '--agent', self.spec['agent'], '--record', self.spec['record']]
        self.command(argv, directory, 'run', limit=190)
        code, raw = self.command([sys.executable, str(self.protocol / 'score.py'), '--trial', str(directory / 'trial'),
                                  '--labels', case['labels'], '--record', self.spec['record'], '--ask', self.spec['ask']],
                                 directory, 'score', limit=40)
        if code != 0:
            raise ValueError('trial has broken or incomplete evidence')
        score = strict(raw)
        if not score.get('reported_models') or not score.get('model_calls'):
            raise ValueError('trial did not produce model evidence; fix execution before searching')
        score.update(repeat=repeat, case=case['id'])
        write(directory / 'score.json', score)
        self.costs[index] = {'stage': 'worker', 'cost': score['cost'], 'model_calls': score['model_calls'],
                             'input_tokens': score['input_tokens'], 'output_tokens': score['output_tokens']}
        return score

    def measure(self, old, new, split, directory):
        directory.mkdir()
        data = {'baseline': [], 'candidate': []}
        for repeat in range(self.spec['repeats']):
            for i, case in enumerate(self.cases[split]):
                order = [('baseline', old), ('candidate', new)]
                if repeat % 2:
                    order.reverse()
                for arm, expert in order:
                    data[arm].append(self.trial(expert, case, repeat, directory / f'r{repeat}-c{i}-{arm}'))
        write(directory / 'scores.json', data)
        code, raw = self.command([sys.executable, str(self.protocol / 'judge.py')], directory, 'judge',
                                 json.dumps(data).encode(), limit=30)
        if code not in (0, 1):
            raise ValueError('comparison rejected evidence')
        result = strict(raw)
        if (code == 0) != (result['decision'] == 'supported'):
            raise ValueError('comparison stream/status mismatch')
        write(directory / 'decision.json', result)
        return result, data

    def propose(self, iteration, development):
        directory = self.out / f'iteration-{iteration:02d}'
        directory.mkdir()
        request = {'version': 1, 'iteration': iteration, 'expert': str(self.best),
                   'instructions': (self.best / 'AGENTS.md').read_text(),
                   'development': [{'id': c['id'], 'request': strict(Path(c['input']).read_bytes()),
                                    'labels': strict(Path(c['labels']).read_bytes())} for c in self.cases['development']],
                   'scores': development, 'history': self.history,
                   'objective': 'Improve usable correct outputs in every repeat without case regressions; '
                                'or preserve perfect output with smaller instructions, fewer input tokens and no extra calls.'}
        # Strip trial locations from feedback; held-out files are never supplied.
        request['scores'] = [{k: v for k, v in s.items() if k != 'trial'} for s in development]
        write(directory / 'request.json', request)
        self.proposals += 1
        cost_index = len(self.costs)
        self.costs.append({'stage': 'proposer', 'cost': None})
        argv = [self.spec['record'], 'run', '-f', str(directory / 'process.jsonl'), '-grace', '5s', '--', *self.spec['proposer_argv']]
        code, raw = self.command(argv, directory, 'propose', json.dumps(request).encode(), limit=230)
        verified, _ = self.command([self.spec['record'], 'check', '-f', str(directory / 'process.jsonl')], directory, 'verify', limit=30)
        if verified != 0:
            raise ValueError('proposal recording failed')
        try:
            proposal = strict(raw)
        except ValueError:
            proposal = {}
        if not isinstance(proposal, dict):
            proposal = {}
        if isinstance(proposal, dict) and isinstance(proposal.get('usage'), dict):
            usage = proposal['usage']
            cost = usage.get('cost')
            if cost is not None and (type(cost) not in (int, float) or not math.isfinite(cost) or cost < 0):
                raise ValueError('invalid proposer cost')
            self.costs[cost_index] = dict(usage, stage='proposer', cost=cost)
        text, hypothesis = proposal.get('instructions'), proposal.get('hypothesis')
        if code != 0 or not isinstance(text, str) or not text.strip() or '\0' in text or not 1 <= len(text.encode()) <= 16000 or not isinstance(hypothesis, str) or not 1 <= len(hypothesis.encode()) <= 8000:
            self.history.append({'iteration': iteration, 'decision': 'proposal_failed', 'exit': code,
                                 'error': str(proposal.get('error', 'No valid proposal returned within the limits'))[:2000]})
            return None
        candidate = directory / 'candidate'
        shutil.copytree(self.best, candidate)
        (candidate / 'AGENTS.md').write_text(text.rstrip() + '\n')
        candidate_sha = fingerprint(candidate)
        if candidate_sha in self.definitions.values():
            self.history.append({'iteration': iteration, 'hypothesis': hypothesis, 'decision': 'duplicate'})
            return None
        self.definitions[str(candidate)] = candidate_sha
        write(directory / 'proposal.json', {'hypothesis': hypothesis, 'sha256': candidate_sha})
        return candidate, hypothesis

    def execute(self):
        diagnostic = self.out / 'development-baseline'
        diagnostic.mkdir()
        development = [self.trial(self.original, case, repeat, diagnostic / f'r{repeat}-c{i}')
                       for repeat in range(self.spec['repeats']) for i, case in enumerate(self.cases['development'])]
        holdout_trials = 2 * self.spec['repeats'] * len(self.cases['holdout'])
        for iteration in range(1, self.spec['iterations'] + 1):
            needed = 2 * self.spec['repeats'] * len(self.cases['development']) + holdout_trials
            if self.trials + needed > self.spec['max_trials']:
                break
            print(f'search: proposal {iteration}/{self.spec["iterations"]}; worker trials {self.trials}', file=sys.stderr, flush=True)
            proposal = self.propose(iteration, development)
            if proposal is None:
                continue
            candidate, hypothesis = proposal
            result, scores = self.measure(self.best, candidate, 'development', candidate.parent / 'evaluation')
            retained = result['decision'] == 'supported'
            self.history.append({'iteration': iteration, 'hypothesis': hypothesis,
                                 'candidate_sha256': fingerprint(candidate), 'decision': 'retain' if retained else 'discard',
                                 'comparison': result})
            if retained:
                self.best = candidate
            development = scores['candidate' if retained else 'baseline']
            write(self.out / 'history.json', self.history)
        if self.best == self.original:
            tested = any(h['decision'] in ('discard', 'duplicate') for h in self.history)
            return 'keep_baseline' if tested else 'inconclusive'
        # Search is over. No proposer invocation is possible after this point.
        print('search: final held-out comparison', file=sys.stderr, flush=True)
        result, _ = self.measure(self.original, self.best, 'holdout', self.out / 'holdout')
        if result['decision'] != 'supported':
            return 'keep_baseline'
        self.guard()
        proposal = self.out / 'proposal'
        proposal.mkdir()
        shutil.copytree(self.best, proposal / 'expert')
        if fingerprint(proposal / 'expert') != fingerprint(self.best):
            raise ValueError('export does not match evaluated candidate')
        diff = ''.join(difflib.unified_diff((self.original / 'AGENTS.md').read_text().splitlines(True),
                                          (self.best / 'AGENTS.md').read_text().splitlines(True),
                                          fromfile='a/AGENTS.md', tofile='b/AGENTS.md'))
        (proposal / 'change.diff').write_text(diff)
        write(proposal / 'manifest.json', {'version': 1, 'baseline_sha256': fingerprint(self.original),
              'candidate_sha256': fingerprint(self.best), 'patch_sha256': digest(diff.encode()),
              'comparison': str(self.out / 'holdout/decision.json'),
              'rollback': str(self.original), 'status': 'tested proposal; not deployed'})
        return 'supported'

    def finish(self, decision, error=None):
        known = [c['cost'] for c in self.costs if c.get('cost') is not None]
        try:
            source_modified = fingerprint(Path(self.spec['expert'])) != self.definitions[self.spec['expert']]
        except (OSError, ValueError):
            source_modified = None
        result = {'version': 1, 'decision': decision, 'error': error, 'worker_trials': self.trials,
                  'proposals': self.proposals, 'seconds': time.monotonic() - self.started,
                  'history': self.history, 'costs': self.costs, 'known_provider_cost': sum(known),
                  'total_provider_cost': sum(known) if len(known) == len(self.costs) else None,
                  'proposal': str(self.out / 'proposal') if decision == 'supported' else None,
                  'source_modified': source_modified, 'weigh': 'disabled in default recipe'}
        write(self.out / 'result.json', result)
        print(json.dumps(result, sort_keys=True, allow_nan=False))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--spec', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--live', action='store_true', help='execute the selected proposer and fresh model trials')
    args = parser.parse_args()
    spec, cases = load_spec(args.spec.resolve())
    out = args.out.resolve()
    source_root = next((p for p in (HERE, *HERE.parents) if (p / '.git').exists()), HERE)
    forbidden = [source_root, Path(spec['expert']), Path(spec['cases']), args.spec.resolve()]
    forbidden += [Path(c[k]) for entries in cases.values() for c in entries for k in ('input', 'labels')]
    if out.exists() or any(out == p or p in out.parents or out in p.parents for p in forbidden):
        raise ValueError('output must be new, outside source, and disjoint from selected inputs')
    repeats = spec['repeats']
    maximum = repeats * len(cases['development']) * (1 + 2 * spec['iterations']) + 2 * repeats * len(cases['holdout'])
    if not args.live:
        print(json.dumps({'version': 1, 'mode': 'plan', 'max_worker_trials': min(maximum, spec['max_trials']),
                          'max_proposals': spec['iterations'], 'max_seconds': spec['max_seconds'],
                          'worker_turns_per_trial': 3, 'proposer_timeout_seconds': 230,
                          'paid_calls': False, 'out': str(out)}))
        return 0
    search = Search(spec, cases, out)
    search.prepare()
    def interrupt(*_):
        raise KeyboardInterrupt
    for signum in (signal.SIGTERM, signal.SIGHUP):
        signal.signal(signum, interrupt)
    try:
        decision = search.execute()
        search.finish(decision)
        return 0 if decision == 'supported' else 1
    except BudgetExhausted as error:
        search.finish('inconclusive', str(error))
        return 1
    except KeyboardInterrupt:
        search.finish('inconclusive', 'interrupted; no retry')
        return 1
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        search.finish('invalid_evidence', str(error))
        return 2


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f'search: {type(error).__name__}: {error}', file=sys.stderr)
        sys.exit(2)
