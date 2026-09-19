"""Offline deployment contracts; Docker stand-ins never claim confinement."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
TEMPLATE = ROOT / 'examples/omnigent'
BIN = ROOT / '.build/bin'


class OmnigentDeployment(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='bench-omni-test-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve() / 'deployment with spaces'
        shutil.copytree(TEMPLATE, self.root)
        for name in ('runs', 'inputs', 'prefix/bin', 'definition/expert/bin', 'fake-bin', 'home', 'tmp'):
            (self.root / name).mkdir(parents=True, exist_ok=True)
        (self.root / 'definition/expert/AGENTS.md').write_text('Synthetic fixture.\n')
        check = self.root / 'definition/expert/bin/check'
        check.write_text('#!/bin/sh\nexit 0\n')
        check.chmod(0o755)
        cfg = {'kind': 'worker', 'name': 'tiny', 'adapter_sha256': {}}
        (self.root / 'deployment.json').write_text(json.dumps(cfg))
        (self.root / 'definition/worker.lock.json').write_text('{"files": {}}')
        (self.root / 'installed.json').write_text(json.dumps({
            'image': 'sha256:' + 'a' * 64, 'pass_env': ['ASK_MODEL', 'OPENAI_API_KEY'],
            'cpus': '2', 'memory': '4g'}))
        spec = importlib.util.spec_from_file_location('deployment_runtime', self.root / 'runtime.py')
        self.runtime = importlib.util.module_from_spec(spec)
        with patch.object(sys, 'path', [str(self.root), *sys.path]):
            spec.loader.exec_module(self.runtime)
        (self.root / 'manifest.json').write_text(json.dumps(self.runtime.manifest()))
        self.env = {'PATH': str(self.root / 'fake-bin') + os.pathsep + str(BIN) + os.pathsep + os.defpath,
                    'HOME': str(self.root / 'home'), 'ASK_MODEL': 'openai/offline-fixture',
                    'DOCKER_HOST': 'unix:///fixture', 'FIXTURE_ROOT': str(self.root),
                    'FIXTURE_BENCH_BIN': str(BIN), 'PYTHONDONTWRITEBYTECODE': '1'}
        self.env['TMPDIR'] = str((self.root / 'tmp').resolve())
        docker = self.root / 'fake-bin/docker'
        docker.write_text('#!' + sys.executable + '\n' + '''
import json, os, pathlib, subprocess, sys
root = pathlib.Path(os.environ['FIXTURE_ROOT'])
args = sys.argv[1:]
(root / 'docker-argv.json').write_text(json.dumps(args))
mount = args[args.index('--mount') + 1]
job = mount.removeprefix('type=bind,src=').removesuffix(',dst=/job')
index = next(i for i, v in enumerate(args) if v.startswith('sha256:'))
command = args[index + 1:]
if os.environ.get('FIXTURE_REAL_AGENT'):
    command = [v.replace('/definition', str(root / 'definition')).replace('/job', job)
               for v in command]
    command[:2] = [os.environ['FIXTURE_BENCH_BIN'] + '/agent', 'run', '-no-cage']
    sys.exit(subprocess.call(command))
print('fixture output')
print('fixture diagnostics', file=sys.stderr)
sys.exit(int(os.environ.get('FIXTURE_EXIT', '0')))
''')
        docker.chmod(0o755)
        for name in ('mcpserve',):
            if (BIN / name).exists():
                (self.root / 'prefix/bin' / name).symlink_to(BIN / name)

    def call(self, *args, text='goal', extra=None):
        return subprocess.run([sys.executable, str(self.root / 'runtime.py'), *args],
                              input=text, text=True, capture_output=True,
                              env=dict(self.env, **(extra or {})))

    def test_streams_status_and_duplicate_refusal(self):
        result = self.call('run', 'job-1', extra={'FIXTURE_EXIT': '75'})
        self.assertEqual(result.returncode, 75, result.stderr)
        self.assertEqual(result.stdout, 'fixture output\n')
        self.assertIn('fixture diagnostics', result.stderr)
        self.assertEqual(self.runtime.read_job('job-1')['exit_code'], 75)
        repeated = self.call('run', 'job-1')
        self.assertNotEqual(repeated.returncode, 0)
        self.assertEqual(self.runtime.read_job('job-1')['exit_code'], 75)
        argv = json.loads((self.root / 'docker-argv.json').read_text())
        self.assertIn('--read-only', argv)
        self.assertIn('--cap-drop=ALL', argv)
        self.assertIn('--security-opt=no-new-privileges', argv)
        self.assertNotIn('-no-cage', argv)  # The image wrapper owns this explicit choice.
        self.assertEqual(argv[-1], '/definition/expert')
        self.assertEqual(argv.count('--mount'), 1)
        self.assertIn('/sandbox,dst=/job', argv[argv.index('--mount') + 1])

    def test_literal_goal_and_selected_credentials(self):
        request = 'literal $(touch BAD); `touch BAD2`\n--flag'
        result = self.call('run', 'literal', text=request,
                           extra={'OPENAI_API_KEY': 'synthetic-secret', 'PRIVATE_UNRELATED': 'excluded'})
        self.assertEqual(result.returncode, 0, result.stderr)
        argv = json.loads((self.root / 'docker-argv.json').read_text())
        self.assertEqual((self.root / 'runs/literal/sandbox/input/request.txt').read_text(), request)
        self.assertNotIn(request, argv)
        self.assertIn('OPENAI_API_KEY', argv)
        self.assertNotIn('synthetic-secret', json.dumps(argv))
        self.assertNotIn('PRIVATE_UNRELATED', argv)

    def test_missing_runtime_fails_without_host_execution(self):
        (self.root / 'fake-bin/docker').unlink()
        result = self.call('run', 'missing')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.runtime.read_job('missing')['state'], 'unknown')

    def test_input_and_job_escape_refused(self):
        (self.root / 'inputs/escape').symlink_to(self.root / 'deployment.json')
        self.assertNotEqual(self.call('run', 'escaped').returncode, 0)
        self.assertNotEqual(self.call('run', '../outside').returncode, 0)
        self.assertFalse((self.root / 'outside').exists())

    def test_output_symlink_refused(self):
        self.assertEqual(self.call('run', 'output').returncode, 0)
        output = self.root / 'runs/output/stdout.txt'
        output.unlink()
        output.symlink_to(self.root / 'deployment.json')
        with self.assertRaises(OSError):
            self.runtime.read_job('output')

    def test_remote_docker_refused(self):
        result = self.call('run', 'remote', extra={'DOCKER_HOST': 'ssh://elsewhere'})
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / 'docker-argv.json').exists())

    def test_team_entry_and_materials(self):
        cfg = json.loads((self.root / 'deployment.json').read_text())
        cfg.update(kind='team', name='vendor-comparison-team')
        (self.root / 'deployment.json').write_text(json.dumps(cfg))
        (self.root / 'definition/team.lock.json').write_text('{"files": {}}')
        (self.root / 'inputs/material.txt').write_text('selected evidence')
        result = self.call('run', 'comparison', text='{"business_case":"fixture"}')
        self.assertEqual(result.returncode, 0, result.stderr)
        argv = json.loads((self.root / 'docker-argv.json').read_text())
        self.assertEqual(argv[-3:], ['/definition/expert/bin/compare-team',
                                    '/job/input/request.txt', '/job/team'])
        self.assertEqual((self.root / 'runs/comparison/sandbox/input/material.txt').read_text(),
                         'selected evidence')

    def test_chat_spec_no_secrets_no_retries(self):
        with patch.dict(os.environ, self.env, clear=True), patch.object(self.runtime.os, 'execve') as execute:
            self.runtime.chat([])
        config = json.loads((self.root / 'chat-agent/config.yaml').read_text())
        self.assertEqual(config['tools']['retry']['max_retries'], 0)
        self.assertNotIn('os_env', config)
        self.assertEqual(execute.call_args.args[1][1], 'run')

    def test_chat_flags_reach_omnigent(self):
        cli = self.root / 'omnigent/bin/omnigent'
        cli.parent.mkdir(parents=True)
        cli.write_text('#!/bin/sh\nprintf "%s\\n" "$@"\n')
        cli.chmod(0o755)
        result = self.call('chat', '--help')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines(), ['run', str(self.root / 'chat-agent'), '--help'])
        self.assertTrue((self.root / 'chat-agent/config.yaml').exists())

    @unittest.skipUnless(os.environ.get('BENCH_OMNIGENT_PYTHON'),
                         'set BENCH_OMNIGENT_PYTHON to a Python with Omnigent 0.14.0')
    def test_upstream_omnigent_connection(self):
        with patch.dict(os.environ, self.env, clear=True), patch.object(self.runtime.os, 'execve'):
            self.runtime.chat([])
        code = '''
import asyncio,json,os
from pathlib import Path
from omnigent.spec.parser import parse
from omnigent.tools.mcp import McpServerConnection
async def check():
    root=Path(os.environ['FIXTURE_ROOT'])
    spec=parse(root/'chat-agent')
    connection=McpServerConnection(spec.mcp_servers[0],cwd=root)
    try:
        assert {t.name for t in await connection.connect()} == {'run_job','read_job'}
        result=await connection.call_tool('run_job',{'job':'upstream','request':'fixture'})
        assert json.loads(result)['exit_code']==0, result
        result=await connection.call_tool('read_job',{'job':'upstream'})
        assert json.loads(result)['state']=='finished', result
    finally:
        await connection.close()
asyncio.run(check())
'''
        result = subprocess.run([os.environ['BENCH_OMNIGENT_PYTHON'], '-c', code],
                                env=self.env, capture_output=True, text=True, timeout=60)
        self.assertEqual(result.returncode, 0, result.stderr)

    @unittest.skipUnless((BIN / 'mcpserve').exists() and (BIN / 'mcp-legacy').exists(),
                         'build the mcp component to test both public MCP lifecycles')
    def test_real_mcp_discovery_execution_and_failure(self):
        for client in ('mcp', 'mcp-legacy'):
            server = [str(BIN / 'mcpserve'), '-allow-legacy', str(self.root / 'manifest.json'),
                      '--', str(self.root / 'dispatch')]
            def request(method, payload):
                return subprocess.run([str(BIN / client), 'request', method, '--', *server],
                    input=json.dumps(payload), capture_output=True, text=True, env=self.env)
            listed = request('tools/list', {})
            self.assertEqual(listed.returncode, 0, listed.stderr)
            self.assertEqual({t['name'] for t in json.loads(listed.stdout)['tools']}, {'run_job', 'read_job'})
            payload = {'name': 'run_job', 'arguments': {'job': client, 'request': 'fixture'}}
            ran = request('tools/call', payload)
            self.assertEqual(ran.returncode, 0, ran.stderr)
            self.assertFalse(json.loads(ran.stdout).get('isError', False))
            rejected = request('tools/call', payload)
            self.assertEqual(rejected.returncode, 1, rejected.stderr)
            self.assertTrue(json.loads(rejected.stdout)['isError'])
            bad = request('tools/call', {'name': 'run_job', 'arguments': {'job': '../escape', 'request': 'x'}})
            self.assertNotEqual(bad.returncode, 0)

    @unittest.skipUnless((BIN / 'agent').exists(), 'build Agent and companions for the precheck fixture')
    def test_actual_agent_precheck_through_adapter(self):
        result = self.call('run', 'precheck', extra={'FIXTURE_REAL_AGENT': '1'})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.runtime.read_job('precheck')['exit_code'], 0)
        evidence = self.root / 'runs/precheck/sandbox/evidence'
        self.assertTrue(list(evidence.rglob('index.jsonl')))

    def test_package_real_committed_worker_and_team(self):
        pin = subprocess.check_output(['git', '-C', str(ROOT), 'rev-parse', 'HEAD'], text=True).strip()
        for kind, name in [('worker', 'product-owner'), ('team', 'vendor-comparison-team')]:
            dest = Path(self.temp.name) / kind
            result = subprocess.run([sys.executable, str(ROOT / 'scripts/deploy-omnigent'),
                kind, name, str(dest), '--ref', pin, '--allow-experimental'],
                capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(json.loads((dest / 'deployment.json').read_text())['bench_commit'], pin)
            self.assertTrue((dest / 'definition' / (kind + '.lock.json')).exists())
            self.assertFalse((dest / 'runs').exists())
            if kind == 'team':
                self.assertTrue((dest / 'definition/expert/agents/analyst/AGENTS.md').exists())


if __name__ == '__main__':
    unittest.main()
