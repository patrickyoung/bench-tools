"""Executable application checks. Model, Docker and chat networks are fixtures."""
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
APP = ROOT / 'examples/matterbridge'
sys.path.insert(0, str(APP))
import bridge
import files
import provision


def candidate(name='helper', kind='worker'):
    contents = {'AGENTS.md': 'Write reply.md for the current request.\n',
                'README.md': 'Synthetic application fixture.\n',
                'bin/check': '#!/bin/sh\ntest -s reply.md\n'}
    return {'schema': 'bench.candidate/v1', 'kind': kind, 'name': name,
            'smoke_request': 'Write hello to reply.md.',
            'files': {name: {'base64': base64.b64encode(data.encode()).decode(),
                             'mode': 0o755 if name == 'bin/check' else 0o644}
                      for name, data in contents.items()}}


class Matterbridge(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='bench-matterbridge-test-')
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name).resolve()
        self.state = self.base / 'state'
        self.cfg = {'bench_source': str(ROOT),
                    'bench_ref': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip(),
                    'matterbridge': '/usr/bin/false', 'api_url': 'http://127.0.0.1:4242',
                    'api_token_file': str(self.base / 'api-token'),
                    'telegram_token_file': str(self.base / 'telegram-token'),
                    'provider_file': str(self.base / 'provider.json'),
                    'telegram_chat_id': '-100123456', 'builder_topic_id': 10,
                    'allowed_user_ids': ['123']}
        for name in ('api-token', 'telegram-token', 'provider.json'):
            path = self.base / name
            path.write_text('test-only')
            path.chmod(0o600)
        bridge.initialize(self.state, self.cfg)
        self.app = files.load(self.state / 'app.json')
        self.app['routes']['builder'] = {'channel': '-100123456/10',
            'binding': {'package': '/fixture', 'state': '/fixture-state'},
            'history': [], 'pending': None, 'draft': None}
        self.save()

    def save(self):
        files.save(self.state / 'app.json', self.app)

    def event(self, text='request', **extra):
        return dict({'gateway': 'builder', 'account': 'telegram.bench', 'channel': '-100123456/10',
                     'userid': '123', 'username': 'arbitrary display name', 'id': '1', 'text': text}, **extra)

    def admit(self, text, **extra):
        self.assertTrue(bridge.ingest(self.state, self.app, self.event(text, **extra)))

    def test_identity_route_edit_and_loop_rejection(self):
        for changed in ({'account': 'slack.other'}, {'userid': 'attacker'}, {'channel': '-100123456/99'},
                        {'gateway': 'missing'}, {'event': 'msg_delete'}, {'id': ''}):
            self.assertFalse(bridge.ingest(self.state, self.app, self.event(**changed)))
        self.admit('first')
        self.assertFalse(bridge.ingest(self.state, self.app, self.event('edited')))
        self.assertEqual(len(self.app['messages']), 1)

    def test_process_seals_input_before_submit_and_deduplicates_lost_response(self):
        self.admit('/new worker helper', id='1')
        self.admit('literal $(touch BAD); `touch BAD2`', id='2')
        observed = []
        def lost(binding, op, value):
            observed.append(value)
            persisted = files.load(self.state / 'app.json')
            self.assertIn(files.digest(value), [m.get('submission_sha256') for m in persisted['messages'].values()])
            self.assertEqual(value, files.load(self.state / 'submissions' / (value['key'] + '.json')))
            raise RuntimeError('response lost')
        with patch.object(bridge, 'service', side_effect=lost):
            with self.assertRaises(RuntimeError):
                bridge.process_messages(self.state, self.app)
        self.app = files.load(self.state / 'app.json')
        with patch.object(bridge, 'service', return_value={'id': 'job', 'status': 'ready'}) as call:
            bridge.process_messages(self.state, self.app)
        self.assertEqual(observed[0], call.call_args.args[2])
        self.assertEqual(self.app['routes']['builder']['pending'], files.digest(['telegram.bench', '-100123456/10', '2']))

    def test_persistent_build_revisions_use_selected_candidate_and_history(self):
        self.admit('/new worker helper', id='1')
        self.admit('first brief', id='2')
        with patch.object(bridge, 'service', return_value={'id': 'job', 'status': 'ready'}):
            bridge.process_messages(self.state, self.app)
        with patch.object(bridge, 'service', return_value={'id': 'job', 'status': 'done'}), \
             patch.object(bridge, 'result_file', side_effect=[b'Built the draft.', json.dumps(candidate()).encode()]):
            bridge.finish_jobs(self.state, self.app)
        self.save()
        self.app = files.load(self.state / 'app.json')  # Restart the application.
        self.admit('refine the earlier draft', id='3')
        with patch.object(bridge, 'service', return_value={'id': 'job2', 'status': 'ready'}) as call:
            bridge.process_messages(self.state, self.app)
        request = call.call_args.args[2]
        self.assertIn('first brief', request['request'])
        self.assertIn('Built the draft.', request['request'])
        self.assertEqual(json.loads(base64.b64decode(request['files']['previous-candidate.json'])), candidate())

    def test_unknown_fences_followups_but_keeps_status_available(self):
        self.app['routes']['builder']['draft'] = {'kind': 'worker', 'name': 'helper'}
        self.admit('first', id='1')
        with patch.object(bridge, 'service', return_value={'id': 'job', 'status': 'ready'}):
            bridge.process_messages(self.state, self.app)
        self.admit('second', id='2')
        self.admit('/status', id='3')
        with patch.object(bridge, 'service', return_value={'id': 'job', 'status': 'unknown'}) as calls:
            bridge.finish_jobs(self.state, self.app)
            bridge.process_messages(self.state, self.app)
        self.assertTrue(all(c.args[1] == 'get' for c in calls.call_args_list))
        self.assertEqual(sum(m['status'] == 'received' for m in self.app['messages'].values()), 1)

    def test_no_deployment_on_ordinary_builder_text_and_explicit_deploy_is_deduplicated(self):
        value = candidate()
        files.save(self.state / 'candidates/abc.json', value)
        self.app['drafts']['helper'] = {'candidate': 'abc', 'kind': 'worker'}
        self.app['routes']['builder']['draft'] = {'name': 'helper', 'kind': 'worker'}
        self.admit('/deploy helper')
        with patch.object(bridge, 'tend', return_value='job') as call:
            bridge.process_messages(self.state, self.app)
            bridge.process_messages(self.state, self.app)
        self.assertEqual(call.call_count, 1)
        self.assertEqual(len(self.app['deployments']), 1)
        args = call.call_args.args
        self.assertIn('submit', args)
        self.assertIn(str(APP / 'provision'), args)
        self.assertFalse(any('ready' == d['status'] for d in self.app['deployments'].values()))

    def test_config_keeps_each_topic_in_separate_gateway(self):
        self.app['routes']['deployment-123'] = dict(self.app['routes']['builder'], channel='-100123456/20')
        text = bridge.render(self.state, self.app)
        self.assertEqual(text.count('[[gateway]]'), 2)
        self.assertEqual(text.count('channel="-100123456/10"'), 1)
        self.assertEqual(text.count('channel="-100123456/20"'), 1)
        self.assertIn('EditDisable=true', text)

    def test_environment_credentials_are_selected_without_entering_config_or_argv(self):
        cfg = dict(self.cfg)
        for key in ('telegram_token_file', 'api_token_file', 'provider_file'):
            del cfg[key]
        cfg.update(telegram_token_env='TELEGRAM_BOT_TOKEN', api_token_env='MATTERBRIDGE_API_TOKEN',
                   provider_env=['ASK_MODEL', 'OPENAI_API_KEY'])
        bridge.validate_config(cfg)
        files.save(self.state / 'config.json', cfg)
        values = {'TELEGRAM_BOT_TOKEN': 'telegram-secret', 'MATTERBRIDGE_API_TOKEN': 'bridge-secret',
                  'ASK_MODEL': 'fixture', 'OPENAI_API_KEY': 'provider-secret', 'UNRELATED_SECRET': 'unrelated'}
        with patch.dict(os.environ, values):
            self.assertEqual(files.credential(cfg, 'telegram_token'), 'telegram-secret')
            self.assertEqual(files.provider(cfg), {'ASK_MODEL': 'fixture', 'OPENAI_API_KEY': 'provider-secret'})
            env = bridge.queue_env(self.state)
            rendered = bridge.render(self.state, self.app)
        self.assertNotIn('UNRELATED_SECRET', env)
        self.assertNotIn('MATTERBRIDGE_API_TOKEN', env)
        self.assertEqual(env['TELEGRAM_BOT_TOKEN'], 'telegram-secret')
        self.assertNotIn('telegram-secret', rendered)
        self.assertNotIn('provider-secret', json.dumps(cfg))
        cfg['provider_env'].append('PATH')
        with self.assertRaises(ValueError):
            bridge.validate_config(cfg)

    def test_saved_receive_batch_resumes_without_fetching_again(self):
        files.save(self.state / 'received.json', {'events': [self.event('retained')], 'offset': 0})
        with patch.object(bridge, 'http_json') as network:
            bridge.receive(self.state, self.app)
        network.assert_not_called()
        self.assertEqual(len(self.app['messages']), 1)
        self.assertEqual(files.load(self.state / 'received.json')['offset'], 1)

    def test_supervisor_refuses_ready_when_worker_exits_at_startup(self):
        class Process:
            def __init__(self, argv, **kwargs):
                self.code = 2 if str(argv[0]).endswith('service-worker') else None
            def poll(self):
                return self.code
            def terminate(self):
                self.code = 0
            def wait(self, timeout=None):
                return self.code
            def kill(self):
                self.code = -9
        with patch.object(bridge.subprocess, 'Popen', Process), \
             patch.object(bridge, 'http_json', return_value='OK'), \
             patch.object(bridge.signal, 'signal'):
            with self.assertRaisesRegex(RuntimeError, 'before readiness'):
                bridge.run(self.state)
        self.assertEqual(files.load(self.state / 'app.json')['outbox'], [])

    def test_successful_smoke_precedes_topic_and_bridge_readiness(self):
        key = 'b' * 64
        directory = self.state / 'deployments' / key
        (directory / 'package').mkdir(parents=True)
        (directory / 'queue').mkdir()
        files.save(directory / 'installed.json', {})
        files.save(self.state / 'candidates/abc.json', candidate())
        files.save(directory / 'request.json', {'candidate': 'abc', 'name': 'helper',
            'sha256': hashlib.sha256(files.read(self.state / 'candidates/abc.json')).hexdigest()})
        (self.base / 'provider.json').write_text('{"ASK_MODEL":"fixture"}')
        def create(*args):
            self.assertEqual(files.load(directory / 'smoke.json')['status'], 'done')
            return 99
        with patch.object(provision, 'service', side_effect=[{}, {'id': 'smoke', 'status': 'done'}]), \
             patch.object(provision, 'topic', side_effect=create):
            provision.provision(self.state, key)
        self.assertEqual(files.load(directory / 'receipt.json')['status'], 'awaiting-bridge')
        self.app['deployments'][key] = {'name': 'helper', 'status': 'queued'}
        with patch.object(bridge, 'tend', return_value='{"status":"done"}'):
            bridge.collect_deployments(self.state, self.app)
        self.assertEqual(self.app['deployments'][key]['status'], 'awaiting-bridge')
        self.assertEqual(self.app['outbox'], [])

    def test_topic_create_lost_response_is_not_repeated(self):
        with patch.object(files, 'http_json', side_effect=RuntimeError('lost')) as call:
            for _ in range(2):
                with self.assertRaises(RuntimeError):
                    files.topic(self.state, self.cfg, self.state / 'builder', 'Builder')
        self.assertEqual(call.call_count, 1)
        files.save(self.state / 'builder/topic.json', {'id': 123})
        self.assertEqual(files.topic(self.state, self.cfg, self.state / 'builder', 'Builder'), 123)

    def test_outbox_lost_response_is_fenced_and_http_contract(self):
        bridge.reply(self.app, 'builder', 'Hello')
        with patch.object(bridge, 'http_json', side_effect=RuntimeError('lost')) as call:
            with self.assertRaises(RuntimeError):
                bridge.flush(self.state, self.app)
            bridge.flush(self.state, self.app)
        self.assertEqual(call.call_count, 1)
        self.assertEqual(files.load(self.state / 'app.json')['outbox'][0]['status'], 'unknown')
        self.assertEqual(call.call_args.args[1]['gateway'], 'builder')
        self.assertNotIn('channel', call.call_args.args[1])

    def test_loopback_api_streams_and_no_secret_errors(self):
        received = []
        event = self.event('hello')
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass
            def do_GET(self):
                received.append(self.headers.get('Authorization'))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b'OK' if self.path == '/api/health' else json.dumps([event]).encode())
            def do_POST(self):
                payload = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                received.append(payload)
                self.send_response(200)
                self.end_headers()
                self.wfile.write(json.dumps(dict(payload, account='api.bench')).encode())
        server = HTTPServer(('127.0.0.1', 0), Handler)
        worker = threading.Thread(target=server.serve_forever)
        worker.start()
        self.addCleanup(server.server_close)
        self.addCleanup(worker.join)
        self.addCleanup(server.shutdown)
        self.cfg['api_url'] = 'http://127.0.0.1:' + str(server.server_port)
        files.save(self.state / 'config.json', self.cfg)
        self.assertEqual(files.http_json(self.cfg['api_url'] + '/api/health'), 'OK')
        bridge.receive(self.state, self.app)
        bridge.reply(self.app, 'builder', 'reply')
        bridge.flush(self.state, self.app)
        self.assertEqual(len(self.app['messages']), 1)
        self.assertIn('Bearer test-only', received)
        self.assertEqual(received[-1]['gateway'], 'builder')

    def test_candidate_export_rejects_paths_links_and_unsafe_modes(self):
        import importlib.machinery
        loader = importlib.machinery.SourceFileLoader('deploy_omnigent', str(ROOT / 'scripts/deploy-omnigent'))
        spec = importlib.util.spec_from_loader(loader.name, loader)
        packager = importlib.util.module_from_spec(spec)
        loader.exec_module(packager)
        for name, mode in [('../outside', 0o644), ('.env', 0o644), ('safe', 0o4755), ('evidence/log', 0o644)]:
            value = candidate()
            value['files'][name] = {'base64': 'eA==', 'mode': mode}
            files.save(self.base / 'candidate.json', value)
            with self.assertRaises(ValueError):
                packager.candidate_export(self.base / 'candidate.json', self.base / 'export', 'worker')
        value = candidate(kind='team')
        files.save(self.base / 'candidate.json', value)
        packager.candidate_export(self.base / 'candidate.json', self.base / 'good', 'team')
        self.assertTrue((self.base / 'good/team.lock.json').exists())
        self.assertTrue(os.access(self.base / 'good/expert/bin/check', os.X_OK))

    def test_candidate_team_packages_with_generic_agent_entry(self):
        files.save(self.base / 'candidate.json', candidate(kind='team'))
        result = subprocess.run([sys.executable, str(ROOT / 'scripts/deploy-omnigent'), 'team', 'helper',
            str(self.base / 'package'), '--ref', self.cfg['bench_ref'], '--candidate', str(self.base / 'candidate.json')],
            capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(files.load(self.base / 'package/deployment.json')['entry'], 'agent')

    def test_builder_packages_pinned_catalogs_and_container_only_hire_entry(self):
        package = self.base / 'builder-package'
        result = subprocess.run([sys.executable, str(ROOT / 'scripts/deploy-omnigent'), 'builder', 'builder',
            str(package), '--ref', self.cfg['bench_ref']], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        cfg = files.load(package / 'deployment.json')
        self.assertEqual(cfg['entry'], 'hire')
        self.assertTrue((package / 'definition/library/workers/README.md').exists())
        self.assertTrue((package / 'definition/library/teams/README.md').exists())
        result = subprocess.run([sys.executable, '-c',
            'import contracts, sandbox; from pathlib import Path; '
            'cfg = contracts.verify(Path.cwd()); '
            'assert sandbox.job_command(cfg, "fixture") == ["python3", "/sandbox-builder.py"]; '
            'team = {"kind":"team", "name":"custom", "entry":"agent"}; '
            'contracts.validate_submission("session", "key", "plain text", {}, team); '
            'assert sandbox.job_command(team, "fixture")[-1] == "/definition/expert"'],
            cwd=package, capture_output=True, text=True, env=dict(os.environ, PYTHONDONTWRITEBYTECODE='1'))
        self.assertEqual(result.returncode, 0, result.stderr)
        # Exercise the actual installer without uv/Omnigent on PATH. Builds and
        # Docker are process fixtures; this is not a native confinement test.
        fake_bin = self.base / 'install-bin'
        fake_bin.mkdir()
        go = fake_bin / 'go'
        go.write_text('#!' + sys.executable + '\nimport pathlib, sys\n'
            'target = pathlib.Path(sys.argv[sys.argv.index("-o")+1])\n'
            'target.symlink_to(pathlib.Path(' + repr(str(ROOT / '.build/bin')) + ') / target.name)\n')
        docker = fake_bin / 'docker'
        docker.write_text('#!' + sys.executable + '\nimport sys\n'
            'if sys.argv[1] == "image": print("sha256:" + "a" * 64)\n'
            'elif sys.argv[1] not in ("build", "run"): sys.exit(2)\n')
        for path in (go, docker):
            path.chmod(0o755)
        result = subprocess.run([str(package / 'install'), '--without-chat'],
            capture_output=True, text=True, env={'PATH': str(fake_bin) + os.pathsep + os.defpath,
                'HOME': str(self.base), 'DOCKER_HOST': 'unix:///fixture', 'PYTHONDONTWRITEBYTECODE': '1'})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((package / 'installed.json').exists())
        self.assertFalse((package / 'omnigent').exists())

    @unittest.skipUnless((ROOT / '.build/bin/tend').exists(), 'build Tend first')
    def test_real_tend_admits_exact_provision_command_and_retains_one_request(self):
        target = self.state / 'builder/package/prefix/bin'
        target.mkdir(parents=True)
        (target / 'tend').symlink_to(ROOT / '.build/bin/tend')
        files.save(self.state / 'candidates/abc.json', candidate())
        self.app['drafts']['helper'] = {'candidate': 'abc', 'kind': 'worker'}
        self.admit('/deploy helper')
        bridge.process_messages(self.state, self.app)
        key = next(iter(self.app['deployments']))
        row = json.loads(bridge.tend(self.state, 'show', key))
        self.assertEqual(row['argv'], [str(APP / 'provision'), '--state', str(self.state), key])
        self.assertEqual(row['status'], 'ready')
        self.assertEqual(len(bridge.tend(self.state, 'list').splitlines()), 1)
        bridge.tend(self.state, 'check')

    def test_provision_does_not_create_topic_on_smoke_failure(self):
        key = 'a' * 64
        directory = self.state / 'deployments' / key
        (directory / 'package').mkdir(parents=True)
        (directory / 'queue').mkdir()
        files.save(directory / 'installed.json', {})
        files.save(self.state / 'candidates/abc.json', candidate())
        files.save(directory / 'request.json', {'candidate': 'abc', 'name': 'helper',
            'sha256': hashlib.sha256(files.read(self.state / 'candidates/abc.json')).hexdigest()})
        (self.base / 'provider.json').write_text('{"ASK_MODEL":"fixture"}')
        with patch.object(provision, 'service', side_effect=[{}, {'id': 'smoke', 'status': 'failed'}]), \
             patch.object(provision, 'topic') as create:
            with self.assertRaises(RuntimeError):
                provision.provision(self.state, key)
        create.assert_not_called()
        self.assertFalse((directory / 'receipt.json').exists())

    @unittest.skipUnless((ROOT / '.build/bin/tend').exists(), 'build Tend first')
    def test_public_bridge_commands_build_and_revise_through_real_service_and_tend(self):
        # Reuse only the test harness. Production programs compose executables.
        from test_omnigent_service import OmnigentService
        fixture = OmnigentService('test_deduplication_and_client_ownership')
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        fixture.assert_ok(fixture.cli('add-client', 'chat', text='{"ASK_MODEL":"fixture"}'))
        cfg = json.loads((fixture.root / 'deployment.json').read_text())
        cfg['entry'] = 'hire'
        (fixture.root / 'deployment.json').write_text(json.dumps(cfg))
        state_cfg = json.loads((fixture.state / 'config.json').read_text())
        state_cfg['deployment_sha256'] = hashlib.sha256((fixture.root / 'deployment.json').read_bytes()).hexdigest()
        fixture.runtime.write_json(fixture.state / 'config.json', state_cfg)
        docker = fixture.root / 'fake-bin/docker'
        content = docker.read_text().replace("code = behavior.get('exit', 0)",
            "if '/sandbox-builder.py' in args:\n"
            "    (sandbox / 'work/candidate.json').write_text(" + repr(json.dumps(candidate())) + ")\n"
            "    (sandbox / 'work/reply.md').write_text('Synthetic Hire result, not a model evaluation.')\n"
            "code = behavior.get('exit', 0)")
        docker.write_text(content)
        self.app['routes']['builder']['binding'] = {'package': str(fixture.root), 'state': str(fixture.state)}
        self.save()
        def cli(operation, value=None):
            result = subprocess.run([str(APP / 'bridge'), '--state', str(self.state), operation],
                input=json.dumps(value) if value is not None else '', text=True,
                capture_output=True, env=fixture.env, timeout=30)
            self.assertEqual(result.returncode, 0, result.stderr)
            return result
        cli('ingest', self.event('/new worker helper', id='1'))
        cli('ingest', self.event('build a helper', id='2'))
        cli('tick')
        fixture.assert_ok(fixture.cli('work'))
        cli('tick')
        app = files.load(self.state / 'app.json')
        self.assertIn('helper', app['drafts'])
        self.assertIsNone(app['routes']['builder']['pending'])
        cli('ingest', self.event('revise the helper', id='3'))
        cli('tick')
        app = files.load(self.state / 'app.json')
        event = app['messages'][app['routes']['builder']['pending']]
        submission = files.load(self.state / 'submissions' / (event['key'] + '.json'))
        self.assertIn('Synthetic Hire result', submission['request'])
        self.assertIn('previous-candidate.json', submission['files'])
        jobs = json.loads(fixture.cli('list', 'chat', text='{}').stdout)['jobs']
        self.assertEqual(len(jobs), 2)
        fixture.assert_ok(fixture.cli('check'))


if __name__ == '__main__':
    unittest.main()
