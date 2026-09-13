#!/usr/bin/env python3
"""Offline application-boundary tests. No model, network, or native art tools."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import struct
import subprocess
import sys
import tempfile
import zlib

HERE = Path(__file__).resolve().parent
if len(sys.argv) != 2:
    raise SystemExit('usage: contracts.py /absolute/path/to/assembled-team/expert (use scripts/workers export-team)')
EXPERT = Path(sys.argv[1]).resolve()


def run(program, work, data=None, env=None, expected=0, args=()):
    result = subprocess.run([str(program), *args], cwd=work, input=data,
                            capture_output=True, env=env, timeout=15)
    assert result.returncode == expected, (program, result.returncode, result.stdout, result.stderr)
    return result


def write_program(path, text):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)
    path.chmod(0o755)


def handoff(work, ident, role, files):
    value = {'schema': 'page-team.handoff/v1', 'task_id': ident, 'specialist': role,
             'summary': 'Offline checker fixture, not execution evidence', 'files': [],
             'provenance': [], 'limitations': []}
    for name, data in files.items():
        path = work / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)
        value['files'].append({'path': name, 'source': str(path), 'bytes': len(data),
                              'sha256': hashlib.sha256(data).hexdigest(), 'media_type': 'text/plain'})
    return json.dumps(value) + '\n'


def dependency(ident, job, content):
    return {'id': ident, 'job': job, 'content': content,
            'sha256': 'sha256:' + hashlib.sha256(content.encode()).hexdigest()}


def root_cases(base):
    jobs = [base / 'jobs' / f'bench-manage-{n:06}' for n in range(1, 5)]
    for job in jobs:
        (job / 'work').mkdir(parents=True)
        (job / 'control').mkdir()
    page = (HERE / 'browser-fixture.html').read_bytes()
    original = handoff(jobs[0] / 'work', 'page', 'frontend', {'output/index.html': page})
    report = handoff(jobs[1] / 'work', 'review', 'review', {'output/review.json': b'{"verdict":"pass"}'})
    final = handoff(jobs[2] / 'work', 'copy', 'frontend', {'output/index.html': page})
    tasks = [{'id': ident, 'job': job.name, 'state': 'accepted', 'candidate': content,
              'needs': needs, 'input': {'kind': kind, 'worker': role}}
             for ident, job, content, needs, kind, role in [
                 ('page', jobs[0], original, [], 'integrate', 'frontend'),
                 ('review', jobs[1], report, ['page'], 'work', 'review'),
                 ('copy', jobs[2], final, ['review', 'page'], 'integrate', 'frontend')]]
    goal = 'A supplied offline contract fixture'
    packet = {'candidate': final, 'input': {'snapshot': {'goal': goal, 'tasks': tasks},
              'dependencies': [dependency('copy', jobs[2].name, final)]}}
    receipt = {'html_sha256': hashlib.sha256(page).hexdigest(),
               'goal_sha256': hashlib.sha256(goal.encode()).hexdigest(),
               'browser_passed': True, 'visual_passed': True, 'browser': {}}
    receipt_path = jobs[1] / 'control/review/receipt.json'
    receipt_path.parent.mkdir()
    def case(name, change=lambda p, r: None, expected=1):
        p, r = json.loads(json.dumps(packet)), dict(receipt)
        change(p, r)
        receipt_path.write_text(json.dumps(r))
        shutil.rmtree(jobs[3] / 'work/inputs', ignore_errors=True)
        run(EXPERT / 'bin/root-check', jobs[3] / 'work', json.dumps(p).encode(), expected=expected)
        print('ok root:', name)
    case('exact reviewed artifact', expected=0)
    case('changed HTML hash', lambda p, r: r.update(html_sha256='0' * 64))
    case('different original brief', lambda p, r: r.update(goal_sha256='0' * 64))
    case('browser failure', lambda p, r: r.update(browser_passed=False))
    case('visual rejection', lambda p, r: r.update(visual_passed=False))
    case('unaccepted review', lambda p, r: p['input']['snapshot']['tasks'][1].update(state='rejected'))
    def revised(p, r):
        p['input']['snapshot']['tasks'][1]['candidate'] = handoff(
            jobs[1] / 'work', 'review', 'review', {'output/review.json': b'{"verdict":"revise"}'})
    case('review expert requests changes', revised)
    case('review report changed after acceptance')

    # The preceding mutation deliberately changed the report on disk. Restore
    # the accepted report before testing a different acceptance boundary.
    (jobs[1] / 'work/output/review.json').write_bytes(b'{"verdict":"pass"}')

    def creative(p, r, visible):
        job = base / 'jobs/bench-manage-000005'
        (job / 'work').mkdir(parents=True, exist_ok=True)
        content = handoff(job / 'work', 'art', 'blender', {'output/preview.png': b'synthetic image identity'})
        p['input']['snapshot']['tasks'].append({'id': 'art', 'job': job.name,
            'state': 'accepted', 'candidate': content, 'needs': [],
            'input': {'kind': 'work', 'worker': 'blender'}})
        p['input']['snapshot']['tasks'][0]['needs'] = ['art']
        digest = hashlib.sha256(b'synthetic image identity').hexdigest()
        r['browser'] = {'visible_asset_sha256': [digest] if visible else []}
    case('creative input must be visible in any domain', lambda p,r: creative(p,r,False))
    case('creative input visibly bound in any domain', lambda p,r: creative(p,r,True), expected=0)


def image_cases(base):
    base.mkdir()
    # Valid tiny PNG with a bounded ancillary comment: a format fixture, not art.
    def chunk(name, data):
        return struct.pack('>I', len(data)) + name + data + struct.pack('>I', zlib.crc32(name + data))
    png = (b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('>IIBBBBB', 1, 1, 8, 2, 0, 0, 0))
           + chunk(b'tEXt', b'Purpose\x00' + b'contract fixture ' * 80)
           + chunk(b'IDAT', zlib.compress(b'\x00\x00\x00\x00')) + chunk(b'IEND', b''))
    (base / 'fixture.png').write_bytes(png)
    backend = base / 'backend'
    write_program(backend, '''#!/usr/bin/env python3
import json,pathlib,sys
p=json.load(sys.stdin)
assert pathlib.Path(p['output']).parent==pathlib.Path.cwd()
pathlib.Path(p['output']).write_bytes(pathlib.Path(sys.argv[0]).with_name('fixture.png').read_bytes())
print(json.dumps({'backend':'fixture','purpose':'contract test, not generated artwork'}))
''')
    failure = base / 'failure'
    write_program(failure, '#!/bin/sh\nexit 17\n')
    for name, exe, refs, symlink, expected in [
        ('export', backend, [], False, 0), ('missing', base / 'missing', [], False, 1),
        ('failure status', failure, [], False, 17), ('escaping ref', backend, ['../../secret'], False, 1),
        ('symlink output', backend, [], True, 1)]:
        job = base / name
        work, control = job / 'work', job / 'control'
        work.mkdir(parents=True)
        control.mkdir()
        if symlink:
            (job / 'outside').mkdir()
            (work / 'output').symlink_to(job / 'outside', target_is_directory=True)
        else:
            (work / 'output').mkdir()
        (work / 'output/request.json').write_text(json.dumps({'prompt': 'Contract fixture', 'references': refs}))
        (work / 'output/image-notes.md').write_text('Explicit fixture, not a creative evaluation.')
        env = dict(os.environ, PAGE_TEAM_IMAGEGEN=str(exe), BENCH_MANAGE_SESSION=str(control / 'session.jsonl'),
                   PAGE_TEAM_TASK_ID='image-test')
        run(EXPERT / 'bin/generate-image', work, env=env, expected=expected)
        assert (work / 'handoff.json').exists() == (expected == 0)
        if expected == 0:
            assert (work / 'output/generated.png').read_bytes() == png
        print('ok image capability:', name)


def mcp_cases(base):
    base.mkdir()
    dispatch = EXPERT / 'interfaces/mcp/dispatch'
    cases = [[], {'name': 'unknown'}, {'name': 'build_page', 'arguments': []}]
    for args in [{'brief': 'fixture', 'run_id': '../escape'}, {'brief': 'fixture', 'run_id': 1},
                 {'brief': None, 'run_id': 'test'}, {'brief': 'fixture', 'run_id': 'test', 'extra': True}]:
        cases.append({'name': 'build_page', 'arguments': args})
    for value in cases:
        result = run(dispatch, base, json.dumps(value).encode(), args=('tools/call',), expected=1)
        assert json.loads(result.stdout)['code'] == -32602
    copy = base / 'interfaces/mcp/dispatch'
    copy.parent.mkdir(parents=True)
    shutil.copy2(dispatch, copy)
    write_program(base / 'bin/page-team', '#!/bin/sh\nexit 17\n')
    result = run(copy, base, b'{"name":"build_page","arguments":{"brief":"fixture","run_id":"test"}}',
                 args=('tools/call',), env=dict(os.environ, PAGE_TEAM_MCP_RUN_ROOT=str(base / 'runs')))
    value = json.loads(result.stdout)
    assert value['isError'] is True and value['structuredContent']['exit'] == 17
    print('ok MCP: malformed inputs and logical tool failure')


def manager_case(base):
    tasks = [{'id': str(i), 'state': state, 'diagnostic': 'large trace ' * 800,
              'input': {'goal': 'Keep the original task', 'turns': 12},
              'candidate': 'Exact candidate bytes\n', 'needs': ['prior'],
              'receipt': {'check_exit': 0}, 'job': 'bench-manage-000001'}
             for i, state in enumerate(['accepted'] * 6 + ['incomplete'])]
    packet = {'snapshot_sha256': 'sha256:' + 'a' * 64,
              'snapshot': {'tasks': tasks, 'criteria': {'outcome': 'Original criterion'},
                           'goal': 'Original goal', 'allowance': {'remaining': 30}}}
    raw = json.dumps(packet).encode()
    result = run(EXPERT / 'bin/manager-input', base, raw)
    compact = json.loads(result.stdout)
    assert len(raw) > 65536 and len(result.stdout) < 65536
    assert compact['snapshot_sha256'] == packet['snapshot_sha256']
    for before, after in zip(tasks, compact['snapshot']['tasks']):
        assert {k: v for k, v in before.items() if k != 'diagnostic'} == {k: v for k, v in after.items() if k != 'diagnostic'}
    assert {k: v for k, v in packet['snapshot'].items() if k != 'tasks'} == {k: v for k, v in compact['snapshot'].items() if k != 'tasks'}
    print('ok manager: large trace reduction preserves every planning fact and original digest')


def review_cases(base):
    base.mkdir()
    sections = {name: [] for name in ['structural', 'functional', 'visual', 'responsive',
                                     'accessibility', 'provenance', 'limitations']}
    for verdict in ['pass', 'revise']:
        value = dict(sections, verdict=verdict)
        manifest = handoff(base, 'review-fixture', 'review', {
            'output/review.json': json.dumps(value).encode(), 'output/review.md': b'# Review fixture\n'})
        (base / 'handoff.json').write_text(manifest)
        run(EXPERT / 'agents/review/bin/check', base)
    (base / 'output/review.json').write_text(json.dumps({'verdict': 'pass', 'findings': sections}))
    result = run(EXPERT / 'agents/review/bin/check', base, expected=1)
    assert b'output/review.json' in result.stdout and b'top-level array' in result.stdout
    print('ok review: candid verdicts accepted, observed nested-JSON mistake rejected precisely')


with tempfile.TemporaryDirectory(prefix='page-team-contracts-') as temp:
    base = Path(temp).resolve()
    run(sys.executable, base, args=(str(HERE / 'handoffs.py'), str(EXPERT)))
    print('ok handoffs: eight positive/negative transfer cases')
    root_cases(base / 'root')
    image_cases(base / 'image')
    mcp_cases(base / 'mcp')
    manager_case(base)
    review_cases(base / 'review')
print('page-team: offline contracts passed; no live-quality claim')
