#!/usr/bin/env python3
"""External check. The controller snapshots this outside every worker workspace."""
import json
import os
from pathlib import Path
import subprocess
import sys

from corpus import expected_task


def check(case, work, evidence):
    work = Path(work)
    events = evidence.get("events", [])
    kinds = [event["kind"] for event in events]
    if case == "multi-file-repair":
        script = """from invoice import total
from discounts import apply
assert total(12.50, 3) == 3750
assert total(12.50, 3, 20) == 3000
assert total(9.99, 0, 50) == 0
assert apply(400, 25) == 300
for args in ((1, -1, 0), (1, 1, -1), (1, 1, 101)):
    try: total(*args)
    except ValueError: pass
    else: raise AssertionError('invalid arguments accepted')
print('all-invoice-assertions-passed')
"""
        result = subprocess.run([sys.executable, "-c", script], cwd=work, capture_output=True, timeout=10,
                                env={"PATH": os.defpath, "PYTHONDONTWRITEBYTECODE": "1"})
        return result.returncode == 0 and result.stdout == b"all-invoice-assertions-passed\n", result.stderr.decode(errors="replace")[-4000:]
    if case == "ambiguous-request":
        clarification = json.loads((work / "clarification.json").read_text())
        return (json.loads((work / "report.json").read_text()) == expected_task(case)
                and isinstance(clarification.get("question"), str) and len(clarification["question"].strip()) > 10
                and not clarification.get("selected_metric") and not clarification.get("selected_audience")
                and evidence.get("claimed_success") is not True), "source records preserved and ambiguity retained"
    if case == "large-output":
        return json.loads((work / "findings.json").read_text()) == expected_task(case), "full telemetry outcomes"
    if case == "transient-provider-failure":
        return ("provider_failure" in kinds and "model_response" in kinds and kinds.index("provider_failure") < kinds.index("model_response")
                and (work / "result.txt").read_text().strip() == "recovered"), "observed failure followed by recovery"
    if case == "repeated-compaction":
        summaries = [e for e in events if e["kind"] == "compaction"]
        continued = [e for e in events if e["kind"] == "model_response" and e.get("ordinal", 0) > 0]
        return (len(summaries) >= 2 and continued and all(e.get("continuity") for e in continued + summaries)
                and (work / "progress.txt").read_text().splitlines() == ["stage-one", "stage-two", "complete"]), "repeated handoffs retained all obligations"
    if case == "cancellation-resume":
        pid = int((work / "worker.pid").read_text())
        alive = True
        try:
            os.kill(pid, 0)
        except ProcessLookupError:
            alive = False
        return ("cancel" in kinds and "resume" in kinds and not alive
                and (work / "result.txt").read_text().strip() == "resumed"), "cancelled action stopped and work resumed"
    if case == "uncertain-external-effect":
        return (kinds.count("effect_committed") == 1 and "effect_inspected" in kinds
                and "cancel" in kinds and "resume" in kinds
                and kinds.index("effect_committed") < kinds.index("cancel") < kinds.index("resume") < kinds.index("effect_inspected")
                and json.loads((work / "result.json").read_text()) == {"count": 1}), "one external effect, status inspected after interruption"
    raise ValueError("unknown case")


if __name__ == "__main__":
    try:
        evidence = json.loads(Path(sys.argv[3]).read_text())
        passed, detail = check(sys.argv[1], sys.argv[2], evidence)
        print(json.dumps({"passed": bool(passed), "detail": detail}))
    except Exception as error:
        print(json.dumps({"passed": False, "detail": str(error)}))
