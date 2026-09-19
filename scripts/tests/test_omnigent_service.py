"""Real Tend/MCP integration with a fake Docker boundary; no paid model calls."""
import base64
import concurrent.futures
import getpass
import hashlib
import json
import os
from pathlib import Path
import signal
import shlex
import socket
import subprocess
import sys
import time
import unittest

import test_omnigent
from test_omnigent import BIN


@unittest.skipUnless((BIN / 'tend').exists(), 'build Tend before running service integration')
class OmnigentService(unittest.TestCase):
    def setUp(self):
        test_omnigent.OmnigentDeployment.setUp(self)
        self.state = Path(self.temp.name).resolve() / 'selected state with spaces'
        (self.root / 'prefix/bin/tend').symlink_to(BIN / 'tend')
        for name in ('active', 'observed', 'release'):
            (self.root / name).mkdir()
        (self.root / 'behavior.json').write_text('{}')
        docker = self.root / 'fake-bin/docker'
        docker.write_text('#!' + sys.executable + '\n' + '''
import json, os, pathlib, subprocess, sys, time
root = pathlib.Path(__file__).resolve().parents[1]
args = sys.argv[1:]
if args[0] == 'ps':
    pattern = args[args.index('--filter') + 1].removeprefix('name=^/').removesuffix('$')
    for p in (root / 'active').iterdir():
        if p.name == pattern:
            print(p.name)
    sys.exit(0)
if args[0] == 'inspect':
    print((root / 'active' / args[-1]).read_text())
    sys.exit(0)
if args[0] == 'rm':
    (root / 'active' / args[-1]).unlink()
    sys.exit(0)
name = args[args.index('--name') + 1]
labels = dict(args[i+1].split('=', 1) for i,v in enumerate(args) if v == '--label')
job = labels['bench.job']
mount = args[args.index('--mount') + 1].removeprefix('type=bind,src=').removesuffix(',dst=/job')
sandbox = pathlib.Path(mount)
behavior = json.loads((root / 'behavior.json').read_text()).get(job, {})
(root / 'active' / name).write_text(json.dumps(labels))
(root / 'observed' / job).write_text(json.dumps({'argv': args, 'env': dict(os.environ), 'mount': mount}))
if behavior.get('gate'):
    deadline = time.monotonic() + 20
    while not (root / 'release' / job).exists() and time.monotonic() < deadline:
        time.sleep(.03)
(sandbox / 'work/output.txt').write_text(os.environ.get('ASK_MODEL','missing'))
code = behavior.get('exit', 0)
if behavior.get('real_agent'):
    index = next(i for i,v in enumerate(args) if v.startswith('sha256:'))
    command = [v.replace('/definition', str(root / 'definition')).replace('/job', mount) for v in args[index+1:]]
    command[:2] = ['REAL_AGENT', 'run', '-no-cage']
    code = subprocess.call(command)
print('worker stream for ' + job)
print('worker diagnostics', file=sys.stderr)
if code != 125 and not behavior.get('orphan'):
    (root / 'active' / name).unlink(missing_ok=True)
sys.exit(code)
'''.replace('REAL_AGENT', str(BIN / 'agent')))
        docker.chmod(0o755)
        self.assert_ok(self.cli('init'))
        self.assert_ok(self.cli('add-client', 'alice', text=json.dumps({
            'ASK_MODEL': 'openai/alice', 'OPENAI_API_KEY': 'alice-secret'})))
        self.assert_ok(self.cli('add-client', 'bob', text=json.dumps({
            'ASK_MODEL': 'openai/bob', 'OPENAI_API_KEY': 'bob-secret'})))
        self.env.update(ANTHROPIC_API_KEY='ambient-secret', OPENAI_API_KEY='ambient-secret',
                        PRIVATE_UNRELATED='ambient-secret')

    def assert_ok(self, result):
        self.assertEqual(result.returncode, 0, result.stderr)
        return result

    def cli(self, *args, text='', **kwargs):
        return subprocess.run([str(self.root / 'service'), '--state', str(self.state), *args], input=text,
            capture_output=True, text=True, env=self.env, timeout=30, **kwargs)

    def tool(self, client, name, args, error=False):
        result = self.assert_ok(subprocess.run([str(self.root / 'service-mcp'), '--state', str(self.state),
            client, 'tools/call'], input=json.dumps({'name': name, 'arguments': args}),
            capture_output=True, text=True, env=self.env, timeout=30))
        response = json.loads(result.stdout)
        self.assertEqual(response['isError'], error, response)
        return json.loads(response['content'][0]['text'])

    def submit(self, client='alice', session='session', key='key', **extra):
        return self.tool(client, 'submit_job', {'session': session, 'key': key, 'request': 'fixture', **extra})['id']

    def status(self, job, client='alice'):
        return self.tool(client, 'get_job', {'job': job})['status']

    def behavior(self, value):
        self.runtime.write_json(self.root / 'behavior.json', value)

    def background_work(self):
        proc = subprocess.Popen([str(self.root / 'service'), '--state', str(self.state), 'work'], env=self.env,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
        def finish():
            if proc.poll() is None:
                os.killpg(proc.pid, signal.SIGINT)
            proc.communicate(timeout=10)
        self.addCleanup(finish)
        return proc

    def wait_for(self, predicate):
        deadline = time.monotonic() + 10
        while time.monotonic() < deadline:
            if predicate():
                return
            time.sleep(.04)
        self.fail('timed out waiting for real process transition')

    def test_deduplication_and_client_ownership(self):
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
            jobs = list(pool.map(lambda _: self.submit(), range(4)))
        self.assertEqual(len(set(jobs)), 1)
        job = jobs[0]
        self.tool('alice', 'submit_job', {'session': 'session', 'key': 'key', 'request': 'changed'}, error=True)
        for name, args in [('get_job', {'job': job}), ('cancel_job', {'job': job}),
                           ('read_file', {'job': job, 'path': 'work/output.txt'})]:
            self.assertEqual(self.tool('bob', name, args, error=True)['error'], 'job not found')
        self.assertEqual(self.tool('bob', 'list_jobs', {})['jobs'], [])
        self.assertNotEqual(self.submit(client='bob'), job)
        self.assertEqual(self.status(job), 'ready')
        self.assert_ok(self.cli('check'))

    def test_execution_credentials_inputs_and_output(self):
        (self.root / 'inputs/shared-secret').write_text('must never enter service jobs')
        selected = base64.b64encode(b'selected current evidence').decode()
        job = self.submit(files={'materials/input.txt': selected})
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(job), 'done')
        observed = json.loads((self.root / 'observed' / job).read_text())
        env = observed['env']
        self.assertEqual(env['OPENAI_API_KEY'], 'alice-secret')
        self.assertNotIn('ANTHROPIC_API_KEY', env)
        self.assertNotIn('PRIVATE_UNRELATED', env)
        self.assertNotIn('alice-secret', json.dumps(observed['argv']))
        self.assertIn('-checkpoint', observed['argv'])
        sandbox = Path(observed['mount'])
        self.assertFalse((sandbox / 'input/shared-secret').exists())
        self.assertEqual((sandbox / 'work/materials/input.txt').read_bytes(), b'selected current evidence')
        result = self.tool('alice', 'read_file', {'job': job, 'path': 'work/output.txt'})
        self.assertEqual(base64.b64decode(result['base64']), b'openai/alice')
        # Symlinked directories and final files cannot disclose host files.
        (sandbox / 'work/escape').symlink_to(self.state / 'clients', target_is_directory=True)
        (sandbox / 'work/secret').symlink_to(self.state / 'clients/bob.json')
        for path in ['work/escape/bob.json', 'work/secret', '../clients/bob.json', 'evidence/anything']:
            self.tool('alice', 'read_file', {'job': job, 'path': path}, error=True)
        self.assert_ok(self.cli('check'))

    def test_sessions_serialize_and_independent_sessions_overlap(self):
        first = self.submit(key='first')
        second = self.submit(key='second')
        other = self.submit(session='other', key='third')
        self.behavior({first: {'gate': True}, other: {'gate': True}})
        a = self.background_work()
        self.wait_for(lambda: (self.root / 'observed' / first).exists())
        b = self.background_work()
        self.wait_for(lambda: (self.root / 'observed' / other).exists())
        self.assertEqual(self.status(second), 'ready')
        self.assertEqual(self.cli('work').returncode, 1)  # Both configured slots occupied.
        (self.root / 'release' / first).touch()
        a.communicate(timeout=10)
        self.assertEqual(a.returncode, 0)
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(second), 'done')
        (self.root / 'release' / other).touch()
        b.communicate(timeout=10)
        self.assertEqual(self.status(other), 'done')

    def test_unknown_fence_and_explicit_recovery(self):
        first = self.submit(key='first')
        second = self.submit(key='second')
        self.behavior({first: {'exit': 125}})
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(first), 'unknown')
        self.assertEqual(self.cli('work').returncode, 1)
        self.assertEqual(self.submit(key='first'), first)  # No new execution.
        self.assertEqual(len(list((self.root / 'active').iterdir())), 1)
        first_launch = json.loads((self.root / 'observed' / first).read_text())
        sandbox = Path(first_launch['mount'])
        (sandbox / 'work/retained.txt').write_text('earlier attempt workspace')
        (sandbox / 'evidence/retained.txt').write_text('earlier attempt evidence')
        self.behavior({})
        self.assert_ok(self.cli('recover', 'alice', first, 'retry'))
        self.assertEqual(list((self.root / 'active').iterdir()), [])
        for _ in range(2):
            self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(first), 'done')
        self.assertEqual(self.status(second), 'done')
        retry_launch = json.loads((self.root / 'observed' / first).read_text())
        self.assertEqual(retry_launch['mount'], first_launch['mount'])
        self.assertIn('-checkpoint', retry_launch['argv'])
        self.assertEqual((sandbox / 'work/retained.txt').read_text(), 'earlier attempt workspace')
        self.assertEqual((sandbox / 'evidence/retained.txt').read_text(), 'earlier attempt evidence')
        self.assertEqual(self.tool('alice', 'get_job', {'job': first})['attempts'], 2)
        self.assert_ok(self.cli('check'))

    def test_wait_and_cancel(self):
        waiting = self.submit(key='waiting')
        self.behavior({waiting: {'exit': 75}})
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(waiting), 'waiting')
        self.behavior({})
        self.assert_ok(self.cli('recover', 'alice', waiting, 'retry'))
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(waiting), 'done')
        queued = self.submit(key='cancel')
        self.tool('alice', 'cancel_job', {'job': queued})
        self.assertEqual(self.status(queued), 'cancelled')
        self.assertEqual(self.cli('work').returncode, 1)
        self.assert_ok(self.cli('check'))

    def test_timeout_preserves_unknown_and_recovery_stops_orphan(self):
        cfg_path = self.state / 'config.json'
        cfg = json.loads(cfg_path.read_text())
        cfg['job_timeout'] = '1s'
        cfg_path.write_text(json.dumps(cfg))
        job = self.submit()
        self.behavior({job: {'gate': True}})
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(job), 'unknown')
        self.assertTrue(list((self.root / 'active').iterdir()))
        self.assert_ok(self.cli('recover', 'alice', job, 'fail'))
        self.assertEqual(self.status(job), 'failed')
        self.assertFalse(list((self.root / 'active').iterdir()))
        self.assert_ok(self.cli('check'))

    def test_successful_docker_client_cannot_hide_live_container(self):
        job = self.submit()
        self.behavior({job: {'orphan': True}})
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(job), 'unknown')
        # Recovery may not remove a same-named container with different ownership.
        active = next((self.root / 'active').iterdir())
        original = active.read_text()
        active.write_text('{}')
        self.assertNotEqual(self.cli('recover', 'alice', job, 'retry').returncode, 0)
        self.assertEqual(self.status(job), 'unknown')
        active.write_text(original)
        self.assert_ok(self.cli('recover', 'alice', job, 'fail'))

    def test_service_chat_has_separate_client_state(self):
        cli = self.root / 'omnigent/bin/omnigent'
        cli.parent.mkdir(parents=True)
        cli.write_text('#!/bin/sh\nprintf "%s\\n" "$OMNIGENT_DATA_DIR" "$@"\n')
        cli.chmod(0o755)
        for client in ('alice', 'bob'):
            result = self.assert_ok(subprocess.run([str(self.root / 'service-chat'), '--state', str(self.state),
                client, '--help'], capture_output=True, text=True, env=self.env))
            self.assertIn(str(self.state / 'omnigent' / client), result.stdout)
            config = json.loads((self.state / 'chat' / client / 'tools/mcp/bench.yaml').read_text())
            self.assertEqual(config['args'], ['--state', str(self.state), client])
            self.assertNotIn('secret', json.dumps(config))

    @unittest.skipUnless(os.environ.get('BENCH_OMNIGENT_SSHD'), 'set BENCH_OMNIGENT_SSHD for loopback SSH authentication check')
    def test_real_ssh_forced_identity(self):
        directory = Path(self.temp.name) / 'ssh'
        directory.mkdir(mode=0o700)
        keys = {}
        for name in ('host', 'alice', 'bob', 'untrusted'):
            key = directory / name
            subprocess.run(['ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', str(key)], check=True)
            keys[name] = key
        authorized = directory / 'authorized_keys'
        lines = []
        for client in ('alice', 'bob'):
            forced = shlex.join([str(self.root / 'service-mcp'), '--state', str(self.state), client])
            escaped = forced.replace('\\', '\\\\').replace('"', '\\"')
            lines.append('restrict,command="' + escaped + '" ' + keys[client].with_suffix('.pub').read_text())
        authorized.write_text(''.join(lines))
        authorized.chmod(0o600)
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        config = directory / 'sshd_config'
        config.write_text('\n'.join([
            'ListenAddress 127.0.0.1', 'Port ' + str(port), 'HostKey ' + str(keys['host']),
            'PidFile ' + str(directory / 'pid'), 'AuthorizedKeysFile ' + str(authorized),
            'PasswordAuthentication no', 'KbdInteractiveAuthentication no', 'UsePAM no',
            'PermitUserEnvironment no', 'AllowUsers ' + getpass.getuser(), 'LogLevel VERBOSE']))
        log = (directory / 'sshd.log').open('w+')
        self.addCleanup(log.close)
        daemon = subprocess.Popen([os.environ['BENCH_OMNIGENT_SSHD'], '-D', '-e', '-f', str(config)],
                                  stdout=log, stderr=log)
        def stop():
            daemon.terminate()
            daemon.wait(timeout=10)
        self.addCleanup(stop)
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            if daemon.poll() is not None:
                log.seek(0)
                self.fail('test SSH daemon did not start: ' + log.read())
            try:
                with socket.create_connection(('127.0.0.1', port), timeout=.2):
                    break
            except OSError:
                time.sleep(.05)
        known = directory / 'known_hosts'
        known.write_text('[127.0.0.1]:' + str(port) + ' ' + keys['host'].with_suffix('.pub').read_text())
        def request(client, tool, args):
            ssh = ['ssh', '-F', '/dev/null', '-T', '-p', str(port), '-i', str(keys[client]),
                   '-o', 'BatchMode=yes', '-o', 'IdentitiesOnly=yes', '-o', 'StrictHostKeyChecking=yes',
                   '-o', 'UserKnownHostsFile=' + str(known), getpass.getuser() + '@127.0.0.1',
                   'ignored-remote-command client bob']
            return subprocess.run([str(BIN / 'mcp'), 'request', 'tools/call', '--', *ssh],
                input=json.dumps({'name': tool, 'arguments': args}), capture_output=True, text=True, timeout=20)
        accepted = self.assert_ok(request('alice', 'submit_job', {'session': 'ssh', 'key': 'one', 'request': 'fixture'}))
        job = json.loads(json.loads(accepted.stdout)['content'][0]['text'])['id']
        self.assertEqual(self.status(job), 'ready')
        rejected = request('bob', 'get_job', {'job': job})
        self.assertNotEqual(rejected.returncode, 0)
        self.assertIn('job not found', rejected.stdout)
        self.assertNotEqual(request('untrusted', 'list_jobs', {}).returncode, 0)
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(job), 'done')

    def test_team_retry_uses_fresh_workspace(self):
        cfg = json.loads((self.root / 'deployment.json').read_text())
        cfg.update(kind='team', name='vendor-comparison-team')
        (self.root / 'deployment.json').write_text(json.dumps(cfg))
        (self.root / 'definition/team.lock.json').write_text('{"files": {}}')
        # This fixture changes its selected package before admitting any work.
        state_config = json.loads((self.state / 'config.json').read_text())
        state_config['deployment_sha256'] = hashlib.sha256((self.root / 'deployment.json').read_bytes()).hexdigest()
        self.runtime.write_json(self.state / 'config.json', state_config)
        job = self.submit(request='{"business_case":"fixture"}')
        self.behavior({job: {'exit': 2}})
        self.assert_ok(self.cli('work'))
        first = json.loads((self.root / 'observed' / job).read_text())['mount']
        self.assert_ok(self.cli('recover', 'alice', job, 'retry'))
        self.behavior({})
        self.assert_ok(self.cli('work'))
        second = json.loads((self.root / 'observed' / job).read_text())['mount']
        self.assertNotEqual(first, second)
        self.assertTrue(Path(first).is_dir())
        self.assertEqual(self.status(job), 'done')

    def test_limits_and_bad_files(self):
        for files in [{'../outside': ''}, {'a': '', 'a/b': ''}, {'request.txt/x': ''}, {'x': 'not-base64'}]:
            self.tool('alice', 'submit_job', {'session': 's', 'key': 'k', 'request': 'x', 'files': files}, error=True)
        cfg_path = self.state / 'config.json'
        cfg = json.loads(cfg_path.read_text())
        cfg['max_pending'] = 1
        cfg_path.write_text(json.dumps(cfg))
        job = self.submit()
        self.assertEqual(self.submit(), job)
        self.tool('alice', 'submit_job', {'session': 's', 'key': 'other', 'request': 'x'}, error=True)
        self.submit(client='bob')
        self.assertNotEqual(self.cli('add-client', 'bad', text='{"ASK_MODEL":"x","PATH":"evil"}').returncode, 0)

    def test_plain_json_commands_work_without_mcp_or_omnigent(self):
        (self.root / 'prefix/bin/mcpserve').unlink(missing_ok=True)
        (self.root / 'service-mcp').unlink()
        submitted = self.assert_ok(self.cli('submit', 'alice', text=json.dumps({
            'session': 'plain', 'key': 'plain', 'request': 'literal $(touch BAD); --flag'})))
        self.assertEqual(submitted.stderr, '')
        job = json.loads(submitted.stdout)['id']
        self.assertFalse((self.root / 'BAD').exists())
        definition = json.loads((self.state / 'tend/jobs' / job / 'definition.json').read_text())
        self.assertEqual(definition['argv'][:4], [str(self.root / 'sandbox-job'), '--state', str(self.state), 'run'])
        self.behavior({job: {'exit': 42}})
        self.assert_ok(self.cli('work'))
        # A successful query reports the job's failure without failing the query.
        queried = self.assert_ok(self.cli('get', 'alice', text=json.dumps({'job': job})))
        self.assertEqual(queried.stderr, '')
        self.assertEqual(json.loads(queried.stdout)['status'], 'failed')
        self.assertEqual(json.loads(queried.stdout)['last_finished_attempt']['exit'], 42)
        rejected = self.cli('get', 'bob', text=json.dumps({'job': job}))
        self.assertEqual(rejected.returncode, 1)
        self.assertEqual(rejected.stdout, '')
        self.assertIn('job not found', rejected.stderr)
        for payload in ('[]', '{broken', '{"state":"/other"}'):
            invalid = self.cli('list', 'alice', text=payload)
            self.assertEqual(invalid.returncode, 1)
            self.assertEqual(invalid.stdout, '')
        binding = self.state / 'config.json'
        cfg = json.loads(binding.read_text())
        cfg['deployment_sha256'] = 'changed'
        self.runtime.write_json(binding, cfg)
        unavailable = self.cli('list', 'alice', text='{}')
        self.assertEqual(unavailable.returncode, 2)
        self.assertEqual(unavailable.stdout, '')

    def test_sandbox_runner_has_no_queue_dependency(self):
        (self.root / 'prefix/bin/tend').unlink()
        (self.root / 'prefix/bin/mcpserve').unlink(missing_ok=True)
        for code in (42, 75):
            job = 'j' + str(code)[0] * 63
            self.behavior({job: {'exit': code}})
            payload = {'client': 'alice', 'session': 'direct', 'key': 'direct',
                       'request': 'fixture', 'files': {}}
            result = subprocess.run([str(self.root / 'sandbox-job'), '--state', str(self.state),
                'run', 'alice', job, 'direct', 'direct', '--attempt', 'operator-selected'],
                input=json.dumps(payload), capture_output=True, text=True, env=self.env, timeout=20)
            self.assertEqual(result.returncode, code, result.stderr)
            self.assertEqual(result.stdout, 'worker stream for ' + job + '\n')
            self.assertEqual(result.stderr, 'worker diagnostics\n')
        self.assertFalse((self.state / 'tend').exists())

    def test_selected_state_isolates_container_recovery(self):
        second_state = self.state.parent / 'second selected state'
        def second(*args, text=''):
            return subprocess.run([str(self.root / 'service'), '--state', str(second_state), *args],
                input=text, capture_output=True, text=True, env=self.env, timeout=20)
        self.assert_ok(second('init'))
        self.assert_ok(second('add-client', 'alice', text='{"ASK_MODEL":"openai/other"}'))
        job = self.submit()
        other = self.assert_ok(second('submit', 'alice', text=json.dumps({
            'session': 'session', 'key': 'key', 'request': 'fixture'})))
        self.assertEqual(json.loads(other.stdout)['id'], job)
        self.behavior({job: {'exit': 125}})
        self.assert_ok(self.cli('work'))
        first_container = next((self.root / 'active').iterdir())
        self.assert_ok(second('work'))
        self.assertEqual(len(list((self.root / 'active').iterdir())), 2)
        self.assert_ok(self.cli('recover', 'alice', job, 'fail'))
        self.assertFalse(first_container.exists())
        self.assertEqual(len(list((self.root / 'active').iterdir())), 1)
        self.assertEqual(json.loads(self.assert_ok(second('get', 'alice', text=json.dumps({'job': job}))).stdout)['status'], 'unknown')
        self.assert_ok(second('recover', 'alice', job, 'fail'))
        self.assertFalse(list((self.root / 'active').iterdir()))
        self.assertFalse((self.root / 'service-state').exists())

    @unittest.skipUnless((BIN / 'mcp').exists(), 'build MCP for executable connection check')
    def test_real_mcp_boundary_and_connection_lifetime(self):
        for executable in ('mcp', 'mcp-legacy'):
            def request(method, payload):
                return subprocess.run([str(BIN / executable), 'request', method, '--',
                    str(self.root / 'service-mcp'), '--state', str(self.state), 'alice'], input=json.dumps(payload),
                    capture_output=True, text=True, env=self.env, timeout=20)
            discovery = self.assert_ok(request('tools/list', {}))
            self.assertEqual(len(json.loads(discovery.stdout)['tools']), 5)
            submitted = self.assert_ok(request('tools/call', {'name': 'submit_job', 'arguments': {
                'session': 'network', 'key': executable, 'request': 'fixture'}}))
            job = json.loads(json.loads(submitted.stdout)['content'][0]['text'])['id']
            # That MCP connection has exited. The independent worker still runs.
            self.assert_ok(self.cli('work'))
            self.assertEqual(self.status(job), 'done')
            self.assert_ok(request('tools/call', {'name': 'get_job', 'arguments': {'job': job}}))

    @unittest.skipUnless((BIN / 'agent').exists(), 'build Agent for actual checkpoint argv/precheck')
    def test_real_agent_and_record(self):
        job = self.submit()
        self.behavior({job: {'real_agent': True}})
        self.assert_ok(self.cli('work'))
        self.assertEqual(self.status(job), 'done')
        evidence = self.state / 'runs' / job / 'sandbox/evidence'
        self.assertTrue(list(evidence.rglob('index.jsonl')))
        self.assert_ok(self.cli('check'))


if __name__ == '__main__':
    unittest.main()
