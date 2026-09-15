#!/usr/bin/env python3
"""Reviewed numerical routines. Inputs and declarative plans are data, never code."""
import hashlib
import json
import math
import os
from pathlib import Path
import sys

import numpy as np
import polars as pl
import scipy
from scipy import stats


def need(value, message):
    if not value:
        raise ValueError(message)


def pairs(items):
    out = {}
    for key, value in items:
        need(key not in out, "duplicate JSON key: " + key)
        out[key] = value
    return out


def parse(raw):
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda s: (_ for _ in ()).throw(ValueError("nonfinite JSON")))


def data(path, maximum=2_000_000):
    p = Path(path)
    need(not any(x.is_symlink() for x in [p, *p.parents]), "symlink input/output")
    need(p.is_file() and p.stat().st_size <= maximum, "missing or oversized file: " + str(p))
    return p.read_bytes()


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def dump(value):
    return (json.dumps(value, indent=2, ensure_ascii=False, allow_nan=False) + "\n").encode()


def shape(value, keys, where):
    need(type(value) is dict and set(value) == set(keys.split()), "fields: " + where)


def text(value):
    need(isinstance(value, str) and value.strip() and len(value) <= 10000, "nonempty bounded text required")


def texts(value):
    need(type(value) is list and len(value) <= 50, "bounded text list required")
    for item in value:
        text(item)


def ids(items):
    need(type(items) is list, "list required")
    result = [i["id"] for i in items]
    for item in result:
        text(item)
    need(len(result) == len(set(result)), "duplicate IDs")
    return set(result)


def validate_request(request):
    shape(request, "schema business_case groups confidence_level datasets comparisons", "request")
    need(request["schema"] == "bench.polars-analysis-request/v1", "request schema")
    text(request["business_case"])
    texts(request["groups"])
    need(1 <= len(request["groups"]) <= 12 and len(set(request["groups"])) == len(request["groups"]), "unique 1–12 groups")
    need(type(request["confidence_level"]) in (int, float) and request["confidence_level"] in (.90, .95, .99), "confidence must be .90/.95/.99")
    ids(request["datasets"])
    need(len(request["datasets"]) <= 8, "at most 8 datasets")
    metrics = {}
    for dataset in request["datasets"]:
        shape(dataset, "id path sha256 group_column unit_column metrics", "dataset")
        p = Path(dataset["path"])
        need(not p.is_absolute() and bool(p.parts) and p.parts[0] == "inputs" and ".." not in p.parts and p.suffix in (".csv", ".parquet"), "local CSV/Parquet under inputs required")
        for key in ("group_column", "unit_column", "sha256"):
            text(dataset[key])
        need(dataset["group_column"] != dataset["unit_column"], "group and unit columns must differ")
        ids(dataset["metrics"])
        need(bool(dataset["metrics"]), "dataset must select metrics")
        for metric in dataset["metrics"]:
            shape(metric, "id column unit measurement_level", "metric")
            for key in ("column", "unit"):
                text(metric[key])
            need(metric["column"] not in (dataset["group_column"], dataset["unit_column"]), "metric cannot be identifier column")
            need(metric["measurement_level"] in ("continuous", "ordinal"), "measurement level")
            need(metric["id"] not in metrics, "globally unique metric IDs")
            metrics[metric["id"]] = metric
    need(len(metrics) <= 24, "at most 24 metrics")
    ids(request["comparisons"])
    need(len(request["comparisons"]) <= 20, "at most 20 planned comparisons")
    for comparison in request["comparisons"]:
        shape(comparison, "id metric_id groups method design", "comparison")
        need(comparison["metric_id"] in metrics, "unknown comparison metric")
        need(type(comparison["groups"]) is list and len(comparison["groups"]) == 2 and len(set(comparison["groups"])) == 2 and set(comparison["groups"]) <= set(request["groups"]), "two distinct supplied groups required")
        need(comparison["method"] in ("descriptive", "welch", "paired"), "supported method required")
        shape(comparison["design"], "independent_units sampling_basis normality_basis", "design")
        need(type(comparison["design"]["independent_units"]) is bool, "independent_units must be boolean")
        for key in ("sampling_basis", "normality_basis"):
            need(isinstance(comparison["design"][key], str), "design basis must be text")


def validate_plan(request, plan, request_hash):
    shape(plan, "schema request_sha256 assessment decisions questions", "analysis plan")
    need(plan["schema"] == "bench.polars-analysis-plan/v1", "plan schema")
    need(plan["request_sha256"] == request_hash, "stale request hash")
    need(os.environ.get("POLARS_REQUEST_SHA256", request_hash) == request_hash, "admitted request changed")
    text(plan["assessment"])
    texts(plan["questions"])
    need(ids(plan["decisions"]) == ids(request["comparisons"]), "exactly one decision per comparison")
    for decision in plan["decisions"]:
        shape(decision, "id action design_supported rationale", "decision")
        need(decision["action"] in ("infer", "describe") and type(decision["design_supported"]) is bool, "decision action/design")
        text(decision["rationale"])
        original = next(c for c in request["comparisons"] if c["id"] == decision["id"])
        need(decision["action"] != "infer" or original["method"] != "descriptive", "cannot upgrade descriptive request")


def finite_or_none(value):
    return float(value) if value is not None and math.isfinite(value) else None


def profile(request, root):
    profiles, frames, hashes = [], {}, []
    for dataset in request["datasets"]:
        path = root / dataset["path"]
        raw = data(path, 20_000_000)
        need(digest(raw) == dataset["sha256"], "dataset changed: " + dataset["id"])
        hashes.append({"id": dataset["id"], "path": dataset["path"], "sha256": digest(raw)})
        columns = [dataset["group_column"], dataset["unit_column"], *[m["column"] for m in dataset["metrics"]]]
        if path.suffix == ".csv":
            scan = pl.scan_csv(path, schema_overrides={c: pl.String for c in columns}, infer_schema_length=100, try_parse_dates=False)
        else:
            scan = pl.scan_parquet(path)
        schema = scan.collect_schema()
        need(set(columns) <= set(schema.names()), "missing dataset columns")
        frame = scan.select(list(dict.fromkeys(columns))).limit(100001).collect()
        need(frame.height <= 100000, "dataset exceeds 100000 rows")
        group = dataset["group_column"]
        unit = dataset["unit_column"]
        frame = frame.with_columns(pl.col(group).cast(pl.String), pl.col(unit).cast(pl.String))
        need(frame.filter(pl.col(group).is_null() | pl.col(unit).is_null() | (pl.col(group) == "") | (pl.col(unit) == "")).height == 0, "missing group/unit identity")
        need(set(frame[group].to_list()) <= set(request["groups"]), "unrecognized group in dataset")
        for metric in dataset["metrics"]:
            values = frame.select(pl.col(group).alias("group"), pl.col(unit).alias("unit"), pl.col(metric["column"]).cast(pl.String).alias("raw"))
            values = values.with_columns(pl.col("raw").cast(pl.Float64, strict=False).alias("value"))
            frames[metric["id"]] = values
            groups = []
            for gid in request["groups"]:
                selected = values.filter(pl.col("group") == gid)
                valid = selected.filter(pl.col("value").is_finite().fill_null(False))
                summary = valid.select(
                    pl.col("value").mean().alias("mean"), pl.col("value").median().alias("median"),
                    pl.col("value").std(ddof=1).alias("sample_sd"), pl.col("value").min().alias("minimum"),
                    pl.col("value").max().alias("maximum"), pl.col("value").quantile(.25, interpolation="linear").alias("q25"),
                    pl.col("value").quantile(.75, interpolation="linear").alias("q75"), pl.col("value").quantile(.95, interpolation="linear").alias("p95")
                ).row(0, named=True)
                groups.append({"group": gid, "n_rows": selected.height, "n_valid": valid.height,
                    "n_units": selected["unit"].n_unique(), "duplicate_unit_rows": selected.height - selected["unit"].n_unique(),
                    "missing": selected["raw"].null_count(),
                    "invalid_numeric": selected.filter(pl.col("raw").is_not_null() & pl.col("value").is_null()).height,
                    "nan": selected.filter(pl.col("value").is_nan()).height,
                    "positive_infinity": selected.filter(pl.col("value") == float("inf")).height,
                    "negative_infinity": selected.filter(pl.col("value") == float("-inf")).height,
                    "nonfinite": selected.filter(pl.col("value").is_not_null() & ~pl.col("value").is_finite()).height,
                    **{k: finite_or_none(v) for k, v in summary.items()}})
            profiles.append({"dataset_id": dataset["id"], "metric_id": metric["id"], "unit": metric["unit"],
                             "measurement_level": metric["measurement_level"], "groups": groups})
    return profiles, frames, hashes


def holm(values):
    """Holm step-down adjusted p-values in input order; absent tests use p=1."""
    order = sorted(range(len(values)), key=lambda i: values[i])
    adjusted = [1.0] * len(values)
    running = 0.0
    for rank, index in enumerate(order):
        running = max(running, min(1.0, (len(values) - rank) * values[index]))
        adjusted[index] = running
    return adjusted


def compute(request, plan, root, request_hash, plan_hash):
    validate_request(request)
    validate_plan(request, plan, request_hash)
    profiles, frames, hashes = profile(request, root)
    results = []
    for comparison in request["comparisons"]:
        decision = next(d for d in plan["decisions"] if d["id"] == comparison["id"])
        metric = next(p for p in profiles if p["metric_id"] == comparison["metric_id"])
        first, second = comparison["groups"]
        a = next(g for g in metric["groups"] if g["group"] == first)
        b = next(g for g in metric["groups"] if g["group"] == second)
        values = frames[metric["metric_id"]]
        left = values.filter((pl.col("group") == first) & pl.col("value").is_finite().fill_null(False)).select("unit", "value").sort("unit")
        right = values.filter((pl.col("group") == second) & pl.col("value").is_finite().fill_null(False)).select("unit", "value").sort("unit")
        reasons = []
        method = comparison["method"]
        design = comparison["design"]
        if method == "descriptive" or decision["action"] == "describe":
            reasons.append("Descriptive-only request or analyst decision.")
        if not decision["design_supported"] or not design["independent_units"]:
            reasons.append("Independence/sampling design is not supported by supplied evidence.")
        if any(not design[k].strip() or design[k].strip().lower() in ("unknown", "none", "not supplied") for k in ("sampling_basis", "normality_basis")):
            reasons.append("Sampling/normality basis absent; row count does not establish assumptions.")
        if metric["measurement_level"] != "continuous":
            reasons.append("Ordinal/subjective scores are not continuous independent measurements.")
        if any(g[k] for g in (a, b) for k in ("missing", "invalid_numeric", "nonfinite")):
            reasons.append("Missing/invalid/nonfinite observations require a reviewed missingness plan; no inference after silent exclusion.")
        if any(g["duplicate_unit_rows"] for g in (a, b)):
            reasons.append("Repeated unit IDs: row count would create pseudoreplication.")
        if min(a["n_valid"], b["n_valid"]) < 2:
            reasons.append("Fewer than two valid units per group; variance is not estimable.")
        shared = set(left["unit"]) & set(right["unit"])
        if method == "welch" and shared:
            reasons.append("Shared unit IDs contradict independent-group design; consider a paired design explicitly.")
        paired_count = len(shared) if method == "paired" else None
        if method == "paired" and (set(left["unit"]) != set(right["unit"]) or left.height != right.height):
            reasons.append("Paired IDs differ; do not silently drop unmatched pairs or align by row position.")
        result = {"id": comparison["id"], "metric_id": metric["metric_id"], "unit": metric["unit"], "groups": [first, second],
                  "requested_method": method, "status": "descriptive-only", "refusal_reasons": reasons,
                  "n_a": a["n_valid"], "n_b": b["n_valid"], "paired_count": paired_count,
                  "mean_difference_a_minus_b": a["mean"] - b["mean"] if a["mean"] is not None and b["mean"] is not None else None,
                  "statistic": None, "df": None, "p_value": None, "p_holm": None, "ci_low": None, "ci_high": None,
                  "confidence_level": request["confidence_level"], "ci_scope": "marginal, not multiplicity-adjusted", "holm_reject": None}
        if not reasons:
            x = left["value"].to_numpy()
            y = right["value"].to_numpy()
            if method == "paired":
                joined = left.join(right, on="unit", how="inner", validate="1:1", suffix="_b").sort("unit")
                x, y = joined["value"].to_numpy(), joined["value_b"].to_numpy()
                variance = float(np.var(x - y, ddof=1))
            else:
                variance = float(np.var(x, ddof=1) / len(x) + np.var(y, ddof=1) / len(y))
            if not math.isfinite(variance) or variance <= 0:
                reasons.append("Degenerate/zero sampling variance; a finite t inference cannot be justified here.")
            else:
                test = stats.ttest_rel(x, y, alternative="two-sided", nan_policy="raise") if method == "paired" else stats.ttest_ind(x, y, equal_var=False, alternative="two-sided", nan_policy="raise")
                interval = test.confidence_interval(confidence_level=request["confidence_level"])
                numerical = [test.statistic, test.df, test.pvalue, interval.low, interval.high]
                need(all(math.isfinite(float(v)) for v in numerical), "nonfinite statistical result")
                result.update(status="inferential", statistic=float(test.statistic), df=float(test.df), p_value=float(test.pvalue), ci_low=float(interval.low), ci_high=float(interval.high))
        results.append(result)
    family = [r for r in results if r["requested_method"] != "descriptive"]
    for result, adjusted in zip(family, holm([r["p_value"] if r["p_value"] is not None else 1.0 for r in family])):
        if result["status"] == "inferential":
            result["p_holm"] = adjusted
            result["holm_reject"] = adjusted <= round(1 - request["confidence_level"], 12)
    return {"schema": "bench.polars-statistics/v1", "request_sha256": request_hash, "plan_sha256": plan_hash,
            "datasets": hashes, "versions": {"polars": pl.__version__, "numpy": np.__version__, "scipy": scipy.__version__},
            "profiles": profiles, "comparisons": results, "holm_family_size": len(family),
            "assessment": plan["assessment"], "questions": plan["questions"],
            "limits": ["All inference is conditional on supplied sampling/design assumptions; no automatic causal or population generalization.",
                       "Holm corrects the complete declared inferential family. Reported confidence intervals are marginal, not simultaneous.",
                       "Missing/nonfinite/invalid values are counted, never zero-filled. No missingness model or robust/cluster/time-series inference is implemented.",
                       "A non-significant result does not establish equivalence; p-values do not quantify practical importance or the probability a vendor is better."]}


def report(value):
    out = ["# Statistical analysis", "", value["assessment"], "", "## Data quality and descriptive profiles", "",
           "Values below describe supplied observations. n_rows and n_units differ for repeated observations; only the latter can support independent-unit inference.", ""]
    if not value["profiles"]:
        out += ["No observational dataset was supplied. Statistical inference is not applicable. Vendor claims and weighted decision scores are not samples.", ""]
    for profile_ in value["profiles"]:
        out += ["### " + profile_["metric_id"] + " (" + profile_["unit"] + ")", "", "```json", json.dumps(profile_, ensure_ascii=False, indent=2), "```", ""]
    out += ["## Requested comparisons", ""]
    for result in value["comparisons"]:
        out += ["### " + result["id"], "", "```json", json.dumps(result, ensure_ascii=False, indent=2), "```", ""]
    out += ["## Interpretation limits", ""] + ["- " + line for line in value["limits"]]
    out += ["", "## Followups", ""] + ["- " + line for line in value["questions"]]
    out += ["", "## Reproducibility", "", "Dependencies: " + json.dumps(value["versions"], sort_keys=True),
            "", "Exact request SHA-256: " + value["request_sha256"], "Exact analysis-plan SHA-256: " + value["plan_sha256"], ""]
    return "\n".join(out).encode()


def main():
    need(len(sys.argv) == 2 and sys.argv[1] in ("profile", "render", "check"), "usage: analyze.py profile|render|check from analyst workspace")
    raw_request = data("request.json")
    request = parse(raw_request)
    validate_request(request)
    if sys.argv[1] == "profile":
        profiles, _, hashes = profile(request, Path.cwd())
        print(dump({"profiles": profiles, "datasets": hashes}).decode(), end="")
        return
    raw_plan = data("output/analysis-plan.json")
    plan = parse(raw_plan)
    calculated = compute(request, plan, Path.cwd(), digest(raw_request), digest(raw_plan))
    artifacts = {"statistics.json": dump(calculated), "report.md": report(calculated)}
    output = Path("output")
    need(not output.is_symlink(), "output directory symlink")
    if sys.argv[1] == "render":
        for name, raw in artifacts.items():
            need(not (output / name).is_symlink(), "output file symlink")
            (output / name).write_bytes(raw)
    else:
        need({p.name for p in output.iterdir()} == set(artifacts) | {"analysis-plan.json"}, "unexpected/missing analyst output")
        for name, raw in artifacts.items():
            need(data(output / name) == raw, "stale/incorrect analyst artifact: " + name)
    print("valid current Polars statistical package")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, TypeError, OSError, pl.exceptions.PolarsError) as error:
        print("polars-analyst: " + str(error), file=sys.stderr)
        sys.exit(1)
