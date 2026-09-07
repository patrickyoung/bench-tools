#!/usr/bin/env python3
"""Bounded intervention search: finite rounds, verified evidence, frozen final test."""
import argparse
import fcntl
import io
import json
import os
from pathlib import Path
import random
import subprocess
import sys
import time
from types import SimpleNamespace

import research_domain as domain
import research_tasks
import run as recipe
from protocol import command, digest, encoded, exact, json_rows, read, replace

HERE = Path(__file__).resolve().parent
ARMS = ("search", "single", "team")
ROLES = ("Examine flow and queue order.", "Examine receipt handling and total effort.",
         "Examine missed exceptions, service, and stress failures.")
SOURCES = ("research.py", "research_tasks.py", "research_domain.py", "run.py", "worker.py", "protocol.py")


def task(identity, kind, needs=(), **data):
    return {"id": identity, "needs": list(needs), "input": {"kind": kind, **data}}


def feasible(result):
    return result["development"]["feasible"] and result["validation"]["feasible"]


def eligible(result):
    return result["development"]["eligible"] and result["validation"]["eligible"]


def selection_key(result):
    # Every arm uses the same predeclared scorer. Never score proposal prose.
    return (not eligible(result), not feasible(result),
            *domain.rank_key(result["validation"]), encoded(result["candidate"]))


def feedback_record(slot, result, observation, proposal=None):
    row = {"slot": slot, "state": observation["state"],
           "evidence": observation.get("evidence", []),
           "result_sha256": observation.get("result_sha256")}
    if result is not None:
        row.update({"candidate": result["candidate"], "eligible": eligible(result),
                    "development": result["development"], "validation": result["validation"]})
    if proposal is not None:
        row["hypothesis"] = proposal["hypothesis"]
        row["expected_tradeoff"] = proposal["expected_tradeoff"]
    return row


def public_feedback(history):
    # Feed measured results, not transcripts or local paths, to the next round.
    rows = []
    for index, row in enumerate(history):
        recent = index >= len(history) - 6
        item = {k: row[k] for k in ("slot", "state", "candidate", "eligible") if k in row}
        if recent:
            for key in ("hypothesis", "expected_tradeoff"):
                if key in row:
                    raw = row[key].encode()
                    item[key] = raw[:256].decode(errors="ignore") + ("…" if len(raw) > 256 else "")
        for split in ("development", "validation"):
            if split in row:
                score = public_score(row[split])
                item[split] = score if recent else {k: score[k] for k in ("feasible", "eligible", "objective", "relative")}
        rows.append(item)
    return rows


def public_score(result):
    """Keep the measured decision evidence without repeating baseline rows."""
    item = {k: result[k] for k in ("feasible", "relative_improved", "eligible", "objective", "relative")}
    metrics = ("median_cycle_minutes", "p95_cycle_minutes", "on_time_pct", "rework_pct",
               "labor_minutes", "backlog_at_horizon")
    item["scenarios"] = [{"id": row["id"], "metrics": {k: row["metrics"][k] for k in metrics},
                           "feasibility": row["feasibility"], "relative": row["relative"]}
                          for row in result["scenarios"]]
    return item


def round_plan(root, benchmark_binding, arm, start, count, configs, evidence, baseline, history):
    tasks = []
    for offset in range(count):
        slot = start + offset
        identity = f"experiment-{slot:03d}"
        data = dict(benchmark_binding)
        if arm == "search":
            data["candidate"] = configs[slot - 1]
            needs = []
        else:
            proposal = f"proposal-{slot:03d}"
            role = ROLES[offset % len(ROLES)] if arm == "team" else "Evaluate all process tradeoffs as one researcher."
            tasks.append(task(proposal, "review", slot=slot, role=role,
                              public_evidence=evidence,
                              baseline={"candidate": baseline["candidate"],
                                        **{k: public_score(baseline[k]) for k in ("development", "validation")}},
                              feedback=public_feedback(history)))
            data["proposal"] = proposal
            needs = [proposal]
        tasks.append(task(identity, "evaluate", needs, **data))
    return tasks


def run_round(path, tasks, args, deadline):
    path.mkdir(parents=True, exist_ok=True)
    with open(path / ".lock", "a+b") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        plan, tools = recipe.prepare_plan(path, "invoice-research", tasks, args.mode, args.model,
                                          HERE / "research_tasks.py", {"research_domain.py": HERE / "research_domain.py"})
        env = os.environ.copy()
        env.update({"TEND_ROOT": str(path / "tend"), "TEND_JOB_MAX": "5m", "TEND_LEASE": "3s"})
        if args.mode == "model":
            env.update({"ASK": tools["ask"], "PLY": tools["ply"]})
        observations, results, jobs = recipe.observe(path, plan, tools, env, research_tasks)
        states = {o["id"]: o["state"] for o in observations}
        # A failed proposal consumes its slot. Its never-admitted dependent is
        # terminally blocked, including when rebuilding after the deadline.
        while True:
            blocked = [t["id"] for t in plan if t["id"] not in states and
                       any(states.get(p) in ("rejected", "blocked") for p in t["needs"])]
            if not blocked:
                break
            states.update({identity: "blocked" for identity in blocked})
        unfinished = len(states) != len(plan) or any(s in ("queued", "running") for s in states.values())
        if "unknown" in states.values():
            raise RuntimeError("unknown execution outcome; inspect Tend, never auto-retry: " + str(path))
        if unfinished:
            remaining = deadline - time.time()
            if remaining <= 0:
                raise InterruptedError("frozen experiment deadline expired")
            options = SimpleNamespace(seconds=max(1, int(remaining)), jobs=args.team_size,
                                      mode=args.mode, model=args.model)
            sink = io.BytesIO()
            code = recipe.drive(options, path, plan, tools, env, research_tasks, sink)
            if code not in (0, 1):
                raise RuntimeError("round controller failed: " + str(path))
            observations, results, jobs = recipe.observe(path, plan, tools, env, research_tasks)
        states = {o["id"]: o for o in observations}
        if any(o["state"] in ("unknown", "running", "queued") for o in observations):
            raise RuntimeError("round has unresolved execution; inspect Tend: " + str(path))
        while len(states) != len(plan):
            blocked = [t for t in plan if t["id"] not in states and
                       any(states.get(p, {}).get("state") in ("rejected", "blocked") for p in t["needs"])]
            if not blocked:
                raise RuntimeError("unexplained blocked task in " + str(path))
            states.update({t["id"]: {"id": t["id"], "state": "blocked", "needs": t["needs"]} for t in blocked})
        usage = {"requests": 0, "retries": 0, "input_tokens": 0, "output_tokens": 0, "reasoning_tokens": 0,
                 "cached_input_tokens": 0, "cached_write_input_tokens": 0, "model_ms": 0, "complete": True,
                 "execution_span_ms": 0, "timing_complete": True}
        starts, finishes = [], []
        for job in jobs:
            events = json_rows(recipe.tend(tools, env, "events", job["id"]))
            begun = {e["payload"]["attempt"]: e["created_us"] for e in events if e["kind"] == "attempt.started"}
            ended = {e["payload"]["attempt"]: e["created_us"] for e in events if e["kind"] == "attempt.finished"}
            usage["timing_complete"] &= bool(begun) and set(begun) == set(ended)
            starts.extend(begun.values())
            finishes.extend(ended.values())
        if starts and finishes:
            usage["execution_span_ms"] = (max(finishes) - min(starts)) / 1000
        for t in plan:
            if not research_tasks.agent_task(t):
                continue
            taskdir = path / "tasks" / recipe.job_id(t)
            session = taskdir / "session.jsonl"
            if not session.exists():
                usage["complete"] = False
                continue
            verified = command([tools["ask"], "replay", "-check", "-json", str(session)])
            events = json_rows(verified)
            requests = [e for e in events if e["type"] == "request"]
            answers = [e for e in events if e["type"] == "assistant"]
            usage["requests"] += len(requests)
            usage["retries"] += sum(e["type"] == "retry" for e in events)
            usage["complete"] &= len(requests) == len(answers) == 1 and not any(e["data"].get("partial") for e in answers)
            for event in answers:
                data = event["data"]
                counts = data.get("usage")
                counts = counts if isinstance(counts, dict) else {}
                measured = {"model_ms": data.get("ms"), "input_tokens": counts.get("in"),
                            "output_tokens": counts.get("out"), "reasoning_tokens": counts.get("reasoning", 0),
                            "cached_input_tokens": counts.get("cache_read", 0),
                            "cached_write_input_tokens": counts.get("cache_write", 0)}
                valid = all(type(value) is int and value >= 0 for value in measured.values())
                # Ask can serialize zero counts when a provider omits usage.
                # A substantive proposal cannot establish complete accounting
                # from an all-zero input or output count. Fully cached input
                # remains observable through its separate cache counters.
                usage["complete"] &= (valid and measured["output_tokens"] > 0
                    and sum(measured[k] for k in ("input_tokens", "cached_input_tokens",
                                                  "cached_write_input_tokens")) > 0)
                for field, value in measured.items():
                    if type(value) is int and value >= 0:
                        usage[field] += value
        return results, states, usage


def prepare(root, args):
    obj = json.loads(read(args.dataset)) if args.dataset else domain.synthetic_benchmark()
    domain.validate_benchmark(obj)
    now = time.time()
    old = json.loads(read(root / "experiment.json")) if (root / "experiment.json").exists() else {}
    created = old.get("created_unix", now)
    tools = {name: recipe.tool(name) for name in (("weave", "tend", "ask", "ply") if args.mode == "model" else ("weave", "tend"))}
    manifest = {"version": 1, "created_unix": created, "deadline_unix": created + args.seconds,
                "arms": args.arms, "model": args.model, "mode": args.mode,
                "evaluations_per_arm": args.evaluations, "trials": args.trials,
                "team_size": args.team_size, "seed": args.seed,
                "benchmark_sha256": domain.benchmark_digest(obj),
                "sources": {name: digest(read(HERE / name)) for name in SOURCES},
                "tools": {name: {"path": path, "sha256": recipe.digest_file(path)} for name, path in tools.items()},
                "selection": "same declared rank; development and feedback-validation only; final test after all selections freeze",
                "budget": "failed and duplicate proposals consume slots; one Ask invocation per proposal; no replenishment"}
    exact(root / "experiment.json", encoded(manifest))
    exact(root / "benchmark.json", encoded(obj))
    binding = {"benchmark_path": str(root / "benchmark.json"),
               "benchmark_file_sha256": digest(encoded(obj)), "benchmark_sha256": domain.benchmark_digest(obj)}
    return obj, manifest, binding


def experiment(args, root):
    benchmark, manifest, binding = prepare(root, args)
    deadline = manifest["deadline_unix"]
    baseline_tasks = [task("baseline", "evaluate", candidate=domain.baseline_config(), **binding)]
    base_results, _, _ = run_round(root / "baseline", baseline_tasks, args, deadline)
    baseline = base_results["baseline"]
    evidence = domain.public_evidence(benchmark)
    selections, summaries, ledger = [], [], []
    for trial in range(args.trials):
        configs = [c for c in domain.all_configs() if c != domain.baseline_config()]
        random.Random(args.seed + trial).shuffle(configs)
        for arm in args.arms:
            history = []
            best = baseline
            best_ref = {"round": str(root / "baseline"), "task": "baseline", "result_sha256": digest(encoded(baseline))}
            usage = {"requests": 0, "retries": 0, "input_tokens": 0, "output_tokens": 0, "reasoning_tokens": 0,
                     "cached_input_tokens": 0, "cached_write_input_tokens": 0, "model_ms": 0, "complete": True,
                     "execution_span_ms": 0, "timing_complete": True}
            start = 1
            while start <= args.evaluations:
                count = min(args.team_size if arm in ("search", "team") else 1, args.evaluations - start + 1)
                round_root = root / f"trial-{trial+1:02d}" / arm / f"round-{start:03d}"
                tasks = round_plan(root, binding, arm, start, count, configs, evidence, baseline, history)
                results, states, used = run_round(round_root, tasks, args, deadline)
                for key in usage:
                    usage[key] = usage[key] and used[key] if key.endswith("complete") else usage[key] + used[key]
                for slot in range(start, start + count):
                    identity = f"experiment-{slot:03d}"
                    result = results.get(identity)
                    proposal = results.get(f"proposal-{slot:03d}")
                    record = feedback_record(slot, result, states[identity], proposal)
                    record.update({"trial": trial + 1, "arm": arm, "round": str(round_root)})
                    if arm != "search":
                        record["proposal_observation"] = states[f"proposal-{slot:03d}"]
                    record["duplicate"] = result is not None and any(r.get("candidate") == result["candidate"] for r in history)
                    record["decision"] = "failed"
                    if result is not None:
                        record["decision"] = "retain" if selection_key(result) < selection_key(best) else "discard"
                        if record["decision"] == "retain":
                            best = result
                            best_ref = {"round": str(round_root), "task": identity, "result_sha256": digest(encoded(result))}
                    history.append(record)
                    ledger.append(record)
                replace(root / "experiments.jsonl", b"".join(encoded(row) for row in ledger))
                print(f"trial {trial+1} {arm}: {start+count-1}/{args.evaluations} slots; eligible incumbent={eligible(best)}", file=sys.stderr)
                start += count
            selection = {"trial": trial + 1, "arm": arm, "candidate": best["candidate"],
                         "search_eligible": eligible(best), "search_feasible": feasible(best), "evidence": best_ref}
            selections.append(selection)
            summaries.append({**selection, "search_result": best, "usage": usage,
                              "proposal_slots": 0 if arm == "search" else args.evaluations,
                              "experiment_slots": args.evaluations,
                              "evaluated": sum(r["state"] == "accepted" for r in history),
                              "duplicates": sum(r["duplicate"] for r in history),
                              "failed": sum(r["state"] != "accepted" for r in history)})
    # Freeze EVERY arm and trial before ANY final evaluation is admitted.
    frozen = {"experiment_sha256": digest(read(root / "experiment.json")),
              "benchmark_sha256": domain.benchmark_digest(benchmark), "selections": selections}
    exact(root / "selections.json", encoded(frozen))
    selection_binding = {"selection_path": str(root / "selections.json"), "selection_sha256": digest(encoded(frozen))}
    confirms = [task(f'confirm-{s["trial"]:02d}-{s["arm"]}', "confirm", candidate=s["candidate"],
                     trial=s["trial"], arm=s["arm"], **binding, **selection_binding) for s in selections]
    final_results, final_states, _ = run_round(root / "confirmation", confirms, args, deadline)
    for summary in summaries:
        key = f'confirm-{summary["trial"]:02d}-{summary["arm"]}'
        if key not in final_results:
            raise RuntimeError("final evaluation did not complete: " + key)
        summary["holdout"] = final_results[key]["holdout"]
        summary["confirmed"] = summary["search_eligible"] and summary["holdout"]["eligible"]
        summary["confirmation_evidence"] = final_states[key]["evidence"]
    report = {"version": 1, "benchmark_sha256": domain.benchmark_digest(benchmark),
              "model": args.model, "mode": args.mode, "arms": summaries,
              "claims_live_business_improvement": False,
              "limits": ["This version accepts only explicitly uncalibrated synthetic fixtures; historical evidence needs a separately reviewed ingestion and calibration contract.",
                         "Repeated trials measure search variability on one fixed benchmark, not independent business samples.",
                         "Equal proposal/evaluation caps are not equal realized token costs; actual usage is recorded.",
                         "Final cases are withheld from model prompts until every selection is frozen; report-only model calls have no file tools."]}
    replace(root / "report.json", encoded(report))
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory", type=Path)
    parser.add_argument("--model", default=os.environ.get("ASK_MODEL", ""))
    parser.add_argument("--arms", nargs="+", choices=ARMS, default=list(ARMS))
    parser.add_argument("--evaluations", type=int, default=9)
    parser.add_argument("--trials", type=int, default=3)
    parser.add_argument("--team-size", type=int, default=3)
    parser.add_argument("--seed", type=int, default=7301)
    parser.add_argument("--seconds", type=int, default=1800)
    parser.add_argument("--dataset", type=Path, help="alternate uncalibrated synthetic v1 fixture; historical data is not supported")
    args = parser.parse_args()
    if (len(set(args.arms)) != len(args.arms) or not 1 <= args.evaluations < len(domain.all_configs())
            or not 1 <= args.trials <= 5 or not 1 <= args.team_size <= 3 or not 1 <= args.seconds <= 86400):
        parser.error("distinct arms, evaluations 1..35, trials 1..5, team-size 1..3, seconds 1..86400 required")
    args.mode = "reference" if args.arms == ["search"] else "model"
    if args.mode == "model" and not args.model:
        parser.error("agent comparisons require --model or ASK_MODEL")
    os.umask(0o077)
    root = args.directory.resolve()
    root.mkdir(parents=True, exist_ok=True)
    with open(root / ".lock", "a+b") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        report = experiment(args, root)
    sys.stdout.buffer.write(encoded(report))
    return 0


if __name__ == "__main__":
    try:
        code = main()
    except (InterruptedError, KeyboardInterrupt) as error:
        print("research: " + (str(error) or "interrupted"), file=sys.stderr)
        code = 130
    except (OSError, ValueError, RuntimeError, KeyError, subprocess.SubprocessError) as error:
        print("research: " + str(error), file=sys.stderr)
        code = 2
    sys.exit(code)
