"""Exercise real finite subprocess I/O without models, Tend, or credentials."""
import importlib.util
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest import mock

SOURCE = Path(__file__).resolve().parents[1] / "examples" / "protocol.py"
SPEC = importlib.util.spec_from_file_location("weave_protocol", SOURCE)
protocol = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(protocol)


def python(script, *args):
    return [sys.executable, "-c", script, *map(str, args)]


class ProtocolTests(unittest.TestCase):
    def tearDown(self):
        protocol.DEADLINE = None
        protocol.INTERRUPTED = False

    def test_finite_stdin_and_default_eof(self):
        script = "import sys; sys.stdout.buffer.write(sys.stdin.buffer.read())"
        self.assertEqual(protocol.command(python(script)), b"")
        self.assertEqual(protocol.command(python(script), b""), b"")
        data = bytes(range(256)) * 8192
        self.assertEqual(protocol.command(python(script), data), data)

    def test_simultaneous_large_output_and_input_cannot_deadlock(self):
        script = ("import sys; sys.stdout.buffer.write(b'x' * 200000); sys.stdout.flush(); "
                  "data=sys.stdin.buffer.read(); sys.stdout.buffer.write(data)")
        self.assertEqual(protocol.command(python(script), b"y" * 200000),
                         b"x" * 200000 + b"y" * 200000)

    def test_early_closed_stdin_and_allowed_idle_status(self):
        script = "import os; os.close(0); os.write(1, b'idle'); raise SystemExit(1)"
        self.assertEqual(protocol.command(python(script), b"x" * 200000, allowed=(1,)), b"idle")

    def test_producer_failure_does_not_return_partial_answer(self):
        with self.assertRaisesRegex(RuntimeError, "exited 7"):
            protocol.command(python("print('partial'); raise SystemExit(7)"))

    def test_limit_accepts_exact_bytes_and_stops_noisy_producer(self):
        with mock.patch.object(protocol, "MAX_BYTES", 65536):
            self.assertEqual(len(protocol.command(python("import os; os.write(1,b'x'*65536)"))), 65536)
            with tempfile.TemporaryDirectory() as directory:
                cleanup = Path(directory) / "interrupted"
                script = """import os, pathlib, signal, sys
def stop(*args):
    pathlib.Path(sys.argv[1]).write_text('stopped')
    raise SystemExit(0)
signal.signal(signal.SIGINT, stop)
while True:
    os.write(1, b'x' * 65536)
"""
                # command has no disk spool: even an endless producer is stopped
                # as soon as it crosses the cap, and returns no truncated value.
                with mock.patch.object(protocol.tempfile, "TemporaryFile", side_effect=AssertionError("spooled")):
                    with self.assertRaisesRegex(ValueError, "exceeds 65536 bytes"):
                        protocol.command(python(script, cleanup))
                self.assertEqual(cleanup.read_text(), "stopped")

    def test_timeout_allows_sigint_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            cleanup = Path(directory) / "interrupted"
            script = """import pathlib, signal, sys, time
def stop(*args):
    pathlib.Path(sys.argv[1]).write_text('stopped')
    raise SystemExit(0)
signal.signal(signal.SIGINT, stop)
while True:
    time.sleep(.05)
"""
            with self.assertRaises(subprocess.TimeoutExpired):
                protocol.command(python(script, cleanup), timeout=0.3)
            self.assertEqual(cleanup.read_text(), "stopped")

    def test_timeout_kills_and_reaps_a_program_that_ignores_interrupt(self):
        with tempfile.TemporaryDirectory() as directory:
            pidfile = Path(directory) / "pid"
            script = ("import os,pathlib,signal,sys,time; signal.signal(signal.SIGINT,signal.SIG_IGN); "
                      "pathlib.Path(sys.argv[1]).write_text(str(os.getpid())); time.sleep(20)")
            with self.assertRaises(subprocess.TimeoutExpired):
                protocol.command(python(script, pidfile), timeout=0.3)
            with self.assertRaises(ProcessLookupError):
                os.kill(int(pidfile.read_text()), 0)

    def test_shared_deadline_bounds_an_inflight_command(self):
        protocol.DEADLINE = time.monotonic() + 0.15
        started = time.monotonic()
        with self.assertRaises(subprocess.TimeoutExpired):
            protocol.command(python("import signal,time; signal.signal(signal.SIGINT,lambda *_:exit(0)); time.sleep(20)"), timeout=10)
        self.assertLess(time.monotonic() - started, 1.5)

    def test_expired_deadline_and_interrupt_prevent_launch(self):
        with mock.patch.object(protocol.subprocess, "Popen", side_effect=AssertionError("launched")):
            protocol.DEADLINE = time.monotonic() - 1
            with self.assertRaises(subprocess.TimeoutExpired):
                protocol.command(["unused"])
            protocol.DEADLINE = None
            protocol.INTERRUPTED = True
            with self.assertRaises(InterruptedError):
                protocol.command(["unused"])

    def test_interrupt_is_observed_while_command_runs(self):
        timer = threading.Timer(0.15, lambda: setattr(protocol, "INTERRUPTED", True))
        timer.start()
        try:
            with self.assertRaises(InterruptedError):
                protocol.command(python("import signal,time; signal.signal(signal.SIGINT,lambda *_:exit(0)); time.sleep(20)"))
        finally:
            timer.cancel()
            timer.join()


if __name__ == "__main__":
    unittest.main()
