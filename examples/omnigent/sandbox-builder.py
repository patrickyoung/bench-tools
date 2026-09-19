#!/usr/bin/env python3
"""Container-only composition: Hire authors, verifies structure, exports source."""
import base64
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys


def main():
    work = Path('/job/work')
    metadata = json.loads((work / 'build.json').read_text())
    if metadata['kind'] not in ('worker', 'team'):
        raise ValueError('invalid build kind')
    authoring = work / 'authoring'
    authoring.mkdir(exist_ok=True)
    previous = work / 'previous-candidate.json'
    if previous.exists() and not (authoring / 'expert').exists():
        candidate = json.loads(previous.read_text())
        for name, entry in candidate['files'].items():
            if any(not re.fullmatch(r'[A-Za-z0-9_-][A-Za-z0-9_.-]{0,127}', p)
                   for p in name.split('/')):
                raise ValueError('invalid prior source path')
            path = authoring / 'expert' / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(base64.b64decode(entry['base64'], validate=True))
            path.chmod(entry['mode'])
    goal = work / 'hire-goal.txt'
    goal.write_text(
        'Build or revise a reusable Bench ' + metadata['kind'] + '.\n'
        'Read /definition/library/START-HERE.md and its Bench skill. Inspect both '
        'worker and team catalogs before building; reuse their expertise. The source '
        'library is a pinned read-only snapshot, not a Git checkout. Copy selected '
        'source explicitly when adapting it; installed command manuals are under '
        '/opt/bench/share. Write your reusable definition to expert/ here. '
        'For a team, produce an Agent-runnable manager with reusable specialists '
        'under expert/agents, explicit handoffs, and real integrated acceptance. '
        'Do not invent a new provider client, model loop, scheduler or host service. '
        'Use installed Agent/Hire and the public Bench commands. The deployment '
        'will run the root expert through Agent. Keep current cases, build records, '
        'secrets and runtime state outside expert/. Include an honest README with '
        'input/output contracts and any dependencies. Only dependencies available '
        'in this container can be assumed installed.\n'
        'Also write smoke.txt beside expert/: a self-contained, harmless example '
        'goal for a fresh deployment workspace, requiring no external credentials '
        'beyond the selected model. Write reply.md beside expert/ summarizing the '
        'result and limitations. This is a candidate; do not deploy or publish. '
        'The deployed expert must also write a user-facing reply.md in its job '
        'workspace, alongside its actual deliverables. '
        'Do not claim semantic quality from structural verification.\n\n'
        + (work / 'request.txt').read_text())
    env = dict(os.environ, HIRE_AGENT='/sandbox/bin/agent')
    code = subprocess.run(['hire', 'build', '-C', str(authoring),
        '-evidence', '/job/evidence', '-m', os.environ['ASK_MODEL'],
        '-turns', '12', '-timeout', '25m', '-goal-file', str(goal)], env=env).returncode
    if code:
        return code
    code = subprocess.run(['hire', 'verify', str(authoring / 'expert')], env=env).returncode
    if code:
        return code
    files, total = {}, 0
    for path in sorted((authoring / 'expert').rglob('*')):
        info = path.lstat()
        if stat.S_ISDIR(info.st_mode):
            continue
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
            raise ValueError('candidate contains a link or special file')
        if info.st_size > 8 * 1024 * 1024:
            raise ValueError('candidate source too large')
        name = str(path.relative_to(authoring / 'expert'))
        if any(not re.fullmatch(r'[A-Za-z0-9_-][A-Za-z0-9_.-]{0,127}', p)
               or p in {'node_modules', '__pycache__', 'runs', 'evidence'} for p in name.split('/')):
            raise ValueError('candidate contains runtime or invalid paths')
        data = path.read_bytes()
        total += len(data)
        if total > 8 * 1024 * 1024 or len(files) >= 64:
            raise ValueError('candidate exceeds bounded export')
        files[name] = {'base64': base64.b64encode(data).decode(),
                       'mode': 0o755 if info.st_mode & 0o111 else 0o644}
    smoke = (authoring / 'smoke.txt').read_text()
    if not 1 <= len(smoke.encode()) <= 65536:
        raise ValueError('smoke.txt must contain a bounded example goal')
    candidate = {'schema': 'bench.candidate/v1', 'kind': metadata['kind'],
                 'name': metadata['name'], 'smoke_request': smoke, 'files': files}
    (work / 'candidate.json').write_text(json.dumps(candidate))
    reply = authoring / 'reply.md'
    (work / 'reply.md').write_text((reply if reply.exists() else authoring / 'expert/README.md').read_text()[:12000])
    print('Candidate exported. Structural verification passed; deployment smoke run is separate.')
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError) as error:
        print('builder: ' + str(error), file=sys.stderr)
        sys.exit(2)
