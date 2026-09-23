#!/usr/bin/env python3
"""Offline public Weigh/Record/Ask/Ply integration. No hosted model calls."""
import argparse
from contextlib import contextmanager
import json
import os
from pathlib import Path
import runpy
import shlex
import subprocess
import sys
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

ROOT = Path(__file__).resolve().parents[1]
SUPPORT = runpy.run_path(str(ROOT / "scripts/check-integration.py"))
require, invoke = SUPPORT["require"], SUPPORT["invoke"]


@contextmanager
def decisions():
    calls = []

    class Fixture(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            raw = self.rfile.read(int(self.headers["Content-Length"]))
            request = json.loads(raw)
            calls.append((self.path, request, self.headers.get("Authorization")))
            state = request["state"]
            if isinstance(state, dict) and state.get("mode") == "failure":
                wire = b'{"error":{"message":"fixture unavailable"}}'
                self.send_response(503)
            else:
                answers = {}
                for key, question in request["questions"].items():
                    if question["type"] == "choice":
                        choices = list(question["criteria"])
                        selected = ("violated" if "bad" in state.get("candidate", "") else "satisfied") if "satisfied" in choices else ("artifact" if "artifact" in choices else choices[0])
                        answers[key] = {"type": "choice", "choice": selected,
                                        "probabilities": {name: int(name == selected) for name in choices}, "confidence": 1}
                    elif question["type"] == "score":
                        answers[key] = {"type": "score", "score": .75, "legend": {str(i): v for i, v in enumerate(question["criteria"])},
                                        "probabilities": {"0": .25, "1": .75}}
                    else:
                        answers[key] = {"type": "noul", "noul": .8}
                wire = json.dumps({"model": "typesafe/fixture-resolved", "answers": answers,
                                   "usage": {"input_tokens": 10, "output_tokens": 2, "cost": .000001}}).encode()
                self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(wire)))
            self.end_headers()
            self.wfile.write(wire)

    server = HTTPServer(("127.0.0.1", 0), Fixture)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:%s/api/alpha/decisions" % server.server_port, calls
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, required=True)
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    for name in ("weigh", "ask", "record", "ply"):
        require((bins / name).is_file(), "missing binary: " + name)
    with tempfile.TemporaryDirectory(prefix="weigh-contract-") as tmp:
        root = Path(tmp).resolve()
        env = {name: os.environ[name] for name in ("PATH", "TMPDIR", "BENCH_REPLAY_RECORD_DIR", "BENCH_REPLAY_BIN_DIR") if name in os.environ}
        env.update(HOME=str(root), PATH=str(bins) + os.pathsep + env.get("PATH", os.defpath),
                   OPENROUTER_API_KEY="offline-key", PYTHONDONTWRITEBYTECODE="1")
        request = {"version": 1, "state": {"exact": 9007199254740993}, "questions": {
            "choice": {"type": "choice", "question": "Which?", "options": {"a": "A", "b": "B"}},
            "score": {"type": "score", "question": "How much?", "levels": ["none", "complete"]},
            "prob": {"type": "probability", "question": "Is the proposition true?"}}}
        raw = json.dumps(request).encode()
        receipt = root / "judgment.jsonl"
        with decisions() as (endpoint, calls):
            command = [bins / "weigh", "-m", "openrouter/~typesafe/jev-latest", "-endpoint", endpoint]
            expanded = dict(request, state="<" * 1400000)
            no_key = {key: value for key, value in env.items() if key != "OPENROUTER_API_KEY"}
            bounded = invoke(command, cwd=root, env=no_key, data=json.dumps(expanded).encode(), code=2)
            require(not bounded.stdout and not calls and b"encoded provider request exceeds" in bounded.stderr,
                    "encoded request bound did not precede credentials and networking")
            answer = invoke(command, cwd=root, env=env, data=raw)
            result = json.loads(answer.stdout)
            require(result["answers"]["score"]["value"] == .75 and result["answers"]["prob"]["value"] == .8, "native typed values changed")
            require(calls[0][0] == "/api/alpha/decisions" and calls[0][1]["model"] == "~typesafe/jev-latest", "Decisions route or literal model changed")
            require(calls[0][1]["state"]["exact"] == 9007199254740993, "state numeric precision changed")
            require(calls[0][2] == "Bearer offline-key", "explicit fixture key missing")
            structured = {"version": 1, "state": {"candidate": "good"}, "questions": {
                "choice": {"type": "choice", "question": {"ask": "Which?", "exact": 9007199254740993},
                           "options": {"a": {"covers": ["A"]}, "b": ["B"], "other": None}},
                "score": {"type": "score", "question": ["How much?"],
                          "levels": [{"meaning": "none", "exact": 9007199254740993}, ["complete"]]},
                "prob": {"type": "probability", "question": {"ask": "Is there evidence?"},
                         "criteria": {"true": {"includes": "Direct evidence"}, "false": ["No evidence"]}}}}
            before = len(calls)
            typed = invoke(command, cwd=root, env=env, data=json.dumps(structured).encode())
            require(len(calls) == before + 1 and json.loads(typed.stdout)["answers"]["score"]["value"] == .75,
                    "structured questions did not yield one validated call")
            for name, question in structured["questions"].items():
                native = calls[-1][1]["questions"][name]
                require(native["instructions"] == question["question"], "structured instructions changed")
                expected = question.get("options", question.get("levels", question.get("criteria")))
                require(native["criteria"] == expected, "structured criteria or numeric precision changed")
            direct = invoke(command, cwd=root, env=dict(env, BENCH_WEIGH="0"), data=raw)
            require(direct.stdout == answer.stdout, "Bench opt-in changed the independent Weigh executable")
            captured = invoke([bins / "record", "run", "-ask", bins / "ask", "-f", receipt, "--", *command], cwd=root, env=env, data=raw)
            require(captured.stdout == answer.stdout, "record changed validated output")
            negative = dict(request, state={"mode": "failure"})
            before = len(calls)
            rejected = invoke(command, cwd=root, env=env, data=json.dumps(negative).encode(), code=1)
            require(not rejected.stdout and len(calls) == before + 1, "runtime failure emitted data or retried")
            invoke(command, cwd=root, env=env, data=b'{"version":1,"version":1}', code=2)
            # Private descriptor forwarding crosses the real Record boundary.
            read_fd, write_fd = os.pipe()
            os.write(write_fd, b"Authorization: Bearer private-fixture-secret\n")
            os.close(write_fd)
            private = root / "private.jsonl"
            private_command = [str(bins / "record"), "run", "-ask", str(bins / "ask"), "-f", str(private),
                               "-pass-fd", str(read_fd), "--", *map(str, command), "-header-fd", str(read_fd)]
            try:
                child = subprocess.run(private_command, input=raw, capture_output=True, env=env, cwd=root,
                                       pass_fds=(read_fd,), timeout=30)
            finally:
                os.close(read_fd)
            require(child.returncode == 0 and calls[-1][2] == "Bearer private-fixture-secret", "private descriptor composition failed")
            require(b"private-fixture-secret" not in private.read_bytes() + child.stdout + child.stderr, "private credential leaked into receipt/streams")

            checker = ROOT / "tools/weigh/examples/semantic-check/check.py"
            rubric = root / "rubric.json"
            rubric.write_text(json.dumps({"version": 1, "criteria": [{"id": "content", "requirement": "Candidate must say good.", "feedback": "Replace bad with good."}]}))
            candidate = root / "candidate.txt"
            candidate.write_text("bad\n")
            check_command = [sys.executable, str(checker), "--backend", "weigh", "--model", "openrouter/~typesafe/jev-latest",
                             "--rubric", str(rubric), "--candidate", str(candidate), "--records", str(root / "checks"),
                             "--accept-at", ".95", "--reject-at", ".95", "--endpoint", endpoint,
                             "--ask", str(bins / "ask"), "--weigh", str(bins / "weigh"), "--record", str(bins / "record")]
            for setting in (None, "", "0", "invalid"):
                disabled_env = dict(env)
                if setting is not None:
                    disabled_env["BENCH_WEIGH"] = setting
                before = len(calls)
                disabled = invoke(check_command, cwd=root, env=disabled_env, code=2)
                require(not disabled.stdout and len(calls) == before and b"BENCH_WEIGH" in disabled.stderr,
                        "disabled or invalid Weigh checker called a model or silently skipped acceptance")
            enabled_env = dict(env, BENCH_WEIGH="1")
            rejected = invoke(check_command, cwd=root, env=enabled_env, code=1)
            require(json.loads(rejected.stdout)["feedback"] == ["Replace bad with good."], "lost criterion repair feedback")
            snapshot = {"version": 1, "id": "one-selected-run", "candidate": "bad", "evidence": "good is required",
                        "criteria": json.loads(rubric.read_text())["criteria"], "check": {"verdict": "reject"},
                        "review": {"verdict": "reject", "independent": True, "findings": ["The output is bad."]}}
            triage = [sys.executable, ROOT / "tools/weigh/examples/improve-checks/triage.py",
                      "--model", "openrouter/~typesafe/jev-latest", "--records", root / "triage",
                      "--ask", bins / "ask", "--record", bins / "record", "--weigh", bins / "weigh"]
            for setting in (None, "", "0", "invalid"):
                disabled_env = dict(env)
                if setting is not None:
                    disabled_env["BENCH_WEIGH"] = setting
                before = len(calls)
                disabled = invoke([*triage, "--backend", "weigh", "--endpoint", endpoint], cwd=root,
                                  env=disabled_env, data=json.dumps(snapshot).encode(), code=2)
                require(not disabled.stdout and len(calls) == before and b"BENCH_WEIGH" in disabled.stderr,
                        "disabled or invalid Weigh diagnosis called a model or emitted a hypothesis")
            classified = invoke([*triage, "--backend", "weigh", "--endpoint", endpoint], cwd=root, env=enabled_env,
                                data=json.dumps(snapshot).encode())
            hypothesis = json.loads(classified.stdout)
            require(hypothesis["status"] == "hypothesis" and hypothesis["hypothesis"]["target"]["value"] == "artifact",
                    "diagnosis did not preserve hypothesis boundary")
            require(calls[-1][1]["state"] == snapshot, "diagnosis changed the explicit selected snapshot")
            invoke([bins / "record", "check", "-ask", bins / "ask", "-f", hypothesis["record"]], cwd=root, env=env)
            turns = []

            def repair(_):
                turns.append(1)
                return "```sh\nprintf 'good\\n' > candidate.txt\n```" if len(turns) == 1 else "Updated candidate.txt."

            with SUPPORT["model_fixture"](enabled_env, repair) as (generation_env, _):
                loop = invoke([bins / "ply", "-sh", "-m", "openai/fixture", "-f", root / "repair.jsonl",
                               "-turns", "4", "-check", shlex.join(check_command), "Correct candidate.txt according to the checker."],
                              cwd=root, env=generation_env)
                require(candidate.read_text() == "good\n" and len(turns) == 2, "Ply did not repair from semantic feedback")
            # Ask-only route uses no Weigh executable and no OpenRouter access.
            ask_env = {key: value for key, value in env.items() if key != "OPENROUTER_API_KEY"}
            ask_env["BENCH_WEIGH"] = "invalid"
            with SUPPORT["model_fixture"](ask_env, lambda _: '{"answers":{"content":"satisfied"}}') as (ask_env, _):
                accepted = invoke([sys.executable, checker, "--backend", "ask", "--model", "openai/fixture",
                                   "--rubric", rubric, "--candidate", candidate, "--records", root / "ask-only",
                                   "--ask", bins / "ask", "--record", bins / "record", "--weigh", root / "unavailable-weigh"], cwd=root, env=ask_env)
                require(json.loads(accepted.stdout)["verdict"] == "accept", "optional Ask-only checker failed")
            with SUPPORT["model_fixture"](ask_env, lambda _: '{"target":"rubric","action":"revise"}') as (only_ask, _):
                classified = invoke([*triage, "--backend", "ask", "--model", "openai/fixture",
                                     "--weigh", root / "unavailable-weigh"], cwd=root, env=only_ask,
                                    data=json.dumps(snapshot).encode())
                hypothesis = json.loads(classified.stdout)
                require(hypothesis["hypothesis"]["target"]["value"] == "rubric"
                        and hypothesis["hypothesis"]["target"]["probabilities"] is None,
                        "Ask-only diagnosis invented probabilities or required Weigh")
        # Replay must succeed after the fixture service is gone and no key exists.
        offline = {key: value for key, value in env.items() if key != "OPENROUTER_API_KEY"}
        replayed = invoke([bins / "record", "replay", "-ask", bins / "ask", "-f", receipt], cwd=root, env=offline)
        require(replayed.stdout == answer.stdout, "offline judgment replay differed")
        extracted = invoke([bins / "record", "replay", "-ask", bins / "ask", "-f", receipt, "-stream", "stdin"], cwd=root, env=offline)
        require(extracted.stdout == raw, "record lost explicit state/questions")
        for path in (root / "checks").glob("*/weigh.record.jsonl"):
            invoke([bins / "record", "check", "-ask", bins / "ask", "-f", path], cwd=root, env=offline)
        visual = invoke([sys.executable, ROOT / "tools/weigh/examples/visual-check/integration.py", "--bin-dir", bins],
                        cwd=root, env=offline)
        sys.stdout.buffer.write(visual.stdout)
        sys.stderr.buffer.write(visual.stderr)
        selection = invoke([sys.executable, ROOT / "scripts/check-fix-selection.py", "--bin-dir", bins],
                           cwd=root, env=offline)
        sys.stdout.buffer.write(selection.stdout)
        sys.stderr.buffer.write(selection.stderr)
    print("ok Weigh: independent direct CLI, default-off adapters, Decisions wire, typed results, errors, private auth, offline Record replay, Ply repair and unaffected Ask-only checker")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, subprocess.SubprocessError) as error:
        print("weigh integration: " + str(error), file=sys.stderr)
        raise SystemExit(1)
