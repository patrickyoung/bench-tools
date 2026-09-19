#!/usr/bin/env python3
"""Run a selected semantic checker only for complete, current visual observations."""
import argparse
import json
import pathlib
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
from visual_contract import MAX_JSON, digest, load_json, read_regular, validate_current


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--observations", required=True)
    parser.add_argument("--rubric", required=True)
    parser.add_argument("--checker", required=True, help="explicit semantic-check/check.py executable")
    parser.add_argument("arguments", nargs=argparse.REMAINDER, help="-- followed by checker options")
    args = parser.parse_args()
    extra = args.arguments
    if extra and extra[0] == "--":
        extra = extra[1:]
    if any(x in ("--", "--help", "-h") or (x.startswith("--") and
           any(name.startswith(x.split("=", 1)[0]) for name in ("--candidate", "--rubric"))) for x in extra):
        raise ValueError("candidate/rubric overrides and help are not checker runs")
    if "--backend" not in extra and not any(x.startswith("--backend=") for x in extra):
        raise ValueError("select the checker's --backend explicitly")
    observations = pathlib.Path(args.observations).absolute()
    rubric = pathlib.Path(args.rubric).absolute()
    original = read_regular(observations, MAX_JSON)
    rubric_data = read_regular(rubric, MAX_JSON)
    doc = load_json(original)
    raw = validate_current(doc, rubric_data)
    incomplete = [row for row in raw["observations"] if row["status"] != "observed"]
    if incomplete:
        print(json.dumps({"version": 1, "verdict": "reject", "feedback": [
            "%s / %s: %s. Supply another view or obtain manual review." %
            (row["criterion_id"], row["image_id"], row["status"]) for row in incomplete]}))
        return 1
    command = [args.checker, "--rubric", str(rubric), "--candidate", str(observations)] + extra
    # Do not expose a passing answer before verifying that its inputs stayed current.
    with tempfile.TemporaryFile() as out:
        result = subprocess.run(command, stdout=out)
        if read_regular(observations, MAX_JSON) != original or read_regular(rubric, MAX_JSON) != rubric_data:
            raise ValueError("observation document or rubric changed during checking")
        validate_current(doc, rubric_data)
        out.seek(0)
        data = out.read(MAX_JSON + 1)
        if len(data) > MAX_JSON:
            raise ValueError("checker output exceeds 1 MiB")
        if result.returncode in (0, 1):
            report = load_json(data)
            if (not isinstance(report, dict) or type(report.get("version")) is not int or report["version"] != 1
                    or report.get("verdict") != ("accept" if result.returncode == 0 else "reject")
                    or report.get("candidate_sha256") != digest(original)
                    or report.get("rubric_sha256") != digest(rubric_data)):
                raise ValueError("checker response does not match this candidate, rubric, and exit status")
            sys.stdout.buffer.write(data)
    return result.returncode if result.returncode >= 0 else 128 - result.returncode


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print("visual check: " + str(error), file=sys.stderr)
        sys.exit(2)
