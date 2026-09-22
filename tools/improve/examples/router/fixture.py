#!/usr/bin/env python3
"""Deterministic plumbing fixture. Its scores are invented, not model evidence."""
import json
from pathlib import Path
import sys

request = json.load(sys.stdin)
if sys.argv[1] == 'propose':
    print(json.dumps({'version': 1, 'hypothesis': 'Require a raw JSON answer.',
        'changes': [{'path': 'AGENTS.md', 'content': request['files']['AGENTS.md'] + '\nReturn raw JSON only.\n'}], 'cost': 0}))
else:
    case = json.loads(Path(request['case']['file']).read_text())
    candidate = 'raw JSON only' in (Path(request['source']) / 'AGENTS.md').read_text()
    accepted = not (request['settings'].get('reject_holdout') and candidate and request['case']['id'] == 'held')
    rows = [{'id': row['id'], 'correct': accepted} for row in case['labels']]
    print(json.dumps({'version': 1, 'score': {'accepted': accepted,
        'correct': sum(row['correct'] for row in rows), 'total': len(rows),
        'rows': rows, 'model_calls': 1, 'synthetic': True}, 'cost': .8 if candidate else 1.0}))
