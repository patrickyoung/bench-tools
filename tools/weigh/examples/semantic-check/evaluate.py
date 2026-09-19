#!/usr/bin/env python3
"""Compare explicitly selected checkers on labeled candidates, not model consensus.

This measures fixed-candidate judgments. It does not claim improved creative
output or repair-loop quality; run the documented Ply trials for that evidence.
"""
import argparse
import hashlib
import json
import math
from pathlib import Path
import statistics
import subprocess
import sys
import time


def percentile(values, fraction):
    ordered = sorted(values)
    return ordered[max(0, math.ceil(len(ordered) * fraction) - 1)] if ordered else None


def summary(rows):
    completed = [r for r in rows if r["exit"] in (0, 1)]
    positive_attempted = [r for r in rows if r["expected"] == "accept"]
    negative_attempted = [r for r in rows if r["expected"] == "reject"]
    positive = [r for r in completed if r["expected"] == "accept"]
    negative = [r for r in completed if r["expected"] == "reject"]
    false_passes = sum(r["exit"] == 0 for r in negative)
    false_rejections = sum(r["exit"] == 1 for r in positive)
    timings = [r["seconds"] for r in rows]
    costs = [r.get("cost") for r in rows]
    return {"cases": len(rows), "completed": len(completed), "errors": len(rows) - len(completed),
            "expected_accept_attempted": len(positive_attempted), "expected_accept_completed": len(positive),
            "expected_reject_attempted": len(negative_attempted), "expected_reject_completed": len(negative),
            "false_passes": false_passes, "false_pass_rate": false_passes / len(negative) if negative else None,
            "false_rejections": false_rejections, "false_rejection_rate": false_rejections / len(positive) if positive else None,
            "escalations": sum(r.get("escalated", False) for r in rows),
            "latency_p50_seconds": statistics.median(timings) if timings else None,
            "latency_p95_seconds": percentile(timings, .95),
            "reported_cost_total": sum(costs) if costs and all(c is not None for c in costs) else None,
            "reported_cost_known_subtotal": sum(r.get("cost_known_subtotal", r.get("cost") or 0) for r in rows),
            "cost_unknown_cases": sum(c is None for c in costs)}


def text_value(value):
    if not isinstance(value, str):
        return False
    try:
        value.encode("utf-8")
    except UnicodeEncodeError:
        return False
    return True


def nonempty_text(value):
    return text_value(value) and bool(value.strip())


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON key")
        result[key] = value
    return result


def reject_constant(_):
    raise ValueError("non-finite JSON number")


def parse_document(raw):
    return json.loads(raw.decode("utf-8"), object_pairs_hook=unique_object,
                      parse_constant=reject_constant)


def validate_inputs(cases, profiles):
    """Validate every profile and case before creating output or spending on one."""
    if (not isinstance(cases, dict) or set(cases) - {"version", "cases", "label_source"}
            or type(cases.get("version")) is not int or cases["version"] != 1
            or not isinstance(cases.get("cases"), list) or not cases["cases"]):
        raise ValueError("cases require version 1 and a nonempty cases array")
    if "label_source" in cases and not nonempty_text(cases["label_source"]):
        raise ValueError("label_source must be nonempty text when supplied")
    seen = set()
    for case in cases["cases"]:
        if (not isinstance(case, dict) or set(case) != {"id", "candidate", "expected", "label_basis"}
                or not nonempty_text(case["id"]) or not nonempty_text(case["label_basis"])
                or not text_value(case["candidate"]) or case["expected"] not in ("accept", "reject")):
            raise ValueError("each case needs a text ID, candidate text, expected accept/reject and a text label basis")
        if case["id"] in seen:
            raise ValueError("case IDs must be unique")
        seen.add(case["id"])
    if (not isinstance(profiles, dict) or set(profiles) != {"version", "profiles"}
            or type(profiles.get("version")) is not int or profiles["version"] != 1
            or not isinstance(profiles["profiles"], list) or not profiles["profiles"]):
        raise ValueError("profiles require version 1 and a nonempty profiles array")
    seen = set()
    base = {"name", "backend", "model"}
    for profile in profiles["profiles"]:
        if (not isinstance(profile, dict) or not base <= set(profile)
                or not nonempty_text(profile["name"]) or not nonempty_text(profile["model"])
                or profile["backend"] not in ("ask", "weigh")):
            raise ValueError("each profile needs nonempty name/model text and an ask or weigh backend")
        if profile["name"] in seen:
            raise ValueError("profile names must be unique")
        seen.add(profile["name"])
        if profile["backend"] == "ask":
            if set(profile) != base:
                raise ValueError("Ask profiles cannot contain Weigh thresholds, fallback or other options")
            continue
        required = base | {"accept_at", "reject_at"}
        if not required <= set(profile) or set(profile) - (required | {"fallback_model"}):
            raise ValueError("Weigh profiles require accept_at/reject_at and allow only an optional fallback_model")
        for key in ("accept_at", "reject_at"):
            value = profile[key]
            if type(value) not in (int, float) or not .5 < value <= 1 or not math.isfinite(value):
                raise ValueError("Weigh thresholds must be finite numbers greater than 0.5 and at most 1")
        if "fallback_model" in profile and not nonempty_text(profile["fallback_model"]):
            raise ValueError("fallback_model must be nonempty text when supplied")


def reported_cost(result, ask):
    """Keep known charges even when another stage leaves total cost unknown."""
    total = 0
    complete = True
    if not result.get("stages"):
        return {"cost": 0 if result.get("judgments") == {} else None, "cost_known_subtotal": 0}
    for stage in result["stages"]:
        if stage["backend"] == "weigh":
            cost = stage.get("metadata", {}).get("usage", {}).get("cost")
            amounts = [cost] if cost is not None else []
        else:
            checked = subprocess.run([ask, "replay", "-check", "-json", stage["session"]],
                                     stdin=subprocess.DEVNULL, capture_output=True, timeout=30)
            if checked.returncode:
                complete = False
                continue
            turns = [row for row in map(json.loads, checked.stdout.splitlines()) if row.get("type") == "assistant"]
            amounts = [row.get("data", {}).get("usage", {}).get("cost") for row in turns]
        if not amounts:
            complete = False
        for value in amounts:
            try:
                valid = type(value) in (int, float) and value >= 0 and math.isfinite(value)
            except OverflowError:
                valid = False
            if valid:
                total += value
            else:
                complete = False
    return {"cost": total if complete else None, "cost_known_subtotal": total}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", type=Path, required=True)
    parser.add_argument("--profiles", type=Path, required=True)
    parser.add_argument("--rubric", type=Path, required=True)
    parser.add_argument("--evidence", type=Path)
    parser.add_argument("--output", type=Path, required=True, help="new external results directory")
    parser.add_argument("--live", action="store_true", help="explicitly permit selected paid model calls")
    parser.add_argument("--checker", type=Path, default=Path(__file__).with_name("check.py"))
    parser.add_argument("--ask", default="ask")
    parser.add_argument("--weigh", default="weigh")
    parser.add_argument("--record", default="record")
    parser.add_argument("--endpoint", help="explicit Decisions endpoint, for compatibility fixtures")
    args = parser.parse_args()
    if not args.live:
        parser.error("execution requires --live; it may spend money. Offline contract tests use unittest instead")
    try:
        cases_raw, profiles_raw = args.cases.read_bytes(), args.profiles.read_bytes()
        cases, profiles = parse_document(cases_raw), parse_document(profiles_raw)
        rubric_raw = args.rubric.read_bytes()
        evidence_raw = args.evidence.read_bytes() if args.evidence else None
    except (OSError, ValueError, TypeError, OverflowError):
        parser.error("evaluation inputs must be readable; cases/profiles must be valid UTF-8 JSON without duplicate keys or non-finite numbers")
    try:
        validate_inputs(cases, profiles)
    except ValueError as error:
        parser.error(str(error))
    args.output.mkdir(parents=True, exist_ok=False)
    # Freeze all evaluated inputs, including labels, for later independent audit.
    rubric = args.output / "rubric.json"
    rubric.write_bytes(rubric_raw)
    (args.output / "cases.json").write_bytes(cases_raw)
    (args.output / "profiles.json").write_bytes(profiles_raw)
    evidence = None
    if args.evidence:
        evidence = args.output / "evidence.txt"
        evidence.write_bytes(evidence_raw)
    rows = []
    for pi, profile in enumerate(profiles["profiles"]):
        for ci, case in enumerate(cases["cases"]):
            directory = args.output / ("profile-%02d-case-%03d" % (pi, ci))
            directory.mkdir()
            candidate = directory / "candidate.txt"
            raw = case["candidate"].encode()
            candidate.write_bytes(raw)
            command = [sys.executable, str(args.checker.resolve()), "--backend", profile["backend"],
                       "--model", profile["model"], "--rubric", str(rubric.resolve()),
                       "--candidate", str(candidate.resolve()), "--records", str((directory / "records").resolve()),
                       "--ask", args.ask, "--weigh", args.weigh, "--record", args.record]
            for key in ("accept_at", "reject_at", "fallback_model"):
                if key in profile:
                    command += ["--" + key.replace("_", "-"), str(profile[key])]
            if args.endpoint and profile["backend"] == "weigh":
                command += ["--endpoint", args.endpoint]
            if evidence:
                command += ["--evidence", str(evidence.resolve())]
            started = time.monotonic()
            checked = subprocess.run(command, stdin=subprocess.DEVNULL, capture_output=True)
            elapsed = time.monotonic() - started
            (directory / "stdout.json").write_bytes(checked.stdout)
            (directory / "stderr.txt").write_bytes(checked.stderr)
            result = json.loads(checked.stdout) if checked.returncode in (0, 1) else {}
            costs = reported_cost(result, args.ask) if result else {"cost": None, "cost_known_subtotal": 0}
            row = {"profile": profile["name"], "case": case["id"], "expected": case["expected"],
                   "exit": checked.returncode, "seconds": elapsed, "escalated": result.get("escalated", False),
                   **costs,
                   "candidate_sha256": hashlib.sha256(raw).hexdigest(), "evidence": str(directory.resolve())}
            rows.append(row)
            print("%s / %s: %s" % (profile["name"], case["id"], "accept" if checked.returncode == 0 else "reject" if checked.returncode == 1 else "error"), flush=True)
            (args.output / "results.jsonl").write_text("".join(json.dumps(x) + "\n" for x in rows))
    report = {"version": 1, "scope": "fixed candidates; no claim of final-output or repair-loop improvement",
              "label_source": cases.get("label_source", "caller supplied"),
              "profiles": {p["name"]: summary([r for r in rows if r["profile"] == p["name"]]) for p in profiles["profiles"]}}
    (args.output / "summary.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))
    return 2 if any(r["exit"] not in (0, 1) for r in rows) else 0


if __name__ == "__main__":
    raise SystemExit(main())
