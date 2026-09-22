#!/usr/bin/env python3
"""Frozen router search gate: usable quality, then instruction simplicity."""
import json
import sys

from score import strict


def judge(data):
    if set(data) != {'baseline', 'candidate'}:
        raise ValueError('expected two score arrays')
    old, new = data['baseline'], data['candidate']
    if not isinstance(old, list) or not isinstance(new, list) or len(old) != len(new) or len(old) < 2:
        raise ValueError('at least two matched pairs required')
    trials = [s['trial'] for s in old + new]
    if len(set(trials)) != len(trials):
        raise ValueError('every observation requires a distinct trial')
    for arm in (old, new):
        for key in ('expert_sha256', 'settings', 'scorer_sha256', 'instruction_bytes'):
            if any(s[key] != arm[0][key] for s in arm):
                raise ValueError('treatment or evaluation drift')
    repeats = {}
    regressions, pairs = [], []
    valid, simpler = True, True
    for a, b in zip(old, new):
        for key in ('input_sha256', 'labels_sha256', 'scorer_sha256', 'settings', 'repeat', 'case'):
            if a[key] != b[key]:
                raise ValueError(f'unmatched {key}')
        if a['reported_models'] and b['reported_models'] and a['reported_models'] != b['reported_models']:
            raise ValueError('provider model changed')
        for s in (a, b):
            if (s['version'] != 1 or s['status'] not in ('ok', 'failed')
                    or type(s['repeat']) is not int or s['repeat'] < 0
                    or type(s['total']) is not int or s['total'] <= 0
                    or s['total'] != len(s['rows'])
                    or type(s['correct']) is not int
                    or s['correct'] != sum(r['correct'] for r in s['rows'])
                    or any(type(r['correct']) is not bool or r['correct'] != (r['actual'] == r['expected']) for r in s['rows'])
                    or type(s['instruction_bytes']) is not int or s['instruction_bytes'] <= 0
                    or type(s['model_calls']) is not int or s['model_calls'] < 0):
                raise ValueError('invalid score')
            if not s['reported_models'] or s['model_calls'] == 0:
                raise ValueError('comparison needs model evidence in both arms')
        if [(r['id'], r['expected']) for r in a['rows']] != [(r['id'], r['expected']) for r in b['rows']]:
            raise ValueError('unmatched output labels')
        repeat = repeats.setdefault(a['repeat'], {'cases': set(), 'delta': 0})
        if a['case'] in repeat['cases']:
            raise ValueError('duplicate case within repeat')
        repeat['cases'].add(a['case'])
        ac = a['correct'] if a['status'] == 'ok' else 0
        bc = b['correct'] if b['status'] == 'ok' else 0
        repeat['delta'] += bc - ac
        lost = [r['id'] for r, t in zip(a['rows'], b['rows'])
                if a['status'] == 'ok' and r['correct'] and not (b['status'] == 'ok' and t['correct'])]
        regressions.extend({'repeat': a['repeat'], 'case': a['case'], 'id': ident} for ident in lost)
        valid = valid and b['status'] == 'ok'
        tokens = all(type(s['input_tokens']) is int and s['input_tokens'] > 0 for s in (a, b))
        simpler = (simpler and a['status'] == b['status'] == 'ok'
                   and ac == bc == a['total'] and tokens
                   and b['input_tokens'] < a['input_tokens']
                   and b['instruction_bytes'] < a['instruction_bytes']
                   and 0 < b['model_calls'] <= a['model_calls'])
        pairs.append({'case': a['case'], 'repeat': a['repeat'], 'baseline_correct': ac,
                      'candidate_correct': bc, 'total': a['total'], 'regressions': lost})
    if len(repeats) < 2 or any(r['cases'] != next(iter(repeats.values()))['cases'] for r in repeats.values()):
        raise ValueError('each case needs at least two complete repeats')
    distinct = old[0]['expert_sha256'] != new[0]['expert_sha256']
    quality = all(r['delta'] > 0 for r in repeats.values())
    supported = distinct and valid and not regressions and (quality or simpler)
    return {'version': 1, 'decision': 'supported' if supported else 'keep_baseline',
            'basis': 'quality' if supported and quality else 'simplicity' if supported else None,
            'baseline_sha256': old[0]['expert_sha256'], 'candidate_sha256': new[0]['expert_sha256'],
            'pairs': pairs, 'regressions': regressions,
            'scope': 'repeated synthetic router cases; no statistical or broad transfer claim'}


if __name__ == '__main__':
    try:
        result = judge(strict(sys.stdin.buffer.read()))
        print(json.dumps(result, sort_keys=True))
        sys.exit(0 if result['decision'] == 'supported' else 1)
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f'judge: invalid evidence ({type(error).__name__})', file=sys.stderr)
        sys.exit(2)
