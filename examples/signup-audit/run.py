#!/usr/bin/env python3
"""Submit an offline signup audit to a local Tend queue and inspect its result."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("signups", nargs="?", type=Path, default=Path("signups.txt"))
    parser.add_argument("--job-id", default="signup-audit-001")
    args = parser.parse_args()
    env = dict(os.environ, TEND_ROOT=str(Path("queue").resolve()))

    def tend(*arguments, data=None, check=True):
        return subprocess.run(["tend", *arguments], input=data, stdout=subprocess.PIPE,
                              env=env, check=check, timeout=30)

    try:
        data = args.signups.read_bytes()
        worker = Path(__file__).resolve().parent / "audit.py"
        job_id = tend("submit", "-id", args.job_id, "--", sys.executable, str(worker),
                      data=data).stdout.decode().strip()
        record = json.loads(tend("show", job_id).stdout)
        if record["status"] == "ready":
            tend("work")
            record = json.loads(tend("show", job_id).stdout)
        if record["status"] != "done":
            raise ValueError(f"Job {job_id} is {record['status']}; inspect it before continuing")
        tend("check")
        events = [json.loads(line) for line in tend("events", job_id).stdout.splitlines()]
        finished = [event["payload"] for event in events if event["kind"] == "attempt.finished"]
        if not finished or finished[-1]["status"] != "done":
            raise ValueError("No observed successful attempt; inspect the job record")
        prepared = [event["payload"] for event in events if event["kind"] == "attempt.prepared"
                    and event["payload"]["attempt"] == finished[-1]["attempt"]]
        if len(prepared) != 1:
            raise ValueError("Cannot identify the successful attempt")
        number = prepared[0]["number"]
        output = Path(record["run_dir"]) / "attempts" / f"{number:03d}.out"
        print(output.read_text(encoding="utf-8"), end="")
        print(f"Job: {job_id}; queue: {env['TEND_ROOT']}", file=sys.stderr)
    except (OSError, UnicodeError, ValueError, KeyError, subprocess.SubprocessError) as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
