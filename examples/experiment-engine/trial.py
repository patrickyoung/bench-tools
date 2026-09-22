#!/usr/bin/env python3
"""Run one bounded Agent trial. No retries, evaluation, or promotion."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def terminate_group(process, grace=7):
    """Let Record/Agent/Ply forward cancellation before escalating."""
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    deadline = time.monotonic() + grace
    while time.monotonic() < deadline:
        process.poll()
        try:
            os.killpg(process.pid, 0)
        except ProcessLookupError:
            break
        time.sleep(.02)
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    process.wait()


def fingerprint(root):
    files = {}
    for path in sorted(root.rglob('*')):
        if path.is_symlink():
            raise ValueError('definition symlinks are unsupported')
        if path.is_file():
            files[path.relative_to(root).as_posix()] = {
                'sha256': digest(path.read_bytes()),
                'executable': bool(path.stat().st_mode & 0o111),
            }
    if 'AGENTS.md' not in files or 'bin/check' not in files:
        raise ValueError('not an expert definition')
    return digest(json.dumps(files, sort_keys=True).encode())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--expert', type=Path, required=True)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--model', required=True)
    parser.add_argument('--effort', default='low', help='explicit Ask effort; off disables reasoning configuration')
    parser.add_argument('--agent', default='agent')
    parser.add_argument('--record', default='record')
    args = parser.parse_args()
    expert = args.expert.resolve(strict=True)
    source_sha = fingerprint(expert)
    request = args.input.read_bytes()
    # Resolve public executables before creating any partial trial directory.
    for executable in (args.agent, args.record):
        if shutil.which(executable) is None:
            raise ValueError('selected executable is unavailable')
    out = args.out.resolve()
    if out == expert or expert in out.parents:
        raise ValueError('trial output must be outside the definition')
    if out == args.input.resolve() or out in args.input.resolve().parents:
        raise ValueError('trial output must not contain the selected input')
    out.mkdir(parents=True, exist_ok=False)
    shutil.copytree(expert, out / 'expert')
    if fingerprint(out / 'expert') != source_sha:
        raise ValueError('definition changed during snapshot')
    work = out / 'work'
    work.mkdir()
    (work / 'request.json').write_bytes(request)
    argv = [args.agent, 'run', '-C', str(work), '-evidence', str(out / 'evidence'),
            '-m', args.model, '-effort', args.effort, '-turns', '3', '-cycles', '1',
            '-timeout', '40s', '-require-action=false', '-record-input', 'request.json',
            str(out / 'expert'), '--',
            'Route all supplied notes according to the supplied policy. Return only the JSON array.']
    # Record's timeout is a hard group kill. Send a signal ourselves so Agent
    # and Ply can also stop model commands in their separately owned groups.
    command = [args.record, 'run', '-f', str(out / 'process.jsonl'),
               '-grace', '5s', '--', *argv]
    def interrupt(*_):
        raise KeyboardInterrupt
    for signum in (signal.SIGTERM, signal.SIGHUP):
        signal.signal(signum, interrupt)
    started = time.monotonic()
    interrupted = False
    with (out / 'stdout').open('wb') as stdout, (out / 'stderr').open('wb') as stderr:
        process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=stdout, stderr=stderr,
                                   start_new_session=True)
        try:
            process.communicate(request, timeout=180)
        except (subprocess.TimeoutExpired, KeyboardInterrupt):
            interrupted = True
            terminate_group(process)
    elapsed = time.monotonic() - started
    verified = subprocess.run([args.record, 'check', '-f', str(out / 'process.jsonl')],
                              capture_output=True)
    # Source and task snapshots must remain the exact measured treatment/input.
    unchanged = (fingerprint(out / 'expert') == source_sha
                 and (work / 'request.json').read_bytes() == request)
    result = {'version': 1, 'expert_sha256': source_sha,
              'input_sha256': digest(request), 'stdout_sha256': digest((out / 'stdout').read_bytes()),
              'model': args.model, 'effort': args.effort, 'turn_limit': 3, 'cycle_limit': 1,
              'command_timeout': '40s', 'trial_timeout': '180s',
              'argv': argv, 'exit': process.returncode, 'seconds': elapsed,
              'record_verified': verified.returncode == 0, 'snapshots_unchanged': unchanged,
              'interrupted': interrupted}
    (out / 'trial.json').write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result))
    return process.returncode if verified.returncode == 0 and unchanged else 1


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError) as error:
        print(f'trial: {error}', file=sys.stderr)
        sys.exit(2)
