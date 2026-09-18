"""Exercise setup's public CLI, path selection, partial failure and preservation."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
TOOLS = ("hire", "agent", "ask", "brief", "ply", "cage", "record", "trail", "hone")


class SetupTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="bench-setup-test-")
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name).resolve()
        self.prefix = self.work / "runtime with spaces"
        self.state = self.work / "state with spaces"
        self.build = self.work / "build"
        self.env = dict(os.environ, HIRE_AGENT="a-missing-agent-selected-for-another-job")
        for name in TOOLS:
            self.package(name)

    def package(self, name, fail=False):
        directory = self.build / "tools" / name
        (directory / "bin").mkdir(parents=True, exist_ok=True)
        path = directory / "bin" / name
        script = '#!/bin/sh\nif [ "$1" = version ]; then echo "' + name + ' fixture"; exit 0; fi\n'
        if name == "hire":
            script += 'test "$1" = verify && "$HIRE_AGENT" fixture-check\n'
        elif name == "agent":
            script += 'test "$1" = fixture-check\n'
        elif name == "cage":
            script += ('echo "fixture confinement unavailable" >&2\nexit 125\n' if fail else
                       'test "$1" = check && echo "fixture confinement check"\n')
        else:
            script += 'echo "unexpected invocation" >&2\nexit 98\n'
        path.write_text(script)
        path.chmod(0o755)
        (directory / "package.json").write_text(json.dumps({
            "schema": 1, "name": name, "commands": [name],
            "source": {"repository_revision": "older-fixture-source", "files_sha256": "fixture"},
            "files": [{"path": "bin/" + name, "mode": 0o755,
                       "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}]}))

    def run_setup(self, *extra):
        return subprocess.run([sys.executable, str(ROOT / "scripts/setup"),
                               "--prefix", str(self.prefix), "--state-dir", str(self.state),
                               "--from-build", str(self.build), *extra],
                              env=self.env, text=True, capture_output=True, timeout=30)

    def test_fresh_install_repeat_and_fresh_shell_recover_companions(self):
        for _ in range(2):
            result = self.run_setup()
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        record = json.loads((self.state / "setup.json").read_text())
        self.assertEqual(record["prefix"], str(self.prefix))
        self.assertEqual(record["source"], str(ROOT))
        self.assertEqual(len(record["checks"]), 11)
        self.assertTrue(all(c["exit"] == 0 for c in record["checks"]))
        self.assertIn("older-fixture-source", (self.state / "BENCH-SETUP.md").read_text())
        result = subprocess.run(["sh", "-c", '. "$1"; command -v hire; command -v agent; printf "%s\\n" "$BENCH_SOURCE"',
                                 "probe", str(self.state / "env.sh")], text=True, capture_output=True,
                                env={"PATH": os.defpath})
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines(),
                         [str(self.prefix / "bin/hire"), str(self.prefix / "bin/agent"), str(ROOT)])

    def test_confinement_failure_is_nonzero_with_usable_partial_handoff(self):
        self.package("cage", fail=True)
        result = self.run_setup()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        record = json.loads((self.state / "setup.json").read_text())
        self.assertEqual(record["checks"][-1]["exit"], 125)
        self.assertIn("incomplete", (self.state / "BENCH-SETUP.md").read_text())
        self.assertTrue((self.prefix / "bin/agent").exists())

    def test_edited_handoff_and_environment_are_preserved(self):
        self.assertEqual(self.run_setup().returncode, 0)
        for name in ("BENCH-SETUP.md", "env.sh"):
            with self.subTest(name=name):
                path = self.state / name
                original = path.read_bytes()
                edited = original + b"\nOperator note\n"
                path.write_bytes(edited)
                result = self.run_setup()
                self.assertEqual(result.returncode, 1)
                self.assertIn("preserving existing or edited", result.stderr)
                self.assertEqual(path.read_bytes(), edited)
                path.write_bytes(original)

    def test_unmanaged_handoff_blocks_before_install(self):
        self.state.mkdir()
        (self.state / "BENCH-SETUP.md").write_text("existing operator setup")
        result = self.run_setup()
        self.assertEqual(result.returncode, 1)
        self.assertFalse(self.prefix.exists())

    def test_malformed_receipt_is_clean_error_before_install(self):
        self.state.mkdir()
        for value in ([], "unmanaged", {"producer": "bench-tools/scripts/setup", "files": []}):
            (self.state / "setup.json").write_text(json.dumps(value))
            result = self.run_setup()
            self.assertEqual(result.returncode, 1)
            self.assertNotIn("Traceback", result.stderr)
            self.assertFalse(self.prefix.exists())

    def test_existing_command_is_never_replaced(self):
        (self.prefix / "bin").mkdir(parents=True)
        command = self.prefix / "bin/ask"
        command.write_text("operator command")
        result = self.run_setup()
        self.assertEqual(result.returncode, 1)
        self.assertEqual(command.read_text(), "operator command")
        self.assertFalse((self.state / "BENCH-SETUP.md").exists())

    def test_receipt_cannot_silently_switch_runtime(self):
        self.assertEqual(self.run_setup().returncode, 0)
        other = self.work / "other runtime"
        result = self.run_setup("--prefix", str(other))
        self.assertEqual(result.returncode, 1)
        self.assertFalse(other.exists())
        self.assertIn("different source or prefix", result.stderr)


if __name__ == "__main__":
    unittest.main()
