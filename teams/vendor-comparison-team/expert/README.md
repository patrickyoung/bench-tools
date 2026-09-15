# Vendor comparison team

A reusable team for technical product, service and vendor decisions. Supply a
candidate list, business need, presentations, transcripts, website URLs and
human notes. The team returns an evidence-linked comparison matrix, explicit
weights and score anchors, rationale, mandatory-gate results, coverage and
uncertainty, weight sensitivity, and independent review.

## Roles and reused expertise

| Role | Library worker | Responsibility |
|---|---|---|
| Manager | `product-owner` unchanged | Frame the business need, scope, criteria, priorities and diligence plan. |
| Analyst | `polars-analyst` | Profile observations, assess declared design and render supported statistics or explicit descriptive refusals. |
| Comparison | `vendor-comparison` | Read the evidence, score every candidate against anchored criteria, explain tradeoffs and produce deterministic tables. |
| Reviewer | `product-owner` unchanged, new context | Check every scored cell and gate against evidence and business priorities; return proceed, revise or hold. |

The comparison specialist reuses Enterprise Architect's business-outcomes,
product-capabilities and finops skills and Product Owner's evidence-discovery
skill. The new analyst fills the statistical-design and observational-analysis
gap between planning and the existing comparison specialist; it also reuses
evidence-discovery. Weighted-comparison expertise, its numeric contract, source
preparation and explicit handoffs remain in place. The reviewer has a separate
model history and workspace; it shares the selected model and is not an
independent human auditor. Operator evaluation and domain experts remain
appropriate for consequential procurement.

The four sequential stages (manager → analyst → comparison → reviewer) are a
fixed composition of public `agent run` commands in
`bin/compare-team`. There is no new scheduler, provider client, model loop,
background service or host-subagent controller. Members can be exported and
run separately with their documented input contracts. A source template must
be assembled with `scripts/workers export-team` before running: the exporter
adds its roster under `agents/`.

## Input packet

Place `job.json` beside its local materials. The only required fields are a
nonempty business case and 2–12 candidates. Use stable IDs for source mapping.
The following is a generic input template, not a vendor evaluation:

```json
{
  "business_case": "Describe the outcome, consumers, scale, timeframe and decision needed.",
  "candidates": [
    {"id": "option-a", "name": "First option", "plan": "Selected edition"},
    {"id": "option-b", "name": "Second option", "plan": "Selected edition"}
  ],
  "materials": [
    {"id": "a-deck", "candidate_ids": ["option-a"], "kind": "presentation", "path": "materials/a.pptx"},
    {"id": "b-demo", "candidate_ids": ["option-b"], "kind": "transcript", "path": "materials/b.vtt"},
    {"id": "a-site", "candidate_ids": ["option-a"], "kind": "website", "url": "https://example.com/product"}
  ],
  "notes": "Stakeholder preferences, known constraints, questions and prior human input.",
  "gates": []
}
```

Candidates may also be names as strings; IDs become candidate-1, candidate-2,
etc. Optional candidate fields are `offering`, `version`, `plan`. Material IDs
must be unique, with exactly one `path` or `url`. Paths are relative to the job
directory, regular files only; no traversal or symlinks. `candidate_ids: []`
denotes a shared source whose actual statements still need candidate-specific
attribution. Source kind is recorded for human interpretation; it does not
automatically imply truth or strength.

Optional `criteria` is an ordered array of 3–12 records with `id`, `name` and
positive numeric `weight`, summing to 100. Optional `reason` and `anchors`
are retained exactly when supplied. Anchors have six keys `"0"` through `"5"`,
with criterion-specific observable meanings, higher always better. The team
fills missing reasons/anchors and labels the assumptions. Without criteria it
proposes them and flags weights for stakeholder review. Supplied weights are
never silently renormalized or replaced.

Optional `gates` is an array of `{ "id": "stable-id", "requirement": "The
mandatory capability/constraint" }`. Gates are evaluated as pass/fail/unknown
for every candidate. A failed gate prohibits selection, and an unknown gate
prohibits an unconditional recommendation. Narrative constraints are discussed
even without gates, but callers should encode mandatory requirements here for
mechanical enforcement.

## Optional statistics

Add an optional `statistics` object to the job above. This generic example
declares independent measurements; replace the design assertions with supported
study evidence, not merely assurances needed to enable a test:

```json
{
  "statistics": {
    "datasets": [
      {
        "id": "measurements",
        "path": "datasets/measurements.csv",
        "group_column": "candidate_id",
        "unit_column": "unit_id",
        "metrics": [
          {"id": "latency", "column": "latency_ms", "unit": "ms", "measurement_level": "continuous"}
        ]
      }
    ],
    "comparisons": [
      {
        "id": "latency-a-b",
        "metric_id": "latency",
        "groups": ["option-a", "option-b"],
        "method": "welch",
        "design": {
          "independent_units": true,
          "sampling_basis": "Separate randomly sampled units assigned to each option under the supplied study protocol.",
          "normality_basis": "Supplied study evidence supports approximately normal measurements at the independent-unit level."
        }
      }
    ]
  }
}
```

`confidence_level` defaults to .95; supported values are 0.90, 0.95 and 0.99.
Dataset entries have exactly `id,path,group_column,unit_column,metrics`;
the controller adds admitted paths and SHA-256 hashes for the analyst.
Use local regular CSV or Parquet files relative to the job directory, without
traversal or symlinks. Preserve real unit IDs and original observations, never
replace identity with row position. Limits are eight datasets, 20 MB and
100000 rows per dataset. Each row's group must be a candidate ID. Metric entries
have exactly `id,column,unit,measurement_level`, with at most 24 globally unique metric IDs
and measurement level `continuous` or `ordinal`.

At most 20 comparisons have exactly `id,metric_id,groups,method,design`; groups are two
distinct candidate IDs in A,B order. Methods are `descriptive`, `welch` or
`paired`. Design has exactly boolean `independent_units` and text
`sampling_basis,normality_basis`. For paired inference, independence concerns
pairs and normality concerns differences. These are declared assertions, not
software-verified sampling facts. Absent statistics or valid empty selections
still receive a no-inference assessment; malformed input is not an empty case.

The manager receives `inputs/statistical-intake.json`: admitted dataset identities
and computed preflight profiles. Raw datasets are already available to the
analyst; absence of rows from the manager context is not an evidence gap.
The analyst owns statistical outputs; Vendor Comparison owns the matrix.

Before any model calls, preparation validates schemas, runtime dependencies
and the full selected data through the analyst profiler. Invalid input or
unavailable dependencies stop the run. Valid data with unsupported inference
can instead produce descriptive profiles and explicit refusal reasons.
The analyst authors a method plan and uses the reviewed deterministic renderer;
it may downgrade to description, never upgrade design or substitute a test.

Polars profiles include missing/nonfinite/invalid values, row and unique-unit
counts and descriptive summaries; missing values never become zero. Welch
requires supported independent continuous samples with disjoint unit IDs.
Paired tests match exact unit IDs with validated 1:1 joins, never row order.
Missing/invalid values, repeated units, fewer than two units per group, ordinal
metrics, unsupported design, unmatched pairs or degenerate variance block
inference. Repeated-measurement descriptions describe rows, not independent
units. Sampling and normality/robustness need substantive support, not row count
or an automatic normality test alone.

Effects and marginal confidence intervals retain original units and A-minus-B
orientation. Two-sided raw p-values and Holm-adjusted p-values cover the full
declared inferential family, with ineligible members conservatively treated as
p=1. Confidence intervals are marginal, not multiplicity-adjusted or simultaneous.
Small p-values do not establish practical or causal superiority; large p-values
do not establish equivalence. Never automatically convert p-values into vendor
scores, evidence confidence or procurement decisions. Weight sensitivity is
decision-preference sensitivity, not statistical confidence.

See `agents/analyst/CONTRACT.md` and `agents/analyst/README.md` for the standalone
request, plan, output and recomputation contracts.

## Materials and source acquisition

- UTF-8 TXT, Markdown, VTT, SRT, CSV, JSON and logs preserve line locators.
- PPTX extracts slide text and linked speaker notes with slide locators.
- PDF uses installed `pdftotext -layout`, retaining page locators.
- DOCX extracts body text, including paragraph/table text, with a body locator.
- HTML extracts static text and removes scripts/style content. Only explicitly
  listed HTTP(S) URLs are fetched, once, with bounded requests. No link crawling,
  browser execution, authentication, cookies or private-network access is added.
- Use `--offline` to retain URLs as unavailable and use supplied snapshots.
  Supply downloaded files for authenticated sites or dynamic pages. Individual
  acquisition/extraction failures become explicit evidence gaps; they do not
  silently disappear or become negative scores.

Raw snapshots and hashes, extraction limitations and timestamps are retained.
A retrieval timestamp does not prove publication freshness. Text extraction
does not interpret diagrams, images, charts or scanned pages. Provide OCR,
captions, a transcript or a separate reviewed visual summary for those; the team
must report that limitation. DOCX headers/footnotes/comments and old .doc/.ppt
formats need an explicit export to a supported format. No model claims visual
inspection from text alone.

Limits: 32 materials, 20 MB per raw source, 40 MB decompressed Office archive,
3,000 ZIP entries, 900 KB normalized packet, 12 gates, and 2–12 candidates.
Oversized packets require explicit curation or splitting; they are not silently
truncated. Product Owner inputs/outputs are bounded to 1 MiB per file; the statistical contract bounds its own files to 2 MB.

## Run

Requirements: Python >=3.9, Bench Agent/Ask/Brief/Ply/Cage/Record from the declared
toolkit pin, `pdftotext` for PDF inputs, and an operator-configured model/credential
connection. Export and structural validation require no model credentials.
Keep definition and controller records outside worker-writable directories.

The analyst additionally requires an operator-provisioned isolated Python >=3.12
environment. From this definition directory, use that environment's pip to run
`pip install -r agents/analyst/requirements.txt`, then set `POLARS_PYTHON` to its
absolute Python interpreter path. It otherwise defaults to `python3`, which must
meet the same analyst requirements. Polars supplies data processing and SciPy
the t-distribution calculations. The host maintains the software pins in
`agents/analyst/requirements.txt`; dependency metadata is not source evidence.
Keep the installed environment, datasets, runs and evaluation records outside
the source definition; never install packages during analysis. Existing
deterministic code is reviewed source; analysis runs cannot replace it.

```sh
export ASK_MODEL='YOUR_CONFIGURED_PROVIDER/MODEL'
/absolute/assembled/expert/bin/compare-team \
  /absolute/current-input/job.json /absolute/new-run
```

Select exact executables with `COMPARISON_AGENT` and normal Agent selectors
such as `AGENT_ASK`, `AGENT_RECORD` and PATH. Optional controls are
`COMPARISON_TURNS` (default 12 per role), `COMPARISON_EFFORT` (high), and
`COMPARISON_TIMEOUT` (12m per role). These are effort bounds, not a dollar cap.
Credentials stay in the existing operator connection. No secret values belong
in job files, source, output, logs or the runbook.

A **fresh run directory is mandatory**. New notes, new sources, changed weights
or a revised business case are a new packet and run. Nothing reads or learns
from prior runs automatically. Follow up by changing the current job/materials
and running again with a new destination. Existing stages and records are
retained if work stops; the entry command does not retry or resume automatically.
Diagnose unknown outcomes before deliberately running a new attempt.

## Outputs and outcomes

```
new-run/
  control/       original job/packet, analyst request/profile, derived comparison
                 packet, admissions and input hashes
  materials/     fetched/copied raw snapshots
  datasets/      admitted raw CSV/Parquet datasets
  stages/        manager, analyst, comparison, reviewer work and state
  records/       each Agent's stdout/stderr, model sessions, Record receipts
  result/        report.md, matrix.csv, matrix.json, analysis.json, evidence.md,
                 intake.md/json, review.md/json, statistics.json,
                 statistical-plan.json, statistics.md, manifest.json
  status.json
```

The analyst has its own `stages/analyst/` workspace and `records/analyst/`
streams, sessions and receipts, alongside the other three roles. Its
`request.json`, advisory manager `planning.md` and admitted `inputs/datasets/`
are explicitly handed off and bound. It writes `output/analysis-plan.json`,
`output/statistics.json` and `output/report.md`, exported under `result/` as
`statistical-plan.json`, `statistics.json` and `statistics.md`.

Only after the analyst check accepts does the controller create the derived
comparison packet. It retains original sources and appends `analyst-profile-*`
and `analyst-test-*` computed source passages with locators and the statistics
artifact hash. Manager planning and analyst assessment remain advisory.
`control/comparison-admission.json` binds original/derived packet hashes and
the statistics hash; the final manifest records both packet hashes and final
artifact hashes.

The fresh Product Owner reviewer receives the original source documents as
normalized in the original packet, the derived packet and declared study design,
manager plan, comparison artifacts, and the full computed profiles/results,
statistical plan and report. It audits counts, missingness, pair matching, effect
sign/units, interval scope, Holm adjustment and interpretation against design.
Raw datasets remain in the analyst's admitted input and are recomputed by the
trusted checker; the reviewer is not claimed to read every raw row. It must
check every scored cell and gate against cited evidence and anchors, including
faithful use of computed passages. Descriptive-only results are legitimate;
a passing calculation check does not establish defensible assumptions.

The manager can stop with questions before matrix work. Otherwise `result/`
contains the comparison and review, including results that require revision.
The report heading and status explicitly distinguish these. Exit codes:

| Code | Meaning |
|---:|---|
| 0 | Four stages completed; independent review says proceed. Advice is reviewable, not approved for purchase. |
| 2 | Reviewer requires revision, or an Agent stopped unfinished; inspect status and role records. |
| 75 | Manager/reviewer needs human input, or Agent waits. |
| Other | Exact Agent failure/decline/boundary/interruption status, or invalid input/check failure. |

Missing or conflicting cells retain null scores. The report shows lower and
upper bounds, evidence coverage and known-only fit together; confidence remains
separate. Bounds describe missing evidence, not probabilities. The JSON includes
the 2-per-criterion ±20% relative weight scenarios; equal conservative scores
share rank and leaders. Gate eligibility precedes score. A provisional ranking
does not remove the need to resolve decision-sensitive gaps.

The team checks every handoff against admitted input bytes, validates member
output contracts, and binds final artifacts with a manifest. `bin/check` runs
from a completed run (or with `COMPARISON_RUN=/absolute/run`) and is read-only.
It does not rerun Agent. The analyst's read-only check recomputes statistics
from hashed datasets and the plan using the host-reviewed code and configured
interpreter. It checks calculations and byte bindings, not sampling truth or
design validity. Structural checks cannot establish source truth,
entailment, visual extraction completeness, procurement quality or approval.
Agent's Cage limits writes/network, not host reads; separate role directories
are not a filesystem secrecy boundary.

## Verification

Source tests live beside the team, outside the exportable definition. Set
`COMPARISON_TEAM_EXPERT` to a clean assembled export and run the supplied
`tests/test_contracts.py`. The suite exercises deterministic acceptance and
rejection, extraction, local disposable HTTP snapshots and handoff integrity.
Synthetic case generation writes to an explicit external destination. Model
trials and their qualitative review belong in an external `EVALUATION.md`.
Retain automatic Agent recordings and verify their terminal indexes and Record
receipts; valid recordings establish observed bytes/outcomes, not business truth.
