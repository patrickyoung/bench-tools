#!/usr/bin/env python3
"""Score a retained router trial against explicitly selected independent labels."""
import argparse
import base64
import json
from pathlib import Path
import subprocess
import sys

from trial import digest, fingerprint


def strict(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError('duplicate JSON key')
            result[key] = value
        return result
    def invalid(value):
        raise ValueError('nonfinite JSON number')
    return json.loads(raw, object_pairs_hook=pairs, parse_constant=invalid)


def capture(argv):
    return subprocess.run(argv, check=True, capture_output=True).stdout


def score(root, labels_path, record='record', ask='ask'):
    meta = strict((root / 'trial.json').read_bytes())
    receipt = strict(capture([record, 'replay', '-f', str(root / 'process.jsonl'), '-json']))
    terminal = receipt['terminal']
    output = capture([record, 'replay', '-f', str(root / 'process.jsonl'), '-stream', 'stdout'])
    input_bytes = capture([record, 'replay', '-f', str(root / 'process.jsonl'), '-stream', 'stdin'])
    if (not terminal['complete'] or not terminal['started']
            or terminal['exit'] != meta['exit']
            or terminal.get('signal', 0) != 0 or terminal.get('interrupted', False)
            or [base64.b64decode(v).decode() for v in receipt['intent']['argv']] != meta['argv']
            or digest(output) != meta['stdout_sha256']
            or digest(input_bytes) != meta['input_sha256']
            or fingerprint(root / 'expert') != meta['expert_sha256']
            or not meta['snapshots_unchanged']):
        raise ValueError('trial receipt or source does not match')
    request = strict(input_bytes)
    labels_raw = labels_path.read_bytes()
    labels = strict(labels_raw)
    ids = [note['id'] for note in request['notes']]
    queues = {'security', 'billing', 'technical', 'general'}
    if (not isinstance(labels, list) or not ids or len(set(ids)) != len(ids)
            or len(labels) != len(ids)
            or any(not isinstance(row, dict) or set(row) != {'id', 'queue'}
                   or row['id'] != ident or row['queue'] not in queues
                   for row, ident in zip(labels, ids))):
        raise ValueError('labels must cover the exact ordered input IDs')
    result, valid = [], False
    try:
        result = strict(output)
        valid = (isinstance(result, list) and len(result) == len(ids)
                 and all(isinstance(row, dict) and set(row) == {'id', 'queue'}
                         and row['id'] == ident and row['queue'] in queues
                         for row, ident in zip(result, ids)))
    except (ValueError, TypeError):
        pass
    events = []
    sessions = sorted((root / 'evidence' / 'runs').glob('*.jsonl'))
    for session in sessions:
        raw = capture([ask, 'replay', '-check', '-json', str(session)])
        events.extend(strict(line) for line in raw.splitlines() if line.strip())
    responses = [event['data'] for event in events if event['type'] == 'assistant']
    calls = sum(event['type'] == 'request' for event in events)
    def total(key):
        values = [response.get('usage', {}).get(key) for response in responses]
        return sum(values) if values and len(values) == calls and all(type(v) in (int, float) for v in values) else None
    rows = [{'id': label['id'], 'expected': label['queue'],
             'actual': result[i]['queue'] if valid else None,
             'correct': bool(valid and result[i]['queue'] == label['queue'])}
            for i, label in enumerate(labels)]
    return {
        'version': 1, 'trial': str(root.resolve()),
        'status': 'ok' if meta['exit'] == 0 and valid and responses else 'failed',
        'expert_sha256': meta['expert_sha256'], 'input_sha256': meta['input_sha256'],
        'labels_sha256': digest(labels_raw),
        'scorer_sha256': digest(Path(__file__).read_bytes() + Path(__file__).with_name('trial.py').read_bytes()),
        'settings': {**{key: meta[key] for key in ('model', 'effort', 'turn_limit', 'cycle_limit', 'trial_timeout')},
                     # Older recipe manifests mislabeled Agent's command timeout.
                     'command_timeout': meta.get('command_timeout', meta.get('call_timeout'))},
        'reported_models': sorted({r['model'] for r in responses if r.get('model')}),
        'correct': sum(row['correct'] for row in rows), 'total': len(rows), 'rows': rows,
        'instruction_bytes': (root / 'expert' / 'AGENTS.md').stat().st_size,
        'seconds': meta['seconds'], 'model_calls': calls,
        'input_tokens': total('in'), 'output_tokens': total('out'), 'cost': total('cost'),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--trial', type=Path, required=True)
    parser.add_argument('--labels', type=Path, required=True)
    parser.add_argument('--record', default='record')
    parser.add_argument('--ask', default='ask')
    args = parser.parse_args()
    print(json.dumps(score(args.trial, args.labels, args.record, args.ask), sort_keys=True))


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError, subprocess.CalledProcessError) as error:
        print(f'score: invalid or incomplete evidence ({type(error).__name__})', file=sys.stderr)
        sys.exit(2)
