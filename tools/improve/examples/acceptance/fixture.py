#!/usr/bin/env python3
"""Synthetic Improve adapter: invented scores/costs, zero inference."""
import json
from pathlib import Path
import sys

data = json.load(sys.stdin)
if sys.argv[1] == 'propose':
    print(json.dumps({'version': 1, 'hypothesis': 'Synthetic candidate for protocol verification.',
                      'changes': [{'path': 'AGENTS.md', 'content': data['files']['AGENTS.md'] + '\nFixture candidate.\n'}], 'cost': 0}))
else:
    candidate = 'Fixture candidate.' in (Path(data['source']) / 'AGENTS.md').read_text()
    scenario = data['settings']['fixture_scenario']
    qualified = candidate or scenario == 'efficiency' or data['repeat'] == 0
    cost = (0.8 if scenario == 'efficiency' else 1.25) if candidate else 1
    if scenario == 'over-budget' and candidate:
        cost = 2
    if scenario == 'unknown-cost' and candidate:
        cost = None
    critical = scenario == 'critical-holdout' and candidate and data['case']['id'] == 'held'
    print(json.dumps({'version': 1, 'score': {'qualified': qualified and not critical,
        'critical_failure': critical, 'elapsed_seconds': 1, 'synthetic': True}, 'cost': cost}))
