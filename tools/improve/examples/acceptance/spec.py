#!/usr/bin/env python3
"""Bind an app-defined policy to an experiment, or generate an explicit fixture."""
import argparse
import json
from pathlib import Path
import shutil
import sys
from judge import strict, validate_policy

parser = argparse.ArgumentParser(description=__doc__)
mode = parser.add_mutually_exclusive_group(required=True)
mode.add_argument('--scenario', choices=('efficiency', 'quality', 'over-budget', 'critical-holdout', 'unknown-cost'), help='explicit synthetic fixture; never a business policy')
mode.add_argument('--experiment', type=Path, help='existing draft specification created by the application')
parser.add_argument('--policy', type=Path, help='required app-defined policy for --experiment')
args = parser.parse_args()
root = Path(__file__).resolve().parent
if args.experiment:
    if not args.policy:
        parser.error('--experiment requires an explicit use-case --policy; no default is selected')
    try:
        policy_path = args.policy.resolve(strict=True)
        policy = strict(policy_path.read_bytes())
        validate_policy(policy)
        spec = strict(args.experiment.read_bytes())
        if type(spec['version']) is not int or spec['version'] != 1:
            raise ValueError('unsupported experiment version')
        if set(policy['coverage']) != {'development', 'holdout'}:
            raise ValueError('an Improve experiment requires policy coverage for both splits')
        for split in ('development', 'holdout'):
            ids = [case['id'] for case in spec[split]]
            coverage = policy['coverage'][split]
            if len(set(ids)) != len(ids) or set(ids) != set(coverage['cases']):
                raise ValueError(split + ' policy coverage does not match experiment cases')
            if type(spec['repeats']) is not int or spec['repeats'] != coverage['repeats']:
                raise ValueError(split + ' policy repeats do not match experiment repeats')
        if not isinstance(spec['settings'], dict):
            raise ValueError('experiment settings must be an object')
        dependencies = spec['dependencies']
        if not isinstance(dependencies, list) or any(not isinstance(p, str) for p in dependencies):
            raise ValueError('dependencies must be a list of paths')
        spec['settings']['acceptance'] = policy
        spec['commands']['judge'] = {'argv': [sys.executable, str(root / 'judge.py'), '--report', 'assessment.json']}
        spec['dependencies'] = list(dict.fromkeys(dependencies + [str(root / 'judge.py'), str(policy_path)]))
        print(json.dumps(spec, indent=2, allow_nan=False))
    except (ValueError, KeyError, TypeError, OSError, AttributeError) as error:
        parser.error(str(error))
    sys.exit(0)
if args.policy:
    parser.error('--policy requires --experiment; fixture policy cannot be mistaken for a production selection')
policy = json.loads((root / 'fixture-policy.json').read_text())
validate_policy(policy)
record = shutil.which('record')
if record is None:
    parser.error('missing public record executable')
print(json.dumps({
    'version': 1, 'source': str(root / 'source'), 'mutable': ['AGENTS.md'],
    'development': [{'id': 'dev', 'family': 'fixture-development', 'file': str(root / 'development.json')}],
    'holdout': [{'id': 'held', 'family': 'fixture-holdout', 'file': str(root / 'holdout.json')}],
    'commands': {
        'propose': {'argv': [sys.executable, str(root / 'fixture.py'), 'propose']},
        'trial': {'argv': [sys.executable, str(root / 'fixture.py'), 'trial']},
        'judge': {'argv': [sys.executable, str(root / 'judge.py'), '--report', 'assessment.json']}},
    'dependencies': [str(root / name) for name in ('fixture.py', 'judge.py', 'fixture-policy.json')],
    'record': str(Path(record).resolve()), 'settings': {'acceptance': policy, 'fixture_scenario': args.scenario},
    'repeats': 2, 'command_seconds': 30, 'max_seconds': 300}, indent=2))
