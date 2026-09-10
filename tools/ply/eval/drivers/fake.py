#!/usr/bin/env python3
"""Deterministic harness self-test, deliberately not an agent implementation."""
import json
from pathlib import Path
import sys
import time
import subprocess

request = json.load(sys.stdin)
work, state = Path(request["workdir"]), Path(request["state_dir"])
mode = request["options"].get("behavior", "correct")
if (state / "used").exists():
    raise RuntimeError("state leaked between independent trials")
(state / "used").write_text("used")
if mode == "timeout":
    time.sleep(60)
if mode == "orphan":
    subprocess.Popen(["/bin/sh", "-c", 'trap "" INT TERM; while :; do printf x >> "$1"; sleep .03; done', "fixture", str(work / "ticks")],
                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
if mode == "tamper":
    oracle = work.parents[4] / "control" / "oracle.py"
    oracle.chmod(0o644)
    oracle.write_text("print('{\"passed\":true,\"detail\":\"forged\"}')\n")
if mode == "malformed":
    print("not JSON")
    sys.exit(0)
if mode == "correct":
    if request["case"] == "multi-file-repair":
        (work / "invoice.py").write_text("from discounts import apply\ndef total(price, quantity, discount=0):\n    if quantity < 0: raise ValueError('quantity')\n    return apply(round(price * 100) * quantity, discount)\n")
        (work / "discounts.py").write_text("def apply(cents, percent):\n    if not 0 <= percent <= 100: raise ValueError('percent')\n    return round(cents * (100 - percent) / 100)\n")
    elif request["case"] == "large-output":
        ids = [line.split()[1] for line in (work / "telemetry.log").read_text().splitlines() if line.startswith("ERROR ")]
        (work / "findings.json").write_text(json.dumps({"error_count": len(ids), "first_error": ids[0], "last_error": ids[-1]}))
    elif request["case"] == "ambiguous-request":
        (work / "clarification.json").write_text(json.dumps({"question": "Which audience and metric should this monthly report serve?"}))
print(json.dumps({"answer": "selftest fixture result", "claimed_success": mode == "liar" or request["case"] != "ambiguous-request",
                  "usage": None, "interventions": []}))
