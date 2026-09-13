#!/usr/bin/env python3
"""Run copied starters against real commands and a loopback model fixture."""
import argparse
import importlib.util
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
REQUIRED = ("ask", "brief", "context", "cite", "tend", "agent", "hire", "ply", "cage")


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
            answer = self.server.answer(request) if callable(self.server.answer) else self.server.answer
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


def check_support(bins, scratch, env, server, native_cage):
    root = starter("support-reply", scratch)
    expert = root / "expert"
    original = {p.relative_to(expert): p.read_bytes() for p in expert.rglob("*") if p.is_file()}
    invoke([bins / "hire", "verify", expert], root, env)
    records = [json.loads(line) for line in (expert / "sources.jsonl").read_text().splitlines()]
    answer = "\n".join(row["content"]["text"] + f" [{row['ref']}]({row['citation']['url']})"
                       for row in records) + "\nThe policy does not specify how long the export takes.\n"
    boundary = [] if native_cage else ["-no-cage"]
    first_question = (root / "question.txt").read_text().strip()
    for name in ("customer-work", "next-customer"):
        work = root / name
        work.mkdir()
        question = first_question if name == "customer-work" else "Who is allowed to export our data?"
        (work / "question.txt").write_text(question + "\n")
        evidence = root / (name + "-records")
        invoke([expert / "bin/check"], work, env, code=1)
        turns = []

        def respond(request):
            if "You are choosing which skill" in request.get("instructions", ""):
                return "support-reply"
            turns.append(request)
            replies = ["```ply\ncat question.txt \"$AGENT_HOME/sources.jsonl\"\nprintf '%s\\n' 'An uncited reply.' > reply.md\n```",
                       "First candidate.",
                       "```ply\ncat > reply.md <<'REPLY'\n" + answer + "REPLY\n```",
                       "Drafted reply.md. Export duration is not specified."]
            require(len(turns) <= len(replies), "support expert did not accept the repaired citation")
            return replies[len(turns) - 1]

        server.answer = respond
        command = [bins / "agent", "run", *boundary, "-C", work, "-evidence", evidence,
                   "-turns", "8", expert, "--", "Draft reply.md for the customer using the supplied policy."]
        if name == "next-customer":
            command[2:2] = ["-checkpoint", name]
            queue_env = dict(env, TEND_ROOT=str(root / "queue"),
                             TEND_PASS="ASK_MODEL OPENAI_API_KEY OPENAI_BASE_URL")
            invoke([bins / "tend", "submit", "-id", name, "-C", work, "--", *command],
                   root, queue_env, data=b"")
            invoke([bins / "tend", "work"], root, queue_env)
            job = json.loads(invoke([bins / "tend", "show", name], root, queue_env).stdout)
            require(job["status"] == "done", "Tend did not retain the successful Agent outcome")
            report = (root / "queue/jobs" / name / "attempts/001.out").read_bytes()
            invoke([bins / "tend", "check"], root, queue_env)
            require((evidence / "checkpoints" / (name + ".current")).is_file(),
                    "Agent did not retain its checkpoint")
        else:
            report = invoke(command, root, env, data=b"").stdout
        require(report == b"Drafted reply.md. Export duration is not specified.\n",
                "support expert mixed its report with diagnostics")
        require((work / "reply.md").read_text() == answer, "support expert lost the checked reply")
        require(len(turns) == 4, "support expert did not exercise rejection and repair")
        require(question in json.dumps(turns[1]), "workspace question never reached the expert")
        if name == "next-customer":
            require(first_question not in json.dumps(turns), "second workspace inherited the first question")
        instructions = turns[0].get("instructions", "")
        require("Support reply expert" in instructions and "Identify each question" in instructions,
                "support expert lost its definition or selected Brief skill")
        sessions = list((evidence / "runs").glob("*.jsonl"))
        require(len(sessions) == 1, "support expert has no unique execution session")
        replay = invoke([bins / "ask", "replay", "-check", "-json", sessions[0]], root, env)
        verdicts = [event["data"]["body"].get("outcome")
                    for event in map(json.loads, replay.stdout.splitlines())
                    if event["type"] == "note" and event["data"].get("kind") == "ply.verifier/v2"]
        require("rejected" in verdicts and "accepted" in verdicts, "support expert lost verifier evidence")
        before = len(server.calls)
        require(not invoke(command, root, env, data=b"").stdout, "pre-check invented a report")
        require(len(server.calls) == before, "accepted support reply called the model on re-entry")
    invalid = invoke([bins / "cite", expert / "sources.jsonl"], root, env, code=1,
                     data=b"[ctx:demo:invented](https://example.com/policies/exports)\n")
    require(not invalid.stdout, "invalid support citation leaked output")
    require(original == {p.relative_to(expert): p.read_bytes() for p in expert.rglob("*") if p.is_file()},
            "running support expert changed its reusable definition")
    print("ok support-reply: one expert, distinct inputs/contexts, real Agent/Brief/Ask/Ply/Cite, Tend checkpoint, rejection/repair and replay"
          + (", native Cage" if native_cage else "; host boundary selected for this offline fixture"), flush=True)


def check_page_worker_feedback(bins, scratch, env, assembled):
    """Use the existing wire fixture to exercise the real worker's repair loop."""
    spec = importlib.util.spec_from_file_location("bench_integration_fixture", ROOT / "scripts/check-integration.py")
    fixture = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(fixture)
    bundle = scratch / "page-feedback-expert"
    shutil.copytree(assembled, bundle,
                    ignore=shutil.ignore_patterns("node_modules", "__pycache__"))
    shutil.copytree(ROOT / "teams/page-team/tests/copy-editor", bundle / "agents/copy-editor")
    # A test-only extension and the real exported frontend both use public bindings.
    shutil.copy2(bundle / "bin/worker-adapter", bundle / "bin/workers/copy-editor")
    expected_copy = "The player shows composure under pressure. Further live observation is needed.\n"
    expected_page = (ROOT / "teams/page-team/tests/browser-fixture.html").read_text()
    for role, filename, expected in [("copy-editor", "edited.txt", expected_copy),
                                      ("frontend", "index.html", expected_page)]:
        job = scratch / ("page-feedback-" + role) / "jobs/bench-manage-000001"
        work, control = job / "work", job / "control"
        work.mkdir(parents=True)
        control.mkdir()
        def action(text):
            return ("```ply\nmkdir -p output\ncat > output/" + filename + " <<'FIXTURE'\n"
                    + text + "FIXTURE\nmake-handoff " + role + " 'Feedback fixture' output/" + filename + "\n```")
        replies = [action("WRONG\n"), "First candidate.", action(expected), "Corrected and checked."]
        turns = []
        def respond(request):
            turns.append(request)
            return replies[min(len(turns), len(replies)) - 1]
        selected = dict(env, AGENT=str(bins / "agent"),
                        BENCH_MANAGE_MODEL="openai/fixture", BENCH_MANAGE_EFFORT="medium",
                        BENCH_MANAGE_TURNS="6", BENCH_MANAGE_SESSION=str(control / "session.jsonl"))
        packet = {"task": {"id": role + "-repair", "input": {"worker": role, "goal": "Check fixture output"}},
                  "dependencies": []}
        with fixture.model_fixture(selected, respond) as (fixture_env, calls):
            result = invoke([bundle / "bin/workers" / role], work, fixture_env, data=json.dumps(packet).encode())
            manifest = json.loads(result.stdout)
            require(manifest["task_id"] == role + "-repair" and len(turns) == 4,
                    role + " failed to preserve Agent's rejected-candidate correction loop")
            require((work / "output" / filename).read_text() == expected, role + " accepted the wrong bytes")
        session = next((control / "agent-evidence/runs").glob("*.jsonl"))
        replay = invoke([bins / "ask", "replay", "-check", "-json", session], work, env)
        verdicts = [e["data"]["body"].get("outcome") for e in map(json.loads, replay.stdout.splitlines())
                    if e["type"] == "note" and e["data"].get("kind") == "ply.verifier/v2"]
        require("rejected" in verdicts and "accepted" in verdicts, role + " lost rejection/acceptance evidence")
        print("ok page-team " + role + ": exported binding, real Agent/Ply/Ask, rejection/repair, native Cage", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, default=ROOT / ".build" / "bin")
    parser.add_argument("--native-cage", action="store_true", help="run the expert with its default Cage boundary")
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    try:
        for name in REQUIRED:
            require((bins / name).is_file() and os.access(bins / name, os.X_OK),
                    f"Missing {bins / name}; run python3 scripts/build {' '.join(REQUIRED)}")
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
                check_support(bins, scratch, env, server, args.native_cage)
                # Exercise the actual committed source assembly, never an incomplete template.
                commit = subprocess.check_output(["git", "-C", str(ROOT), "rev-parse", "HEAD"]).decode().strip()
                exported = scratch / "page-team-export"
                invoke([sys.executable, ROOT / "scripts/workers", "export-team", "page-team",
                        exported, "--ref", commit, "--allow-experimental"], scratch, env)
                assembled = exported / "expert"
                invoke([sys.executable, ROOT / "teams/page-team/tests/contracts.py", assembled], scratch, env)
                page_work = scratch / "page-team-work"
                page_work.mkdir()
                invoke([bins / "agent", "check", "-C", page_work,
                        "-evidence", scratch / "page-team-evidence",
                        assembled], scratch, env)
                print("ok page-team: nested definition structure and offline artifact/protocol contracts", flush=True)
                if args.native_cage:
                    check_page_worker_feedback(bins, scratch, env, assembled)
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
