#!/usr/bin/env python3
"""Matterbridge application: persistent conversations over public Bench commands.

No model client or action loop. Hire builds; Agent runs; Tend owns attempts.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
from urllib.parse import urlparse

from files import command, config, credential, digest, http_json, load, lock, provider, read, save, service, topic

ROOT = Path(__file__).resolve().parent
HELP = ('Use /new worker NAME or /new team NAME in the builder topic, then describe '
        'what it should do. Follow-up messages revise that draft. /deploy NAME '
        'deploys its latest completed candidate after a smoke run. /status shows '
        'progress; /cancel requests cancellation of the current build/run. '
        'Each deployment gets its own topic. Text messages are supported; '
        'files and voice messages are not imported automatically.')


def builder(state):
    return {'package': str(state / 'builder/package'), 'state': str(state / 'builder/queue')}


def queue_env(state):
    names = ('PATH', 'HOME', 'LANG', 'TMPDIR', 'DOCKER_HOST', 'DOCKER_CONTEXT', 'DOCKER_CONFIG')
    env = {name: os.environ[name] for name in names if name in os.environ}
    cfg = config(state)
    selected = list(cfg.get('provider_env', []))
    if 'telegram_token_env' in cfg:
        selected.append(cfg['telegram_token_env'])
    env.update({name: os.environ[name] for name in selected if name in os.environ})
    env.update(TEND_ROOT=str(state / 'provision-queue'), TEND_JOB_MAX='2h',
               TEND_PASS=' '.join(['DOCKER_HOST', 'DOCKER_CONTEXT', 'DOCKER_CONFIG', *selected]))
    return env


def tend(state, *args, value=None):
    result = subprocess.run([str(state / 'builder/package/prefix/bin/tend'), *args],
        input=json.dumps(value).encode() if value is not None else b'',
        capture_output=True, env=queue_env(state), timeout=30)
    if result.returncode:
        raise RuntimeError('provisioning queue operation failed')
    return result.stdout.decode()


def validate_config(cfg):
    required = {'bench_source', 'bench_ref', 'matterbridge', 'api_url',
                'telegram_chat_id', 'allowed_user_ids'}
    optional = {'builder_topic_id', 'api_token_file', 'api_token_env',
                'telegram_token_file', 'telegram_token_env', 'provider_file', 'provider_env'}
    if not required <= set(cfg) or set(cfg) - required - optional:
        raise ValueError('configuration fields do not match README')
    for name in ('api_token', 'telegram_token', 'provider'):
        if (name + '_env' in cfg) == (name + '_file' in cfg):
            raise ValueError('select exactly one environment or file source for ' + name)
    for name in ('api_token_env', 'telegram_token_env'):
        if name in cfg and (not isinstance(cfg[name], str) or not re.fullmatch(r'[A-Z][A-Z0-9_]{0,127}', cfg[name])):
            raise ValueError('invalid credential environment variable name')
    if 'provider_env' in cfg:
        allowed = {'ASK_MODEL', 'OPENAI_API_KEY', 'ANTHROPIC_API_KEY', 'OPENAI_BASE_URL', 'ANTHROPIC_BASE_URL'}
        names = cfg['provider_env']
        if not isinstance(names, list) or any(not isinstance(n, str) for n in names) or 'ASK_MODEL' not in names or set(names) - allowed:
            raise ValueError('provider_env must select ASK_MODEL and supported provider settings')
    for name in ('bench_source', 'matterbridge', *[n for n in optional if n.endswith('_file') and n in cfg]):
        if not isinstance(cfg[name], str) or not Path(cfg[name]).is_absolute():
            raise ValueError(name + ' must be an absolute path')
    if not re.fullmatch(r'[0-9a-f]{40}', cfg['bench_ref']):
        raise ValueError('bench_ref must be a full reviewed commit')
    url = urlparse(cfg['api_url'])
    if (url.scheme != 'http' or url.hostname != '127.0.0.1' or not url.port
            or url.path not in ('', '/') or url.query or url.fragment or url.username):
        raise ValueError('api_url must select a dedicated http://127.0.0.1:PORT')
    if not re.fullmatch(r'-100[0-9]+', str(cfg['telegram_chat_id'])):
        raise ValueError('telegram_chat_id must be a forum supergroup ID')
    users = cfg['allowed_user_ids']
    if (not isinstance(users, list) or not users or
            any(not isinstance(u, str) or not re.fullmatch(r'[1-9][0-9]*', u) for u in users)):
        raise ValueError('allowed_user_ids must contain Telegram numeric user IDs as strings')
    if 'builder_topic_id' in cfg and (type(cfg['builder_topic_id']) is not int or cfg['builder_topic_id'] < 1):
        raise ValueError('builder_topic_id must be positive')


def initialize(state, cfg):
    validate_config(cfg)
    source = Path(cfg['bench_source']).resolve()
    if state == source or source in state.parents:
        raise ValueError('state must be outside the source checkout')
    state.mkdir(mode=0o700)
    for name in ('builder', 'candidates', 'deployments', 'logs', 'submissions'):
        (state / name).mkdir()
    save(state / 'config.json', cfg)
    save(state / 'app.json', {'routes': {}, 'messages': {}, 'outbox': [],
                             'drafts': {}, 'deployments': {}, 'sequence': 0})


def bootstrap(state):
    cfg = config(state)
    credential(cfg, 'api_token')
    provider(cfg)
    # Verify group and bot rights before building anything expensive.
    token = credential(cfg, 'telegram_token')
    def tg(method, args=None):
        value = http_json('https://api.telegram.org/bot' + token + '/' + method, args or {})
        if not value.get('ok'):
            raise RuntimeError('Telegram setup check failed')
        return value['result']
    me = tg('getMe')
    chat = tg('getChat', {'chat_id': cfg['telegram_chat_id']})
    member = tg('getChatMember', {'chat_id': cfg['telegram_chat_id'], 'user_id': me['id']})
    if not chat.get('is_forum') or member.get('status') not in ('administrator', 'creator'):
        raise ValueError('bot must be an administrator in a forum-enabled group')
    if member.get('status') != 'creator' and not member.get('can_manage_topics'):
        raise ValueError('bot needs the Manage Topics right')
    if tg('getWebhookInfo').get('url'):
        raise ValueError('bot already has a webhook; use a dedicated bot for Matterbridge')
    binding = builder(state)
    package = Path(binding['package'])
    if not package.exists():
        subprocess.run([sys.executable, str(Path(cfg['bench_source']) / 'scripts/deploy-omnigent'),
                        'builder', 'builder', str(package), '--ref', cfg['bench_ref']], check=True)
    if not (state / 'builder/installed.json').exists():
        subprocess.run([str(package / 'install'), '--without-chat'], check=True)
        save(state / 'builder/installed.json', {'installed': True})
    if not Path(binding['state']).exists():
        command([package / 'service', '--state', binding['state'], 'init'])
    service(binding, 'add-client', provider(cfg))
    topic_id = cfg.get('builder_topic_id') or topic(state, cfg, state / 'builder', 'Bench builder')
    with lock(state / 'app.lock'):
        app = load(state / 'app.json')
        if 'builder' not in app['routes']:
            app['routes']['builder'] = {'channel': str(cfg['telegram_chat_id']) + '/' + str(topic_id),
                'binding': binding, 'history': [], 'pending': None, 'draft': None}
            reply(app, 'builder', HELP)
        save(state / 'app.json', app)


def reply(app, gateway, text):
    for index in range(0, len(text), 2800):
        app['outbox'].append({'gateway': gateway, 'text': text[index:index+2800], 'status': 'pending'})


def ingest(state, app, event):
    cfg = config(state)
    gateway = event.get('gateway')
    route = app['routes'].get(gateway)
    if (not route or event.get('account') != 'telegram.bench' or
            event.get('channel') != route['channel'] or
            event.get('userid') not in cfg['allowed_user_ids'] or event.get('event')):
        return False
    # Never authenticate by display name, message text, forwarded identity or nick.
    event_id = event.get('id')
    if not isinstance(event_id, str) or not event_id or len(event_id) > 256:
        return False
    key = digest([event['account'], event['channel'], event_id])
    if key in app['messages']:
        return False  # Includes edits of an already admitted post.
    if len(app['messages']) >= 10000:
        raise RuntimeError('transport retention limit reached; archive this instance before admitting more')
    if len(json.dumps(app).encode()) > 12 * 1024 * 1024:
        raise RuntimeError('conversation record limit reached; archive this instance before admitting more')
    text = event.get('text', '')
    if not isinstance(text, str) or len(text.encode()) > 65536:
        return False
    app['sequence'] += 1
    app['messages'][key] = {'key': key, 'gateway': gateway, 'user': event['userid'],
                           'text': text, 'sequence': app['sequence'], 'status': 'received'}
    if event.get('Extra') and event['Extra'].get('file'):
        reply(app, gateway, 'Attachments are not imported by this adapter. Describe the task in text.')
        app['messages'][key]['status'] = 'ignored'
    elif not text.strip():
        app['messages'][key]['status'] = 'ignored'
    return True


def result_file(binding, job, path, maximum=12 * 1024 * 1024):
    result, offset = bytearray(), 0
    while True:
        part = service(binding, 'read-file', {'job': job, 'path': path, 'offset': offset})
        if part['size'] > maximum:
            raise ValueError('result file exceeds limit')
        result.extend(base64.b64decode(part['base64'], validate=True))
        if len(result) > maximum:
            raise ValueError('result file exceeds limit')
        if part['next_offset'] is None:
            return bytes(result)
        if part['next_offset'] <= offset:
            raise ValueError('invalid result pagination')
        offset = part['next_offset']


def finish_jobs(state, app):
    for gateway, route in app['routes'].items():
        if not route['pending']:
            continue
        event = app['messages'][route['pending']]
        row = service(route['binding'], 'get', {'job': event['job']})
        if row['status'] in ('ready', 'running', 'waiting', 'unknown'):
            if row['status'] in ('waiting', 'unknown') and event.get('notice') != row['status']:
                reply(app, gateway, 'Job ' + row['id'] + ' is ' + row['status'] +
                      '. This conversation is paused until the operator resolves it.')
                event['notice'] = row['status']
            continue
        response = 'Job ' + row['id'] + ': ' + row['status'] + '.'
        if row['status'] == 'done':
            try:
                response = result_file(route['binding'], row['id'], 'work/reply.md', 65536).decode()
            except (RuntimeError, ValueError, UnicodeError):
                response += '\n' + row.get('stdout_tail', '')[-8000:]
            if gateway == 'builder':
                try:
                    raw = result_file(route['binding'], row['id'], 'work/candidate.json', 8 * 1024 * 1024 - 65536)
                    value = json.loads(raw)
                    if (value.get('schema') != 'bench.candidate/v1' or value.get('kind') != event['draft']['kind']
                            or value.get('name') != event['draft']['name']):
                        raise ValueError('builder candidate does not match admitted draft')
                    save(state / 'candidates' / (event['key'] + '.json'), value)
                    app['drafts'][event['draft']['name']] = {'candidate': event['key'], 'kind': value['kind']}
                    response += '\n\nCandidate saved. To install and smoke-test it: /deploy ' + event['draft']['name']
                except (RuntimeError, ValueError, KeyError, TypeError):
                    row['status'] = 'failed'
                    response = 'The build returned no valid candidate. Nothing was deployed. Revise the request or inspect job ' + row['id']
        response = response[:12000]
        event['status'], event['response'] = row['status'], response
        route['history'].extend([{'role': 'user', 'text': event['text']}, {'role': 'assistant', 'text': response}])
        route['history'] = route['history'][-30:]
        while len(json.dumps(route['history']).encode()) > 128 * 1024:
            route['history'].pop(0)
        route['pending'] = None
        reply(app, gateway, response[:12000])


def deploy(state, app, event, name):
    cfg = config(state)
    if not re.fullmatch(r'[a-z][a-z0-9-]{0,47}', name) or name not in app['drafts']:
        return 'No completed candidate with that name. Build it first.'
    if any(d['name'] == name for d in app['deployments'].values()):
        return 'That deployment name is already reserved. Use /status; it will not be launched twice.'
    if len(app['deployments']) >= 20:
        return 'Deployment limit reached (20). Ask the operator to archive unused deployments.'
    selected = event.get('deployment_request', {}).get('candidate', app['drafts'][name]['candidate'])
    candidate = state / 'candidates' / (selected + '.json')
    value = load(candidate)
    if not isinstance(value.get('smoke_request'), str) or not 1 <= len(value['smoke_request'].encode()) <= 65536:
        return 'Candidate has no bounded smoke request. Revise it in the builder before deployment.'
    directory = state / 'deployments' / event['key']
    directory.mkdir(exist_ok=True)
    request = {'name': name, 'candidate': selected, 'sha256': hashlib.sha256(read(candidate)).hexdigest()}
    if 'deployment_request' in event and event['deployment_request'] != request:
        raise RuntimeError('sealed deployment admission changed')
    if (directory / 'request.json').exists() and load(directory / 'request.json') != request:
        raise RuntimeError('deployment admission changed')
    save(directory / 'request.json', request)
    event['deployment_request'] = request
    save(state / 'app.json', app)
    # Durable idempotency lives in Tend, not this transport state file.
    tend(state, 'submit', '-id', event['key'], '-key', 'provision', '-C', str(ROOT), '--',
         str(ROOT / 'provision'), '--state', str(state), event['key'], value=request)
    app['deployments'][event['key']] = {'name': name, 'status': 'queued'}
    return 'Deploying ' + name + '. I will create its topic after installation and the smoke run succeed.'


def process_messages(state, app):
    for event in sorted(app['messages'].values(), key=lambda v: v['sequence']):
        if event['status'] not in ('received', 'prepared'):
            continue
        gateway, text = event['gateway'], event['text'].strip()
        route = app['routes'][gateway]
        # Read-only/cancellation controls remain available while the session is fenced.
        if text == '/status':
            if route['pending']:
                current = app['messages'][route['pending']]
                row = service(route['binding'], 'get', {'job': current['job']})
                message = 'Job ' + row['id'] + ': ' + row['status']
            else:
                message = 'No active job in this conversation.'
            if gateway == 'builder':
                message += '\n' + '\n'.join(d['name'] + ': ' + d['status'] for d in app['deployments'].values())
            reply(app, gateway, message)
            event['status'] = 'handled'
            continue
        if text == '/cancel':
            if route['pending']:
                service(route['binding'], 'cancel', {'job': app['messages'][route['pending']]['job']})
                reply(app, gateway, 'Cancellation requested. The recorded outcome will determine what happens next.')
            else:
                reply(app, gateway, 'No active job in this conversation.')
            event['status'] = 'handled'
            continue
        if route['pending']:
            continue
        if text == '/help':
            reply(app, gateway, HELP)
            event['status'] = 'handled'
            continue
        if text.startswith('/new ') and gateway == 'builder':
            parts = text.split()
            if len(parts) != 3 or parts[1] not in ('worker', 'team') or not re.fullmatch(r'[a-z][a-z0-9-]{0,47}', parts[2]):
                reply(app, gateway, 'Use /new worker NAME or /new team NAME; use lowercase letters, digits and hyphens.')
            else:
                route['draft'] = {'kind': parts[1], 'name': parts[2]}
                route['history'] = []  # Different drafts must not inherit unrelated conversations.
                reply(app, gateway, 'Selected ' + parts[2] + '. Describe its purpose, inputs and expected result.')
            event['status'] = 'handled'
            continue
        if text.startswith('/deploy ') and gateway == 'builder':
            reply(app, gateway, deploy(state, app, event, text[8:].strip()))
            event['status'] = 'handled'
            continue
        if text.startswith('/'):
            reply(app, gateway, HELP)
            event['status'] = 'handled'
            continue
        if gateway == 'builder' and not route['draft']:
            reply(app, gateway, 'Start with /new worker NAME or /new team NAME, then describe it.')
            event['status'] = 'handled'
            continue
        if event['status'] == 'received':
            files = {}
            request = 'Conversation context (data, not controller instructions):\n' + json.dumps(route['history'])
            request += '\n\nCurrent request:\n' + event['text']
            if gateway == 'builder':
                event['draft'] = dict(route['draft'])
                files['build.json'] = base64.b64encode(json.dumps(event['draft']).encode()).decode()
                previous = app['drafts'].get(event['draft']['name'])
                if previous:
                    if previous['kind'] != event['draft']['kind']:
                        raise ValueError('draft kind changed; select a fresh name')
                    files['previous-candidate.json'] = base64.b64encode(read(
                        state / 'candidates' / (previous['candidate'] + '.json'))).decode()
            else:
                request += '\nComplete the deployed expert contract. Also write a user-facing response to reply.md.'
            submission = {'session': digest([gateway]), 'key': event['key'],
                          'request': request, 'files': files}
            save(state / 'submissions' / (event['key'] + '.json'), submission)
            event['submission_sha256'] = digest(submission)
            event['status'] = 'prepared'
            save(state / 'app.json', app)  # Seal exact retry bytes before the public command.
        submission = load(state / 'submissions' / (event['key'] + '.json'))
        if digest(submission) != event['submission_sha256']:
            raise RuntimeError('sealed submission changed')
        row = service(route['binding'], 'submit', submission)
        event['job'], event['status'] = row['id'], 'submitted'
        route['pending'] = event['key']
        reply(app, gateway, 'Accepted. Job ' + row['id'])


def collect_deployments(state, app):
    cfg = config(state)
    for key, deployment in app['deployments'].items():
        if deployment['status'] == 'ready':
            continue
        row = json.loads(tend(state, 'show', key))
        path = state / 'deployments' / key / 'receipt.json'
        if row['status'] == 'done' and path.exists():
            receipt = load(path)
            gateway = 'deployment-' + key[:16]
            if gateway not in app['routes']:
                app['routes'][gateway] = {'channel': str(cfg['telegram_chat_id']) + '/' + str(receipt['topic']),
                    'binding': receipt['binding'], 'history': [], 'pending': None, 'draft': None}
            deployment.update(status='awaiting-bridge', gateway=gateway)
        elif deployment['status'] != row['status']:
            deployment['status'] = row['status']
            if row['status'] in ('failed', 'unknown', 'waiting', 'cancelled'):
                reply(app, 'builder', deployment['name'] + ': provisioning ' + row['status'] +
                      '. Operator inspection is required; no replacement deployment was started.')


def receive(state, app):
    cfg = config(state)
    spool = state / 'received.json'
    batch = load(spool) if spool.exists() else {'events': [], 'offset': 0}
    if batch['offset'] == len(batch['events']):
        events = http_json(cfg['api_url'].rstrip('/') + '/api/messages', token=credential(cfg, 'api_token'))
        if not isinstance(events, list):
            raise ValueError('Matterbridge did not return a message array')
        batch = {'events': events, 'offset': 0}
        save(spool, batch)
    while batch['offset'] < len(batch['events']):
        event = batch['events'][batch['offset']]
        ingest(state, app, event)
        save(state / 'app.json', app)
        batch['offset'] += 1
        save(spool, batch)


def flush(state, app):
    cfg = config(state)
    for entry in app['outbox']:
        if entry['status'] != 'pending':
            continue
        entry['status'] = 'unknown'
        save(state / 'app.json', app)
        # Matterbridge accepts a gateway, not a private reply-to-user address.
        response = http_json(cfg['api_url'].rstrip('/') + '/api/message',
                  {'gateway': entry['gateway'], 'username': 'Bench', 'text': entry['text']},
                  credential(cfg, 'api_token'))
        if not isinstance(response, dict) or response.get('gateway') != entry['gateway'] or response.get('account') != 'api.bench':
            raise RuntimeError('unexpected Matterbridge response; delivery remains uncertain')
        entry['status'] = 'accepted-by-bridge'
        save(state / 'app.json', app)


def render(state, app):
    cfg = config(state)
    quote = json.dumps  # TOML basic strings accept JSON's escaping used here.
    lines = ['[telegram.bench]', 'Token=""',
             'RemoteNickFormat="{NICK}: "', 'EditDisable=true', 'MessageFormat=""',
             '[api.bench]', 'BindAddress=' + quote('127.0.0.1:' + str(urlparse(cfg['api_url']).port)),
             'Token=""', 'Buffer=1000', 'RemoteNickFormat="{NICK}"']
    for name, route in sorted(app['routes'].items()):
        # Exactly one network conversation per gateway, never broadcast across clients.
        lines += ['[[gateway]]', 'name=' + quote(name), 'enable=true',
                  '[[gateway.inout]]', 'account="telegram.bench"', 'channel=' + quote(route['channel']),
                  '[[gateway.inout]]', 'account="api.bench"', 'channel="api"']
    return '\n'.join(lines) + '\n'


def status(state):
    app = load(state / 'app.json')
    return {'conversations': list(app['routes']), 'deployments': app['deployments'],
            'messages': len(app['messages']),
            'uncertain_deliveries': [i for i, row in enumerate(app['outbox']) if row['status'] == 'unknown']}


def run(state):
    cfg = config(state)
    children, handles, bridge_signature = {}, [], None
    stopped = False
    def stop(*unused):
        nonlocal stopped
        stopped = True
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    def launch(name, argv, env=None):
        log = (state / 'logs' / (name + '.log')).open('ab')
        handles.append(log)
        children[name] = subprocess.Popen([str(v) for v in argv], stdout=log, stderr=log, env=env)
    try:
        with lock(state / 'run.lock'):
            if 'builder' not in load(state / 'app.json')['routes']:
                raise ValueError('run bootstrap before run')
            launch('provision', [ROOT / 'queue-worker', state / 'builder/package/prefix/bin/tend'], queue_env(state))
            while not stopped:
                for name, child in children.items():
                    if child.poll() is not None:
                        raise RuntimeError(name + ' stopped; inspect its log before restarting')
                with lock(state / 'app.lock'):
                    app = load(state / 'app.json')
                    collect_deployments(state, app)
                    for name, route in app['routes'].items():
                        if name not in children:
                            binding = route['binding']
                            launch(name, [Path(binding['package']) / 'service-worker', '--state', binding['state']], queue_env(state))
                    content = render(state, app)
                    signature = hashlib.sha256(content.encode()).hexdigest()
                    if bridge_signature != signature:
                        if 'matterbridge' in children:
                            receive(state, app)  # Best-effort drain; upstream has no durable ACK protocol.
                            children['matterbridge'].terminate()
                            children['matterbridge'].wait(timeout=15)
                        path = state / 'matterbridge.toml'
                        path.write_text(content)
                        path.chmod(0o600)
                        bridge_env = {name: os.environ[name] for name in ('PATH', 'HOME', 'LANG', 'TMPDIR') if name in os.environ}
                        bridge_env.update(MATTERBRIDGE_TELEGRAM_BENCH_TOKEN=credential(cfg, 'telegram_token'),
                                          MATTERBRIDGE_API_BENCH_TOKEN=credential(cfg, 'api_token'))
                        launch('matterbridge', [cfg['matterbridge'], '-conf', str(path)], bridge_env)
                        deadline = time.monotonic() + 15
                        while True:
                            try:
                                if http_json(cfg['api_url'].rstrip('/') + '/api/health', token=credential(cfg, 'api_token')) != 'OK':
                                    raise RuntimeError('unexpected Matterbridge health response')
                                break
                            except RuntimeError:
                                if time.monotonic() >= deadline or children['matterbridge'].poll() is not None:
                                    raise RuntimeError('Matterbridge failed its health check')
                                time.sleep(.2)
                        bridge_signature = signature
                    for name, child in children.items():
                        if child.poll() is not None:
                            raise RuntimeError(name + ' stopped before readiness; inspect its log')
                    for deployment in app['deployments'].values():
                        if deployment['status'] == 'awaiting-bridge':
                            deployment['status'] = 'ready'
                            route = app['routes'][deployment['gateway']]
                            topic_id = route['channel'].split('/')[1]
                            link = 'https://t.me/c/' + str(cfg['telegram_chat_id'])[4:] + '/' + topic_id
                            reply(app, 'builder', deployment['name'] + ' is installed, smoke-tested, and connected: ' + link)
                            reply(app, deployment['gateway'], 'Ready. Send a request here. Conversation context persists across messages.')
                    save(state / 'app.json', app)
                    receive(state, app)
                    finish_jobs(state, app)
                    process_messages(state, app)
                    save(state / 'app.json', app)
                    flush(state, app)
                time.sleep(1)
    finally:
        for child in children.values():
            if child.poll() is None:
                child.terminate()
        for child in children.values():
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait()
        for handle in handles:
            handle.close()


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--state', required=True, type=Path)
    commands = parser.add_subparsers(dest='operation', required=True)
    for name in ('init', 'bootstrap', 'run', 'status', 'ingest', 'tick', 'receive', 'flush', 'render'):
        commands.add_parser(name)
    attach = commands.add_parser('attach-topic')
    attach.add_argument('deployment', help='builder or the full deployment ID')
    attach.add_argument('topic_id', type=int)
    resolve = commands.add_parser('resolve-delivery')
    resolve.add_argument('index', type=int)
    resolve.add_argument('decision', choices=['sent', 'retry'])
    args = parser.parse_args()
    if not args.state.is_absolute():
        raise ValueError('--state must be absolute')
    state = args.state.resolve()
    if args.operation == 'init':
        initialize(state, json.loads(sys.stdin.buffer.read(65537)))
        return
    config(state)
    if args.operation == 'bootstrap':
        with lock(state / 'setup.lock'):
            bootstrap(state)
        return
    if args.operation == 'run':
        run(state)
        return
    with lock(state / 'app.lock'):
        app = load(state / 'app.json')
        if args.operation == 'status':
            print(json.dumps(status(state)))
        elif args.operation == 'render':
            path = state / 'matterbridge.toml'
            path.write_text(render(state, app))
            path.chmod(0o600)
            print(json.dumps({'configuration': str(path)}))
        elif args.operation == 'attach-topic':
            if args.topic_id < 1 or (args.deployment != 'builder' and
                    not re.fullmatch(r'[0-9a-f]{64}', args.deployment)):
                raise ValueError('invalid deployment/topic')
            directory = state / 'builder' if args.deployment == 'builder' else state / 'deployments' / args.deployment
            if (directory / 'receipt.json').exists():
                raise ValueError('cannot change a completed deployment topic')
            save(directory / 'topic.json', {'id': args.topic_id, 'status': 'operator-confirmed'})
        elif args.operation == 'resolve-delivery':
            row = app['outbox'][args.index]
            if row['status'] != 'unknown':
                raise ValueError('delivery is not uncertain')
            row['status'] = 'accepted-by-bridge' if args.decision == 'sent' else 'pending'
        elif args.operation == 'ingest':
            raw = sys.stdin.buffer.read(1024 * 1024 + 1)
            if len(raw) > 1024 * 1024:
                raise ValueError('message exceeds limit')
            print(json.dumps({'admitted': ingest(state, app, json.loads(raw))}))
        elif args.operation == 'tick':
            collect_deployments(state, app)
            finish_jobs(state, app)
            process_messages(state, app)
        elif args.operation == 'receive':
            receive(state, app)
        elif args.operation == 'flush':
            flush(state, app)
        save(state / 'app.json', app)


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, KeyError, TypeError, RuntimeError, subprocess.SubprocessError) as error:
        print('matterbridge adapter: ' + str(error), file=sys.stderr)
        sys.exit(2)
