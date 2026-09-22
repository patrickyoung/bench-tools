#!/usr/bin/env python3
"""Independent quality and total-dollar gate; no inference."""
import json
from decimal import Decimal
import math
import sys


def savings_percent(settings):
    value = settings.get('min_savings_percent', 10)
    if type(value) not in (int, float) or not math.isfinite(value) or not 0 <= value < 100:
        raise ValueError('min_savings_percent must be a finite number from 0 to less than 100')
    return value


def judge(data):
    savings = savings_percent(data.get('settings', {}))
    old, new = data['baseline'], data['candidate']
    if not old or len(old) != len(new):
        raise ValueError('unmatched observations')
    reasons, lower, old_cost, new_cost = [], 0, 0, 0
    identities = set()
    for a, b in zip(old, new):
        key = (a['case'], a['repeat'])
        if key in identities or key != (b['case'], b['repeat']) or a['input_sha256'] != b['input_sha256']:
            raise ValueError('mismatched cases or input bytes')
        identities.add(key)
        if a['source_sha256'] == b['source_sha256']:
            raise ValueError('unchanged source')
        x, y = a['observation'], b['observation']
        costs_valid = True
        for observation in (x, y):
            if type(observation['cost']) not in (float, int) or not math.isfinite(observation['cost']) or observation['cost'] <= 0:
                reasons.append('incomplete positive cost evidence')
                costs_valid = False
        xs, ys = x['score'], y['score']
        if not ys['accepted'] or ys['correct'] != ys['total']:
            reasons.append('candidate not perfect')
        if ys['model_calls'] > xs['model_calls'] or ys['model_calls'] <= 0:
            reasons.append('extra or missing model calls')
        if len(xs['rows']) != len(ys['rows']) or any(p['id'] != q['id'] for p, q in zip(xs['rows'], ys['rows'])):
            raise ValueError('mismatched labels')
        if any(p['correct'] and not q['correct'] for p, q in zip(xs['rows'], ys['rows'])):
            reasons.append('label regression')
        if costs_valid:
            old_cost += Decimal(str(x['cost']))
            new_cost += Decimal(str(y['cost']))
            lower += y['cost'] < x['cost']
    if 'incomplete positive cost evidence' not in reasons and (
            new_cost * 100 > old_cost * (100 - Decimal(str(savings))) or not old_cost):
        reasons.append(f'less than {savings:g}% measured cost reduction')
    if 'incomplete positive cost evidence' not in reasons and lower <= len(old) // 2:
        reasons.append('cost not lower in a majority of pairs')
    return {'version': 1, 'decision': 'discard' if reasons else 'keep',
            'reason': '; '.join(dict.fromkeys(reasons)) if reasons else 'perfect outputs, no regressions or extra calls, and lower measured cost',
            'cost': 0}


if __name__ == '__main__':
    try:
        result = judge(json.load(sys.stdin))
        print(json.dumps(result))
        sys.exit(result['decision'] == 'discard')
    except (ValueError, KeyError, TypeError) as error:
        print('judge: ' + str(error), file=sys.stderr)
        sys.exit(2)
