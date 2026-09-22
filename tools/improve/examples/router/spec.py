#!/usr/bin/env python3
"""Print a portable experiment spec. Generating it executes no model calls."""
import argparse
import json
from pathlib import Path
import shutil
import sys
from judge import savings_percent

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--offline', action='store_true', help='use synthetic observations; no inference')
p.add_argument('--model', help='explicit Agent runner model')
p.add_argument('--proposer-model', help='explicit Hire authoring model')
p.add_argument('--effort', default='low')
p.add_argument('--source', type=Path)
p.add_argument('--min-savings-percent', type=float, default=10)
a = p.parse_args()
r = Path(__file__).resolve().parent
if not a.offline and (not a.model or not a.proposer_model):
    p.error('select --offline or both --model and --proposer-model')
settings = {'effort': a.effort, 'min_savings_percent': a.min_savings_percent}
try:
    savings_percent(settings)
except ValueError as error:
    p.error(str(error))
adapter = r / ('fixture.py' if a.offline else 'bench.py')
names = ('record',) if a.offline else ('record', 'ask', 'hire', 'agent', 'ply', 'brief', 'cage')
tools = {}
for name in names:
    executable = shutil.which(name)
    if not executable:
        p.error('missing public command: ' + name)
    tools[name] = str(Path(executable).resolve())
if not a.offline:
    settings.update(runner_model=a.model, proposer_model=a.proposer_model, tools=tools)
print(json.dumps({'version': 1, 'source': str((a.source or r / 'source').resolve()),
    'mutable': ['AGENTS.md'], 'development': [{'id': 'dev', 'family': 'direct', 'file': str(r / 'development.json')}],
    'holdout': [{'id': 'held', 'family': 'resolved-and-quoted', 'file': str(r / 'holdout.json')}],
    'commands': {name: {'argv': [sys.executable, str(r / 'judge.py' if name == 'judge' else adapter)] + ([] if name == 'judge' else [name])}
                 for name in ('propose', 'trial', 'judge')},
    'dependencies': [str(adapter), str(r / 'judge.py'), *tools.values()],
    'record': tools['record'], 'settings': settings, 'repeats': 3, 'command_seconds': 230, 'max_seconds': 2400}, indent=2))
