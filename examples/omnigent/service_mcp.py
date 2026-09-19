#!/usr/bin/env python3
"""Translate MCP tool calls to the deployment's ordinary JSON commands.

No admission, job state, Docker lifecycle, credentials or execution policy here.
The existing mcpserve supplies the wire protocol and process supervision.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parent
MAX_ENVELOPE = 20 * 1024 * 1024
OPERATIONS = {'submit_job': 'submit', 'get_job': 'get', 'list_jobs': 'list',
              'cancel_job': 'cancel', 'read_file': 'read-file'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--state', required=True)
    parser.add_argument('client')
    parser.add_argument('method', nargs='?', choices=['tools/call'])
    args = parser.parse_args()
    command = [str(ROOT / 'service'), '--state', args.state]
    # Authentication is the caller's fixed SSH command or trusted local argv.
    # Caller-supplied JSON can never select a client, state directory or program.
    names = ('PATH', 'HOME', 'LANG', 'LC_ALL', 'TMPDIR', 'DOCKER_HOST', 'DOCKER_CONTEXT', 'DOCKER_CONFIG')
    env = {name: os.environ[name] for name in names if name in os.environ}
    env['PYTHONDONTWRITEBYTECODE'] = '1'
    if args.method is None:
        checked = subprocess.run([*command, 'identity', args.client], env=env,
                                 stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL)
        if checked.returncode:
            return checked.returncode
        os.execve(str(ROOT / 'prefix/bin/mcpserve'), ['mcpserve', '-allow-legacy',
            '-timeout', '30s', '-max-input', str(MAX_ENVELOPE), str(ROOT / 'service-mcp.json'),
            '--', str(ROOT / 'service-mcp'), '--state', args.state, args.client], env)
    try:
        raw = sys.stdin.buffer.read(MAX_ENVELOPE + 1)
        if len(raw) > MAX_ENVELOPE:
            raise ValueError('request envelope too large')
        params = json.loads(raw)
        if not isinstance(params, dict) or params.get('name') not in OPERATIONS:
            raise ValueError('unknown tool')
        arguments = params.get('arguments', {})
        if not isinstance(arguments, dict):
            raise ValueError('arguments must be an object')
        result = subprocess.run([*command, OPERATIONS[params['name']], args.client],
            input=json.dumps(arguments).encode(), capture_output=True, env=env)
        if result.stderr:
            sys.stderr.buffer.write(result.stderr)
        error = result.returncode != 0
        if error:
            # Operational diagnostics remain on stderr. Request rejections use
            # the filter's fixed validation messages, without host error paths.
            message = (result.stderr.decode(errors='replace').removeprefix('service: ').strip()
                       if result.returncode == 1 else 'Operation unavailable; ask the operator to inspect.')
            value = {'error': message}
        else:
            value = json.loads(result.stdout)
    except (ValueError, KeyError, TypeError) as e:
        value, error = {'error': str(e)}, True
    except OSError as e:
        print('service-mcp: ' + str(e), file=sys.stderr)
        value, error = {'error': 'Operation unavailable; ask the operator to inspect.'}, True
    print(json.dumps({'content': [{'type': 'text', 'text': json.dumps(value)}], 'isError': error}))
    return 0


if __name__ == '__main__':
    sys.exit(main())
