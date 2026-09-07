"""Small file/CLI helpers for the examples; no task state lives here."""
import hashlib
import json
import os
from pathlib import Path
import selectors
import subprocess
import signal
import tempfile
import time

MAX_BYTES = 16 * 1024 * 1024
DEADLINE = None
INTERRUPTED = False


def encoded(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"),
                      ensure_ascii=False, allow_nan=False).encode() + b"\n"


def digest(data):
    return "sha256:" + hashlib.sha256(data).hexdigest()


def read(path):
    with open(path, "rb") as f:
        data = f.read(MAX_BYTES + 1)
    if len(data) > MAX_BYTES:
        raise ValueError(f"{path}: exceeds {MAX_BYTES} bytes")
    return data


def exact(path, data):
    """Install immutable input, or prove that an interrupted install agrees."""
    path = Path(path)
    if path.exists():
        if read(path) != data:
            raise ValueError(f"immutable input changed: {path}; use a new run directory")
        return
    replace(path, data)


def replace(path, data):
    path = Path(path)
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as f:
        tmp = f.name
        f.write(data)
        f.flush()
        os.fsync(f.fileno())
    try:
        os.replace(tmp, path)
        fd = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def _stop(child):
    """Give the program its interrupt cleanup; never accumulate cleanup output."""
    if child.stdin is not None and not child.stdin.closed:
        child.stdin.close()
    if child.poll() is not None:
        return
    try:
        child.send_signal(signal.SIGINT)
    except ProcessLookupError:
        return
    if not child.stdout.closed:
        os.set_blocking(child.stdout.fileno(), False)
    until = time.monotonic() + 2
    while child.poll() is None and time.monotonic() < until:
        if not child.stdout.closed:
            try:
                os.read(child.stdout.fileno(), 65536)
            except BlockingIOError:
                pass
        try:
            child.wait(timeout=0.02)
        except subprocess.TimeoutExpired:
            pass
    if child.poll() is None:
        child.kill()
    child.wait()


def command(argv, data=None, env=None, timeout=120, allowed=(0,)):
    """Finite stdin and bounded stdout, with cooperative timeout cancellation."""
    start = time.monotonic()
    deadlines = [d for d in (DEADLINE, None if timeout is None else start + timeout)
                 if d is not None]
    deadline = min(deadlines) if deadlines else None

    def check_stop():
        if INTERRUPTED:
            raise InterruptedError("command interrupted")
        if deadline is not None and time.monotonic() >= deadline:
            raise subprocess.TimeoutExpired(argv, max(0, deadline - start))

    check_stop()
    pending = memoryview(data) if data is not None else memoryview(b"")
    child = subprocess.Popen(argv, stdin=subprocess.PIPE if data is not None else subprocess.DEVNULL,
                             stdout=subprocess.PIPE, env=env, bufsize=0)
    body = bytearray()
    try:
        with selectors.DefaultSelector() as poller:
            os.set_blocking(child.stdout.fileno(), False)
            poller.register(child.stdout, selectors.EVENT_READ)
            if child.stdin is not None:
                if pending:
                    os.set_blocking(child.stdin.fileno(), False)
                    poller.register(child.stdin, selectors.EVENT_WRITE)
                else:
                    child.stdin.close()
            while poller.get_map() or child.poll() is None:
                check_stop()
                delay = 0.05 if deadline is None else max(0, min(0.05, deadline - time.monotonic()))
                for key, _ in poller.select(delay):
                    if key.fileobj is child.stdin:
                        try:
                            sent = os.write(key.fd, pending[:65536])
                            pending = pending[sent:]
                        except BlockingIOError:
                            continue
                        except BrokenPipeError:
                            pending = pending[len(pending):]
                        if not pending:
                            poller.unregister(child.stdin)
                            child.stdin.close()
                    else:
                        try:
                            chunk = os.read(key.fd, min(65536, MAX_BYTES - len(body) + 1))
                        except BlockingIOError:
                            continue
                        if not chunk:
                            poller.unregister(child.stdout)
                            child.stdout.close()
                        else:
                            body.extend(chunk)
                            if len(body) > MAX_BYTES:
                                raise ValueError(f"{argv[0]} output exceeds {MAX_BYTES} bytes")
            check_stop()
            if child.returncode not in allowed:
                raise RuntimeError(f"{argv[0]} {argv[1:2]} exited {child.returncode}")
            return bytes(body)
    except BaseException:
        _stop(child)
        raise
    finally:
        for stream in (child.stdin, child.stdout):
            if stream is not None and not stream.closed:
                stream.close()


def json_rows(data):
    return [json.loads(line) for line in data.splitlines()]
