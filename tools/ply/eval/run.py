#!/usr/bin/env python3
"""Repeated paired trials through external executables; no product runtime policy."""
import argparse
import contextlib
import hashlib
import itertools
import json
import math
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time

from corpus import CASES, materialize
from fixture import Fixture

HERE = Path(__file__).resolve().parent
LIMIT = 8 * 1024 * 1024
GRACE_SECONDS = 3
USAGE_FIELDS = ("input_tokens", "output_tokens", "cached_input_tokens", "cache_write_tokens", "reasoning_tokens", "cost_usd",
                "observed_input_tokens", "observed_output_tokens", "observed_cached_input_tokens", "observed_cache_write_tokens", "observed_reasoning_tokens", "observed_cost_usd")


def encoded(value):
    return (json.dumps(value, sort_keys=True, allow_nan=False) + "\n").encode()


def digest(data):
    return "sha256:" + hashlib.sha256(data).hexdigest()


def file_digest(path):
    value = hashlib.sha256()
    with Path(path).open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            value.update(block)
    return "sha256:" + value.hexdigest()


def group_exists(process):
    try:
        os.killpg(process.pid, 0)
        return True
    except ProcessLookupError:
        return False


def tree_digest(root):
    rows = []
    for path in sorted(Path(root).rglob("*")):
        if path.is_symlink():
            raise ValueError("symlink in evidence tree: " + str(path))
        if path.is_file():
            rows.append([path.relative_to(root).as_posix(), path.stat().st_mode & 0o777, digest(path.read_bytes())])
    return digest(encoded(rows))


def write(path, value):
    Path(path).write_bytes(encoded(value))


def load_drivers(path):
    drivers = json.loads(Path(path).read_text())
    if not isinstance(drivers, list) or not drivers:
        raise ValueError("drivers must be a nonempty JSON array")
    names = set()
    for driver in drivers:
        if set(driver) - {"name", "argv", "model", "budget", "options", "suites", "pass_env", "files"}:
            raise ValueError("unknown driver configuration field")
        name, argv = driver.get("name"), driver.get("argv")
        if not isinstance(name, str) or not name or any(c not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-" for c in name) or name in names:
            raise ValueError("driver names must be unique simple identifiers")
        names.add(name)
        if not isinstance(argv, list) or not argv or any(not isinstance(a, str) for a in argv):
            raise ValueError("driver argv must be an executable argument array")
        executable = shutil.which(argv[0])
        if executable is None:
            raise ValueError("driver executable unavailable: " + argv[0])
        argv[0] = str(Path(executable).resolve())
        if not isinstance(driver.get("model"), str) or not driver["model"]:
            raise ValueError("every driver must identify its model")
        budget = driver.get("budget", {})
        timeout = budget.get("wall_seconds")
        if not isinstance(timeout, (int, float)) or isinstance(timeout, bool) or not math.isfinite(timeout) or timeout <= 0:
            raise ValueError("every driver must provide a positive budget.wall_seconds")
        driver.setdefault("options", {})
        driver.setdefault("suites", ["task"])
        if not driver["suites"] or set(driver["suites"]) - {"task", "lifecycle"}:
            raise ValueError("driver suites must select task and/or lifecycle")
        driver.setdefault("pass_env", [])
        if any(not isinstance(n, str) or not n.isidentifier() for n in driver["pass_env"]):
            raise ValueError("pass_env contains an invalid environment name")
        paths = [argv[0], *driver.get("files", []), *(a for a in argv[1:] if Path(a).is_file())]
        driver["dependencies"] = {str(Path(p).resolve()): digest(Path(p).read_bytes()) for p in paths}
    return drivers


def dependencies_unchanged(driver):
    return all(Path(path).is_file() and digest(Path(path).read_bytes()) == expected
               for path, expected in driver["dependencies"].items())


def driver_environment(driver, state):
    for child in ("home", "tmp", "config", "cache", "data"):
        (state / child).mkdir(exist_ok=True)
    env = {"PATH": os.environ.get("PATH", os.defpath), "LANG": "C.UTF-8", "PYTHONDONTWRITEBYTECODE": "1"}
    env.update({name: os.environ[name] for name in driver["pass_env"] if name in os.environ})
    env.update({"HOME": str(state / "home"), "TMPDIR": str(state / "tmp"), "XDG_CONFIG_HOME": str(state / "config"),
                "XDG_CACHE_HOME": str(state / "cache"), "XDG_DATA_HOME": str(state / "data")})
    return env


def terminate(process):
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    # The leader may exit before a redirected descendant. Retain the group
    # escalation deadline independently of the leader's wait result.
    deadline = time.monotonic() + GRACE_SECONDS
    while time.monotonic() < deadline:
        process.poll()  # Reap the leader without cancelling group escalation.
        time.sleep(0.02)
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    process.wait(timeout=5)


def validate_result(result):
    if not isinstance(result, dict) or set(result) - {"answer", "claimed_success", "usage", "interventions", "exit_code"}:
        raise ValueError("driver result has unknown fields or is not an object")
    if not isinstance(result.get("answer"), str) or (result.get("claimed_success") is not None and type(result["claimed_success"]) is not bool):
        raise ValueError("driver must report answer and a boolean/null success claim")
    usage = result.get("usage")
    if usage is not None:
        if not isinstance(usage, dict) or set(usage) - set(USAGE_FIELDS) - {"observed_turns", "unreported_turns", "unreported_requests", "unreported_retries", "partial_turns", "summary_turns", "session_count", "source"}:
            raise ValueError("invalid usage observation")
        for field, value in usage.items():
            if field == "source":
                continue
            if value is not None and (isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or value < 0):
                raise ValueError("usage values must be nonnegative measured numbers or null")
        if not isinstance(usage.get("source"), str) or not usage["source"]:
            raise ValueError("usage must identify its measurement source")
    interventions = result.get("interventions", [])
    if not isinstance(interventions, list) or any(not isinstance(e, dict) or not isinstance(e.get("kind"), str) for e in interventions):
        raise ValueError("interventions must be typed event objects")
    return result


def attempt(driver, request, directory, fixture, cancel, deadline):
    number = 2 if request["resume"] else 1
    input_path, output_path, log_path = [directory / f"attempt-{number}.{suffix}" for suffix in ("json", "stdout", "stderr")]
    write(input_path, request)
    started = time.monotonic()
    reason = None
    with input_path.open("rb") as inp, output_path.open("wb") as out, log_path.open("wb") as err:
        env = driver_environment(driver, Path(request["state_dir"]))
        if time.monotonic() >= deadline:
            return {"exit_code": None, "elapsed_seconds": time.monotonic() - started, "stop": "timeout",
                    "result": None, "protocol_error": "wall-clock budget exhausted before launch",
                    "stdout_sha256": digest(b""), "stderr_sha256": digest(b"")}
        process = subprocess.Popen(driver["argv"], stdin=inp, stdout=out, stderr=err, cwd=request["workdir"],
                                   env=env, start_new_session=True)
        try:
            while process.poll() is None:
                if output_path.stat().st_size > LIMIT or log_path.stat().st_size > LIMIT:
                    reason = "capture_limit"
                elif time.monotonic() >= deadline:
                    reason = "timeout"
                elif cancel and ((Path(request["workdir"]) / ".ready").exists()
                                 or any(e["kind"] == "effect_committed" for e in fixture.snapshot())):
                    reason = "scenario_cancel"
                if reason:
                    terminate(process)
                    break
                time.sleep(0.02)
            if reason is None and group_exists(process):
                # A complete driver must not leave writers alive while
                # the independent oracle inspects its resulting state.
                reason = "orphaned_descendants"
                terminate(process)
        except BaseException:
            terminate(process)
            raise
    result, error = None, None
    try:
        if output_path.stat().st_size > LIMIT:
            raise ValueError("driver stdout exceeded capture limit")
        result = validate_result(json.loads(output_path.read_bytes()))
    except (ValueError, UnicodeError) as invalid:
        error = str(invalid)
    return {"exit_code": process.returncode, "elapsed_seconds": time.monotonic() - started,
            "stop": reason, "result": result, "protocol_error": error,
            "stdout_sha256": file_digest(output_path), "stderr_sha256": file_digest(log_path)}


def evaluate_oracle(args, directory, timeout=15):
    """An oracle may execute candidate code; bound its entire owned group too."""
    out_path, err_path = directory / "oracle.stdout", directory / "oracle.stderr"
    reason = None
    with out_path.open("wb") as out, err_path.open("wb") as err:
        process = subprocess.Popen(args, stdout=out, stderr=err, start_new_session=True,
                                   env={"PATH": os.defpath, "PYTHONDONTWRITEBYTECODE": "1"})
        deadline = time.monotonic() + timeout
        try:
            while process.poll() is None:
                if time.monotonic() >= deadline:
                    reason = "timeout"
                elif out_path.stat().st_size > LIMIT or err_path.stat().st_size > LIMIT:
                    reason = "capture_limit"
                if reason:
                    terminate(process)
                    break
                time.sleep(.02)
            if reason is None and group_exists(process):
                reason = "orphaned_descendants"
                terminate(process)
        except BaseException:
            terminate(process)
            raise
    if reason:
        return {"passed": None, "detail": "oracle " + reason}
    try:
        if out_path.stat().st_size > LIMIT or err_path.stat().st_size > LIMIT:
            raise ValueError("oracle capture limit")
        result = json.loads(out_path.read_bytes())
        if process.returncode != 0 or set(result) != {"passed", "detail"} or type(result["passed"]) is not bool:
            raise ValueError("invalid oracle result")
        return result
    except (ValueError, TypeError):
        with err_path.open("rb") as error_log:
            detail = error_log.read(LIMIT).decode(errors="replace")[-1000:]
        return {"passed": None, "detail": "oracle failed: " + detail}


def trial(driver, case_id, repetition, root, control_hash):
    case = CASES[case_id]
    directory = root / "trials" / case_id / str(repetition) / driver["name"]
    directory.mkdir(parents=True)
    work, state = directory / "work", directory / "state"
    state.mkdir()
    shutil.copytree(root / "control" / "cases" / case_id, work)
    for path in work.rglob("*"):
        if path.is_file():
            path.chmod(0o644)
    initial = tree_digest(work)
    base = {"schema": "ply.eval/trial/v1", "case": case_id, "suite": case["suite"], "repetition": repetition,
            "driver": driver["name"], "model": driver["model"] if case["suite"] == "task" else "scripted-messages-fixture", "configured_model": driver["model"], "budget": driver["budget"], "options": driver["options"],
            "initial_sha256": initial, "oracle_sha256": control_hash}
    if case["suite"] not in driver["suites"]:
        return {**base, "status": "unsupported", "reason": "driver did not declare this suite"}
    manager = Fixture(case_id) if case["suite"] == "lifecycle" else contextlib.nullcontext(None)
    started, attempts, events = time.monotonic(), [], []
    deadline = started + driver["budget"]["wall_seconds"]
    with manager as fixture:
        request = {"schema": "ply.eval/request/v1", "case": case_id, "suite": case["suite"], "prompt": case["prompt"] +
                   "\nEnd your report with a line EVAL_SUCCESS only if you claim the full requested work is complete. A clarification request is not a completion claim.",
                   "workdir": str(work), "state_dir": str(state), "model": driver["model"], "budget": driver["budget"],
                   "options": driver["options"], "resume": False, "fixture_url": fixture.url if fixture else None}
        should_cancel = case_id in ("cancellation-resume", "uncertain-external-effect")
        if not dependencies_unchanged(driver):
            raise ValueError("driver dependencies changed after capture")
        attempts.append(attempt(driver, request, directory, fixture, should_cancel, deadline))
        if attempts[-1]["stop"] == "scenario_cancel":
            events.append({"kind": "cancel", "origin": "harness", "reason": "scenario", "time_ns": time.monotonic_ns()})
            request["resume"] = True
            events.append({"kind": "resume" if time.monotonic() < deadline else "resume_skipped_budget",
                           "origin": "harness", "time_ns": time.monotonic_ns()})
            attempts.append(attempt(driver, request, directory, fixture, False, deadline))
        if fixture:
            events.extend(fixture.snapshot())
    events.sort(key=lambda event: event["time_ns"])
    last = attempts[-1]
    result = last["result"] or {}
    claim = result.get("claimed_success")
    events.extend({**event, "origin": "driver"} for event in result.get("interventions", []))
    evidence = {"events": events, "claimed_success": claim}
    evidence_path = directory / "evidence.json"
    write(evidence_path, evidence)
    integrity = tree_digest(root / "control") == control_hash and dependencies_unchanged(driver)
    oracle = {"passed": None, "detail": "controller or driver integrity failed"}
    if integrity:
        oracle = evaluate_oracle([sys.executable, str(root / "control" / "oracle.py"), case_id, str(work), str(evidence_path)], directory)
        integrity = tree_digest(root / "control") == control_hash and dependencies_unchanged(driver)
    status = "measured" if integrity and last["protocol_error"] is None and last["stop"] is None and oracle["passed"] is not None else "invalid"
    record = {**base, "status": status, "elapsed_seconds": time.monotonic() - started, "attempts": attempts,
              "oracle": oracle, "claimed_success": claim, "false_success": (not oracle["passed"] if claim is True and oracle["passed"] is not None else False if claim is False else None),
              "usage": result.get("usage"), "interventions": events, "integrity_verified": integrity}
    write(directory / "result.json", record)
    return record


def summarize(records):
    by_case, pairs = {}, []
    for case in sorted({r["case"] for r in records}):
        rows = [r for r in records if r["case"] == case]
        by_case[case] = {}
        drivers = sorted({r["driver"] for r in rows})
        for name in drivers:
            selected = [r for r in rows if r["driver"] == name]
            measured = [r for r in selected if r["status"] == "measured"]
            usage = {}
            for field in USAGE_FIELDS:
                values = [(r.get("usage") or {}).get(field) for r in measured]
                usage[field] = {"observations": values, "sum": sum(values) if values and all(v is not None for v in values) else None}
            counts = {}
            for r in measured:
                for event in r["interventions"]:
                    key = event["origin"] + "." + event["kind"]
                    counts[key] = counts.get(key, 0) + 1
            by_case[case][name] = {"trials": len(selected), "measured": len(measured),
                                  "oracle_passes": sum(r["oracle"]["passed"] for r in measured),
                                  "false_successes": sum(r["false_success"] is True for r in measured),
                                  "unknown_claims": sum(r["claimed_success"] is None for r in measured),
                                  "elapsed_seconds": [r["elapsed_seconds"] for r in measured], "usage": usage,
                                  "intervention_counts": {"driver_observed": sum(v for k, v in counts.items() if k.startswith("driver.")),
                                                          "harness_cancel": counts.get("harness.cancel", 0), "harness_resume": counts.get("harness.resume", 0)},
                                  "events_by_origin_and_kind": counts}
        for left, right in itertools.combinations(drivers, 2):
            paired = {"case": case, "left": left, "right": right, "both_pass": 0, "left_only": 0, "right_only": 0, "neither": 0, "unpaired": 0}
            for repetition in sorted({r["repetition"] for r in rows}):
                selected = {r["driver"]: r for r in rows if r["repetition"] == repetition}
                a, b = selected[left], selected[right]
                if a["initial_sha256"] != b["initial_sha256"]:
                    raise ValueError("paired trials started from different fixture bytes")
                if a["status"] != "measured" or b["status"] != "measured":
                    paired["unpaired"] += 1
                    continue
                passed = a["oracle"]["passed"], b["oracle"]["passed"]
                paired[{(True, True): "both_pass", (True, False): "left_only", (False, True): "right_only", (False, False): "neither"}[passed]] += 1
            pairs.append(paired)
    return {"schema": "ply.eval/summary/v1", "cases": by_case, "pairs": pairs,
            "interpretation": "Per-case paired observations only; lifecycle fixtures are scripted, not model-quality evidence. No cross-suite aggregate or parity claim."}


def run(config, destination, repetitions, selected):
    if repetitions < 1:
        raise ValueError("repetitions must be positive")
    drivers = load_drivers(config)
    root = Path(destination).resolve()
    root.mkdir(parents=True, exist_ok=False)
    control = root / "control"
    control.mkdir()
    for name in ("oracle.py", "corpus.py"):
        shutil.copyfile(HERE / name, control / name)
    for case_id in selected:
        materialize(case_id, control / "cases" / case_id)
    # Files are operator-owned and read-only; hashes detect edits. This does
    # not claim to sandbox a malicious same-user worker.
    for path in control.rglob("*"):
        if path.is_file():
            path.chmod(0o444)
    control_hash = tree_digest(control)
    write(root / "manifest.json", {"schema": "ply.eval/run/v1", "drivers": drivers, "cases": selected,
                                  "repetitions": repetitions, "control_sha256": control_hash,
                                  "harness_sha256": digest(Path(__file__).read_bytes()), "fixture_sha256": digest((HERE / "fixture.py").read_bytes())})
    records = []
    with (root / "results.jsonl").open("ab") as log:
        for repetition in range(1, repetitions + 1):
            for index, case in enumerate(selected):
                offset = (repetition + index - 1) % len(drivers)
                for driver in drivers[offset:] + drivers[:offset]:
                    record = trial(driver, case, repetition, root, control_hash)
                    records.append(record)
                    log.write(encoded(record))
                    log.flush()
                    os.fsync(log.fileno())
                    print(f"{case} repetition {repetition} {driver['name']}: {record['status']} " + str(record.get("oracle", {}).get("passed", "")), file=sys.stderr)
    summary = summarize(records)
    write(root / "summary.json", summary)
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--drivers", required=True, help="explicit JSON driver configuration")
    parser.add_argument("--out", required=True, help="new result directory; existing runs are never replayed implicitly")
    parser.add_argument("--repetitions", type=int, default=3)
    parser.add_argument("--suite", choices=("task", "lifecycle", "all"), default="task")
    parser.add_argument("--case", action="append", choices=sorted(CASES))
    args = parser.parse_args()
    selected = args.case or [name for name, case in CASES.items() if args.suite in ("all", case["suite"])]
    try:
        summary = run(args.drivers, args.out, args.repetitions, selected)
        print(json.dumps(summary, sort_keys=True))
        return 0
    except (ValueError, OSError, subprocess.TimeoutExpired) as error:
        print("eval: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
