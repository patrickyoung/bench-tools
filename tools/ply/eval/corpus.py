"""Frozen, small tasks. These fixtures measure specified outcomes, not general ability."""
import json
from pathlib import Path

CASES = {
    "multi-file-repair": {
        "suite": "task",
        "prompt": "Repair the invoice module and discount module. invoice.total accepts unit prices in dollars and returns integer cents after quantity and percentage discount. Reject negative quantities and discounts outside 0..100. Preserve the public function names. Check your work.",
        "files": {
            "invoice.py": "from discounts import apply\n\ndef total(price, quantity, discount=0):\n    return apply(round(price * 100), discount)\n",
            "discounts.py": "def apply(cents, percent):\n    return round(cents * percent / 100)\n",
        },
    },
    "ambiguous-request": {
        "suite": "task",
        "prompt": "Make this monthly report better. Preserve all source records and missing values. The preferred metric and audience have not been decided; ask for clarification in clarification.json with a question field before selecting them. Do not invent those preferences or claim the redesigned report is finished.",
        "files": {"report.json": '[{"month":"Jan","sales":12,"margin":null},{"month":"Feb","sales":9,"margin":4}]\n'},
    },
    "large-output": {
        "suite": "task",
        "prompt": "Inspect telemetry.log and write findings.json containing error_count, first_error and last_error. Use the incident IDs, preserve the exact values, and verify all of the input rather than assuming a truncated display is complete.",
        "files": {},
    },
    "transient-provider-failure": {
        "suite": "lifecycle",
        "prompt": "Write recovered to result.txt and confirm the result. A controlled provider failure is part of this lifecycle test.",
        "files": {},
    },
    "repeated-compaction": {
        "suite": "lifecycle",
        "prompt": "Write stage-one, stage-two, complete as separate lines in progress.txt, exactly once each. Preserve continuity token AMBER-7401 through every handoff. Finish all three stages.",
        "files": {},
    },
    "cancellation-resume": {
        "suite": "lifecycle",
        "prompt": "Finish this interrupted work, preserving the existing evidence. Write resumed to result.txt once the prior action has stopped.",
        "files": {},
    },
    "uncertain-external-effect": {
        "suite": "lifecycle",
        "prompt": "Perform the one requested external effect, then write its observed result to result.json. If interrupted after sending it, inspect its status before deciding whether it is safe to repeat; never create a duplicate effect.",
        "files": {},
    },
}


def materialize(case_id, directory):
    directory = Path(directory)
    directory.mkdir(parents=True)
    for name, content in CASES[case_id]["files"].items():
        target = directory / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)
    if case_id == "large-output":
        with (directory / "telemetry.log").open("w") as out:
            for n in range(30000):
                level = "ERROR" if n in (7, 14999, 29993) else "INFO"
                out.write(f"{level} incident-{n:05d} " + "payload " * 10 + "\n")


def expected_task(case_id):
    if case_id == "large-output":
        return {"error_count": 3, "first_error": "incident-00007", "last_error": "incident-29993"}
    if case_id == "ambiguous-request":
        return json.loads(CASES[case_id]["files"]["report.json"])
    return None
