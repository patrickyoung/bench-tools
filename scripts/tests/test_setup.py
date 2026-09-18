"""Exercise setup's public CLI, path selection, partial failure and preservation."""
from contextlib import contextmanager
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shlex
import signal
import subprocess
import sys
import tempfile
import time
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

    def setup_argv(self, *extra):
        return [sys.executable, str(ROOT / "scripts/setup"),
                "--prefix", str(self.prefix), "--state-dir", str(self.state),
                "--from-build", str(self.build), *extra]

    def run_setup(self, *extra):
        return subprocess.run(self.setup_argv(*extra),
                              env=self.env, text=True, capture_output=True, timeout=30)

    @contextmanager
    def paused_setup(self):
        """Pause real CLI verification after install, with a bounded ready signal."""
        gate = Path(tempfile.mkdtemp(prefix="verification-gate-", dir=self.work))
        ready, release = gate / "ready", gate / "release"
        self.package("hire")
        path = self.build / "tools/hire/bin/hire"
        wait = ("from pathlib import Path\nimport time\n"
                f"Path({str(ready)!r}).touch()\n"
                "deadline = time.monotonic() + 20\n"
                f"while not Path({str(release)!r}).exists():\n"
                "    if time.monotonic() >= deadline: raise SystemExit('verification gate timed out')\n"
                "    time.sleep(0.01)\n")
        pause = ('if [ "$1" = verify ]; then\n' +
                 shlex.join([sys.executable, "-c", wait]) + ' || exit $?\nfi\n')
        path.write_text(path.read_text().replace('test "$1" = verify', pause + 'test "$1" = verify'))
        receipt = path.parent.parent / "package.json"
        package = json.loads(receipt.read_text())
        package["files"][0]["sha256"] = hashlib.sha256(path.read_bytes()).hexdigest()
        receipt.write_text(json.dumps(package))
        process = subprocess.Popen(self.setup_argv(), env=self.env, text=True,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                   start_new_session=True)
        try:
            deadline = time.monotonic() + 10
            while not ready.exists():
                if process.poll() is not None:
                    stdout, stderr = process.communicate()
                    self.fail("setup exited before verification gate: " + stdout + stderr)
                self.assertLess(time.monotonic(), deadline, "setup did not reach verification gate")
                time.sleep(0.01)
            yield process, release
        finally:
            release.touch()
            try:
                process.communicate(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.communicate(timeout=5)

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

    def test_handoff_created_during_setup_is_preserved(self):
        with self.paused_setup() as (process, release):
            self.state.mkdir(parents=True, exist_ok=True)
            handoff = self.state / "BENCH-SETUP.md"
            notes = b"Operator notes created while installation was running\n"
            handoff.write_bytes(notes)
            release.touch()
            stdout, stderr = process.communicate(timeout=10)
        self.assertEqual(process.returncode, 1, stdout + stderr)
        self.assertEqual(handoff.read_bytes(), notes)
        self.assertFalse((self.state / "env.sh").exists())
        self.assertFalse((self.state / "setup.json").exists())

    def test_handoff_and_environment_edits_during_setup_are_preserved(self):
        self.assertEqual(self.run_setup().returncode, 0)
        receipt = (self.state / "setup.json").read_bytes()
        for name in ("BENCH-SETUP.md", "env.sh"):
            with self.subTest(name=name):
                path = self.state / name
                original = path.read_bytes()
                edited = original + b"\nOperator edit during installation\n"
                with self.paused_setup() as (process, release):
                    path.write_bytes(edited)
                    release.touch()
                    stdout, stderr = process.communicate(timeout=10)
                self.assertEqual(process.returncode, 1, stdout + stderr)
                self.assertEqual(path.read_bytes(), edited)
                self.assertEqual((self.state / "setup.json").read_bytes(), receipt)
                path.write_bytes(original)

    def test_receipt_edits_during_setup_are_preserved(self):
        self.assertEqual(self.run_setup().returncode, 0)
        files = {name: (self.state / name).read_bytes() for name in ("BENCH-SETUP.md", "env.sh")}
        receipt = self.state / "setup.json"
        edited = receipt.read_bytes() + b"\n"
        with self.paused_setup() as (process, release):
            receipt.write_bytes(edited)
            release.touch()
            stdout, stderr = process.communicate(timeout=10)
        self.assertEqual(process.returncode, 1, stdout + stderr)
        self.assertEqual(receipt.read_bytes(), edited)
        for name, data in files.items():
            self.assertEqual((self.state / name).read_bytes(), data)

    def test_concurrent_setup_cannot_switch_a_shared_state_directory(self):
        other = self.work / "other concurrent runtime"
        with self.paused_setup() as (first, release):
            with (self.state / ".setup.lock").open("rb") as lock:
                with self.assertRaises(BlockingIOError):
                    fcntl.flock(lock.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
            second = subprocess.Popen(self.setup_argv("--prefix", str(other)),
                                      env=self.env, text=True, stdout=subprocess.PIPE,
                                      stderr=subprocess.PIPE)
            try:
                release.touch()
                first_stdout, first_stderr = first.communicate(timeout=10)
                second_stdout, second_stderr = second.communicate(timeout=10)
            finally:
                if second.poll() is None:
                    second.kill()
                    second.communicate(timeout=5)
        self.assertEqual(first.returncode, 0, first_stdout + first_stderr)
        self.assertEqual(second.returncode, 1, second_stdout + second_stderr)
        self.assertFalse(other.exists())
        self.assertEqual(json.loads((self.state / "setup.json").read_text())["prefix"], str(self.prefix))
        # The released state lock must permit a normal same-runtime refresh.
        self.assertEqual(self.run_setup().returncode, 0)

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
