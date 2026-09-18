"""Deployment component selection through the actual build command interface."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
BUILD = ROOT / 'examples/omnigent/build-tools.py'
RUNTIME = {'agent', 'ask', 'brief', 'ply', 'record', 'mcp', 'mcp-legacy',
           'mcpbox', 'mcpserve', 'tend', 'weave'}


class DeploymentBuild(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='bench-image-selection-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        # Stand in only for compilation. The real CLI reads the real component
        # manifest, selects modules and writes its output into an empty prefix.
        go = self.bin / 'go'
        go.write_text('#!' + sys.executable + '\n' +
                      'import pathlib, sys\n'
                      'assert pathlib.Path("go.mod").is_file()\n'
                      'pathlib.Path(sys.argv[sys.argv.index("-o")+1]).write_text("fixture")\n')
        go.chmod(0o755)
        self.prefix = self.root / 'prefix with spaces'

    def run_build(self, *args):
        return subprocess.run([sys.executable, str(BUILD), str(ROOT), str(self.prefix), *args],
                              env={'PATH': str(self.bin) + os.pathsep + os.defpath},
                              capture_output=True, text=True)

    def test_worker_team_and_candidate_omit_authoring(self):
        for cfg in ({'kind': 'worker'}, {'kind': 'team'},
                    {'kind': 'team', 'entry': 'agent'}):
            with self.subTest(cfg=cfg):
                config = self.root / 'deployment.json'
                config.write_text(json.dumps(cfg))
                result = self.run_build('--deployment', str(config))
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual({p.name for p in (self.prefix / 'bin').iterdir()}, RUNTIME)

    def test_explicit_builder_retains_hire(self):
        config = self.root / 'deployment.json'
        config.write_text(json.dumps({'kind': 'worker', 'entry': 'hire'}))
        result = self.run_build('--deployment', str(config))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual({p.name for p in (self.prefix / 'bin').iterdir()}, RUNTIME | {'hire'})

    def test_default_is_runtime(self):
        result = self.run_build()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual({p.name for p in (self.prefix / 'bin').iterdir()}, RUNTIME)

    def test_host_selection_remains_mcp_and_tend(self):
        result = self.run_build('mcp', 'tend')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual({p.name for p in (self.prefix / 'bin').iterdir()},
                         {'mcp', 'mcp-legacy', 'mcpbox', 'mcpserve', 'tend'})

    def test_unknown_component_fails_before_build(self):
        result = self.run_build('agent', 'missing')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('unknown components', result.stderr)
        self.assertFalse(self.prefix.exists())

    def test_conflicting_selection_fails_before_build(self):
        result = self.run_build('hire', '--deployment', 'unused.json')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('cannot be combined', result.stderr)
        self.assertFalse(self.prefix.exists())


if __name__ == '__main__':
    unittest.main()
