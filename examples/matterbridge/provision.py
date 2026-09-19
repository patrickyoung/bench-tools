#!/usr/bin/env python3
"""Provision one explicitly requested deployment; Tend owns its attempt."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time

from files import config, load, lock, read, save, provider, service, topic


def provision(state, deployment):
    cfg = config(state)
    directory = state / 'deployments' / deployment
    with lock(directory / 'provision.lock'):
        request = load(directory / 'request.json')
        candidate = state / 'candidates' / (request['candidate'] + '.json')
        if hashlib.sha256(read(candidate)).hexdigest() != request['sha256']:
            raise ValueError('candidate changed after deployment admission')
        value = load(candidate)
        package, queue = directory / 'package', directory / 'queue'
        binding = {'package': str(package), 'state': str(queue)}
        if not package.exists():
            subprocess.run([sys.executable, str(Path(cfg['bench_source']) / 'scripts/deploy-omnigent'),
                value['kind'], request['name'], str(package), '--ref', cfg['bench_ref'],
                '--candidate', str(candidate)], check=True)
        if not (directory / 'installed.json').exists():
            subprocess.run([str(package / 'install'), '--without-chat'], check=True)
            save(directory / 'installed.json', {'installed': True})
        if not queue.exists():
            subprocess.run([str(package / 'service'), '--state', str(queue), 'init'], check=True)
        service(binding, 'add-client', provider(cfg))
        smoke = service(binding, 'submit', {'session': 'smoke', 'key': 'smoke',
            'request': value['smoke_request']})
        deadline = time.monotonic() + 3600
        while smoke['status'] in ('ready', 'running'):
            if time.monotonic() > deadline:
                raise RuntimeError('smoke run timed out; inspect retained job')
            result = subprocess.run([str(package / 'service'), '--state', str(queue), 'work'])
            if result.returncode not in (0, 1):
                raise RuntimeError('smoke controller failed; inspect retained job')
            smoke = service(binding, 'get', {'job': smoke['id']})
            if smoke['status'] == 'running':
                time.sleep(1)
        save(directory / 'smoke.json', smoke)
        if smoke['status'] != 'done':
            raise RuntimeError('smoke did not succeed: ' + smoke['status'])
        topic_id = topic(state, cfg, directory, request['name'])
        save(directory / 'receipt.json', {
            'id': deployment, 'name': request['name'], 'binding': binding,
            'topic': topic_id, 'smoke_job': smoke['id'], 'candidate_sha256': request['sha256'],
            'status': 'awaiting-bridge'})
        print(json.dumps({'deployment': deployment, 'status': 'awaiting-bridge'}))


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--state', required=True, type=Path)
    parser.add_argument('deployment')
    args = parser.parse_args()
    if not args.state.is_absolute() or not (len(args.deployment) == 64 and
            all(c in '0123456789abcdef' for c in args.deployment)):
        raise ValueError('invalid state or deployment')
    provision(args.state, args.deployment)


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print('provision: ' + str(error), file=sys.stderr)
        sys.exit(2)
