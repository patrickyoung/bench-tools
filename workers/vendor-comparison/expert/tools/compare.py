#!/usr/bin/env python3
"""Deterministic validation and rendering; no model client or generated-code execution."""
import csv
import hashlib
import io
import json
import math
import os
from pathlib import Path
import sys


def require(ok, message):
    if not ok:
        raise ValueError(message)


def unique_pairs(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, "duplicate JSON key: " + key)
        result[key] = value
    return result


def read_bytes(path, limit=2_000_000):
    path = Path(path)
    require(not any(p.is_symlink() for p in [path, *path.parents]), "symlink: " + str(path))
    require(path.is_file() and path.stat().st_size <= limit, "missing/oversized file: " + str(path))
    return path.read_bytes()


def parse(data):
    return json.loads(data, object_pairs_hook=unique_pairs,
                      parse_constant=lambda x: (_ for _ in ()).throw(ValueError("nonfinite JSON")))


def sha(data):
    return hashlib.sha256(data).hexdigest()


def dump(obj):
    return (json.dumps(obj, indent=2, ensure_ascii=False, allow_nan=False) + "\n").encode()


def shape(obj, keys, where):
    require(type(obj) is dict and set(obj) == set(keys.split()), "wrong fields: " + where)


def string(value, where):
    require(isinstance(value, str) and value.strip() and len(value) <= 12000, "missing/bounded text: " + where)


def number(value):
    return type(value) in (int, float) and math.isfinite(value)


def strings(values, where):
    require(type(values) is list and len(values) <= 100, "list: " + where)
    for value in values:
        string(value, where)


def unique(items, field, where):
    require(type(items) is list, "list: " + where)
    ids = [item[field] for item in items]
    for value in ids:
        string(value, where)
    require(len(ids) == len(set(ids)), "duplicate IDs: " + where)
    return set(ids)


def validate(packet, analysis, packet_hash):
    require(packet.get("schema") == "bench.comparison-packet/v1", "packet schema")
    job = packet["job"]
    candidates = unique(job["candidates"], "id", "candidates")
    shape(analysis, "schema packet_sha256 decision_summary weight_basis weight_rationale criteria cells gates recommendation assumptions gaps", "analysis")
    require(analysis["schema"] == "bench.vendor-comparison/v1", "analysis schema")
    require(analysis["packet_sha256"] == packet_hash, "stale packet binding")
    require(os.environ.get("COMPARISON_PACKET_SHA256", packet_hash) == packet_hash, "packet changed since admission")
    string(analysis["decision_summary"], "decision_summary")
    string(analysis["weight_rationale"], "weight_rationale")
    criteria = analysis["criteria"]
    criterion_ids = unique(criteria, "id", "criteria")
    require(3 <= len(criteria) <= 12, "3–12 criteria required")
    for criterion in criteria:
        shape(criterion, "id name weight reason anchors", "criterion")
        for key in ("name", "reason"):
            string(criterion[key], key)
        require(number(criterion["weight"]) and 0 < criterion["weight"] <= 100, "positive finite weight")
        shape(criterion["anchors"], "0 1 2 3 4 5", "anchors")
        for anchor in criterion["anchors"].values():
            string(anchor, "anchor")
        require(len(set(criterion["anchors"].values())) == 6, "anchors must distinguish scores")
    require(abs(sum(c["weight"] for c in criteria) - 100) <= 0.000001, "weights must sum to 100")
    supplied = job.get("criteria", [])
    require(analysis["weight_basis"] == ("supplied" if supplied else "proposed"), "weight provenance")
    if supplied:
        require([c["id"] for c in supplied] == [c["id"] for c in criteria], "supplied criteria/order changed")
        for original, actual in zip(supplied, criteria):
            for key in ("id", "name", "weight", "reason", "anchors"):
                if key in original:
                    require(original[key] == actual[key], "supplied criterion changed: " + key)
    sources = {s["id"]: s for s in packet["sources"]}

    def refs(values, candidate):
        require(type(values) is list and len(values) <= 20, "refs list")
        for ref in values:
            shape(ref, "source_id locator quote", "ref")
            string(ref["quote"], "quote")
            require(len(ref["quote"]) >= 8, "quote too short")
            require(ref["source_id"] in sources, "unknown source")
            source = sources[ref["source_id"]]
            require(source["status"] == "available", "unavailable source cited")
            require(not source["candidate_ids"] or candidate in source["candidate_ids"], "wrong candidate source")
            require(any(s["locator"] == ref["locator"] and ref["quote"] in s["text"] for s in source["segments"]), "quote/locator not in source")

    cells = analysis["cells"]
    require(type(cells) is list and len(cells) == len(candidates) * len(criteria), "complete matrix required")
    seen = set()
    for cell in cells:
        shape(cell, "candidate_id criterion_id score status confidence rationale refs", "cell")
        key = (cell["candidate_id"], cell["criterion_id"])
        require(key[0] in candidates and key[1] in criterion_ids and key not in seen, "unknown/duplicate cell")
        seen.add(key)
        require(cell["status"] in ("supported", "unknown", "conflicting"), "cell status")
        require(cell["confidence"] in ("high", "medium", "low"), "confidence")
        string(cell["rationale"], "rationale")
        refs(cell["refs"], key[0])
        if cell["status"] == "supported":
            require(type(cell["score"]) is int and 0 <= cell["score"] <= 5, "supported score must be 0–5")
            require(bool(cell["refs"]), "scored cell needs evidence")
        else:
            require(cell["score"] is None, "unknown/conflicting must have null score")
            if cell["status"] == "conflicting":
                require(len({(r["source_id"], r["locator"], r["quote"]) for r in cell["refs"]}) >= 2, "conflict requires distinct evidence")
    gates = job.get("gates", [])
    gate_ids = unique(gates, "id", "job gates")
    require(type(analysis["gates"]) is list and len(analysis["gates"]) == len(candidates) * len(gates), "complete gates required")
    seen = set()
    for gate in analysis["gates"]:
        shape(gate, "candidate_id gate_id status rationale refs", "gate")
        key = (gate["candidate_id"], gate["gate_id"])
        require(key[0] in candidates and key[1] in gate_ids and key not in seen, "unknown/duplicate gate")
        seen.add(key)
        require(gate["status"] in ("pass", "fail", "unknown"), "gate status")
        string(gate["rationale"], "gate rationale")
        refs(gate["refs"], key[0])
        require(gate["status"] == "unknown" or bool(gate["refs"]), "gate decision needs evidence")
    recommendation = analysis["recommendation"]
    shape(recommendation, "status candidate_id rationale", "recommendation")
    string(recommendation["rationale"], "recommendation rationale")
    require(recommendation["status"] in ("recommend", "conditional", "defer"), "recommendation status")
    chosen = recommendation["candidate_id"]
    if recommendation["status"] == "defer":
        require(chosen is None, "defer must not select candidate")
    else:
        require(chosen in candidates, "unknown selection")
        selected_gates = [g["status"] for g in analysis["gates"] if g["candidate_id"] == chosen]
        require("fail" not in selected_gates, "cannot select failed gate")
        require(recommendation["status"] != "recommend" or "unknown" not in selected_gates, "unresolved gate requires conditional/defer")
        require(any(c["candidate_id"] == chosen and c["score"] is not None for c in cells), "cannot select zero-evidence candidate")
    strings(analysis["assumptions"], "assumptions")
    require(type(analysis["gaps"]) is list and len(analysis["gaps"]) <= 100, "gaps list")
    for gap in analysis["gaps"]:
        shape(gap, "candidate_id question owner decision_impact", "gap")
        require(gap["candidate_id"] is None or gap["candidate_id"] in candidates, "gap candidate")
        for key in ("question", "owner", "decision_impact"):
            string(gap[key], key)
    uncertain = {c["candidate_id"] for c in cells if c["score"] is None}
    uncertain |= {g["candidate_id"] for g in analysis["gates"] if g["status"] == "unknown"}
    for candidate in uncertain:
        require(any(g["candidate_id"] in (None, candidate) for g in analysis["gaps"]), "uncertainty needs followup")


def calculate(packet, analysis, analysis_hash=None):
    weights = {c["id"]: c["weight"] for c in analysis["criteria"]}
    cells = {(c["candidate_id"], c["criterion_id"]): c for c in analysis["cells"]}
    totals = []
    for candidate in packet["job"]["candidates"]:
        cid = candidate["id"]
        known = [(weight, cells[(cid, criterion)]["score"]) for criterion, weight in weights.items() if cells[(cid, criterion)]["score"] is not None]
        coverage = sum(weight for weight, score in known)
        lower = sum(weight * score / 5 for weight, score in known)
        gates = [g["status"] for g in analysis["gates"] if g["candidate_id"] == cid]
        totals.append({"candidate_id": cid, "name": candidate["name"],
                       "eligibility": "excluded" if "fail" in gates else "conditional" if "unknown" in gates else "eligible",
                       "lower_bound": round(lower, 6), "upper_bound": round(lower + 100 - coverage, 6),
                       "coverage_percent": round(coverage, 6),
                       "known_only_fit": round(100 * lower / coverage, 6) if coverage else None})
    totals.sort(key=lambda r: ({"eligible": 0, "conditional": 1, "excluded": 2}[r["eligibility"]], -r["lower_bound"], r["candidate_id"]))
    previous, rank = None, 0
    for index, row in enumerate(totals, 1):
        key = (row["eligibility"], row["lower_bound"])
        if key != previous:
            rank = index
        row["rank"] = rank
        previous = key
    eligible = [r["candidate_id"] for r in totals if r["eligibility"] == "eligible" and r["coverage_percent"] > 0]
    sensitivity = []
    for criterion in weights:
        for factor in (0.8, 1.2):
            varied = {key: value * (factor if key == criterion else 1) for key, value in weights.items()}
            total = sum(varied.values())
            varied = {key: 100 * value / total for key, value in varied.items()}
            scores = {cid: sum(weight * (cells[(cid, key)]["score"] or 0) / 5 for key, weight in varied.items()) for cid in eligible}
            leaders = [cid for cid, value in scores.items() if abs(value - max(scores.values())) < 1e-7] if scores else []
            sensitivity.append({"criterion_id": criterion, "factor": factor, "weights": {k: round(v, 6) for k, v in varied.items()},
                                "lower_bounds": {k: round(v, 6) for k, v in scores.items()}, "leaders": leaders})
    return {"schema": "bench.comparison-matrix/v1", "packet_sha256": analysis["packet_sha256"],
            "analysis_sha256": analysis_hash or sha(dump(analysis)), "totals": totals, "sensitivity": sensitivity,
            "criteria": analysis["criteria"], "cells": analysis["cells"], "gates": analysis["gates"],
            "recommendation": analysis["recommendation"]}


def md(value):
    return str(value).replace("|", "\\|").replace("\n", " ").replace("<", "&lt;").replace(">", "&gt;")


def cite(refs):
    return "; ".join(md(r["source_id"] + " · " + r["locator"]) for r in refs) or "Evidence missing"


def csv_safe(value):
    # Avoid spreadsheet formula interpretation of arbitrary human/vendor text.
    if isinstance(value, str) and value.lstrip().startswith(("=", "+", "-", "@", "\t", "\r")):
        return "'" + value
    return value


def render(packet, analysis, analysis_hash=None):
    matrix = calculate(packet, analysis, analysis_hash)
    names = {c["id"]: c["name"] for c in packet["job"]["candidates"]}
    criteria = {c["id"]: c for c in analysis["criteria"]}
    rows = ["# Technical vendor comparison", "", analysis["decision_summary"], "", "## Recommendation", "",
            "**" + analysis["recommendation"]["status"].title() + "**" +
            (": " + md(names[analysis["recommendation"]["candidate_id"]]) if analysis["recommendation"]["candidate_id"] else ""), "",
            analysis["recommendation"]["rationale"], "", "## Weighted comparison", "",
            "Bounds measure missing evidence, not statistical confidence. Unknown is not a failed requirement. Scores are 0–5; higher is better.", "",
            "| Rank | Candidate | Gate eligibility | Lower–upper /100 | Evidence coverage | Known-only fit /100 |",
            "|---:|---|---|---:|---:|---:|"]
    for total in matrix["totals"]:
        rows.append("| {rank} | {name} | {eligibility} | {lower_bound:.2f}–{upper_bound:.2f} | {coverage_percent:.1f}% | {known} |".format(**{**total, "name": md(total["name"])}, known="unknown" if total["known_only_fit"] is None else f'{total["known_only_fit"]:.2f}'))
    rows += ["", "## Criteria and weights", "", "Weight basis: **" + analysis["weight_basis"] + "**. " + analysis["weight_rationale"], "",
             "| Criterion | Weight | Business rationale |", "|---|---:|---|"]
    for criterion in criteria.values():
        rows.append(f'| {md(criterion["name"])} | {criterion["weight"]:g}% | {md(criterion["reason"])} |')
    rows += ["", "## Scores by criterion", "",
             "| Criterion / weight | " + " | ".join(md(name) for name in names.values()) + " |",
             "|---|" + "---:|" * len(names)]
    indexed = {(c["candidate_id"], c["criterion_id"]): c for c in analysis["cells"]}
    for criterion in criteria.values():
        values = []
        for candidate_id in names:
            cell = indexed[(candidate_id, criterion["id"])]
            values.append(str(cell["score"]) + "/5" if cell["score"] is not None else cell["status"])
        rows.append("| " + md(criterion["name"]) + f' / {criterion["weight"]:g}% | ' + " | ".join(values) + " |")
    rows += ["", "## Scoring rationale", "", "| Candidate | Criterion | Score | Points | Confidence | Rationale | Source location |", "|---|---|---:|---:|---|---|---|"]
    csv_output = io.StringIO(newline="")
    writer = csv.writer(csv_output)
    writer.writerow(["candidate", "criterion", "weight_percent", "score_0_to_5", "weighted_points", "status", "confidence", "rationale", "source_locations"])
    for cell in analysis["cells"]:
        criterion = criteria[cell["criterion_id"]]
        points = criterion["weight"] * cell["score"] / 5 if cell["score"] is not None else None
        value = cell["score"] if cell["score"] is not None else cell["status"]
        rows.append(f'| {md(names[cell["candidate_id"]])} | {md(criterion["name"])} | {value} | {points:.2f} | {cell["confidence"]} | {md(cell["rationale"])} | {cite(cell["refs"])} |' if points is not None else
                    f'| {md(names[cell["candidate_id"]])} | {md(criterion["name"])} | {value} | unknown | {cell["confidence"]} | {md(cell["rationale"])} | {cite(cell["refs"])} |')
        writer.writerow([csv_safe(v) for v in (names[cell["candidate_id"]], criterion["name"], criterion["weight"], "" if cell["score"] is None else cell["score"], "" if points is None else round(points, 6), cell["status"], cell["confidence"], cell["rationale"], cite(cell["refs"]))])
    rows += ["", "## Mandatory requirements", ""]
    if not analysis["gates"]:
        rows.append("No mandatory gates were supplied. Validate any mandatory requirements with the decision owner.")
    for gate in analysis["gates"]:
        requirement = next(g["requirement"] for g in packet["job"]["gates"] if g["id"] == gate["gate_id"])
        rows.append(f'- **{md(names[gate["candidate_id"]])} / {md(requirement)} — {gate["status"]}**: {md(gate["rationale"])} ({cite(gate["refs"])}).')
    rows += ["", "## Weight sensitivity", "", "Each criterion changes by ±20% relative, with all weights renormalized to 100. Leaders use conservative lower bounds among eligible candidates with evidence. This is a local check, not proof of robustness to all preferences or unknown facts.", "",
             "| Changed criterion | Factor | Conservative-score leaders |", "|---|---:|---|"]
    for scenario in matrix["sensitivity"]:
        rows.append(f'| {md(criteria[scenario["criterion_id"]]["name"])} | {scenario["factor"]:.1f} | {md(", ".join(names[x] for x in scenario["leaders"]) or "None eligible with evidence")} |')
    rows += ["", "## Assumptions", ""] + ["- " + md(x) for x in analysis["assumptions"]]
    rows += ["", "## Follow-up questions", ""]
    for gap in analysis["gaps"]:
        rows.append(f'- **{md(gap["owner"])}** / {md(names.get(gap["candidate_id"], "Shared"))}: {md(gap["question"])} Decision impact: {md(gap["decision_impact"])}')
    rows += ["", "## Source limitations", ""]
    for source in packet["sources"]:
        rows.append(f'- **{md(source["id"])}** ({md(source["kind"])}, {source["status"]}): {md(source["origin"])}. Retrieved {md(source["retrieved_at"])}. {md(source["limitations"])}')
    rows += ["", "## Score anchors", ""]
    for criterion in criteria.values():
        rows += ["### " + md(criterion["name"]), ""] + [f'- **{i}**: {md(criterion["anchors"][str(i)])}' for i in range(6)] + [""]
    rows += ["Checker acceptance establishes a bound, internally consistent package. Source truth, entailment and business judgment require the separate review. No purchase or adoption is authorized by this report.", ""]
    evidence = ["# Evidence ledger", "", "Verbatim excerpts selected for review. Citation containment does not prove entailment.", ""]
    used = set()
    for item in analysis["cells"] + analysis["gates"]:
        for ref in item["refs"]:
            key = tuple(ref[k] for k in ("source_id", "locator", "quote"))
            if key in used:
                continue
            used.add(key)
            evidence += ["## " + md(ref["source_id"] + " · " + ref["locator"]), "", "> " + md(ref["quote"]), ""]
    return {"matrix.json": dump(matrix), "report.md": "\n".join(rows).encode(),
            "matrix.csv": csv_output.getvalue().encode(), "evidence.md": "\n".join(evidence).encode()}


def main():
    require(len(sys.argv) == 2 and sys.argv[1] in ("render", "check"), "usage: compare.py render|check (from workspace)")
    raw = read_bytes("packet.json")
    packet = parse(raw)
    raw_analysis = read_bytes("output/analysis.json")
    analysis = parse(raw_analysis)
    validate(packet, analysis, sha(raw))
    artifacts = render(packet, analysis, sha(raw_analysis))
    output = Path("output")
    require(not output.is_symlink(), "output symlink")
    if sys.argv[1] == "render":
        for filename, data in artifacts.items():
            target = output / filename
            require(not target.is_symlink(), "artifact symlink")
            target.write_bytes(data)
    else:
        require(set(p.name for p in output.iterdir()) == set(artifacts) | {"analysis.json"}, "unexpected/missing output files")
        for filename, data in artifacts.items():
            require(read_bytes(output / filename) == data, "stale/mismatched artifact: " + filename)
    print("valid current comparison package")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, TypeError, OSError) as error:
        print("comparison: " + str(error), file=sys.stderr)
        sys.exit(1)
