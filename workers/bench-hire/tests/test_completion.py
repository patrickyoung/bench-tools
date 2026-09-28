#!/usr/bin/env python3
"""Offline structural contract fixtures; no model or worker execution."""
import hashlib
import json
import subprocess
import tempfile
from pathlib import Path

CHECK = Path(__file__).resolve().parents[1] / "expert/task/bin/check"
base = {"stage": "present", "message": "Make a note", "history": [],
        "catalog": [], "source": "", "now": "", "execution":
        {"exit_code": 0, "stdout": "", "stderr": "", "files": []}}
count = 0


def case(label, changes, response, accept, hash_override=None):
    global count
    request = dict(base, **changes)
    with tempfile.TemporaryDirectory() as temp:
        folder = Path(temp)
        raw = json.dumps(request, ensure_ascii=False).encode()
        (folder / "request.json").write_bytes(raw)
        reply = dict(response, request_sha256=hash_override or hashlib.sha256(raw).hexdigest())
        (folder / "response.json").write_text(json.dumps(reply))
        run = subprocess.run([str(CHECK)], cwd=temp, capture_output=True, text=True)
        assert (run.returncode == 0) == accept, (label, run.returncode, run.stderr)
        count += 1


legacy = {"message": "Result recorded."}
complete = {"message": "Checked current output.", "status": "complete",
            "next_goal": "", "question": ""}
cont = dict(complete, status="continue", next_goal="Fix the omitted section.")
need = dict(complete, status="needs_input", question="Which date is intended?")
blocked = dict(complete, status="blocked", message="Required asset is inaccessible.")
case("legacy", {}, legacy, True)
case("legacy extra", {}, complete, False)
for label, reply, code in (("complete", complete, 0), ("continue zero", cont, 0),
                           ("continue two", cont, 2), ("needs input", need, None),
                           ("blocked", blocked, 75)):
    case(label, {"completion_version": 1, "execution": dict(base["execution"], exit_code=code)}, reply, True)
for code in (None, True, False, 1, 2, 3, 75, 125, 130, 137, 99):
    case("complete invalid exit " + str(code), {"completion_version": 1,
         "execution": dict(base["execution"], exit_code=code)}, complete, False)
for code in (None, True, False, 1, 3, 75, 125, 130, 137, 99):
    case("continue unsafe " + str(code), {"completion_version": 1,
         "execution": dict(base["execution"], exit_code=code)}, cont, False)
for version in (None, True, False, 0, 2, "1"):
    case("unsupported version " + str(version), {"completion_version": version}, complete, False)
for label, reply in (
    ("missing status", {k: v for k, v in complete.items() if k != "status"}),
    ("bad status", dict(complete, status="retry")),
    ("wrong question", dict(complete, question="Ask again?")),
    ("inactive whitespace question", dict(complete, question=" ")),
    ("inactive whitespace goal", dict(blocked, next_goal=" ")),
    ("missing question", dict(need, question="")),
    ("next goal on blocked", dict(blocked, next_goal="Run it again.")),
    ("no next goal", dict(cont, next_goal="")),
    ("blank next goal", dict(cont, next_goal="  ")),
    ("blank question", dict(need, question=" ")),
    ("long message", dict(complete, message="x" * 6001)),
    ("empty message", dict(complete, message="")),
    ("long goal", dict(cont, next_goal="x" * 16001)),
    ("long question", dict(need, question="x" * 1001)),
    ("nonstring", dict(complete, status=0)),
    ("legacy under version", legacy),
):
    case(label, {"completion_version": 1}, reply, False)
case("limits boundary", {"completion_version": 1}, dict(cont, message="m"*6000, next_goal="g"*16000), True)
case("question boundary", {"completion_version": 1}, dict(need, question="q"*1000), True)
case("wrong hash", {"completion_version": 1}, complete, False, hash_override="0"*64)
print(f"{count} offline cases passed")
