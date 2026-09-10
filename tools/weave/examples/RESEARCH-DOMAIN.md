# Invoice research domain

`research_domain.py` is a new, deterministic synthetic benchmark for comparing
bounded parameter search, one agent, and a small adaptive team. It supplies a
shared candidate contract and evaluator. It does not schedule work, optimize,
call models, enforce evaluation budgets, or hide the final dataset.

The earlier `business.py` workflows and their reported results remain historical
demos. They are not retroactively reinterpreted as this benchmark.

## Provenance and scope

The fixture has 24 development cases, 24 feedback-validation cases, and 24 final
holdout cases, with disjoint identities. Every processing duration and treatment
outcome is explicitly **synthetic and uncalibrated**. Recorded observations here
are stipulated counterfactuals, not measurements from a company or estimates of
causal effects. The fixture includes imperfect risk flags, incomplete review
detection, and defects outside the audit cohort in later splits.

The input benchmark and code should be frozen and hashed before an experimental
arm begins. `benchmark_digest()` binds the complete canonical JSON benchmark;
evaluation results also name their split and its digest. `public_evidence()`
returns the development cases, their retrospective adjudication, the evaluator
contract and intervention meanings. It omits feedback and final rows, labels,
statistics, and rankings. There is no best-configuration lookup.

The controller may reveal aggregate feedback-validation results within the
predeclared evaluation allowance. It must freeze **every arm's selection before
releasing any final-holdout result**. These pure Python APIs do not enforce that
rule. Public source and synthetic data are not a confidential evaluation set;
report-only agents must receive only the evidence the controller admits. Final
results must not be used to revise candidates in this trial.

Historical data, when supplied, need a separate reviewed ingestion and evaluation
design. At minimum, establish arrival and completion timestamps, waiting versus
staff labor, receipt availability, deadlines, case features known at decision
time, correction outcomes, and intervention provenance. Historical associations
alone do not establish the counterfactual effects stipulated in this simulator.
This version deliberately rejects claims of real calibration.

## Candidate contract

Exactly four choices define each of 36 candidates:

| Field | Choices | Meaning |
| --- | --- | --- |
| `order` | `fifo`, `earliest_due`, `shortest` | Select only among currently ready stages; work is nonpreemptive |
| `receipt_precheck` | `false`, `true` | Pay recorded staff time before handling; any receipt wait releases staff |
| `review` | `all`, `flagged`, `flagged_or_audit` | Review every case, risk-flagged cases, or flagged plus the fixed audit cohort |
| `batch` | `single`, `pairs` | Consecutive arrival pairs wait for both arrivals and use their recorded setup cost |

Agents cannot change handling times, detection outcomes, staffing, deadlines,
audit membership, ground truth, or evaluator thresholds. Extra candidate fields
are rejected. The baseline is FIFO, no early receipt check, full review, and
single invoices. `all_configs()` enumerates the space in declared option order;
it performs no search or ranking.

Receipt checks, handling/review and corrections share the same staff pool. Waits
release staff. Undetected defects return after their recorded discovery delay
and consume recorded correction labor. Review is not assumed to catch every
defect. A pair's setup savings do not erase its initial waiting time. A final
unpaired invoice uses the single-invoice setup observation.

`shortest` uses the fixed observable duration of a ready stage, never adjudication
or future arrivals. Actual operations would require validated duration estimates;
their availability here is another synthetic assumption.

## Frozen evaluator

Every split runs three scenarios with equal objective weights: normal demand,
40% faster arrival rate, and one staff member absent. Staff is fixed at two,
reduced to one in the absence scenario. These equal weights are a stress-test
choice, not observed business frequencies. The observation horizon is 480
minutes; the simulation drains remaining work to measure final cycle times and
also reports backlog at the horizon. Durations and metrics are in minutes,
with displayed metrics rounded to six decimal places. P95 uses nearest rank.

**Absolute feasibility is required in every scenario:**

- At least 95% of invoices meet their case-specific deadline.
- At most 5% require rework after first-pass processing.
- No arrived invoice remains in backlog at the horizon.

**Relative improvement is a separate question:** the equally weighted mean of
the three scenario medians must fall by at least **10%**, and each scenario must
have no p95 or labor increase relative to its own baseline. Both absolute and
relative tests must pass for `eligible: true`. Beating a baseline with unacceptable
service is insufficient. Feasible but slower results and all-negative searches
are valid completed experiments.

This differs from the historical v1 demo's requirement to lower the median in
every scenario. A receipt check can add routine-case time while improving an
absence scenario. The new contract exposes these tradeoffs through the declared
equal-weight objective while preserving per-scenario absolute gates and p95/labor
guards. The 10% hurdle is retained; it is not lowered to ensure a winner. All
rules must be fixed before live agent runs.

`rank_key()` orders eligible results first, then other feasible results, followed
by lower mean median, lower worst p95 and lower mean labor. Canonical candidate
JSON breaks ties. Ranking an infeasible result does not make it eligible. Report
the absolute and relative findings alongside the selected candidate, including
an inconclusive or unsuccessful final result.

## API

```python
benchmark = synthetic_benchmark()
validate_benchmark(benchmark)
evidence = public_evidence(benchmark)
config = candidate(baseline_config())
result = evaluate(benchmark, config, "development")
key = rank_key(result)
```

Additional entry points are `candidate_schema()`, `all_configs()` and
`benchmark_digest()`. `candidate()` returns a fresh canonical object and raises
`ValueError` for invalid choices. `validate_benchmark()` rejects altered evaluator
rules, unsupported provenance, overlapping IDs, missing fields, nonfinite values
and invalid observations. Each split must have 1–1000 cases arriving within the
horizon. Durations cannot exceed 14,400 minutes; active staff stages must take at
least 0.001 minute. These are input bounds, not configurable interventions.

An evaluation contains `candidate`, `split`, both digests, per-scenario candidate
and baseline metrics, feasibility findings and relative changes, aggregate
`feasible`, `relative_improved`, `eligible`, and the objective summary. It always
states `claims_real_improvement: false`.

Run the focused offline checks from the Weave directory:

```sh
python3 -B -m unittest discover -s tests -p test_research_domain.py -v
```
