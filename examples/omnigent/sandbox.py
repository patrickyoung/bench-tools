#!/usr/bin/env python3
"""Run one selected sandbox command, or stop its owned container.

run reads one admitted job JSON on stdin; worker stdout/stderr stay separate.
Tend can supervise this executable, but direct invocation needs no queue or MCP.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import sys
import tempfile

from contracts import (MAX_ENVELOPE, client_config, container_name,
                       durable_json, scope_digest, settings,
                       state_path, sync_dir, validate_submission, verify)

ROOT = Path(__file__).resolve().parent

def require_local_docker():
    # Bind mounts resolve on the daemon host. SSH into remote machines and
    # install there; never accidentally send local paths to a remote daemon.
    endpoint = os.environ.get('DOCKER_HOST')
    context = os.environ.get('DOCKER_CONTEXT')
    if context or endpoint is None:
        context = context or subprocess.check_output(['docker', 'context', 'show'], text=True).strip()
        endpoint = subprocess.check_output(['docker', 'context', 'inspect', context,
            '--format', '{{.Endpoints.docker.Host}}'], text=True).strip()
    if not endpoint.startswith('unix://'):
        raise ValueError('use a local Docker socket; run the bundle over SSH on remote hosts')


def docker_argv(path, probe=False, env=None, scope=ROOT):
    env = os.environ if env is None else env
    installed = json.loads((ROOT / 'installed.json').read_text())
    image = installed['image']
    if not re.fullmatch(r'sha256:[0-9a-f]{64}', image):
        raise ValueError('installed image must be an immutable local image ID')
    require_local_docker()
    if ',' in str(path):
        raise ValueError('Docker mount paths must not contain commas')
    args = ['docker', 'run', '--rm', '-i', '--read-only', '--cap-drop=ALL',
            '--security-opt=no-new-privileges', '--pids-limit=512',
            '--memory=' + installed['memory'], '--cpus=' + installed['cpus'],
            '--user', f'{os.getuid()}:{os.getgid()}',
            '--tmpfs', '/tmp:rw,nosuid,nodev,size=256m,mode=1777',
            '--mount', f'type=bind,src={path},dst=/job',
            '--workdir', '/job' if probe else '/job/work']
    if not probe:
        args += ['--name', container_name(path.parent.name, scope)]
        args += ['--label', 'bench.deployment=' + hashlib.sha256(str(ROOT).encode()).hexdigest(),
                 '--label', 'bench.job=' + path.parent.name,
                 '--label', 'bench.scope=' + scope_digest(scope)]
        for key in installed['pass_env']:
            if not re.fullmatch(r'[A-Z][A-Z0-9_]*', key):
                raise ValueError('invalid environment name')
            if key in env:
                args += ['--env', key]
    return args + [image]


def job_command(cfg, model, checkpoint=False):
    expert = Path('/definition/expert')
    if cfg.get('entry') == 'hire':
        return ['python3', '/sandbox-builder.py']
    if cfg['kind'] == 'worker' or cfg.get('entry') == 'agent':
        return ['/sandbox/bin/agent', 'run', '-C', '/job/work',
                '-evidence', '/job/evidence', '-m', model, '-turns', '8',
                '-timeout', '15m', '-goal-file', '/job/input/request.txt',
                *(['-checkpoint', 'job'] if checkpoint else []), str(expert)]
    if cfg['name'] == 'page-team':
        return ['env', 'PAGE_TEAM_RUN=/job/team', str(expert / 'bin/page-team')]
    entry = 'compare-team' if cfg['name'] == 'vendor-comparison-team' else 'compare-studio'
    return [str(expert / 'bin' / entry), '/job/input/request.txt', '/job/team']


def prepare_sandbox(state, job, payload, cfg, attempt_key):
    parent = state / 'runs' / job
    parent.mkdir(exist_ok=True)
    name = 'sandbox' if cfg['kind'] == 'worker' else 'sandbox-' + hashlib.sha256(attempt_key.encode()).hexdigest()[:16]
    sandbox = parent / name
    if not sandbox.exists():
        temporary = Path(tempfile.mkdtemp(prefix='.prepare-', dir=parent))
        try:
            for directory in ('input', 'work', 'evidence'):
                (temporary / directory).mkdir()
            for name, encoded in payload['files'].items():
                path = temporary / 'input' / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(base64.b64decode(encoded, validate=True))
            (temporary / 'input/request.txt').write_text(payload['request'])
            if cfg['kind'] == 'worker' or cfg.get('entry') in ('agent', 'hire'):
                shutil.copytree(temporary / 'input', temporary / 'work', dirs_exist_ok=True)
            for path in temporary.rglob('*'):
                if path.is_file():
                    with path.open('rb') as stream:
                        os.fsync(stream.fileno())
            for directory in sorted((p for p in temporary.rglob('*') if p.is_dir()),
                                    key=lambda p: len(p.parts), reverse=True):
                sync_dir(directory)
            sync_dir(temporary)
            temporary.rename(sandbox)
            sync_dir(parent)
        finally:
            if temporary.exists():
                shutil.rmtree(temporary)
    elif sandbox.is_symlink() or not sandbox.is_dir():
        raise ValueError('invalid sandbox directory')
    durable_json(parent / 'launch.json', {'sandbox': sandbox.name, 'attempt_key': attempt_key,
                                         'container': container_name(job, state)})
    return sandbox


def run(state, client, job, session, key, attempt_key):
    cfg = verify(ROOT)
    settings(state, ROOT)
    if not re.fullmatch(r'j[0-9a-f]{63}', job):
        raise ValueError('invalid job id')
    if not attempt_key:
        raise ValueError('select --attempt or run under Tend with TEND_ATTEMPT_KEY')
    if os.environ.get('TEND_JOB_ID', job) != job:
        raise ValueError('Tend job id does not match the invocation')
    raw = sys.stdin.buffer.read(MAX_ENVELOPE + 1)
    if len(raw) > MAX_ENVELOPE:
        raise ValueError('request envelope too large')
    payload = json.loads(raw)
    if (not isinstance(payload, dict) or set(payload) != {'client', 'session', 'key', 'request', 'files'}
            or [payload['client'], payload['session'], payload['key']] != [client, session, key]):
        raise ValueError('job input does not match the invocation')
    validate_submission(session, key, payload['request'], payload['files'], cfg)
    profile = client_config(state, client, ROOT)
    sandbox = prepare_sandbox(state, job, payload, cfg, attempt_key)
    names = ('PATH', 'HOME', 'LANG', 'LC_ALL', 'TMPDIR', 'DOCKER_HOST', 'DOCKER_CONTEXT', 'DOCKER_CONFIG')
    env = {name: os.environ[name] for name in names if name in os.environ}
    env.update(profile)
    command = docker_argv(sandbox, env=env, scope=state) + job_command(cfg, profile['ASK_MODEL'], checkpoint=True)
    def interrupted(signum, frame):
        raise InterruptedError('attempt interrupted; inspect the container')
    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    try:
        stdin = payload['request'].encode() if cfg['kind'] == 'team' and cfg['name'] == 'page-team' else b''
        code = subprocess.run(command, input=stdin, env=env, cwd=ROOT).returncode
        if container_present(job, state):
            print('Docker client exited but the container remains; operator inspection required.', file=sys.stderr)
            return 125
        # Bare exit 75 is Tend's documented manual-wait case. The runner does
        # not wake, defer, retry, inspect a queue or decide recovery policy.
        return 125 if code < 0 or code >= 128 else code
    except (OSError, subprocess.SubprocessError):
        print('Docker outcome uncertain; operator inspection required.', file=sys.stderr)
        return 125


def container_present(job, scope):
    require_local_docker()
    name = container_name(job, scope)
    names = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'name=^/' + name + '$',
                                    '--format', '{{.Names}}'], text=True, timeout=15).splitlines()
    return name in names


def stop_container(job, scope):
    name = container_name(job, scope)
    if not container_present(job, scope):
        return
    labels = json.loads(subprocess.check_output(['docker', 'inspect', '--format', '{{json .Config.Labels}}', name],
                                                text=True, timeout=15))
    if labels.get('bench.scope') != scope_digest(scope) or labels.get('bench.job') != job or labels.get('bench.deployment') != hashlib.sha256(str(ROOT).encode()).hexdigest():
        raise ValueError('container ownership labels do not match; inspect manually')
    subprocess.run(['docker', 'rm', '-f', name], check=True, stdout=subprocess.DEVNULL, timeout=30)
    if container_present(job, scope):
        raise ValueError('container is still present; retaining the fence')


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--state', type=state_path)
    commands = parser.add_subparsers(dest='command', required=True)
    run_parser = commands.add_parser('run')
    for name in ('client', 'job', 'session', 'key'):
        run_parser.add_argument(name)
    run_parser.add_argument('--attempt', default=os.environ.get('TEND_ATTEMPT_KEY'))
    commands.add_parser('stop').add_argument('job')
    for name in ('execute', 'probe'):
        commands.add_parser(name).add_argument('directory', type=Path)
    commands.add_parser('doctor')
    args = parser.parse_args()
    if args.command in ('run', 'stop'):
        if args.state is None:
            parser.error('--state is required for run/stop')
        settings(args.state, ROOT)
        if args.command == 'run':
            return run(args.state, args.client, args.job, args.session, args.key, args.attempt)
        if not re.fullmatch(r'j[0-9a-f]{63}', args.job):
            raise ValueError('invalid job id')
        stop_container(args.job, args.state)
        print(json.dumps({'job': args.job, 'container_absent': True}))
        return 0
    if args.command == 'doctor':
        require_local_docker()
        return 0
    cfg = verify(ROOT)
    path = args.directory.resolve(strict=True)
    if args.command == 'probe':
        command = docker_argv(path, probe=True) + ['sh', '-c',
            'test ! -e /var/run/docker.sock && test ! -w /definition/expert/AGENTS.md && '
            'test ! -w /opt/bench/bin/agent && touch /job/probe && rm /job/probe']
        return subprocess.run(command, check=False).returncode
    if not os.environ.get('ASK_MODEL'):
        raise ValueError('set ASK_MODEL')
    # Synchronous adapter: selected prepared directory, no queue semantics.
    command = docker_argv(path) + job_command(cfg, os.environ['ASK_MODEL'])
    try:
        result = subprocess.run(command)
    except OSError as e:
        print('sandbox-job: ' + str(e), file=sys.stderr)
        return 125
    return result.returncode if result.returncode >= 0 else 128 - result.returncode


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as e:
        print('sandbox-job: ' + str(e), file=sys.stderr)
        sys.exit(2)
