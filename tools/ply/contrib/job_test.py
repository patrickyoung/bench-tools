"""Executable-seam tests: python3 contrib/job_test.py."""

import hashlib
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
import tempfile
import time
import unittest


PROGRAM = Path(__file__).with_name("job")


class JobTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name) / "jobs"
        self.env = dict(os.environ, JOB_DIR=str(self.root))
        self.handles = []

    def tearDown(self):
        for handle in self.handles:
            self.call("cancel", handle, "--timeout", "6", timeout=9)
        self.temp.cleanup()

    def call(self, *args, timeout=10):
        return subprocess.run([sys.executable, str(PROGRAM), *args], env=self.env,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                              timeout=timeout)

    def launch(self, *command, timeout="30"):
        result = self.call("start", "--timeout", timeout, "--", *command)
        self.assertEqual(result.returncode, 0, result.stderr)
        handle = result.stdout.decode().strip()
        self.assertRegex(handle, r"^[0-9a-f]{32}$")
        self.handles.append(handle)
        return handle

    def current(self, handle):
        result = self.call("status", handle)
        return result.returncode, json.loads(result.stdout)

    def await_file(self, path):
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            if path.exists() and path.read_text().strip():
                return path.read_text().strip()
            time.sleep(0.01)
        self.fail(f"child never wrote {path}")

    def assert_stopped(self, pid):
        deadline = time.monotonic() + 2
        while time.monotonic() < deadline:
            try:
                os.kill(pid, 0)
            except ProcessLookupError:
                return
            state = subprocess.run(["ps", "-o", "stat=", "-p", str(pid)],
                                   stdout=subprocess.PIPE, timeout=2).stdout.strip()
            if state.startswith(b"Z"):
                return
            time.sleep(0.02)
        self.fail(f"owned process {pid} remained alive")

    def test_completed_binary_streams_are_retained_and_verified(self):
        code = "import sys; sys.stdout.buffer.write(b'out\\xff\\n'); sys.stderr.buffer.write(b'err\\x80\\n')"
        handle = self.launch(sys.executable, "-c", code)
        result = self.call("wait", handle, "--timeout", "5")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, b"out\xff\n")
        self.assertIn(b"err\x80\n", result.stderr)
        status_code, current = self.current(handle)
        self.assertEqual(status_code, 0)
        self.assertEqual(current["state"], "succeeded")
        self.assertEqual(current["returncode"], 0)
        self.assertEqual(current["stdout"], {"bytes": 5, "sha256": hashlib.sha256(b"out\xff\n").hexdigest()})
        self.assertEqual(self.call("wait", handle).stdout, result.stdout)
        for path in [self.root, self.root / handle, *list((self.root / handle).iterdir())]:
            self.assertEqual(path.stat().st_mode & 0o077, 0, path)

    def test_failure_has_the_actual_command_exit_code(self):
        handle = self.launch(sys.executable, "-c", "print('rejected'); raise SystemExit(7)")
        result = self.call("wait", handle, "--timeout", "5")
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertEqual(result.stdout, b"rejected\n")
        self.assertEqual(self.current(handle)[1]["exit_code"], 7)

    def test_missing_executable_records_failure(self):
        handle = self.launch(str(Path(self.temp.name) / "missing"))
        result = self.call("wait", handle, "--timeout", "5")
        self.assertEqual(result.returncode, 127, result.stderr)
        self.assertEqual(self.current(handle)[1]["state"], "failed")

    def test_wait_is_bounded_and_cancel_stops_an_ignoring_command(self):
        handle = self.launch("/bin/sh", "-c", "trap '' INT TERM; exec sleep 30")
        _, current = self.current(handle)
        self.assertEqual(current["state"], "running")
        started = time.monotonic()
        result = self.call("wait", handle, "--timeout", "0.05")
        self.assertLess(time.monotonic() - started, 2)
        self.assertEqual(result.returncode, 2)
        self.assertEqual(result.stdout, b"")
        self.assertIn(b'"state": "running"', result.stderr)
        result = self.call("cancel", handle, "--timeout", "6")
        self.assertEqual(result.returncode, 130, result.stderr)
        terminal = self.current(handle)[1]
        self.assertEqual(terminal["state"], "cancelled")
        self.assertEqual(terminal["returncode"], -signal.SIGKILL)
        self.assert_stopped(current["command_pid"])

    def test_cancel_finishes_group_cleanup_after_leader_exits_zero(self):
        pidfile = Path(self.temp.name) / "descendant.pid"
        child = f"trap '' INT TERM; echo $$ > '{pidfile}'; exec sleep 30"
        script = "trap 'exit 0' INT; /bin/sh -c " + "'" + child.replace("'", "'\\''") + "' >/dev/null 2>&1 & wait"
        handle = self.launch("/bin/sh", "-c", script)
        pid = int(self.await_file(pidfile))
        try:
            result = self.call("cancel", handle, "--timeout", "6")
            self.assertEqual(result.returncode, 130, result.stderr)
            self.assertEqual(self.current(handle)[1]["returncode"], 0)
            self.assert_stopped(pid)
        finally:
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass

    def test_command_timeout_is_a_distinct_terminal_outcome(self):
        handle = self.launch("/bin/sh", "-c", "trap '' INT; exec sleep 30", timeout="0.2")
        result = self.call("wait", handle, "--timeout", "5")
        self.assertEqual(result.returncode, 124, result.stderr)
        self.assertEqual(self.current(handle)[1]["state"], "timed_out")

    def test_lost_supervisor_is_unknown_and_never_restarts_the_command(self):
        marker = Path(self.temp.name) / "runs"
        handle = self.launch("/bin/sh", "-c", f"echo run >> '{marker}'; exec sleep 30")
        _, current = self.current(handle)
        self.assertEqual(current["state"], "running")
        self.await_file(marker)
        os.kill(current["supervisor_pid"], signal.SIGKILL)
        try:
            deadline = time.monotonic() + 3
            while time.monotonic() < deadline:
                code, state = self.current(handle)
                if state["state"] == "unknown":
                    break
                time.sleep(0.02)
            self.assertEqual(code, 1)
            self.assertEqual(state["state"], "unknown")
            self.assertIn("effects may exist", state["detail"])
            self.assertEqual(self.call("wait", handle).returncode, 1)
            self.assertEqual(self.call("cancel", handle).returncode, 1)
            restarted = self.call("_supervise", str(self.root), handle)
            self.assertEqual(restarted.returncode, 1)
            self.assertEqual(marker.read_text(), "run\n")
        finally:
            # These are the just-observed processes owned by this test;
            # the production client never kills a PID from stored metadata.
            os.killpg(current["command_pid"], signal.SIGKILL)
            (Path("/tmp") / f"ply-job-{os.getuid()}" / (handle + ".sock")).unlink(missing_ok=True)

    def test_unauthenticated_cancel_cannot_control_a_job(self):
        handle = self.launch("/bin/sh", "-c", "exec sleep 30")
        path = Path("/tmp") / f"ply-job-{os.getuid()}" / (handle + ".sock")
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
            connection.settimeout(2)
            connection.connect(str(path))
            body = {"handle": handle, "op": "cancel", "nonce": "bad"}
            connection.sendall(json.dumps({"body": body, "mac": "invalid"}).encode() + b"\n")
            self.assertEqual(connection.recv(1), b"")
        self.assertEqual(self.current(handle)[1]["state"], "running")

    def test_malformed_control_messages_do_not_stop_the_supervisor(self):
        handle = self.launch("/bin/sh", "-c", "exec sleep 30")
        path = Path("/tmp") / f"ply-job-{os.getuid()}" / (handle + ".sock")
        for envelope in ([], {"body": [], "mac": "bad"}, {"body": {}, "mac": 42}):
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
                connection.settimeout(2)
                connection.connect(str(path))
                connection.sendall(json.dumps(envelope).encode() + b"\n")
                self.assertEqual(connection.recv(1), b"")
        self.assertEqual(self.current(handle)[1]["state"], "running")

    def test_a_modified_terminal_status_is_rejected(self):
        handle = self.launch("/bin/echo", "recorded")
        self.assertEqual(self.call("wait", handle, "--timeout", "5").returncode, 0)
        path = self.root / handle / "result.json"
        record = json.loads(path.read_bytes())
        record["body"]["exit_code"] = 7
        path.write_text(json.dumps(record))
        result = self.call("wait", handle)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, b"")
        self.assertIn(b"terminal record authentication", result.stderr)

    def test_modified_completed_output_is_rejected_before_printing(self):
        handle = self.launch("/bin/echo", "recorded")
        self.assertEqual(self.call("wait", handle, "--timeout", "5").returncode, 0)
        (self.root / handle / "stdout").write_bytes(b"changed")
        result = self.call("wait", handle)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, b"")
        self.assertIn(b"output digest or byte count", result.stderr)


if __name__ == "__main__":
    unittest.main()
