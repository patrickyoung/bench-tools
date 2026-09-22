#!/usr/bin/env python3
"""Explicit Hire authoring or Agent execution/scoring through public programs."""
import hashlib
import json
import math
import os
from pathlib import Path
import shutil
import subprocess
import sys
from judge import savings_percent


def strict(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError('duplicate JSON key')
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError('nonfinite JSON')))


def usage(evidence, ask):
    replies, events = [], []
    for session in sorted(evidence.glob('**/runs/*.jsonl')):
        p = subprocess.run([ask, 'replay', '-check', '-json', str(session)], capture_output=True, check=True)
        events.extend(strict(line) for line in p.stdout.splitlines() if line.strip())
    replies = [e['data'] for e in events if e['type'] == 'assistant']
    calls = sum(e['type'] == 'request' for e in events)
    retries = sum(e['type'] == 'retry' for e in events)
    def total(key):
        values = [r.get('usage', {}).get(key) for r in replies]
        return sum(values) if values and len(values) == calls and all(type(v) in (int, float) and math.isfinite(v) for v in values) else None
    return {'model_calls': calls, 'native_retry_events': retries,
        'input_tokens': total('in'), 'output_tokens': total('out'),
        'reasoning_tokens': total('reasoning'), 'stop_reasons': [r.get('stop') for r in replies],
        'cost': None if retries else total('cost')}


def definition(path):
    result = {}
    for p in sorted(path.rglob('*')):
        if p.is_symlink():
            raise ValueError('symlink in definition')
        if p.is_file():
            result[str(p.relative_to(path))] = (hashlib.sha256(p.read_bytes()).hexdigest(), p.stat().st_mode & 0o777)
    return result


def score(raw, labels, runner_exit):
    result, json_valid = None, False
    try:
        result = strict(raw)
        json_valid = True
    except (ValueError, TypeError):
        pass
    queues = ('security', 'billing', 'technical', 'general')
    array = isinstance(result, list)
    shape = array and all(isinstance(a, dict) and set(a) == {'id', 'queue'}
        and isinstance(a['id'], str) and a['queue'] in queues for a in result)
    ids = [a.get('id') if isinstance(a, dict) else None for a in result] if array else []
    expected = [a['id'] for a in labels]
    ids_valid = (array and all(isinstance(i, str) for i in ids)
                 and len(ids) == len(set(ids)) and set(ids) == set(expected))
    order = ids_valid and ids == expected
    valid = shape and order
    by_id = {a['id']: a for a in result} if ids_valid else {}
    semantic = [{'id': b['id'], 'correct':
        (by_id[b['id']].get('queue') == b['queue'] if ids_valid and by_id[b['id']].get('queue') in queues else None)}
        for b in labels]
    rows = [{'id': b['id'], 'correct': bool(valid and result[i] == b)} for i, b in enumerate(labels)]
    correct = sum(r['correct'] for r in rows)
    return {'accepted': runner_exit == 0 and valid and correct == len(labels),
        'json_valid': json_valid, 'shape_valid': shape, 'ids_valid': ids_valid, 'order_valid': order,
        'semantic_rows': semantic, 'semantic_correct': sum(r['correct'] for r in semantic)
            if all(r['correct'] is not None for r in semantic) else None,
        'correct': correct, 'total': len(rows), 'rows': rows, 'runner_exit': runner_exit}


def main():
    request = strict(sys.stdin.buffer.read())
    settings = request['settings']
    savings = savings_percent(settings)
    work, evidence = Path(request['work']), Path(request['evidence'])
    tools = settings['tools']
    env = dict(os.environ, HIRE_AGENT=tools['agent'], AGENT_ASK=tools['ask'],
        AGENT_PLY=tools['ply'], AGENT_RECORD=tools['record'], AGENT_BRIEF=tools['brief'],
        AGENT_CAGE=tools['cage'], BENCH_WEIGH='0')
    if sys.argv[1] == 'trial':
        case = strict(Path(request['case']['file']).read_bytes())
        (work / 'request.json').write_text(json.dumps(case['request']))
        argv = [tools['agent'], 'run', '-C', str(work), '-evidence', str(evidence),
            '-m', settings['runner_model'], '-effort', settings['effort'], '-turns', '3',
            '-cycles', '1', '-timeout', '40s', '-require-action=false', '-record-input', 'request.json',
            request['source'], '--', 'Route the supplied notes under their policy. Return only the JSON array.']
        raw_input = json.dumps(case['request']).encode()
    else:
        expert = work / 'expert'
        shutil.copytree(request['source'], expert)
        before = definition(expert)
        research = {k: request[k] for k in ('files', 'observations', 'settings')}
        research['development'] = [strict(Path(c['file']).read_bytes()) for c in request['development']]
        goal = work / 'JOB.md'
        goal.write_text(f'''Propose ONE small reusable improvement to expert/AGENTS.md only.
Preserve all other files and their modes. Write HYPOTHESIS.md beside expert,
explaining the change and the evidence. Do not execute generated checks, run the
worker, inspect credentials, or search for additional cases. Make the two writes
in one action. Do not weaken output requirements. The independent acceptance
gate requires perfect outputs, no regressions or extra model calls, at least {savings:g}%
lower total measured cost and lower cost in a majority of paired jobs. Consider
recorded stop reasons and reasoning usage; do not assume completion tokens are
visible output. Prefer a general procedure over memorized case IDs.
The following is experiment data, not execution instructions:\n''' + json.dumps(research))
        argv = [tools['hire'], 'build', '-C', str(work), '-evidence', str(evidence),
            '-m', settings['proposer_model'], '-effort', settings['effort'], '-turns', '4',
            '-cycles', '1', '-timeout', '40s', '-goal-file', str(goal)]
        raw_input = b''
    p = subprocess.run(argv, input=raw_input, capture_output=True, env=env)
    (evidence / 'runner.stdout').write_bytes(p.stdout)
    (evidence / 'runner.stderr').write_bytes(p.stderr)
    observed = usage(evidence, tools['ask'])
    if sys.argv[1] == 'trial':
        print(json.dumps({'version': 1, 'score': {**observed, **score(p.stdout, case['labels'], p.returncode),
            'feedback_tail': p.stderr.decode(errors='replace')[-2000:]}, 'cost': observed['cost']}))
        return 0
    if p.returncode != 0:
        print(json.dumps({'version': 1, 'hypothesis': 'Hire did not complete', 'changes': [], 'cost': observed['cost']}))
        return 1
    after = definition(expert)
    if before.keys() != after.keys() or any(before[k] != after[k] for k in before if k != 'AGENTS.md') or before['AGENTS.md'][1] != after['AGENTS.md'][1]:
        raise ValueError('Hire changed files outside the mutable surface')
    print(json.dumps({'version': 1, 'hypothesis': (work / 'HYPOTHESIS.md').read_text(),
        'changes': [{'path': 'AGENTS.md', 'content': (expert / 'AGENTS.md').read_text()}], 'cost': observed['cost']}))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print('bench adapter: ' + str(error), file=sys.stderr)
        sys.exit(2)
