#!/usr/bin/env python3
"""Admit and inspect durable jobs through JSON stdin/stdout.

0: operation succeeded (query the returned job status for completion).
1: request rejected. 2: configuration/controller failure. Diagnostics use stderr.
`work` and `check` preserve Tend's public stream and exit contracts.
"""
import argparse
import base64
import contextlib
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys

from contracts import (MAX_INPUT, MAX_ENVELOPE, TAIL, canonical, client_config,
                       config, durable_json, identifier, read_json, read_regular,
                       relative_path, settings, state_path, validate_credentials,
                       validate_submission, verify)

ROOT = Path(__file__).resolve().parent
STATE = None  # Selected only by --state; never inferred from a client payload.
QUEUE = None
ACTIVE = {'ready', 'running', 'waiting', 'unknown'}

def host_environment():
    # Provider credentials and SSH authentication never flow through Tend.
    names = ('PATH', 'HOME', 'LANG', 'LC_ALL', 'TMPDIR', 'DOCKER_HOST', 'DOCKER_CONTEXT', 'DOCKER_CONFIG')
    env = {key: os.environ[key] for key in names if key in os.environ}
    env.update(TEND_ROOT=str(QUEUE), PYTHONDONTWRITEBYTECODE='1',
               TEND_PASS='DOCKER_HOST DOCKER_CONTEXT DOCKER_CONFIG PYTHONDONTWRITEBYTECODE',
               TEND_JOB_MAX=settings(STATE, ROOT)['job_timeout'])
    return env


def tend(*args, data=b'', check=True):
    result = subprocess.run([str(ROOT / 'prefix/bin/tend'), *args], input=data,
                            capture_output=True, env=host_environment(), cwd=ROOT)
    if check and result.returncode:
        # Keep internal paths and implementation diagnostics off the client wire.
        sys.stderr.buffer.write(result.stderr)
        raise RuntimeError('Tend operation failed')
    return result


@contextlib.contextmanager
def locked(path, blocking=True):
    fd = os.open(path, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | (0 if blocking else fcntl.LOCK_NB))
        yield fd
    finally:
        os.close(fd)


def payload_for(job):
    if not isinstance(job, str) or not re.fullmatch(r'j[0-9a-f]{63}', job):
        raise ValueError('job not found')
    return read_json(QUEUE / 'jobs' / job / 'input')


def owned(client, job):
    if not isinstance(job, str) or not re.fullmatch(r'j[0-9a-f]{63}', job):
        raise ValueError('job not found')
    result = tend('show', job, check=False)
    if result.returncode == 1:
        raise ValueError('job not found')
    if result.returncode:
        sys.stderr.buffer.write(result.stderr)
        raise RuntimeError('Tend query failed')
    row = json.loads(result.stdout)
    if metadata(row)['client'] != client:
        raise ValueError('job not found')
    return row


def metadata(row):
    # Nonsecret identifiers are literal argv recorded by Tend. Listing never
    # needs to decode every retained upload or maintain a second metadata DB.
    argv = row['argv']
    if len(argv) != 8 or argv[:4] != [str(ROOT / 'sandbox-job'), '--state', str(STATE), 'run'] or argv[5] != row['id']:
        raise RuntimeError('unexpected job in the deployment queue; operator inspection required')
    return {'client': argv[4], 'session': argv[6], 'key': argv[7]}


def rows():
    return [json.loads(line) for line in tend('list').stdout.splitlines()]


def public_row(row, payload):
    result = {key: row[key] for key in ('id', 'status', 'created_us', 'updated_us', 'cancel_requested')}
    result.update(session=payload['session'], key=payload['key'])
    return result


def submit(client, session, key, request, files=None):
    files = {} if files is None else files
    validate_submission(session, key, request, files, config(ROOT))
    payload = {'client': client, 'session': session, 'key': key, 'request': request, 'files': files}
    data = canonical(payload)
    if len(data) > MAX_ENVELOPE:
        raise ValueError('request envelope too large')
    job = 'j' + hashlib.sha256(canonical([client, key])).hexdigest()[:63]
    serial = hashlib.sha256(canonical([client, session])).hexdigest()
    with locked(STATE / 'admission.lock'):
        cfg = settings(STATE, ROOT)
        current = rows()
        if (QUEUE / 'jobs' / job / 'input').exists():
            if canonical(payload_for(job)) != data:
                raise ValueError('idempotency key already names different inputs')
        else:
            mine = [row for row in current if metadata(row)['client'] == client]
            if len(mine) >= cfg['max_jobs'] or sum(row['status'] in ACTIVE for row in mine) >= cfg['max_pending']:
                raise ValueError('client job limit reached; inspect or retire retained work')
        # Stable stdin/argv/key let Tend atomically deduplicate, including a
        # crash after its files were durable but before its database commit.
        tend('submit', '-id', job, '-key', serial, '-C', str(ROOT), '--',
             str(ROOT / 'sandbox-job'), '--state', str(STATE), 'run', client, job, session, key, data=data)
    return get_job(client, job)


def get_job(client, job):
    row = owned(client, job)
    result = public_row(row, metadata(row))
    # Only Tend's host-owned streams are exposed here. Live sandbox
    # files are read through openat/O_NOFOLLOW below, never via resolved paths.
    events = [json.loads(line) for line in tend('events', job).stdout.splitlines()]
    result['events'] = [{'seq': e['seq'], 'kind': e['kind']} for e in events[-20:]]
    finished = [e['payload'] for e in events if e['kind'] == 'attempt.finished']
    if finished:
        result['last_finished_attempt'] = {key: finished[-1][key] for key in ('status', 'exit', 'signal')}
    attempt_dir = QUEUE / 'jobs' / job / 'attempts'
    outputs = sorted(attempt_dir.glob('*.out'), key=lambda p: int(p.stem)) if attempt_dir.exists() else []
    if outputs:
        result['attempts'] = len(outputs)
        result['output_may_change'] = row['status'] in {'running', 'unknown'}
        for name, suffix in [('stdout_tail', '.out'), ('stderr_tail', '.err')]:
            path = outputs[-1].with_suffix(suffix)
            result[name] = read_regular(path, TAIL, tail=True).decode(errors='replace')
    if row['status'] == 'unknown':
        result['note'] = 'Effects may exist. Session is fenced until the operator stops any orphan container and resolves it.'
    return result


def list_jobs(client, session=None, offset=0, limit=50):
    if session is not None:
        identifier(session)
    if type(offset) is not int or offset < 0 or type(limit) is not int or not 1 <= limit <= 100:
        raise ValueError('invalid pagination')
    found = []
    for row in rows():
        payload = metadata(row)
        if payload['client'] == client and (session is None or payload['session'] == session):
            found.append(public_row(row, payload))
    return {'jobs': found[offset:offset + limit],
            'next_offset': offset + limit if offset + limit < len(found) else None}


def cancel_job(client, job):
    owned(client, job)
    tend('cancel', job)
    result = get_job(client, job)
    result['note'] = 'Cancellation requested. Started work may have effects or leave a container; inspect its eventual status.'
    return result


def read_file(client, job, path, offset=0):
    owned(client, job)
    parts = relative_path(path)
    if parts[0] not in ('work', 'team') or len(parts) < 2:
        raise ValueError('select a file beneath work/ or team/')
    if type(offset) is not int or offset < 0 or offset > 1024**4:
        raise ValueError('invalid file offset')
    launch = read_json(STATE / 'runs' / job / 'launch.json')
    identifier(launch['sandbox'])
    # Each directory and the final file are opened relative to an already open
    # descriptor. A running container cannot race a symlink into a host read.
    fd = os.open(STATE / 'runs' / job, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        for part in [launch['sandbox'], *parts[:-1]]:
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
            os.close(fd)
            fd = child
        leaf = os.open(parts[-1], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
        with os.fdopen(leaf, 'rb') as stream:
            info = os.fstat(stream.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
                raise ValueError('result must be a regular file without hard links')
            stream.seek(offset)
            content = stream.read(MAX_INPUT)
        return {'job': job, 'path': path, 'offset': offset, 'size': info.st_size,
                'base64': base64.b64encode(content).decode(),
                'next_offset': offset + len(content) if offset + len(content) < info.st_size else None}
    finally:
        os.close(fd)


def work():
    verify(ROOT)
    cfg = settings(STATE, ROOT)
    for slot in range(cfg['workers']):
        try:
            with locked(STATE / ('slot-' + str(slot) + '.lock'), blocking=False) as fd:
                # Retain the slot in Tend as well if this wrapper is killed.
                result = subprocess.run([str(ROOT / 'prefix/bin/tend'), 'work'],
                    env=host_environment(), cwd=ROOT, pass_fds=(fd,))
                return result.returncode
        except BlockingIOError:
            continue
    return 1


def recover(client, job, decision):
    owned(client, job)
    with locked(STATE / 'admission.lock'):
        status = json.loads(tend('show', job).stdout)['status']
        if status not in {'unknown', 'failed', 'waiting'}:
            raise ValueError('recovery requires unknown, failed or waiting work')
        # This must succeed before Tend is allowed to release the session fence.
        subprocess.run([str(ROOT / 'sandbox-job'), '--state', str(STATE), 'stop', job],
                       stdout=subprocess.DEVNULL, env=host_environment(), check=True)
        if status == 'unknown':
            tend('resolve', job, decision)
        elif decision == 'fail':
            if status == 'waiting':
                tend('cancel', job)
        elif status == 'waiting':
            tend('signal', job, 'operator-resume')
        else:
            tend('retry', job)
    return get_job(client, job)


OPERATIONS = {
    'submit': (submit, {'session', 'key', 'request', 'files'}, {'session', 'key', 'request'}),
    'get': (get_job, {'job'}, {'job'}),
    'list': (list_jobs, {'session', 'offset', 'limit'}, set()),
    'cancel': (cancel_job, {'job'}, {'job'}),
    'read-file': (read_file, {'job', 'path', 'offset'}, {'job', 'path'}),
}


def request_input():
    raw = sys.stdin.buffer.read(MAX_ENVELOPE + 1)
    if len(raw) > MAX_ENVELOPE:
        raise ValueError('request envelope too large')
    value = json.loads(raw)
    if not isinstance(value, dict):
        raise ValueError('stdin must contain one JSON object')
    return value


def main():
    global STATE, QUEUE
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--state', required=True, type=state_path)
    commands = parser.add_subparsers(dest='command', required=True)
    for name in ('init', 'work', 'check'):
        commands.add_parser(name)
    for name in ('add-client', 'identity', *OPERATIONS):
        commands.add_parser(name).add_argument('client')
    recover_parser = commands.add_parser('recover')
    recover_parser.add_argument('client')
    recover_parser.add_argument('job')
    recover_parser.add_argument('decision', choices=['retry', 'fail'])
    args = parser.parse_args()
    STATE, QUEUE = args.state, args.state / 'tend'
    if args.command == 'init':
        verify(ROOT)
        if STATE == ROOT or STATE in ROOT.parents or ROOT / 'definition' in STATE.parents:
            raise ValueError('choose a private state directory outside definition/source files')
        STATE.mkdir(mode=0o700)
        for name in ('clients', 'runs'):
            (STATE / name).mkdir()
        durable_json(STATE / 'config.json', {'deployment': str(ROOT),
                     'deployment_sha256': hashlib.sha256((ROOT / 'deployment.json').read_bytes()).hexdigest(), 'workers': 2,
                     'max_pending': 32, 'max_jobs': 1000, 'job_timeout': '2h'})
        print(json.dumps({'state': str(STATE), 'deployment': str(ROOT)}))
        return 0
    settings(STATE, ROOT)
    if args.command == 'work':
        return work()
    if args.command == 'check':
        result = tend('check', check=False)
        sys.stdout.buffer.write(result.stdout)
        sys.stderr.buffer.write(result.stderr)
        return result.returncode
    identifier(args.client)
    if args.command == 'add-client':
        profile = request_input()
        validate_credentials(profile, ROOT)
        durable_json(STATE / 'clients' / (args.client + '.json'), profile)
        print(json.dumps({'client': args.client, 'configured': True}))
        return 0
    client_config(STATE, args.client, ROOT)
    if args.command == 'identity':
        verify(ROOT)
        result = {'client': args.client}
    elif args.command == 'recover':
        result = recover(args.client, args.job, args.decision)
    else:
        function, allowed, required = OPERATIONS[args.command]
        params = request_input()
        if set(params) - allowed or required - set(params):
            raise ValueError('invalid operation arguments')
        result = function(args.client, **params)
    print(json.dumps(result))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, KeyError, TypeError) as e:
        print('service: ' + str(e), file=sys.stderr)
        sys.exit(1)
    except (OSError, RuntimeError, subprocess.SubprocessError) as e:
        print('service: ' + str(e), file=sys.stderr)
        sys.exit(2)
