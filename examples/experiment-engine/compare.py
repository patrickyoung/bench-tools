#!/usr/bin/env python3
"""Compare matched router scores on stdin. Never applies a change."""
import json
import sys

from score import strict


def compare(data):
    if set(data) != {'baseline', 'candidate'}:
        raise ValueError('expected baseline and candidate score arrays')
    baseline, candidate = data['baseline'], data['candidate']
    if not isinstance(baseline, list) or not isinstance(candidate, list) or len(baseline) != len(candidate) or len(baseline) < 2:
        raise ValueError('at least two matched pairs are required')
    reasons, pairs = [], []
    for side in (baseline, candidate):
        if len({s['expert_sha256'] for s in side}) != 1:
            raise ValueError('each treatment must use one frozen definition')
        if len({s['trial'] for s in side}) != len(side):
            raise ValueError('repeats require distinct trials')
        if any(s['settings'] != side[0]['settings'] for s in side):
            raise ValueError('settings changed between repeats')
    for index, (old, new) in enumerate(zip(baseline, candidate)):
        for key in ('input_sha256', 'labels_sha256', 'scorer_sha256', 'settings', 'reported_models'):
            if old[key] != new[key]:
                raise ValueError(f'unmatched {key}')
        for score in (old, new):
            if (score['version'] != 1 or score['total'] != len(score['rows'])
                    or not score['rows'] or score['correct'] != sum(r['correct'] for r in score['rows'])
                    or any(type(r['correct']) is not bool or r['correct'] != (r['actual'] == r['expected']) for r in score['rows'])):
                raise ValueError('invalid score counts')
        if [(r['id'], r['expected']) for r in old['rows']] != [(r['id'], r['expected']) for r in new['rows']]:
            raise ValueError('unmatched cases')
        regressions = [a['id'] for a, b in zip(old['rows'], new['rows']) if a['correct'] and not b['correct']]
        valid = old['status'] == new['status'] == 'ok'
        # The candidate must be perfect; a baseline error is not a reason to
        # reject an otherwise better candidate. Preserve the no-regression gate.
        perfect = new['correct'] == new['total'] and old['total'] == new['total']
        tokens = all(type(s['input_tokens']) is int and s['input_tokens'] > 0 for s in (old, new))
        cheaper = tokens and new['input_tokens'] < old['input_tokens']
        fewer_calls = 0 < new['model_calls'] <= old['model_calls']
        smaller = 0 < new['instruction_bytes'] < old['instruction_bytes']
        distinct = old['expert_sha256'] != new['expert_sha256']
        supported = valid and perfect and not regressions and cheaper and fewer_calls and smaller and distinct
        if not supported:
            reasons.append(f'pair {index + 1} fails the frozen efficiency gate')
        pairs.append({'baseline_correct': old['correct'], 'candidate_correct': new['correct'],
                      'total': old['total'], 'regressions': regressions,
                      'input_token_delta': new['input_tokens'] - old['input_tokens'] if tokens else None,
                      'seconds_delta': new['seconds'] - old['seconds'], 'supported': supported})
    return {'version': 1, 'decision': 'supported' if not reasons else 'keep_baseline',
            'scope': 'local synthetic router efficiency experiment', 'reasons': reasons, 'pairs': pairs,
            'baseline_sha256': baseline[0]['expert_sha256'], 'candidate_sha256': candidate[0]['expert_sha256'],
            'instruction_bytes_delta': candidate[0]['instruction_bytes'] - baseline[0]['instruction_bytes'],
            'cost': 'unknown if any component omits measured cost; never inferred from tokens'}


if __name__ == '__main__':
    try:
        result = compare(strict(sys.stdin.buffer.read()))
        print(json.dumps(result, sort_keys=True))
        sys.exit(0 if result['decision'] == 'supported' else 1)
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f'compare: invalid evidence ({type(error).__name__})', file=sys.stderr)
        sys.exit(2)
