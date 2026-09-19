# Polars analyst contract v1

Standalone input: `request.json` and explicitly selected local datasets under
`inputs/`. Optional `planning.md` is advisory. Output: `output/analysis-plan.json`
(authored), `output/statistics.json` and `output/report.md` (deterministically
rendered by this definition's `tools/analyze.py`). No other output files.
Use `${POLARS_PYTHON:-python3}` to invoke the tool with `render`, then `bin/check`.

request.json has schema `bench.polars-analysis-request/v1`, `business_case`,
`groups` (unique string IDs), `confidence_level` (required: 0.90, 0.95 or 0.99; team default .95),
`datasets` and `comparisons`. Each dataset has exactly:
`id,path,sha256,group_column,unit_column,metrics`. Paths are regular files under
inputs/; CSV or Parquet, at most 20MB and 100000 rows. Metrics have exactly
`id,column,unit,measurement_level` (continuous or ordinal). Metric IDs are unique
across datasets. Each row's group must be a supplied group; unit identifies the
independent experimental/sampling unit, not merely a row number. Original unit
IDs must be preserved. Repeated units remain visible and block inferential tests.
No invented data, imputation, dropped outliers, joins that multiply rows or
silent aggregation of repeated observations.

Each comparison has exactly `id,metric_id,groups,method,design` with two distinct
groups in A,B order and method `descriptive|welch|paired`. design has exactly
`independent_units` (boolean), `sampling_basis` and `normality_basis` strings.
For paired tests, independence refers to pairs and normality to differences.
These fields are supplied design assertions, not facts verified by the software.
An analyst may downgrade a request to descriptive, never upgrade its design or
silently substitute another inferential method. Empty requests/datasets are valid:
assess the lack of observations and do not infer from vendor claims or scores.

analysis-plan.json has exactly `schema,request_sha256,assessment,decisions,questions`.
Schema is `bench.polars-analysis-plan/v1`. request_sha256 hashes exact request bytes.
assessment is a concise substantive analytical assessment and limitations.
decisions has exactly one entry per requested comparison with
`id,action,design_supported,rationale`; action `infer|describe`, design_supported
boolean, rationale explains whether the supplied sampling/design assumptions
support the requested method. questions is a string array of decisive followups.
Only infer when assumptions are supported by the supplied material. Return
`describe` for a claim of independence contradicted by repeated trials on one
subject, sequential correlated data, weak convenience samples, ordinal decision
scores, or evidence that does not support normality/robustness. Do not infer
normality or independence from row count or an automatic normality test alone.

The deterministic engine uses Polars lazy CSV/Parquet scans, explicit casts,
null/NaN/infinity/invalid-value audits, per-group row and unique-unit counts,
mean/median/sample-SD(ddof=1)/range and linear quartiles/p95. No missing value is
replaced with zero. Descriptions of repeated measurements describe rows, not
independent units. It refuses inference for missing/nonfinite/invalid values,
repeated units, fewer than two units/group, ordinal metrics, unsupported design,
shared independent-group unit IDs, mismatched paired IDs or degenerate variance.
Paired observations are joined on unit ID with validated 1:1 cardinality, never
row position. Welch handles independent unequal-variance samples. SciPy provides
two-sided t statistics, p-values, degrees of freedom and marginal confidence
intervals for A minus B; effect sizes are mean differences in original units.
Holm-adjusted p-values cover the full predeclared inferential comparison family,
including ineligible members conservatively treated as p=1. Confidence intervals
are marginal and not multiplicity-adjusted; do not call them simultaneous.
Report precision, small samples and causal/generalization limits. A large p-value
is not evidence of equivalence and a small p-value is not practical importance.
Business thresholds and direction need domain interpretation; no auto-adoption.

statistics.json records input/plan/data hashes, actual dependency versions,
profiles and computed results with explicit refusal reasons and full pair counts.
It is reproducible from the exact inputs, plan, installed dependencies and code.
The read-only check recomputes these using trusted definition code; it never
executes model-authored programs. The checker establishes calculations, structure
and byte bindings, not correct sampling, source truth or defensible assumptions.
The team's separate reviewer audits interpretations and may require revision.
Keep every run, raw dataset, installed environment and evaluation outside source.
