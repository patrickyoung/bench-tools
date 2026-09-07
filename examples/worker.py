#!/usr/bin/env python3
"""One ordinary Tend job. Business policy and model calls stay outside Weave."""
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys

import business
from protocol import command, digest, encoded, exact, read, replace


def validate(payload, candidate):
    business.check(payload["task"], candidate, payload["dependencies"])
    if payload["mode"] == "model" and business.agent_task(payload["task"]) and candidate.get("mode") != "model":
        raise ValueError("model result must be labelled mode=model")


def verify(path):
    payload = json.loads(read(path))
    candidate = sys.stdin.buffer.read(1024 * 1024 + 1)
    try:
        if len(candidate) > 1024 * 1024:
            raise ValueError("candidate exceeds 1 MiB")
        if not candidate:
            candidate = read("result.json")
        validate(payload, json.loads(candidate))
    except (ValueError, FileNotFoundError) as e:
        print(f"not accepted: {e}")
        return 1
    print("accepted by business checker for " + payload["task_sha256"])
    return 0


def work():
    raw = sys.stdin.buffer.read(16 * 1024 * 1024 + 1)
    if len(raw) > 16 * 1024 * 1024:
        raise ValueError("task payload exceeds 16 MiB")
    payload = json.loads(raw)
    exact("input.json", raw)
    task, dependencies = payload["task"], payload["dependencies"]
    session = Path.cwd() / "session.jsonl"
    receipt = None
    ply_binding = None
    evidence_mode = "controller-check"
    if payload["mode"] == "model" and business.agent_task(task):
        ask, ply = payload["tools"]["ask"], payload["tools"]["ply"]
        prompt = business.prompt(task, dependencies)
        if len(prompt.encode()) > 64 * 1024:
            raise ValueError("report-only study prompt exceeds Ply's 64 KiB inline limit")
        kind = task["input"]["kind"]
        if kind == "review":
            exact("result.schema.json", encoded(business.review_schema()))
            output = command([ask, "-f", str(session), "-m", payload["model"],
                              "-schema", str(Path.cwd() / "result.schema.json"),
                              "-q", "Return the requested JSON result."], prompt.encode())
        else:
            (Path.cwd() / "tools").mkdir(exist_ok=True)
            check = shlex.join([sys.executable, str(Path(__file__).resolve()),
                                "verify", str(Path.cwd() / "input.json")])
            tool = Path.cwd() / "tools" / "report-only"
            exact(tool, b"#!/bin/sh\n# Return a JSON report; model shell actions are disabled in this study.\nprintf '%s\\n' 'This study accepts JSON reports only; no model shell actions.' >&2\nexit 125\n")
            tool.chmod(0o700)
            output = command([ply, "-C", str(Path.cwd()), "-t", str(Path.cwd() / "tools"), "-f", str(session),
                              "-m", payload["model"], "-no-delegate", "-turns", "6",
                              "-cycles", "3", "-timeout", "30s", "-contract-id",
                              payload["task_sha256"], "-check", check, "-action-shell", str(tool),
                              "-action-boundary-exit", "125", "-shell", "/bin/sh",
                              "Return a JSON report satisfying the business check."], prompt.encode())
            ply_binding = {"phase": "candidate" if output.strip() else "baseline",
                           "candidate_sha256": digest(output.rstrip(b"\n") + b"\n" if output else b""),
                           "verifier_sha256": digest(("/bin/sh\0" + check).encode()),
                           "contract_id": payload["task_sha256"], "directory": str(Path.cwd())}
        # A genuine pre-check may finish without model output or a new session.
        candidate = json.loads(output if output.strip() else read("result.json"))
        validate(payload, candidate)
        if session.exists():
            receipt = {"task_sha256": payload["task_sha256"], "input_sha256": digest(raw),
                       "result_sha256": digest(encoded(candidate)), "outcome": "accepted",
                       "checker_sha256": digest(read(Path(business.__file__))),
                       "path": "result.json"}
            command([ask, "note", "-q", "-f", str(session), "-s", "weave-example",
                     "-k", "weave.example.check/v1", "-json", "-", "-seal"], encoded(receipt))
            evidence_mode = "ask-sealed-check"
        else:
            evidence_mode = "controller-precheck"
    else:
        candidate = business.execute(task, dependencies)
        validate(payload, candidate)
    result = encoded(candidate)
    if len(result) > 1024 * 1024:
        raise ValueError("result exceeds 1 MiB")
    replace("result.json", result)
    envelope = {"task_sha256": payload["task_sha256"], "input_sha256": digest(raw),
                "result": candidate, "result_sha256": digest(result),
                "checker_sha256": digest(read(Path(business.__file__))),
                "evidence_mode": evidence_mode, "outcome": "accepted"}
    if receipt is not None:
        envelope["session"] = str(session)
    if ply_binding is not None:
        envelope["ply_verifier"] = ply_binding
    sys.stdout.buffer.write(encoded(envelope))
    return 0


if __name__ == "__main__":
    try:
        code = verify(sys.argv[2]) if len(sys.argv) == 3 and sys.argv[1] == "verify" else work()
    except Exception as e:
        print(f"worker: {e}", file=sys.stderr)
        code = 125 if isinstance(e, subprocess.TimeoutExpired) else 1
    sys.exit(code)
