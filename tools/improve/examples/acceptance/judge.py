#!/usr/bin/env python3
"""Caller-owned quality/cost policy. No inference, execution or promotion."""
import argparse
from collections import defaultdict
from decimal import Decimal
import hashlib
import json
import math
from pathlib import Path
import re
import sys


class Invalid(ValueError):
    pass


def number(value, name, minimum=0, maximum=None):
    if type(value) not in (int, float) or (type(value) is float and not math.isfinite(value)):
        raise Invalid(name + ' must be a finite number')
    result = Decimal(str(value))
    if result < minimum or (maximum is not None and result > maximum):
        raise Invalid(name + ' is out of range')
    return result


def keys(value, expected, name):
    if not isinstance(value, dict) or set(value) != set(expected.split()):
        raise Invalid(name + ' requires exactly: ' + expected)


def integer(value, name, minimum=0):
    if type(value) is not int or value < minimum:
        raise Invalid(name + ' must be an integer >= ' + str(minimum))
    return value


def digest(value, name):
    if not isinstance(value, str) or not re.fullmatch('[0-9a-f]{64}', value):
        raise Invalid(name + ' must be a SHA-256 digest')
    return value


def validate_policy(policy):
    keys(policy, 'version use_case quality_floor max_category_regression_pp efficiency quality limits coverage', 'acceptance')
    if type(policy['version']) is not int or policy['version'] != 1:
        raise Invalid('unsupported acceptance version')
    keys(policy['use_case'], 'id purpose qualified_outcome critical_failure policy_basis', 'use_case')
    for key, value in policy['use_case'].items():
        if not isinstance(value, str) or not value.strip():
            raise Invalid('use_case.' + key + ' must explain the intended deployment and policy')
    number(policy['quality_floor'], 'quality_floor', maximum=1)
    number(policy['max_category_regression_pp'], 'max_category_regression_pp', maximum=100)
    if policy['efficiency'] is not None:
        keys(policy['efficiency'], 'min_savings_percent max_quality_loss_pp', 'efficiency')
        if number(policy['efficiency']['min_savings_percent'], 'min_savings_percent', maximum=100) == 0:
            raise Invalid('min_savings_percent must be positive')
        number(policy['efficiency']['max_quality_loss_pp'], 'max_quality_loss_pp', maximum=100)
    if policy['quality'] is not None:
        keys(policy['quality'], 'min_gain_pp max_cost_increase_percent max_cost_per_added_qualified_usd', 'quality')
        if number(policy['quality']['min_gain_pp'], 'min_gain_pp', maximum=100) == 0:
            raise Invalid('min_gain_pp must be positive')
        number(policy['quality']['max_cost_increase_percent'], 'max_cost_increase_percent')
        if policy['quality']['max_cost_per_added_qualified_usd'] is not None:
            number(policy['quality']['max_cost_per_added_qualified_usd'], 'max_cost_per_added_qualified_usd')
    if policy['efficiency'] is None and policy['quality'] is None:
        raise Invalid('select at least one benefit path')
    keys(policy['limits'], 'max_mean_cost_usd max_job_seconds', 'limits')
    for name, value in policy['limits'].items():
        if value is not None:
            number(value, name)
    if not isinstance(policy['coverage'], dict) or not policy['coverage'] or set(policy['coverage']) - {'development', 'holdout'}:
        raise Invalid('coverage must select development and/or holdout')
    for split, selected in policy['coverage'].items():
        keys(selected, 'cases repeats', 'coverage.' + split)
        integer(selected['repeats'], 'repeats', 2)
        if selected['repeats'] > 10:
            raise Invalid('coverage exceeds Improve limit of 10 repeats')
        if not isinstance(selected['cases'], dict) or not 1 <= len(selected['cases']) <= 20:
            raise Invalid('coverage must map 1-20 case IDs to categories')
        if any(not isinstance(k, str) or not k.strip() or not isinstance(v, str) or not v.strip()
               for k, v in selected['cases'].items()):
            raise Invalid('case IDs and categories must be nonempty strings')


def assess(data):
    if type(data.get('version')) is not int or data['version'] != 1:
        raise Invalid('unsupported comparison version')
    policy = data['settings']['acceptance']
    validate_policy(policy)
    split = data['split']
    if split not in policy['coverage']:
        raise Invalid('split has no frozen coverage')
    coverage = policy['coverage'][split]
    expected = {(case, repeat) for case in coverage['cases'] for repeat in range(coverage['repeats'])}
    counts, costs, elapsed, categories, sources, inputs = {}, {}, {}, {}, {}, {}
    unknown, critical = [], 0
    for arm in ('baseline', 'candidate'):
        rows = data[arm]
        if not isinstance(rows, list) or len(rows) != len(expected):
            raise Invalid(arm + ' does not cover the frozen case/repeat grid')
        seen, source_set, case_inputs = set(), set(), {}
        counts[arm], costs[arm], elapsed[arm] = 0, Decimal(0), []
        categories[arm] = defaultdict(int)
        for row in rows:
            case = row['case']
            repeat = integer(row['repeat'], 'repeat')
            if not isinstance(case, str) or (case, repeat) not in expected or (case, repeat) in seen:
                raise Invalid('unexpected or duplicate case/repeat')
            seen.add((case, repeat))
            source_set.add(digest(row['source_sha256'], 'source_sha256'))
            input_hash = digest(row['input_sha256'], 'input_sha256')
            if case in case_inputs and case_inputs[case] != input_hash:
                raise Invalid('case input changed across repeats')
            case_inputs[case] = input_hash
            observation = row['observation']
            if type(observation.get('version')) is not int or observation['version'] != 1:
                raise Invalid('unsupported observation version')
            score = observation['score']
            if type(score['qualified']) is not bool or type(score['critical_failure']) is not bool:
                raise Invalid('qualified and critical_failure must be booleans')
            if score['critical_failure'] and score['qualified']:
                raise Invalid('a critical failure cannot be qualified')
            counts[arm] += score['qualified']
            categories[arm][coverage['cases'][case]] += score['qualified']
            if arm == 'candidate':
                critical += score['critical_failure']
            if observation['cost'] is None:
                unknown.append(arm + ': provider cost unavailable')
            else:
                costs[arm] += number(observation['cost'], 'cost')
            seconds = score['elapsed_seconds']
            if seconds is None:
                if policy['limits']['max_job_seconds'] is not None:
                    unknown.append(arm + ': elapsed time unavailable')
            else:
                elapsed[arm].append(number(seconds, 'elapsed_seconds'))
        if len(source_set) != 1:
            raise Invalid('source changed within ' + arm)
        sources[arm] = source_set.pop()
        inputs[arm] = case_inputs
    if sources['baseline'] == sources['candidate']:
        raise Invalid('unchanged source')
    if inputs['baseline'] != inputs['candidate']:
        raise Invalid('unmatched input bytes')
    n = len(expected)
    gain = Decimal(counts['candidate'] - counts['baseline']) * 100 / n
    category_totals = defaultdict(int)
    for category in coverage['cases'].values():
        category_totals[category] += coverage['repeats']
    report = {
        'version': 1, 'use_case': policy['use_case'], 'split': split, 'pairs': n, 'source_sha256': sources,
        'policy_sha256': hashlib.sha256(json.dumps(policy, sort_keys=True, separators=(',', ':')).encode()).hexdigest(),
        'comparison_sha256': hashlib.sha256(json.dumps(data, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()).hexdigest(),
        'outcome': 'insufficient_evidence' if unknown else 'rejected',
        'paths_met': [], 'reasons': list(dict.fromkeys(unknown)),
        'quality_gain_pp': float(gain), 'candidate_critical_failures': critical,
        'categories': {category: {arm: categories[arm][category] for arm in counts} | {'jobs': total}
                       for category, total in sorted(category_totals.items())},
        'arms': {arm: {'qualified': counts[arm], 'jobs': n, 'quality_rate': counts[arm] / n,
                       'total_cost_usd': None if any(s.startswith(arm + ': provider') for s in unknown) else float(costs[arm]),
                       'max_job_seconds': float(max(elapsed[arm])) if len(elapsed[arm]) == n else None}
                 for arm in counts},
    }
    # Validate every row before deciding: a cheap early failure must not hide bad evidence.
    if unknown:
        return report
    old_cost, new_cost = costs['baseline'], costs['candidate']
    extra = max(Decimal(0), new_cost - old_cost)
    added = counts['candidate'] - counts['baseline']
    report['cost_change_percent'] = float((new_cost - old_cost) * 100 / old_cost) if old_cost else None
    report['cost_per_added_qualified_usd'] = float(extra / added) if added > 0 else None
    reasons = report['reasons']
    if critical:
        reasons.append('candidate has a critical correctness failure')
    if Decimal(counts['candidate']) / n < number(policy['quality_floor'], 'quality_floor'):
        reasons.append('candidate is below the quality floor')
    max_loss = number(policy['max_category_regression_pp'], 'max_category_regression_pp')
    for category, total in category_totals.items():
        if (categories['baseline'][category] - categories['candidate'][category]) * 100 > max_loss * total:
            reasons.append('quality regression exceeds allowance in category: ' + category)
    limit = policy['limits']['max_mean_cost_usd']
    if limit is not None and new_cost > number(limit, 'max_mean_cost_usd') * n:
        reasons.append('candidate exceeds mean cost limit')
    limit = policy['limits']['max_job_seconds']
    if limit is not None and max(elapsed['candidate']) > number(limit, 'max_job_seconds'):
        reasons.append('candidate exceeds job time limit')
    efficiency = policy['efficiency']
    if efficiency is not None and old_cost > 0:
        saving = number(efficiency['min_savings_percent'], 'min_savings_percent')
        loss = number(efficiency['max_quality_loss_pp'], 'max_quality_loss_pp')
        if gain >= -loss and new_cost * 100 <= old_cost * (100 - saving):
            report['paths_met'].append('efficiency')
    quality = policy['quality']
    if quality is not None:
        premium = number(quality['max_cost_increase_percent'], 'max_cost_increase_percent')
        unit_limit = quality['max_cost_per_added_qualified_usd']
        if (gain >= number(quality['min_gain_pp'], 'min_gain_pp') and
                new_cost * 100 <= old_cost * (100 + premium) and
                (unit_limit is None or extra <= number(unit_limit, 'max_cost_per_added_qualified_usd') * added)):
            report['paths_met'].append('quality')
    if not report['paths_met']:
        reasons.append('neither the quality gain nor efficiency benefit meets the frozen trade-off budget')
    if not reasons:
        report['outcome'] = 'accepted'
    return report


def envelope(report):
    if report['outcome'] == 'insufficient_evidence':
        raise Invalid('insufficient evidence: ' + '; '.join(report['reasons']))
    accepted = report['outcome'] == 'accepted'
    reason = ('accepted via ' + ', '.join(report['paths_met']) +
              f"; quality gain {report['quality_gain_pp']:g} pp; " +
              f"cost ${report['arms']['baseline']['total_cost_usd']:g} -> ${report['arms']['candidate']['total_cost_usd']:g}") if accepted else '; '.join(report['reasons'])
    return {'version': 1, 'decision': 'keep' if accepted else 'discard', 'reason': reason, 'cost': 0}


def strict(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise Invalid('duplicate JSON key: ' + key)
            result[key] = value
        return result
    def constant(value):
        raise Invalid('nonfinite JSON: ' + value)
    return json.loads(raw, object_pairs_hook=pairs, parse_constant=constant)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report', type=Path, help='write a new detailed assessment; never overwrite')
    parser.add_argument('--check-policy', action='store_true', help='validate policy JSON on stdin before admitting any evaluation')
    args = parser.parse_args()
    try:
        raw = sys.stdin.buffer.read(2 * 1024 * 1024 + 1)
        if len(raw) > 2 * 1024 * 1024:
            raise Invalid('comparison exceeds 2 MiB')
        data = strict(raw)
        if args.check_policy:
            if args.report:
                raise Invalid('--check-policy cannot write a comparison report')
            validate_policy(data)
            print(json.dumps({'version': 1, 'valid': True, 'use_case': data['use_case']['id'],
                              'executes_commands': False}))
            return 0
        report = assess(data)
        if args.report:
            with args.report.open('x') as output:
                json.dump(report, output, indent=2, allow_nan=False)
                output.write('\n')
        result = envelope(report)
        print(json.dumps(result, allow_nan=False))
        return int(result['decision'] == 'discard')
    except (ValueError, KeyError, TypeError, OSError, AttributeError) as error:
        print('acceptance judge: ' + str(error), file=sys.stderr)
        return 2


if __name__ == '__main__':
    sys.exit(main())
