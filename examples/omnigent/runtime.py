#!/usr/bin/env python3
"""Deployment adapter. Agent runs workers; team entry commands run teams.

No model client, protocol implementation, scheduler, retry loop or daemon.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

from contracts import MAX_INPUT, TAIL, read_regular, durable_json as write_json
import contracts

ROOT = Path(__file__).resolve().parent


def config():
    return contracts.config(ROOT)


def verify():
    return contracts.verify(ROOT)


def environment():
    env = os.environ.copy()
    env['PATH'] = str(ROOT / 'prefix/bin') + os.pathsep + env.get('PATH', '')
    env['BENCH_PREFIX'] = str(ROOT / 'prefix')
    env['PYTHONDONTWRITEBYTECODE'] = '1'
    return env


def install(with_chat=True):
    cfg = verify()
    for command in (('uv',) if with_chat else ()) + ('go', 'git', 'tar', 'docker'):
        if not shutil.which(command):
            raise ValueError('install prerequisite missing: ' + command)
    if sys.platform not in ('darwin', 'linux'):
        raise ValueError('this deployment requires macOS or Linux with Docker')
    subprocess.run([str(ROOT / "sandbox-job"), "doctor"], check=True)
    archive = ROOT / 'bench-source.tar'
    if hashlib.sha256(archive.read_bytes()).hexdigest() != cfg['source_sha256']:
        raise ValueError('Bench source archive changed')
    with tempfile.TemporaryDirectory(prefix='bench-source-') as temporary:
        source = Path(temporary)
        subprocess.run(['tar', '-xf', str(archive), '-C', str(source)], check=True)
        subprocess.run([sys.executable, str(ROOT / 'build-tools.py'), str(source),
                        str(ROOT / 'prefix'), 'mcp', 'tend'], check=True)
    if with_chat:
        if not (ROOT / 'omnigent/bin/python').exists():
            subprocess.run(['uv', 'venv', '--python', '3.12', str(ROOT / 'omnigent')], check=True)
        subprocess.run(['uv', 'pip', 'install', '--python', str(ROOT / 'omnigent/bin/python'),
                        'omnigent==' + cfg['omnigent_version']], check=True)
    tag = 'bench-worker:' + cfg['name'] + '-' + cfg['bench_commit'][:12]
    subprocess.run(['docker', 'build', '--tag', tag, str(ROOT)], check=True)
    image = subprocess.check_output(['docker', 'image', 'inspect', '--format', '{{.Id}}', tag], text=True).strip()
    settings = {'image': image, 'pass_env': ['ASK_MODEL',
        'OPENAI_API_KEY', 'ANTHROPIC_API_KEY', 'OPENAI_BASE_URL', 'ANTHROPIC_BASE_URL'],
        'memory': '4g', 'cpus': '2'}
    if (ROOT / 'installed.json').exists():
        settings.update(json.loads((ROOT / 'installed.json').read_text()))
    settings['image'] = image
    write_json(ROOT / 'installed.json', settings)
    (ROOT / 'runs').mkdir(exist_ok=True, mode=0o700)
    with tempfile.TemporaryDirectory(prefix='probe-', dir=ROOT) as temporary:
        subprocess.run([str(ROOT / 'sandbox-job'), 'probe', temporary], check=True)
    print('Installed. Read definition/expert/README.md for specialty prerequisites.')
    print('For durable local or remote jobs, follow SERVICE.md. Synchronous utilities: ./run or ./chat.')


def container_name(job):
    return 'bench-' + hashlib.sha256(str(ROOT).encode()).hexdigest()[:12] + '-' + job


def job_path(job):
    if not isinstance(job, str) or not re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}', job):
        raise ValueError('job must be 1-64 letters, digits, underscores or hyphens')
    path = ROOT / 'runs' / job
    if path.is_symlink() or (ROOT / 'runs').is_symlink():
        raise ValueError('symlinked run paths are not allowed')
    return path


def copy_inputs(dest):
    # Only explicit inputs enter a job; never previous runs or the host home.
    source = ROOT / 'inputs'
    if source.is_symlink():
        raise ValueError('inputs must not be a symlink')
    for p in source.rglob('*'):
        if p.is_symlink() or not (p.is_dir() or p.is_file()):
            raise ValueError('inputs must contain only regular files and directories')
    shutil.copytree(source, dest)


def run_job(job, request):
    cfg = verify()
    if not isinstance(request, str) or not request.strip() or len(request.encode()) > MAX_INPUT:
        raise ValueError('request must be nonempty text, at most 1 MiB')
    env = environment()
    if not env.get('ASK_MODEL'):
        raise ValueError('set ASK_MODEL to the configured Bench provider/model')
    # Atomic reservation prevents duplicate execution after lost MCP responses.
    path = job_path(job)
    path.mkdir(mode=0o700)
    write_json(path / 'status.json', {'job': job, 'state': 'unknown',
                                    'note': 'No terminal result yet; inspect before any new attempt.'})
    sandbox = path / 'sandbox'
    sandbox.mkdir()
    copy_inputs(sandbox / 'input')
    if (sandbox / 'input/request.txt').exists():
        raise ValueError('inputs/request.txt is reserved for the invocation request')
    (sandbox / 'input/request.txt').write_text(request)
    (sandbox / 'work').mkdir()
    (sandbox / 'evidence').mkdir()
    if cfg['kind'] == 'worker' or cfg.get('entry') == 'agent':
        # Selected files retain relative names in the worker workspace.
        shutil.copytree(sandbox / 'input', sandbox / 'work', dirs_exist_ok=True)
    elif cfg['name'] != 'page-team' and cfg.get('entry') != 'agent':
        # Team JSON refers to materials next to the admitted request file.
        json.loads(request)
    cmd = [str(ROOT / 'sandbox-job'), 'execute', str(sandbox)]
    with (path / 'stdout.txt').open('wb') as out, (path / 'stderr.txt').open('wb') as err:
        stdin = request.encode() if cfg['kind'] == 'team' and cfg['name'] == 'page-team' else b''
        result = subprocess.run(cmd, input=stdin, stdout=out, stderr=err,
                                cwd=sandbox / 'work', env=env)
    code = result.returncode if result.returncode >= 0 else 128 - result.returncode
    write_json(path / 'status.json', {'job': job, 'state': 'unknown' if code == 125 else 'finished', 'exit_code': code})
    return code


def read_job(job):
    path = job_path(job)
    result = json.loads(read_regular(path / 'status.json', MAX_INPUT))
    result['directory'] = str(path)
    result['container'] = container_name(job)
    for name in ('stdout', 'stderr'):
        file = path / (name + '.txt')
        if file.exists():
            result[name + '_tail'] = read_regular(file, TAIL, tail=True).decode(errors='replace')
    return result


def manifest():
    return {'name': 'bench-deployment', 'version': '1.0.0', 'tools': [
        {'name': 'run_job', 'description': 'Run the deployed Bench worker/team once. '
         'Use a new job ID. Never retry an uncertain call. Request is a worker goal, '
         'page-team brief, or comparison/studio job JSON. Results remain on disk.',
         'inputSchema': {'type': 'object', 'properties': {
             'job': {'type': 'string', 'pattern': '^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$'},
             'request': {'type': 'string', 'minLength': 1, 'maxLength': MAX_INPUT}},
             'required': ['job', 'request'], 'additionalProperties': False}},
        {'name': 'read_job', 'description': 'Inspect retained status and bounded output without rerunning.',
         'inputSchema': {'type': 'object', 'properties': {'job': {'type': 'string'}},
                         'required': ['job'], 'additionalProperties': False}}]}


def dispatch():
    try:
        if sys.argv[2:] != ['tools/call']:
            raise ValueError('only tools/call is supported')
        raw = sys.stdin.buffer.read(MAX_INPUT * 6 + 4097)
        if len(raw) > MAX_INPUT * 6 + 4096:
            raise ValueError('request envelope too large')
        params = json.loads(raw)
        args = params['arguments']
        name = params['name']
        keys = {'job', 'request'} if name == 'run_job' else {'job'}
        if name not in ('run_job', 'read_job') or not isinstance(args, dict) or set(args) != keys:
            raise ValueError('invalid tool arguments')
        if name == 'run_job':
            run_job(args['job'], args['request'])
        result = read_job(args['job'])
        error = result.get('state') != 'finished' or result.get('exit_code') != 0
    except (OSError, ValueError, KeyError, TypeError, subprocess.CalledProcessError) as e:
        result, error = {'error': str(e), 'note': 'Inspect the job before retrying; no automatic retry.'}, True
    print(json.dumps({'content': [{'type': 'text', 'text': json.dumps(result)}], 'isError': error}))


def serve():
    env = environment()
    os.execve(str(ROOT / 'prefix/bin/mcpserve'),
              ['mcpserve', '-allow-legacy', '-timeout', '2h', str(ROOT / 'manifest.json'),
               '--', str(ROOT / 'dispatch')], env)


def chat(args, service_client=None, service_state=None):
    verify()
    image = ROOT / 'chat-agent' if service_client is None else service_state / 'chat' / service_client
    (image / 'tools/mcp').mkdir(parents=True, exist_ok=True)
    # JSON is a YAML subset. Store environment references, never secret values.
    prompt = ('Use submit_job to queue this deployed Bench worker/team. '
        'Keep one session ID per conversation and a new key for each intentional request. '
        'Reuse the same key and identical inputs after a lost submission response. '
        'Read get_job for completion; queued/running/waiting/unknown are not success. '
        'Use list_jobs to recover handles. Unknown requires the operator; never submit a replacement. '
        'Use read_file for selected results, and explicitly attach any files needed by the next job. '
        'Sessions do not automatically share files or model history. '
        'Preserve the worker/team process; do not replace it with your own answer.'
        if service_client else
        'Use run_job to execute this deployed Bench ' + config()['kind'] +
        '. Preserve its existing team/worker process. Ask for missing required inputs. '
        'Choose a unique job ID and tell the user that ID before calling. '
        'Never replace the worker with your own answer or split its team into subagents. '
        'After a timeout or error use read_job; never automatically submit a new attempt. '
        'Exit 0 means the command accepted its result; other codes are not success. '
        'Report retained output paths and limitations. Host files are not browser download links.')
    write_json(image / 'config.yaml', {
        'spec_version': 1, 'name': 'bench-' + config()['name'],
        'executor': {'type': 'omnigent', 'config': {'harness': 'openai-agents'}},
        'tools': {'timeout': 7200, 'retry': {'max_retries': 0, 'timeout_per_request_s': 7200}},
        'prompt': prompt})
    write_json(image / 'tools/mcp/bench.yaml', {
        'name': 'bench', 'transport': 'stdio', 'command': str(ROOT / ('service-mcp' if service_client else 'serve')),
        'args': ['--state', str(service_state), service_client] if service_client else [], 'timeout': 7200,
        'retry': {'max_retries': 0, 'timeout_per_request_s': 7200},
        'env': {'BENCH_DEPLOYMENT': str(ROOT)}})
    if service_client is None:
        write_json(ROOT / 'manifest.json', manifest())
    env = environment()
    state = ROOT / 'omnigent-state' if service_client is None else service_state / 'omnigent' / service_client
    env['OMNIGENT_CONFIG_HOME'] = str(state)
    env['OMNIGENT_DATA_DIR'] = str(state)
    os.execve(str(ROOT / 'omnigent/bin/omnigent'),
              ['omnigent', 'run', str(image), *args], env)


def main():
    os.umask(0o077)
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('command', choices=['install', 'chat', 'chat-service', 'run', 'serve', 'dispatch'])
    # Parse only our command; flags after `chat` belong to Omnigent, including
    # --help. Otherwise argparse consumes that flag before generating the spec.
    args = p.parse_args(sys.argv[1:2])
    rest = sys.argv[2:]
    if args.command == 'install':
        if rest not in ([], ['--without-chat']):
            p.error('install [--without-chat]')
        install(with_chat=not rest)
        write_json(ROOT / 'manifest.json', manifest())
    elif args.command == 'run':
        if len(rest) != 1:
            p.error('run JOB_ID < REQUEST_FILE')
        request = sys.stdin.buffer.read(MAX_INPUT + 1).decode()
        code = run_job(rest[0], request)
        path = job_path(rest[0])
        for name, stream in [('stdout', sys.stdout.buffer), ('stderr', sys.stderr.buffer)]:
            with (path / (name + '.txt')).open('rb') as source:
                shutil.copyfileobj(source, stream)
        print('Retained run: ' + str(path), file=sys.stderr)
        sys.exit(code)
    elif args.command == 'chat':
        chat(rest)
    elif args.command == 'chat-service':
        cp = argparse.ArgumentParser()
        cp.add_argument('--state', required=True, type=contracts.state_path)
        cp.add_argument('client')
        cp.add_argument('args', nargs=argparse.REMAINDER)
        selected = cp.parse_args(rest)
        contracts.client_config(selected.state, selected.client, ROOT)
        chat(selected.args, service_client=selected.client, service_state=selected.state)
    elif args.command == 'serve':
        serve()
    else:
        dispatch()


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as e:
        sys.exit('bench deployment: ' + str(e))
