"""A finite synthetic invoice benchmark, not an optimizer or a calibrated model.

Candidates choose procedures, never processing times, staffing, detection rates,
or outcomes. A controller owns immutable benchmark bytes, evaluation budgets,
and the release of the final holdout after every arm has frozen its selection.
This module does not hide its source or enforce that release protocol.
"""

import copy
import hashlib
import heapq
import itertools
import json
import math
import statistics

VERSION = "invoice-research-domain/v1"
SPLITS = ("development", "feedback-validation", "final-holdout")
OPTIONS = {"order": ("fifo", "earliest_due", "shortest"),
           "receipt_precheck": (False, True),
           "review": ("all", "flagged", "flagged_or_audit"),
           "batch": ("single", "pairs")}
OBSERVATIONS = ("handling_minutes", "review_minutes", "precheck_minutes", "precheck_wait_minutes",
                "late_chase_minutes", "late_wait_minutes", "single_setup_minutes",
                "paired_setup_minutes", "correction_minutes", "discovery_delay_minutes")


def _encoded(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"),
                      ensure_ascii=False, allow_nan=False).encode()


def _digest(value):
    return "sha256:" + hashlib.sha256(_encoded(value)).hexdigest()


def candidate(config):
    """Return a fresh canonical procedure object; reject all extra controls."""
    if not isinstance(config, dict) or set(config) != set(OPTIONS):
        raise ValueError("candidate must contain exactly " + ", ".join(OPTIONS))
    for key, choices in OPTIONS.items():
        if not any(type(config[key]) is type(choice) and config[key] == choice for choice in choices):
            raise ValueError("invalid candidate choice: " + key)
    return {key: config[key] for key in OPTIONS}


def candidate_schema():
    """Closed JSON Schema for the same finite candidate contract."""
    properties = {key: {"type": "boolean" if key == "receipt_precheck" else "string",
                        "enum": list(choices)} for key, choices in OPTIONS.items()}
    return {"type": "object", "properties": properties,
            "required": list(properties), "additionalProperties": False}


def baseline_config():
    return candidate({"order": "fifo", "receipt_precheck": False,
                      "review": "all", "batch": "single"})


def all_configs():
    """The 36 configurations in declared option order, without a best lookup."""
    return [candidate(dict(zip(OPTIONS, values)))
            for values in itertools.product(*OPTIONS.values())]


def _rows(split_index):
    rows, arrival = [], 0
    for i in range(24):
        kind = i % 6
        # Audit membership is an observable fixed cohort, not an agent-chosen
        # random seed. Later splits include additional defects outside it.
        audit = kind in (3, 4)
        if kind == 3 and ((split_index == 1 and i == 21) or
                          (split_index == 2 and i in (9, 21))):
            audit = False
        missing = kind in (2, 5)
        rows.append({
            "id": ("D", "V", "H")[split_index] + f"{i + 1:02d}",
            "arrival_minutes": arrival, "deadline_minutes": 65 if missing else 85,
            "risk_flag": kind in (1, 5), "audit_cohort": audit,
            "missing_receipt": missing,
            "observations": {
                "handling_minutes": 3 + (i + split_index) % 4,
                "review_minutes": 5 + (i * 3 + split_index) % 4,
                "precheck_minutes": 1 + (i + split_index) % 2,
                "precheck_wait_minutes": 12 + i % 3 if missing else 0,
                "late_chase_minutes": 4 if missing else 0,
                "late_wait_minutes": 42 + i % 5 if missing else 0,
                "single_setup_minutes": 4, "paired_setup_minutes": 1.5,
                "correction_minutes": 14 + (i + split_index) % 5,
                "discovery_delay_minutes": 90,
            },
            "adjudication": {"defect": kind in (1, 2, 3),
                             "precheck_resolves": kind == 2,
                             "review_resolves": kind in (1, 3) or (kind == 2 and i != 14)},
        })
        arrival += 13 + (i * 7 + split_index * 3) % 5
    return rows


def _contract():
    return {
            "staff": 2, "horizon_minutes": 480,
            "scenarios": [{"id": "normal", "demand_factor": 1.0, "staff_delta": 0},
                          {"id": "demand-40", "demand_factor": 1.4, "staff_delta": 0},
                          {"id": "staff-absence", "demand_factor": 1.0, "staff_delta": -1}],
            "absolute_gates": {"min_on_time_pct": 95, "max_rework_pct": 5,
                               "max_backlog_at_horizon": 0},
            "relative_gates": {"min_mean_median_reduction_pct": 10,
                               "max_p95_increase_pct": 0, "max_labor_increase_pct": 0},
            "baseline": baseline_config(),
            "objective": "Eligible first, then feasible; minimize equally weighted scenario mean median, then worst p95, then mean labor. Exact configuration bytes break ties.",
    }


def synthetic_benchmark():
    """New reproducible fixture, separate from the historical business.py demo."""
    return {
        "version": VERSION,
        "provenance": {"kind": "synthetic", "source": "research_domain.py fixed fixture v1",
                       "calibrated": False,
                       "statement": "All durations and counterfactual intervention outcomes are stipulated synthetic observations; no historical company data or causal estimates are claimed."},
        "contract": _contract(),
        "splits": {name: _rows(i) for i, name in enumerate(SPLITS)},
    }


def benchmark_digest(benchmark):
    return _digest(benchmark)


def public_evidence(benchmark):
    """Only development cases and retrospective adjudication may reach agents.

    Development labels are intentional training evidence. Neither validation nor
    holdout rows, labels, statistics or rankings are returned by this function.
    """
    validate_benchmark(benchmark)
    return copy.deepcopy({"version": VERSION, "benchmark_sha256": benchmark_digest(benchmark),
                          "provenance": benchmark["provenance"],
                          "contract": benchmark["contract"], "candidate_schema": candidate_schema(),
                          "development_rows": benchmark["splits"]["development"],
                          "interventions": {
                              "order": "Choose only among currently ready stages: FIFO, earliest absolute due time, or shortest observed stage duration. Never sort by hidden adjudication.",
                              "receipt_precheck": "A staff-consuming check precedes handling; any receipt wait releases the worker. Fixed recorded outcomes determine whether it resolves a defect.",
                              "review": "Review everyone, risk-flagged cases, or risk-flagged plus the fixed audit cohort. Review uses recorded labor and imperfect recorded detection.",
                              "batch": "Single invoices or consecutive arrival pairs. Pairs wait for both arrivals and use the fixed paired setup observation; the final singleton uses single setup.",
                          }})


def _validate(benchmark):
    if not isinstance(benchmark, dict) or set(benchmark) != {"version", "provenance", "contract", "splits"}:
        raise ValueError("benchmark fields must be version, provenance, contract and splits")
    if benchmark.get("version") != VERSION or benchmark.get("provenance", {}).get("kind") != "synthetic":
        raise ValueError("this evaluator accepts only the versioned synthetic research fixture contract")
    provenance = benchmark["provenance"]
    if (set(provenance) != {"kind", "source", "calibrated", "statement"} or provenance["calibrated"] is not False
            or any(not isinstance(provenance[key], str) or not provenance[key].strip() for key in ("source", "statement"))):
        raise ValueError("explicit uncalibrated synthetic provenance is required")
    if _encoded(benchmark["contract"]) != _encoded(_contract()):
        raise ValueError("evaluator rules differ from the frozen versioned contract")
    if set(benchmark.get("splits", {})) != set(SPLITS):
        raise ValueError("development, feedback-validation and final-holdout splits are required")
    ids = set()
    for rows in benchmark["splits"].values():
        if not isinstance(rows, list) or not 1 <= len(rows) <= 1000:
            raise ValueError("each evaluation split needs 1..1000 cases")
        for row in rows:
            if (not isinstance(row, dict) or set(row) != {"id", "arrival_minutes", "deadline_minutes",
                    "risk_flag", "audit_cohort", "missing_receipt", "observations", "adjudication"}
                    or not isinstance(row["id"], str) or not 1 <= len(row["id"].encode()) <= 256):
                raise ValueError("invalid case fields or identity")
            if row["id"] in ids:
                raise ValueError("case identities must be disjoint across splits")
            ids.add(row["id"])
            for key in ("arrival_minutes", "deadline_minutes"):
                if type(row[key]) not in (int, float) or not math.isfinite(row[key]) or row[key] < 0:
                    raise ValueError("invalid case timing")
            if (row["arrival_minutes"] > benchmark["contract"]["horizon_minutes"] or
                    not 0 < row["deadline_minutes"] <= 14400):
                raise ValueError("cases must arrive within the horizon with positive deadlines")
            if (set(row["observations"]) != set(OBSERVATIONS) or
                    set(row["adjudication"]) != {"defect", "precheck_resolves", "review_resolves"}):
                raise ValueError("incomplete observation or adjudication fields")
            for value in row["observations"].values():
                if type(value) not in (int, float) or not math.isfinite(value) or not 0 <= value <= 14400:
                    raise ValueError("invalid fixed intervention observation")
            if any(row["observations"][key] < .001 for key in
                   ("handling_minutes", "review_minutes", "precheck_minutes", "correction_minutes")):
                raise ValueError("active staff stages need observed durations of at least .001 minute")
            for value in [row["risk_flag"], row["audit_cohort"], row["missing_receipt"],
                          *row["adjudication"].values()]:
                if type(value) is not bool:
                    raise ValueError("case flags and adjudication must be boolean")
            adjudication = row["adjudication"]
            if ((adjudication["precheck_resolves"] or adjudication["review_resolves"]) and not adjudication["defect"]
                    or adjudication["precheck_resolves"] and not row["missing_receipt"]):
                raise ValueError("resolution observations contradict case adjudication")
            if not row["missing_receipt"] and any(row["observations"][key] for key in
                   ("late_chase_minutes", "late_wait_minutes", "precheck_wait_minutes")):
                raise ValueError("receipt waits require a missing receipt")


def validate_benchmark(benchmark):
    """Fail closed on altered rules or malformed externally supplied fixtures."""
    try:
        _validate(benchmark)
    except (KeyError, TypeError, AttributeError, OverflowError, UnicodeError) as error:
        raise ValueError("malformed benchmark contract") from error


def _review(row, config):
    return config["review"] == "all" or row["risk_flag"] or (
        config["review"] == "flagged_or_audit" and row["audit_cohort"])


def _duration(row, stage, paired, config):
    evidence = row["observations"]
    if stage == "precheck":
        return evidence["precheck_minutes"]
    if stage == "correction":
        return evidence["correction_minutes"]
    return (evidence["handling_minutes"] + evidence["paired_setup_minutes" if paired else "single_setup_minutes"]
            + (evidence["review_minutes"] if _review(row, config) else 0)
            + (0 if config["receipt_precheck"] else evidence["late_chase_minutes"]))


def _simulate(rows, config, contract, scenario):
    """Nonpreemptive shared staff queue; waits release capacity and corrections use it."""
    workers = max(1, contract["staff"] + scenario["staff_delta"])
    idle, sequence, now = workers, 0, 0.0
    pending, ready, finished = [], [], {}
    labor, touches, reworks = 0.0, 0, 0
    arrivals = [row["arrival_minutes"] / scenario["demand_factor"] for row in rows]

    def event(at, kind, index, stage, paired):
        nonlocal sequence
        sequence += 1
        heapq.heappush(pending, (at, sequence, kind, index, stage, paired))

    size = 2 if config["batch"] == "pairs" else 1
    ordered = sorted(range(len(rows)), key=lambda i: (arrivals[i], rows[i]["id"]))
    for offset in range(0, len(ordered), size):
        group = ordered[offset:offset + size]
        release = max(arrivals[i] for i in group)
        for i in group:
            event(release, "ready", i, "precheck" if config["receipt_precheck"] else "work", len(group) == 2)

    while pending or ready:
        while pending and pending[0][0] <= now:
            at, seq, kind, i, stage, paired = heapq.heappop(pending)
            row, evidence = rows[i], rows[i]["observations"]
            if kind == "ready":
                ready.append((at, seq, i, stage, paired))
            elif kind == "finished":
                idle += 1
                if stage == "precheck":
                    event(at + evidence["precheck_wait_minutes"], "ready", i, "work", paired)
                elif stage == "correction":
                    finished[i] = at
                else:
                    wait = 0 if config["receipt_precheck"] else evidence["late_wait_minutes"]
                    event(at + wait, "close", i, stage, paired)
            else:
                adjudication = row["adjudication"]
                resolved = ((config["receipt_precheck"] and adjudication["precheck_resolves"])
                            or (_review(row, config) and adjudication["review_resolves"]))
                if adjudication["defect"] and not resolved:
                    reworks += 1
                    event(at + evidence["discovery_delay_minutes"], "ready", i, "correction", False)
                else:
                    finished[i] = at

        def priority(job):
            at, seq, i, stage, paired = job
            if config["order"] == "earliest_due":
                return (arrivals[i] + rows[i]["deadline_minutes"], at, seq)
            if config["order"] == "shortest":
                return (_duration(rows[i], stage, paired, config), at, seq)
            return (at, seq)

        while idle and ready:
            job = min(ready, key=priority)
            ready.remove(job)
            _, _, i, stage, paired = job
            duration = _duration(rows[i], stage, paired, config)
            labor += duration
            touches += 1
            idle -= 1
            event(now + duration, "finished", i, stage, paired)
        if pending:
            now = pending[0][0]
        elif ready:
            raise ValueError("simulation has queued work without a future completion")

    count, horizon = len(rows), contract["horizon_minutes"]
    cycles = sorted(finished[i] - arrivals[i] for i in range(count))
    span = max(finished.values()) - min(arrivals)
    return {"invoices": count, "staff": workers, "staff_hours": workers * horizon / 60,
            "median_cycle_minutes": round(statistics.median(cycles), 6),
            "p95_cycle_minutes": round(cycles[math.ceil(.95 * count) - 1], 6),
            "on_time_pct": round(100 * sum(finished[i] - arrivals[i] <= rows[i]["deadline_minutes"]
                                           for i in range(count)) / count, 6),
            "rework_pct": round(100 * reworks / count, 6),
            "first_pass_pct": round(100 * (count - reworks) / count, 6),
            "labor_minutes": round(labor, 6), "touches": touches,
            "throughput_per_hour": round(60 * count / span, 6) if span else 0,
            "backlog_at_horizon": sum(value > horizon and arrivals[i] <= horizon for i, value in finished.items()),
            "max_backlog_age_minutes": round(max([horizon - arrivals[i] for i, value in finished.items()
                                                   if value > horizon and arrivals[i] <= horizon] or [0]), 6)}


def _assessment(metrics, baseline, contract):
    absolute, relative = contract["absolute_gates"], contract["relative_gates"]
    checks = {"service": metrics["on_time_pct"] >= absolute["min_on_time_pct"],
              "rework": metrics["rework_pct"] <= absolute["max_rework_pct"],
              "backlog": metrics["backlog_at_horizon"] <= absolute["max_backlog_at_horizon"]}
    changes = {}
    for name, key in [("median", "median_cycle_minutes"), ("p95", "p95_cycle_minutes"),
                      ("labor", "labor_minutes")]:
        changes[name + "_change_pct"] = round(100 * (metrics[key] - baseline[key]) / baseline[key], 6)
    guarded = (changes["p95_change_pct"] <= relative["max_p95_increase_pct"] and
               changes["labor_change_pct"] <= relative["max_labor_increase_pct"])
    return {"passed": all(checks.values()), "checks": checks,
            "violations": [key for key, passed in checks.items() if not passed]}, {
                "guardrails_passed": guarded, **changes}


def evaluate(benchmark, config, split):
    """Evaluate one frozen candidate on one split; no search or model call."""
    config = candidate(config)
    validate_benchmark(benchmark)
    if split not in SPLITS:
        raise ValueError("unknown benchmark split")
    contract, rows = benchmark["contract"], benchmark["splits"][split]
    scenarios = []
    for scenario in contract["scenarios"]:
        metrics = _simulate(rows, config, contract, scenario)
        baseline = _simulate(rows, contract["baseline"], contract, scenario)
        feasibility, relative = _assessment(metrics, baseline, contract)
        scenarios.append({"id": scenario["id"], "metrics": metrics, "baseline_metrics": baseline,
                          "feasibility": feasibility, "relative": relative})
    feasible = all(row["feasibility"]["passed"] for row in scenarios)
    mean_median = statistics.mean(row["metrics"]["median_cycle_minutes"] for row in scenarios)
    baseline_mean = statistics.mean(row["baseline_metrics"]["median_cycle_minutes"] for row in scenarios)
    mean_change = 100 * (mean_median - baseline_mean) / baseline_mean
    guarded = all(row["relative"]["guardrails_passed"] for row in scenarios)
    improved = -mean_change >= contract["relative_gates"]["min_mean_median_reduction_pct"] and guarded
    return {"version": VERSION, "kind": "synthetic-evaluation", "candidate": config, "split": split,
            "benchmark_sha256": benchmark_digest(benchmark), "split_sha256": _digest(rows),
            "scenarios": scenarios, "feasible": feasible, "relative_improved": improved,
            "eligible": feasible and improved,
            "relative": {"mean_median_change_pct": round(mean_change, 6),
                         "guardrails_passed": guarded, "passed": improved},
            "objective": {"mean_median_minutes": round(mean_median, 6),
                          "worst_p95_minutes": max(row["metrics"]["p95_cycle_minutes"] for row in scenarios),
                          "mean_labor_minutes": round(statistics.mean(row["metrics"]["labor_minutes"] for row in scenarios), 6)},
            "claims_real_improvement": False}


def rank_key(result):
    """Shared declared ordering; ranking an infeasible result never admits it."""
    objective = result["objective"]
    return (not result["eligible"], not result["feasible"], objective["mean_median_minutes"],
            objective["worst_p95_minutes"], objective["mean_labor_minutes"],
            _encoded(candidate(result["candidate"])).decode())
