#!/usr/bin/env python3
"""Evaluate frozen checker changes from explicitly selected, independent evidence.

This offline JSON filter makes no model calls and changes no checker or policy.
See CALIBRATION.md for the version-1 input and the limits of its conclusions.
"""
import argparse
import hashlib
import json
import math
from pathlib import Path
import statistics
import sys


MAX_BYTES = 8 * 1024 * 1024
MAX_CASES = 10000
MAX_CRITERIA = 128
MAX_SETTINGS = 128
MAX_CELLS = 1000000
STATES = ("accept", "reject", "missing", "unknown", "error")


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
    if len(raw) > MAX_BYTES:
        raise ValueError("input exceeds 8 MiB")
    value = json.loads(raw.decode("utf-8"), object_pairs_hook=unique_object,
                       parse_constant=reject_constant)
    stack = [(value, 0)]
    while stack:
        item, depth = stack.pop()
        if depth > 64:
            raise ValueError("JSON exceeds 64 levels")
        if isinstance(item, dict):
            for key, child in item.items():
                key.encode("utf-8")
                stack.append((child, depth + 1))
        elif isinstance(item, list):
            stack.extend((child, depth + 1) for child in item)
        elif isinstance(item, str):
            item.encode("utf-8")
        elif type(item) in (int, float):
            number(item)
    return value


def object_fields(value, required, optional=()):
    if (not isinstance(value, dict) or not set(required) <= set(value)
            or set(value) - set(required) - set(optional)):
        raise ValueError("missing or unsupported object fields")


def name(value):
    if (not isinstance(value, str) or not value.strip()
            or len(value.encode("utf-8")) > 256 or any(ord(c) < 32 for c in value)):
        raise ValueError("IDs and text must be nonempty, bounded UTF-8 without controls")
    return value


def basis(value):
    if not isinstance(value, str) or not value.strip() or len(value.encode("utf-8")) > 8192:
        raise ValueError("label basis must be nonempty text of at most 8192 bytes")


def number(value, minimum=None, maximum=None):
    if type(value) not in (int, float) or not -1e12 <= value <= 1e12:
        raise ValueError("numbers must be finite and bounded to absolute value 1e12")
    if minimum is not None and value < minimum or maximum is not None and value > maximum:
        raise ValueError("number outside permitted range")
    return value


def integer(value, minimum, maximum):
    if type(value) is not int or not minimum <= value <= maximum:
        raise ValueError("integer outside permitted range")


def digest(value):
    if (not isinstance(value, str) or len(value) != 64
            or any(c not in "0123456789abcdef" for c in value)):
        raise ValueError("expected lowercase SHA256 hexadecimal digest")


def canonical_hash(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"),
                                     ensure_ascii=False, allow_nan=False).encode("utf-8")).hexdigest()


def names(value, allowed):
    if (not isinstance(value, list) or len(value) != len(set(name(v) for v in value))
            or not set(value) <= set(allowed)):
        raise ValueError("ID lists must be unique and refer to declared IDs")


def measurements(value):
    for key in ("cost", "seconds"):
        if key in value and value[key] is not None:
            number(value[key], 0)
    for key in ("tokens", "turns"):
        if key in value and value[key] is not None:
            integer(value[key], 0, 10**12)


def active_criteria(setting, criteria):
    return [c["id"] for c in criteria if c["id"] not in setting["drop_criteria"]
            and c["check"] not in setting["drop_checks"]]


def setting_hash(setting, criteria):
    return canonical_hash({"criteria": criteria, "setting": setting})


def validate(request):
    object_fields(request, {"version", "criteria", "settings", "baseline", "proposal",
                           "proposal_basis", "constraints", "cases"},
                  {"sweeps", "ablations", "repair_trials"})
    if type(request["version"]) is not int or request["version"] != 1:
        raise ValueError("version must be 1")
    if request["proposal_basis"] not in ("input", "calibration"):
        raise ValueError("proposal must have been frozen using input or calibration evidence")
    criteria = request["criteria"]
    if not isinstance(criteria, list) or not 1 <= len(criteria) <= MAX_CRITERIA:
        raise ValueError("require 1 to 128 criteria")
    ids, checks = set(), set()
    for item in criteria:
        object_fields(item, {"id", "check"})
        name(item["id"])
        name(item["check"])
        if item["id"] in ids:
            raise ValueError("duplicate criterion ID")
        ids.add(item["id"])
        checks.add(item["check"])
    settings = request["settings"]
    if not isinstance(settings, list) or not 1 <= len(settings) <= MAX_SETTINGS:
        raise ValueError("require 1 to 128 settings")
    setting_ids, profiles, profile_sources = set(), set(), {}
    for setting in settings:
        object_fields(setting, {"id", "profile", "source_sha256", "thresholds", "drop_checks", "drop_criteria"})
        name(setting["id"])
        name(setting["profile"])
        digest(setting["source_sha256"])
        if setting["id"] in setting_ids or setting["id"].startswith(("sweep-", "ablate-")):
            raise ValueError("setting IDs must be unique and cannot use generated prefixes")
        setting_ids.add(setting["id"])
        profiles.add(setting["profile"])
        if setting["profile"] in profile_sources and profile_sources[setting["profile"]] != setting["source_sha256"]:
            raise ValueError("different implementation sources require separately collected profiles")
        profile_sources[setting["profile"]] = setting["source_sha256"]
        if not isinstance(setting["thresholds"], dict) or set(setting["thresholds"]) != ids:
            raise ValueError("each setting requires exactly all declared criterion thresholds")
        for threshold in setting["thresholds"].values():
            number(threshold)
        names(setting["drop_checks"], checks)
        names(setting["drop_criteria"], ids)
    if request["baseline"] not in setting_ids or request["proposal"] not in setting_ids:
        raise ValueError("baseline and proposal must name explicit settings")
    constraints = request["constraints"]
    object_fields(constraints, {"required_criteria", "required_checks", "min_evaluation_cases",
                               "max_false_pass_rate", "max_false_rejection_rate", "require_final_review"},
                  {"min_evaluation_accept", "min_evaluation_reject", "min_thresholds"})
    names(constraints["required_criteria"], ids)
    names(constraints["required_checks"], checks)
    integer(constraints["min_evaluation_cases"], 1, MAX_CASES)
    for field in ("min_evaluation_accept", "min_evaluation_reject"):
        if field in constraints:
            integer(constraints[field], 1, MAX_CASES)
    for field in ("max_false_pass_rate", "max_false_rejection_rate"):
        number(constraints[field], 0, 1)
    if "min_thresholds" in constraints:
        if not isinstance(constraints["min_thresholds"], dict) or not set(constraints["min_thresholds"]) <= ids:
            raise ValueError("minimum thresholds must name declared criteria")
        for value in constraints["min_thresholds"].values():
            number(value)
    if type(constraints["require_final_review"]) is not bool:
        raise ValueError("require_final_review must be boolean")
    cases = request["cases"]
    if not isinstance(cases, list) or not 1 <= len(cases) <= MAX_CASES:
        raise ValueError("require 1 to 10000 cases")
    seen, candidate_splits = set(), {}
    for case in cases:
        object_fields(case, {"id", "split", "candidate_sha256", "evidence_sha256", "expected",
                             "label_basis", "label_independent", "profiles"})
        name(case["id"])
        if case["id"] in seen:
            raise ValueError("duplicate case ID")
        seen.add(case["id"])
        if case["split"] not in ("input", "calibration", "evaluation"):
            raise ValueError("invalid case split")
        digest(case["candidate_sha256"])
        if case["evidence_sha256"] is not None:
            digest(case["evidence_sha256"])
        key = (case["candidate_sha256"], case["evidence_sha256"])
        if key in candidate_splits:
            raise ValueError("candidate/evidence pairs must be unique across and within splits")
        candidate_splits[key] = case["split"]
        if case["expected"] not in ("accept", "reject", "unknown"):
            raise ValueError("expected must be accept, reject or unknown")
        basis(case["label_basis"])
        if type(case["label_independent"]) is not bool:
            raise ValueError("label_independent must be boolean")
        if not isinstance(case["profiles"], dict) or not set(case["profiles"]) <= profiles:
            raise ValueError("case profiles must refer to explicit setting profiles")
        for result in case["profiles"].values():
            object_fields(result, {"criteria"}, {"cost", "seconds", "tokens", "turns"})
            if not isinstance(result["criteria"], dict) or not set(result["criteria"]) <= ids:
                raise ValueError("unknown criterion in result")
            for outcome in result["criteria"].values():
                object_fields(outcome, {"status"}, {"score"})
                if outcome["status"] not in ("known", "missing", "unknown", "error"):
                    raise ValueError("invalid criterion status")
                if (outcome["status"] == "known") != ("score" in outcome):
                    raise ValueError("exactly known outcomes must supply a score")
                if "score" in outcome:
                    number(outcome["score"])
            measurements(result)
    baseline = next(s for s in settings if s["id"] == request["baseline"])
    variants = []
    sweeps = request.get("sweeps", [])
    ablations = request.get("ablations", [])
    if (not isinstance(sweeps, list) or not isinstance(ablations, list)
            or len(sweeps) > MAX_SETTINGS or len(ablations) > MAX_SETTINGS):
        raise ValueError("sweeps and ablations must be lists")
    for sweep in sweeps:
        object_fields(sweep, {"criterion", "values"})
        if (sweep["criterion"] not in ids or not isinstance(sweep["values"], list)
                or not sweep["values"] or len(settings) + len(variants) + len(sweep["values"]) > MAX_SETTINGS):
            raise ValueError("sweep requires a declared criterion and nonempty values")
        for value in sweep["values"]:
            number(value)
            variant = dict(baseline, id="sweep-%04d" % (len(variants) + 1),
                           thresholds=dict(baseline["thresholds"]))
            variant["thresholds"][sweep["criterion"]] = value
            variants.append(variant)
    for index, ablation in enumerate(ablations):
        if not isinstance(ablation, dict) or set(ablation) not in ({"criterion"}, {"check"}):
            raise ValueError("ablation requires exactly criterion or check")
        kind = next(iter(ablation))
        value = ablation[kind]
        if value not in (ids if kind == "criterion" else checks):
            raise ValueError("ablation must refer to a declared criterion or check")
        field = "drop_criteria" if kind == "criterion" else "drop_checks"
        variant = dict(baseline, id="ablate-%04d" % (index + 1))
        variant[field] = sorted(set(baseline[field]) | {value})
        variants.append(variant)
    if len(settings) + len(variants) > MAX_SETTINGS or len(cases) * (len(settings) + len(variants)) > MAX_CELLS:
        raise ValueError("evaluation exceeds setting or case-setting bounds")
    trials = request.get("repair_trials", [])
    if not isinstance(trials, list) or len(trials) > MAX_CASES:
        raise ValueError("repair_trials must be a bounded list")
    settings_by_id = {s["id"]: s for s in settings}
    seen, seen_tasks = set(), set()
    for trial in trials:
        object_fields(trial, {"id", "split", "task_sha256", "settings"})
        name(trial["id"])
        digest(trial["task_sha256"])
        if trial["id"] in seen or trial["task_sha256"] in seen_tasks:
            raise ValueError("repair trial IDs and task hashes must be unique")
        seen.add(trial["id"])
        seen_tasks.add(trial["task_sha256"])
        if trial["split"] != "evaluation":
            raise ValueError("repair_trials contain heldout evaluation only")
        if not isinstance(trial["settings"], dict) or not set(trial["settings"]) <= {request["baseline"], request["proposal"]}:
            raise ValueError("repair trials may name only the frozen baseline/proposal settings")
        for sid, result in trial["settings"].items():
            object_fields(result, {"setting_sha256", "artifact_sha256", "checker_accepted", "final_review"},
                          {"cost", "seconds", "tokens", "turns"})
            digest(result["setting_sha256"])
            digest(result["artifact_sha256"])
            if result["setting_sha256"] != setting_hash(settings_by_id[sid], criteria):
                raise ValueError("repair result setting digest does not match frozen setting")
            if type(result["checker_accepted"]) is not bool:
                raise ValueError("checker_accepted must be boolean")
            review = result["final_review"]
            object_fields(review, {"quality", "basis", "independent"})
            if review["quality"] not in ("accept", "reject", "unknown") or type(review["independent"]) is not bool:
                raise ValueError("invalid final review quality or independence")
            basis(review["basis"])
            measurements(result)
    return settings + variants


def coverage(setting, request):
    active = active_criteria(setting, request["criteria"])
    retained_checks = {c["check"] for c in request["criteria"] if c["id"] in active}
    reasons = []
    if not active:
        reasons.append("no_active_criteria")
    if not set(request["constraints"]["required_criteria"]) <= set(active):
        reasons.append("required_criterion_removed")
    if not set(request["constraints"]["required_checks"]) <= retained_checks:
        reasons.append("required_check_removed")
    if any(cid not in active or setting["thresholds"][cid] < floor
           for cid, floor in request["constraints"].get("min_thresholds", {}).items()):
        reasons.append("frozen_minimum_threshold_violated")
    return reasons


def decide(case, setting, criteria):
    active = active_criteria(setting, criteria)
    result = case["profiles"].get(setting["profile"], {})
    outcomes = result.get("criteria", {})
    states = [outcomes.get(cid, {"status": "missing"})["status"] for cid in active]
    if not active:
        return "missing"
    # An incomplete check is never converted into a business rejection or pass.
    for state in ("error", "missing", "unknown"):
        if state in states:
            return state
    return "accept" if all(outcomes[cid]["score"] >= setting["thresholds"][cid] for cid in active) else "reject"


def usage(results):
    costs = [r.get("cost") for r in results]
    seconds = [r.get("seconds") for r in results]
    known_seconds = sorted(s for s in seconds if s is not None)
    summary = {"cost_known_subtotal": sum(c for c in costs if c is not None),
            "cost_unknown": sum(c is None for c in costs),
            "cost_total": sum(costs) if costs and all(c is not None for c in costs) else None,
            "seconds_unknown": sum(s is None for s in seconds),
            "seconds_known_total": sum(known_seconds),
            "seconds_total": sum(seconds) if seconds and all(s is not None for s in seconds) else None,
            "seconds_p50_known": statistics.median(known_seconds) if known_seconds else None,
            "seconds_p95_known": known_seconds[math.ceil(len(known_seconds) * .95) - 1] if known_seconds else None}
    for field in ("tokens", "turns"):
        values = [r.get(field) for r in results]
        summary[field + "_known_subtotal"] = sum(v for v in values if v is not None)
        summary[field + "_unknown"] = sum(v is None for v in values)
        summary[field + "_total"] = sum(values) if values and all(v is not None for v in values) else None
    return summary


def summarize(cases, setting, criteria):
    rows = [(case, decide(case, setting, criteria)) for case in cases]
    independent = [(c, d) for c, d in rows if c["label_independent"] and c["expected"] != "unknown"]
    accepts = [(c, d) for c, d in independent if c["expected"] == "accept"]
    rejects = [(c, d) for c, d in independent if c["expected"] == "reject"]
    positive = [d for _, d in accepts if d in ("accept", "reject")]
    negative = [d for _, d in rejects if d in ("accept", "reject")]
    fp, fr = negative.count("accept"), positive.count("reject")
    return {"cases": len(cases), "states": {s: sum(d == s for _, d in rows) for s in STATES},
            "label_unknown": sum(c["expected"] == "unknown" for c in cases),
            "label_not_independent": sum(not c["label_independent"] for c in cases),
            "independent_labeled": len(independent), "expected_accept_attempted": len(accepts),
            "expected_reject_attempted": len(rejects), "expected_accept_completed": len(positive),
            "expected_reject_completed": len(negative), "false_passes": fp, "false_rejections": fr,
            "false_pass_rate": fp / len(negative) if negative else None,
            "false_rejection_rate": fr / len(positive) if positive else None,
            "usage": usage([c["profiles"].get(setting["profile"], {}) for c in cases])}


def paired(cases, baseline, proposal, criteria):
    rows = [{"id": c["id"], "expected": c["expected"], "label_independent": c["label_independent"],
             "baseline": decide(c, baseline, criteria), "proposal": decide(c, proposal, criteria)} for c in cases]
    labeled = [r for r in rows if r["label_independent"] and r["expected"] != "unknown"]
    complete = [r for r in labeled if r["baseline"] in ("accept", "reject") and r["proposal"] in ("accept", "reject")]
    return {"attempted": len(labeled), "completed": len(complete), "incomplete": len(labeled) - len(complete),
            "corrected": sum(r["baseline"] != r["expected"] and r["proposal"] == r["expected"] for r in complete),
            "regressed": sum(r["baseline"] == r["expected"] and r["proposal"] != r["expected"] for r in complete),
            "rows": rows}


def repair_summary(trials, baseline_id, proposal_id):
    rows, baseline_results, proposal_results = [], [], []
    for trial in trials:
        row = {"id": trial["id"]}
        for side, sid, results in (("baseline", baseline_id, baseline_results), ("proposal", proposal_id, proposal_results)):
            result = trial["settings"].get(sid)
            results.append(result or {})
            review = result["final_review"] if result else None
            row[side] = ("missing" if not review else "not_independent" if not review["independent"] else review["quality"])
            row[side + "_checker_accepted"] = result["checker_accepted"] if result else None
        rows.append(row)
    complete = [r for r in rows if r["baseline"] in ("accept", "reject") and r["proposal"] in ("accept", "reject")]
    return {"attempted": len(rows), "completed": len(complete), "incomplete": len(rows) - len(complete),
            "improved": sum(r["baseline"] == "reject" and r["proposal"] == "accept" for r in complete),
            "regressed": sum(r["baseline"] == "accept" and r["proposal"] == "reject" for r in complete),
            "baseline_accepted_bad_outputs": sum(r["baseline"] == "reject" and r["baseline_checker_accepted"] is True for r in rows),
            "proposal_accepted_bad_outputs": sum(r["proposal"] == "reject" and r["proposal_checker_accepted"] is True for r in rows),
            "baseline_states": {s: sum(r["baseline"] == s for r in rows) for s in ("accept", "reject", "unknown", "missing", "not_independent")},
            "proposal_states": {s: sum(r["proposal"] == s for r in rows) for s in ("accept", "reject", "unknown", "missing", "not_independent")},
            "baseline_usage": usage(baseline_results), "proposal_usage": usage(proposal_results), "rows": rows}


def evaluate(request, raw=None):
    settings = validate(request)
    by_id = {s["id"]: s for s in settings}
    baseline, proposal = by_id[request["baseline"]], by_id[request["proposal"]]
    criteria, constraints = request["criteria"], request["constraints"]
    splits = {split: [c for c in request["cases"] if c["split"] == split] for split in ("input", "calibration", "evaluation")}
    baseline_summary = summarize(splits["evaluation"], baseline, criteria)
    proposal_summary = summarize(splits["evaluation"], proposal, criteria)
    comparison = paired(splits["evaluation"], baseline, proposal, criteria)
    repairs = repair_summary(request.get("repair_trials", []), baseline["id"], proposal["id"])
    blocked = coverage(proposal, request)
    incomplete = []
    if coverage(baseline, request):
        incomplete.append("baseline_does_not_meet_frozen_coverage")
    if comparison["completed"] < constraints["min_evaluation_cases"]:
        incomplete.append("insufficient_paired_evaluation_cases")
    if comparison["incomplete"]:
        incomplete.append("incomplete_paired_checker_outcomes")
    for field, minimum in (("expected_accept_completed", constraints.get("min_evaluation_accept", 1)),
                           ("expected_reject_completed", constraints.get("min_evaluation_reject", 1))):
        if min(baseline_summary[field], proposal_summary[field]) < minimum:
            incomplete.append("insufficient_" + field)
    for field, limit in (("false_pass_rate", "max_false_pass_rate"), ("false_rejection_rate", "max_false_rejection_rate")):
        bvalue, pvalue = baseline_summary[field], proposal_summary[field]
        if pvalue is not None and pvalue > constraints[limit]:
            blocked.append(field + "_exceeds_frozen_limit")
        if bvalue is not None and pvalue is not None and pvalue > bvalue:
            blocked.append(field + "_regressed")
    if any(proposal_summary["states"][s] > baseline_summary["states"][s] for s in ("missing", "unknown", "error")):
        blocked.append("incomplete_outcomes_increased")
    need_repairs = constraints["require_final_review"] or bool(request.get("repair_trials"))
    if need_repairs:
        if repairs["completed"] < constraints["min_evaluation_cases"] or repairs["incomplete"]:
            incomplete.append("insufficient_independently_reviewed_repair_pairs")
        if repairs["regressed"]:
            blocked.append("final_output_quality_regressed")
        if repairs["proposal_accepted_bad_outputs"] > repairs["baseline_accepted_bad_outputs"]:
            blocked.append("accepted_bad_final_outputs_increased")
    improvements = []
    if comparison["corrected"] > comparison["regressed"]:
        improvements.append("fewer_fixed_candidate_errors")
    if repairs["improved"] > repairs["regressed"]:
        improvements.append("better_independently_reviewed_final_outputs")
    busage = repairs["baseline_usage"] if need_repairs else baseline_summary["usage"]
    pusage = repairs["proposal_usage"] if need_repairs else proposal_summary["usage"]
    counter_pairs_complete = (bool(repairs["attempted"]) and not repairs["incomplete"] if need_repairs else
                              bool(comparison["rows"]) and all(row["baseline"] in ("accept", "reject") and
                              row["proposal"] in ("accept", "reject") for row in comparison["rows"]))
    for field in ("cost_total", "seconds_total", "tokens_total", "turns_total"):
        if field in ("tokens_total", "turns_total") and not counter_pairs_complete:
            continue
        if busage[field] is not None and pusage[field] is not None and pusage[field] < busage[field]:
            improvements.append("lower_observed_" + field)
    decision = "blocked" if blocked else "inconclusive" if incomplete else "supported" if improvements else "no_improvement"
    return {"version": 1, "decision": decision, "reasons": blocked + incomplete,
            "improvements": improvements, "baseline": baseline["id"], "proposal": proposal["id"],
            "proposal_basis": request["proposal_basis"],
            "scope": "fixed candidates and independent repair-output reviews" if need_repairs else "fixed-candidate judgments only; final-output improvement not established",
            "limitations": ["Independence, source pins, hashes and split assignment are caller attestations, not authenticated facts.",
                            "Paired observations do not establish statistical certainty or generalization.",
                            "Calibration variants are counterfactual rescoring; cost, latency, tokens and turns are observed profile measurements, not estimated savings.",
                            "A supported result permits caller review; it does not modify policy, run Hone or authorize promotion."],
            "hashes": {"request_sha256": hashlib.sha256(raw).hexdigest() if raw is not None else canonical_hash(request),
                       "constraints_sha256": canonical_hash(constraints),
                       "evaluation_cases_sha256": canonical_hash(splits["evaluation"]),
                       "repair_trials_sha256": canonical_hash(request.get("repair_trials", [])),
                       "settings": {s["id"]: setting_hash(s, criteria) for s in settings}},
            "split_counts": {split: len(cases) for split, cases in splits.items()},
            "calibration": [{"id": s["id"], "setting": s, "coverage_violations": coverage(s, request),
                             "summary": summarize(splits["calibration"], s, criteria)} for s in settings],
            "evaluation": {"baseline": baseline_summary, "proposal": proposal_summary, "paired": comparison,
                           "repair_trials": repairs}}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, help="explicit request JSON path, or - for stdin")
    parser.add_argument("--gate", action="store_true", help="exit 1 unless the valid report supports the proposal")
    args = parser.parse_args(argv)
    try:
        if args.input == "-":
            raw = sys.stdin.buffer.read(MAX_BYTES + 1)
        else:
            with Path(args.input).open("rb") as stream:
                raw = stream.read(MAX_BYTES + 1)
        report = evaluate(parse_document(raw), raw)
    except (OSError, ValueError, TypeError, KeyError, OverflowError, RecursionError):
        print("evaluate: invalid or unreadable evaluation request; see CALIBRATION.md", file=sys.stderr)
        return 2
    try:
        sys.stdout.write(json.dumps(report, ensure_ascii=False, sort_keys=True, indent=2, allow_nan=False) + "\n")
    except (OSError, UnicodeError):
        return 1
    return 1 if args.gate and report["decision"] != "supported" else 0


if __name__ == "__main__":
    raise SystemExit(main())
