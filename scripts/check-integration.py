#!/usr/bin/env python3
"""Offline executable contracts for the imported tools; no shared runtime.

Build commands separately, then pass their directory with --bin-dir. All data
and helper programs live in a temporary directory. The only model transport is
a loopback HTTP fixture. May's operating-system-user store cannot be redirected
with HOME: its parked adapter is therefore a strict fixture here, while May's
own isolated acceptance suite remains part of the independent component checks.
"""

import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer


ROOT = Path(__file__).resolve().parents[1]
REQUIRED = ("ask", "brief", "ply", "hone", "context", "cite", "action", "may", "trail", "tend", "weave")


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def invoke(argv, *, cwd, env, data=b"", code=0, timeout=180):
    command = [str(arg) for arg in argv]
    with subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE, cwd=cwd, env=env, start_new_session=True) as process:
        try:
            stdout, stderr = process.communicate(data, timeout=timeout)
        except subprocess.TimeoutExpired:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.communicate()
            raise
        result = subprocess.CompletedProcess(command, process.returncode, stdout, stderr)
    require(result.returncode == code,
            f"{Path(str(argv[0])).name} {list(map(str, argv[1:]))}: exit {result.returncode}, expected {code}\n"
            + result.stdout.decode(errors="replace") + result.stderr.decode(errors="replace"))
    return result


def write_program(path, body):
    path.write_text(f"#!{sys.executable}\n" + body, encoding="utf-8")
    path.chmod(0o700)
    return path


def go_contract(component, test, env):
    result = invoke(["go", "test", "-count=1", "-json", "-run", f"^{test}$", "."],
                    cwd=ROOT / "tools" / component, env=env)
    events = [json.loads(line) for line in result.stdout.splitlines()]
    require(any(e.get("Test") == test and e.get("Action") == "pass" for e in events),
            f"{component}/{test}: required test did not pass")
    require(not any(e.get("Action") == "skip" for e in events),
            f"{component}/{test}: executable contract silently skipped")
    print(f"ok {component}: {test} (required, no skips)", flush=True)


def check_context_cite(bins, work, env):
    row = {"kind": "context", "version": 1, "source": "fixture", "type": "document",
           "id": "hours", "title": "Offline fixture", "retrieved_at": "2026-01-01T00:00:00Z",
           "content": {"text": "The fictional library opens at 09:00."},
           "citation": {"locator": "hours", "url": "https://example.test/hours"},
           "extra": {"preserve": [1, 2]}}
    raw = json.dumps(row).encode() + b"\n"
    result = invoke([bins / "context", "merge"], cwd=work, env=env, data=raw)
    require(result.stderr == b"", "Context mixed diagnostics into a successful fixture merge")
    normalized = json.loads(result.stdout)
    require(normalized["extra"] == row["extra"], "Context lost an unknown structured field")
    checked = invoke([bins / "context", "check"], cwd=work, env=env, data=result.stdout)
    require(checked.stdout == b"", "Context check emitted evidence instead of a status")
    evidence = work / "evidence.jsonl"
    evidence.write_bytes(result.stdout)
    ref = normalized["ref"]
    candidate = f"  Opening time: 09:00. [{ref}]({row['citation']['url']})\n\n".encode()
    accepted = invoke([bins / "cite", evidence], cwd=work, env=env, data=candidate)
    require(accepted.stdout == candidate and accepted.stderr == b"", "Cite changed accepted candidate bytes")
    rejected = invoke([bins / "cite", evidence], cwd=work, env=env,
                      data=candidate.replace(b"example.test/hours", b"example.test/other"), code=1)
    require(rejected.stdout == b"" and rejected.stderr, "Cite leaked a rejected candidate or hid its diagnostic")
    duplicate = dict(normalized)
    duplicate["citation"] = {"locator": "hours", "url": "https://example.test/conflict"}
    evidence.write_bytes(result.stdout + json.dumps(duplicate).encode() + b"\n")
    broken = invoke([bins / "cite", evidence], cwd=work, env=env, data=candidate, code=2)
    require(broken.stdout == b"" and broken.stderr, "Cite failed to separate broken evidence from rejection")
    evidence.write_bytes(result.stdout)
    print("ok Context -> Cite: structured evidence, exact output, rejection and broken-evidence status", flush=True)
    return result.stdout


def create_ask_session(bins, work, env, evidence):
    """Have the actual Ask binary create a session through a loopback wire fixture."""
    calls = []
    answer = "Offline fixture answer."

    class Fixture(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            calls.append((self.path, request))
            response = {"id": "resp_fixture", "object": "response", "status": "in_progress",
                        "model": "fixture", "output": []}
            events = [
                {"type": "response.created", "sequence_number": 0, "response": response},
                {"type": "response.output_text.delta", "sequence_number": 1, "item_id": "msg_fixture",
                 "output_index": 0, "content_index": 0, "delta": answer},
                {"type": "response.completed", "sequence_number": 2, "response": {
                    **response, "status": "completed", "output": [{"type": "message", "id": "msg_fixture",
                    "role": "assistant", "status": "completed", "content": [{"type": "output_text",
                    "text": answer, "annotations": []}]}],
                    "usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}},
            ]
            wire = b"".join(("event: " + item["type"] + "\ndata: " + json.dumps(item) + "\n\n").encode()
                            for item in events)
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Content-Length", str(len(wire)))
            self.end_headers()
            self.wfile.write(wire)

    archive = work / "archive"
    archive.mkdir()
    session = archive / "action.jsonl"
    server = HTTPServer(("127.0.0.1", 0), Fixture)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    fixture_env = dict(env, OPENAI_API_KEY="offline-fixture", OPENAI_BASE_URL=f"http://127.0.0.1:{server.server_port}/v1")
    try:
        result = invoke([bins / "ask", "-q", "-f", session, "-m", "openai/fixture", "Read the supplied evidence."],
                        cwd=work, env=fixture_env, data=evidence)
        require(result.stdout == (answer + "\n").encode(), "Ask did not preserve fixture answer output")
        require(len(calls) == 1, "Ask made an unexpected number of fixture requests")
        require(calls[0][0] == "/v1/responses", "Ask fixture hit an unexpected endpoint")
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
    verified = invoke([bins / "ask", "replay", "-check", "-json", session], cwd=work, env=env)
    events = [json.loads(line) for line in verified.stdout.splitlines()]
    require(any(e["type"] == "user" and e["data"].get("evidence") for e in events),
            "Ask did not record a Context evidence manifest")
    return archive, session


def check_action_receipts(bins, work, env, evidence):
    archive, session = create_ask_session(bins, work, env, evidence)
    (work / "complete").write_bytes(b"already verified\n")
    write_program(work / "precheck", '''import pathlib, sys
assert not sys.stdin.buffer.read()
sys.exit(0 if pathlib.Path("complete").read_bytes() == b"already verified\\n" else 1)
''')
    precheck = invoke([bins / "ply", "-sh", "-C", work, "-f", session,
                       "-contract-id", "sha256:" + hashlib.sha256(b"integration precheck").hexdigest(),
                       "-check", "./precheck", "Keep the verified result."], cwd=work, env=env)
    require(precheck.stdout == b"", "Ply printed an unrequested answer for a passing precheck")
    precheck_events = [json.loads(line) for line in invoke(
        [bins / "ask", "replay", "-check", "-json", session], cwd=work, env=env).stdout.splitlines()]
    require(sum(e["type"] == "assistant" for e in precheck_events) == 1,
            "Ply's passing precheck called the model")
    require(any(e["type"] == "note" and e["data"].get("kind") == "ply.verifier/v2"
                and e["data"]["body"].get("phase") == "baseline"
                and e["data"]["body"].get("outcome") == "accepted" for e in precheck_events),
            "Built Ply did not seal its passing precheck through real Ask")
    print("ok built Ply -> Ask: passing baseline check, sealed verifier receipt, no model call", flush=True)
    connectors = work / "connectors"
    connectors.mkdir()
    marker = work / "effect.json"
    connector = write_program(connectors / "echo-request", '''import json, os, pathlib, sys
if sys.argv[1] == "describe":
    print(json.dumps({"version":1,"name":"echo-request","description":"Echo exact offline input","input_schema":{"type":"object"}}))
elif sys.argv[1] == "run":
    raw = sys.stdin.buffer.read()
    json.loads(raw)
    pathlib.Path(os.environ["FIXTURE_EFFECT"]).write_bytes(raw)
    sys.stderr.write("connector progress\\n")
    sys.stdout.buffer.write(raw)
else:
    sys.exit(2)
''')
    parked = write_program(work / "parked-may", '''import hashlib, json, os, pathlib, sys
assert sys.argv[1] == "request"
action = sys.stdin.buffer.read()
job = sys.argv[2]
digest = hashlib.sha256(b"may-v1\\0" + job.encode() + b"\\0" + action).hexdigest()
pathlib.Path(os.environ["FIXTURE_MAY"]).write_bytes(action)
print(json.dumps({"version":1,"job":job,"digest":digest,"action":action.decode(),"verdict":"parked"}, separators=(",", ":")))
sys.exit(75)
''')
    policy = write_program(work / "fixture-policy", '''import hashlib, json, sys
raw = sys.stdin.buffer.read()
print(json.dumps({"version":1,"action_sha256":"sha256:"+hashlib.sha256(raw).hexdigest(),"decision":"allow","reason":"operator-owned offline fixture"}, separators=(",", ":")))
''')
    action_env = dict(env, ACTION_PATH=str(connectors), FIXTURE_EFFECT=str(marker), FIXTURE_MAY=str(work / "may-envelope"))
    request = b'{"z":2,"message":"offline"}\n'
    parked_result = invoke([bins / "action", "run", "-job", "fixture-parked", "-may", parked, "echo-request"],
                           cwd=work, env=action_env, data=request, code=75)
    require(parked_result.stdout == b"" and not marker.exists(), "Parked Action released connector input")
    may_envelope = json.loads((work / "may-envelope").read_bytes())
    require(may_envelope["input"] == json.loads(request), "Action did not bind exact requested input to May")
    accepted = invoke([bins / "action", "run", "-job", "fixture-allowed", "-policy", policy,
                       "-record", session, "-ask", bins / "ask", "echo-request"],
                      cwd=work, env=action_env, data=request)
    canonical = b'{"message":"offline","z":2}'
    require(accepted.stdout == marker.read_bytes() == canonical, "Action changed or leaked connector output")
    require(accepted.stderr == b"connector progress\n", "Action changed connector diagnostics")
    verified = invoke([bins / "ask", "replay", "-check", "-json", session], cwd=work, env=env)
    events = [json.loads(line) for line in verified.stdout.splitlines()]
    notes = [e for e in events if e["type"] == "note" and e["data"].get("source") == "action"]
    require([e["data"]["kind"] for e in notes] == ["action.proposal/v1", "action.decision/v1",
            "action.attempt/v1", "action.sent/v1", "action.result/v1"], "Action receipt sequence is incomplete")
    terminal = notes[-1]["data"]["body"]
    require(terminal["sent"] and terminal["exit_code"] == 0, "Action receipt did not bind completed execution")
    require(base64.b64decode(terminal["stdout"]) == canonical
            and terminal["stdout_sha256"] == "sha256:" + hashlib.sha256(canonical).hexdigest(),
            "Action receipt does not prove the observed output bytes")
    before = session.read_bytes()
    trail = invoke([bins / "trail", "check", archive], cwd=work, env=env)
    rows = [json.loads(line) for line in trail.stdout.splitlines()]
    require(len(rows) == 1 and rows[0]["kind"] == "check" and rows[0]["ok"], "Trail did not verify the Action archive")
    require(session.read_bytes() == before, "Trail modified a verified session")
    damaged = work / "damaged"
    damaged.mkdir()
    tampered = damaged / "action.jsonl"
    changed = before.splitlines(keepends=True)
    terminal_event = json.loads(changed[-2])
    require(terminal_event["type"] == "note", "Fixture expected a terminal note followed by its seal")
    terminal_event["data"]["body"]["stdout_bytes"] += 1
    changed[-2] = json.dumps(terminal_event, separators=(",", ":")).encode() + b"\n"
    tampered.write_bytes(b"".join(changed))
    tampered_before = tampered.read_bytes()
    broken = invoke([bins / "trail", "check", damaged], cwd=work, env=env, code=1)
    require(any(row.get("kind") == "check" and row.get("ok") is False
                for row in map(json.loads, broken.stdout.splitlines())), "Trail did not expose Ask's damaged-seal verdict")
    require(tampered.read_bytes() == tampered_before, "Trail repaired damaged evidence")
    print("ok Action -> Ask -> Trail: parked fixture refuses effects; exact output, sealed receipts, read-only replay and tamper rejection", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", required=True, type=Path, help="directory of separately built imported commands")
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    for name in REQUIRED:
        require((bins / name).is_file() and os.access(bins / name, os.X_OK), f"missing executable: {bins / name}")
    with tempfile.TemporaryDirectory(prefix="bench-integration-") as tmp:
        work = Path(tmp).resolve()
        home = work / "home"
        home.mkdir()
        # Preserve only build/runtime plumbing. Never inherit model credentials,
        # tool policy, session selectors, or an operator's writable state.
        keep = ("PATH", "TMPDIR", "GOCACHE", "GOMODCACHE", "GOPATH", "CGO_ENABLED",
                "CC", "CXX", "SDKROOT", "DEVELOPER_DIR", "SYSTEMROOT")
        env = {key: os.environ[key] for key in keep if key in os.environ}
        for name in ("GOCACHE", "GOMODCACHE", "GOPATH"):
            if name not in env:
                env[name] = subprocess.check_output(["go", "env", name], text=True).strip()
        env.update(HOME=str(home), PATH=str(bins) + os.pathsep + env.get("PATH", os.defpath),
                   GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local", ASK_LIVE="",
                   XDG_CONFIG_HOME=str(home / ".config"), XDG_STATE_HOME=str(home / ".local/state"),
                   XDG_CACHE_HOME=str(home / ".cache"), PYTHONDONTWRITEBYTECODE="1")
        for name in REQUIRED:
            env[name.upper()] = str(bins / name)
        env["PLY_TEST_ASK_SOURCE"] = str(ROOT / "tools/ask")
        go_contract("ply", "TestAskPlyContract", env)
        go_contract("hone", "TestScaffoldWritesFrontmatterBriefCanRead", env)
        evidence = check_context_cite(bins, work, env)
        check_action_receipts(bins, work, env, evidence)
        print("running Weave's offline executable/example suite", flush=True)
        weave = work / "weave-source"
        shutil.copytree(ROOT / "tools/weave", weave, ignore=shutil.ignore_patterns(".git", "__pycache__", "*.pyc"))
        weave_env = dict(env, WEAVE_TEST_BIN=str(bins))
        # These are tests/check's full smoke/example steps. Go/race/vet belong
        # to the independent runner; do not rebuild or replace its binaries.
        for command in ([sys.executable, "tests/smoke", bins / "weave"],
                        [sys.executable, "-m", "unittest", "discover", "-s", "tests", "-v"]):
            result = invoke(command, cwd=weave, env=weave_env, timeout=600)
            sys.stdout.buffer.write(result.stdout)
            sys.stderr.buffer.write(result.stderr)
        print("ok Weave -> Tend/Ask/Ply offline examples", flush=True)
    print("All offline executable integration checks passed.", flush=True)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, subprocess.SubprocessError, OSError, ValueError, KeyError) as error:
        print(f"integration: {error}", file=sys.stderr)
        sys.exit(1)
