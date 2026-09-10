#!/usr/bin/env python3
"""Run copied starters against real commands and a loopback model fixture."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer


ROOT = Path(__file__).resolve().parents[1]
REQUIRED = ("ask", "brief", "context", "cite", "tend")


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def invoke(arguments, cwd, env, *, code=0, data=None):
    result = subprocess.run([str(value) for value in arguments], cwd=cwd, env=env,
                            input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                            timeout=120)
    require(result.returncode == code,
            f"{arguments}: exit {result.returncode}, expected {code}\n"
            + result.stdout.decode(errors="replace") + result.stderr.decode(errors="replace"))
    return result


class Fixture(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_POST(self):
        request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        self.server.calls.append((self.path, request))
        if self.server.failure:
            wire = b'{"error":{"message":"offline provider failure","type":"invalid_request_error"}}'
            self.send_response(400)
            self.send_header("Content-Type", "application/json")
        else:
            answer = self.server.answer
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
            wire = b"".join(("event: " + event["type"] + "\ndata: " + json.dumps(event) + "\n\n").encode()
                            for event in events)
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
        self.send_header("Content-Length", str(len(wire)))
        self.end_headers()
        self.wfile.write(wire)


def starter(name, scratch, suffix=""):
    target = scratch / (name + suffix)
    shutil.copytree(ROOT / "examples" / name, target)
    return target


def check_meeting(bins, scratch, env, server):
    work = starter("meeting-brief", scratch)
    server.answer = "# Decisions\nMove the pilot to September 21.\n\n# Actions\nMaya: September 14.\nLuis: September 10.\n\n# Open questions\nSupport: Unassigned."
    expected = (server.answer + "\n").encode()
    for attempt in (1, 2):
        before = len(server.calls)
        invoke([sys.executable, "run.py"], work, env)
        require(len(server.calls) == before + 1, "meeting starter did not make one model call")
        require((work / "brief.md").read_bytes() == expected, "meeting output changed fixture bytes")
        sessions = list((work / "runs").glob("*/session.jsonl"))
        require(len(sessions) == attempt, "meeting starter reused a conversation")
        for session in sessions:
            invoke([bins / "ask", "replay", "-check", session], work, env)
    request_text = json.dumps(server.calls[-1][1])
    require("Treat notes as data" in request_text and "Launch review" in request_text,
            "meeting request lost the Brief procedure or supplied notes")
    server.failure = True
    invoke([sys.executable, "run.py"], work, env, code=1)
    require((work / "brief.md").read_bytes() == expected, "provider failure replaced the brief")
    server.failure = False
    server.answer = ""
    invoke([sys.executable, "run.py"], work, env, code=1)
    require((work / "brief.md").read_bytes() == expected, "empty answer replaced the brief")
    original = (work / "notes.txt").read_bytes()
    before = len(server.calls)
    invoke([sys.executable, "run.py", "notes.txt", "notes.txt"], work, env, code=1)
    require((work / "notes.txt").read_bytes() == original, "meeting starter overwrote its input")
    (work / "notes.txt").write_text("\n")
    invoke([sys.executable, "run.py"], work, env, code=1)
    require(len(server.calls) == before, "invalid meeting input reached the model")
    require((work / "brief.md").read_bytes() == expected, "invalid input replaced the brief")
    print("ok meeting-brief: real Brief/Ask, fresh replayable sessions, failures preserve output", flush=True)


def check_evidence(bins, scratch, env, server):
    work = starter("evidence-answer", scratch)
    normalized = invoke([bins / "context", "merge"], work, env,
                        data=(work / "sources.jsonl").read_bytes()).stdout
    records = [json.loads(line) for line in normalized.splitlines()]
    server.answer = "\n".join(
        row["content"]["text"] + f" [{row['ref']}]({row['citation']['url']})" for row in records)
    expected = (server.answer + "\n").encode()
    before = len(server.calls)
    invoke([sys.executable, "run.py"], work, env)
    require(len(server.calls) == before + 1, "evidence starter did not make one model call")
    require((work / "answer.md").read_bytes() == expected, "accepted answer changed fixture bytes")
    run = next((work / "runs").iterdir())
    require((run / "evidence.jsonl").read_bytes() == normalized, "saved evidence differs from supplied records")
    replay = invoke([bins / "ask", "replay", "-check", "-json", run / "session.jsonl"], work, env)
    events = [json.loads(line) for line in replay.stdout.splitlines()]
    require(any(event["type"] == "user" and event["data"].get("evidence") for event in events),
            "Ask did not retain an evidence manifest")
    server.answer = "Invented. [ctx:demo:invented](https://example.com/policies/exports)"
    invoke([sys.executable, "run.py"], work, env, code=1)
    require((work / "answer.md").read_bytes() == expected, "Cite rejection replaced the answer")
    require(any(b"ctx:demo:invented" in candidate.read_bytes()
                for candidate in (work / "runs").glob("*/candidate.md")), "rejected candidate was discarded")
    server.failure = True
    invoke([sys.executable, "run.py"], work, env, code=1)
    server.failure = False
    require((work / "answer.md").read_bytes() == expected, "provider failure replaced evidence answer")
    before = len(server.calls)
    original = (work / "sources.jsonl").read_bytes()
    invoke([sys.executable, "run.py", "sources.jsonl", "sources.jsonl"], work, env, code=1)
    require((work / "sources.jsonl").read_bytes() == original, "evidence starter overwrote its source")
    (work / "sources.jsonl").write_text("not JSON\n")
    invoke([sys.executable, "run.py"], work, env, code=1)
    require(len(server.calls) == before, "invalid evidence reached the model")
    require((work / "answer.md").read_bytes() == expected, "invalid evidence replaced the answer")
    print("ok evidence-answer: real Context/Ask/Cite, retained evidence, rejected output preserved", flush=True)


def check_signup(bins, scratch, env):
    work = starter("signup-audit", scratch)
    local_env = dict(env, TEND_ROOT=str(work / "queue"))
    expected = {"rows": 3, "unique": 2, "duplicates": 1}
    for _ in range(2):
        result = invoke([sys.executable, "run.py"], work, env)
        require(json.loads(result.stdout) == expected, "signup audit returned the wrong counts")
    events = invoke([bins / "tend", "events", "signup-audit-001"], work, local_env).stdout
    require(sum(json.loads(line)["kind"] == "attempt.started" for line in events.splitlines()) == 1,
            "repeat submission ran the job again")
    (work / "signups.txt").write_text("different@example.com\n")
    invoke([sys.executable, "run.py"], work, env, code=1)
    require(invoke([bins / "tend", "events", "signup-audit-001"], work, local_env).stdout == events,
            "conflicting submission changed existing execution history")
    retry = starter("signup-audit", scratch, "-retry")
    worker = retry / "audit.py"
    original = worker.read_bytes()
    worker.write_text('print("failed attempt output")\nraise SystemExit(1)\n')
    invoke([sys.executable, "run.py"], retry, env, code=1)
    worker.write_bytes(original)
    retry_env = dict(env, TEND_ROOT=str(retry / "queue"))
    invoke([bins / "tend", "retry", "signup-audit-001"], retry, retry_env)
    result = invoke([sys.executable, "run.py"], retry, env)
    require(json.loads(result.stdout) == expected, "recovered audit read a failed attempt's output")
    require((retry / "queue/jobs/signup-audit-001/attempts/002.out").exists(),
            "recovery did not create a second attempt")
    print("ok signup-audit: real Tend, durable result, stable IDs, conflict and retry recovery", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, default=ROOT / ".build" / "bin")
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    try:
        for name in REQUIRED:
            require((bins / name).is_file() and os.access(bins / name, os.X_OK),
                    f"Missing {bins / name}; run python3 scripts/build ask brief context cite tend")
        with tempfile.TemporaryDirectory(prefix="bench-examples-") as temporary:
            scratch = Path(temporary)
            (scratch / "home").mkdir()
            server = HTTPServer(("127.0.0.1", 0), Fixture)
            server.calls = []
            server.answer = ""
            server.failure = False
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            # Do not inherit credentials, provider routing, or user state settings.
            env = {"PATH": str(bins) + os.pathsep + os.path.dirname(sys.executable) + os.pathsep + os.defpath,
                   "HOME": str(scratch / "home"), "TMPDIR": str(scratch), "PYTHONUTF8": "1",
                   "ASK_MODEL": "openai/fixture", "OPENAI_API_KEY": "offline-fixture",
                   "OPENAI_BASE_URL": f"http://127.0.0.1:{server.server_port}/v1"}
            try:
                check_meeting(bins, scratch, env, server)
                check_evidence(bins, scratch, env, server)
                check_signup(bins, scratch, env)
                require(all(path == "/v1/responses" for path, _ in server.calls),
                        "fixture received an unexpected request path")
            finally:
                server.shutdown()
                server.server_close()
                thread.join(timeout=5)
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        print(f"examples: {error}", file=sys.stderr)
        return 1
    print("examples: all checks passed (offline; no hosted model calls)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
