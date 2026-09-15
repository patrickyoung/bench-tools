---
name: polars-statistics
description: Assess vendor comparison measurements with Polars and the reviewed statistical contract. Use for schema and unit audits, design-gated Welch or paired inference, descriptive fallbacks, uncertainty and statistical report interpretation.
---

# Design-gated Polars statistics

Read the definition's `CONTRACT.md` first. This skill explains its assumptions;
it adds no methods, output fields or authority. Use the trusted renderer, not
model-authored analysis code.

## Design before computation

Identify the decision, metric, original unit of measurement, A-minus-B contrast,
target population, sampling/recruitment mechanism and observational or experimental
design. A row is not automatically an independent unit. Check whether a unit is a
person, organization, deployment or genuinely independent experimental instance.
Repeated trials, time series, clustered subjects and multiple metrics on one
subject do not create independent sample size. Independence concerns pairs for
paired analysis. Do not assign fresh IDs, silently aggregate, discard trials or
select convenient observations to escape repeated-unit refusals.

The request supplies `independent_units`, `sampling_basis` and `normality_basis`.
They are assertions, not software-verified facts. Corroborate them with selected
evidence. Downgrade to `describe` for unsupported assumptions, correlated
sequential data, repeated trials on one subject, weak convenience samples,
ordinal decision scores or unsupported normality/robustness. Do not infer
independence or normality from row count or an automatic normality test alone.
Do not invent recruitment, randomization, power or distributional evidence.
Supported within-sample estimates still may not generalize or identify causality;
label selection, survivorship, attribution and proxy bias.

Subjective vendor 0–5 ratings are decision judgments, not repeated empirical
observations. Other supplied ordinal measurements receive descriptive treatment
only under this contract. Neither sensitivity to weights nor agreement across
reviewers establishes statistical confidence. A valid no-data assessment states
what cannot be inferred and what observations would change the decision.

## Polars audit discipline

The trusted engine uses lazy `scan_csv` or `scan_parquet`. Lazy scans defer work;
a successful scan construction is not proof that parsing or validation succeeded.
Inspect source schema, then explicit casts and full audit results at execution.
Do not rely on a CSV inference prefix as proof of whole-file types. Preserve
string unit IDs (including leading zeros); do not cast identifiers to numeric.
Inspect invalid casts rather than silently coercing them away. Do not suppress
parse errors or filter bad records into an apparently clean inferential sample.

Distinguish null (missing) from floating NaN; Polars null operations do not
automatically account for NaN. Audit nulls, NaNs, positive/negative infinity and
invalid values separately. Never replace missingness with zero. Do not impute,
drop outliers or silently remove incomplete cases. Missingness may reflect
measurement failure or selective reporting, not random absence; the contract
refuses inference with missing/nonfinite/invalid values.

Confirm each group's membership, metric unit and measurement level, per-group row
count and unique-unit count. Do not mix milliseconds with seconds, currencies,
exposure windows, rates with totals or denominators across groups. If observations
are not commensurate, describe the mismatch and request corrected, explicitly
selected inputs; do not rewrite originals. Descriptions of repeated measures
describe rows, not independent units. Means, medians, sample SD (`ddof=1`), ranges,
linear quartiles and p95 do not alone justify a population model.

Joins must preserve grain and units. No row-multiplying joins. For paired tests,
match by original unit ID with validated 1:1 cardinality, never row order. Audit
duplicates, unmatched units and full pair counts; an inner join that hides
unmatched IDs is not acceptable. Independent groups must not share unit IDs.
Repeated IDs within groups or mismatched paired ID sets block inference. Do not
substitute a different test when a join or design check fails.

## Only the reviewed inferential methods

- **Welch:** two independent groups of continuous measurements; independent units
  within and across groups, a defensible sampling basis and normality/robustness
  basis for the mean comparison. Unequal variances are allowed; Welch is not an
  equal-variance pooled test and does not solve dependence or sampling bias.
- **Paired:** continuous measurements from the same matched units in A and B;
  independent pairs, defensible sampling and normality/robustness of the
  within-pair differences, not necessarily each marginal group. Preserve A,B
  direction and exact unit matches.
- **Descriptive:** the requested method or the conservative downgrade whenever
  inference is unsupported. Do not convert it into inference based on a
  promising result.

The engine also refuses inference for fewer than two units per group, repeated
units, ordinal metrics, unsupported design, missing/nonfinite/invalid values,
shared independent-group IDs, mismatched paired IDs and degenerate variance.
Passing these mechanical gates is necessary, not sufficient. In particular,
two units do not establish adequate precision or credible distributional support.

Do not introduce rank tests, bootstrap intervals, equivalence tests, regression,
cluster corrections, time-series models, power calculations or standardized
effect-size formulas as substitutes. If needed, explicitly defer them for a
scoped extension with reviewed method, assumptions, implementation and tests.

## Interpret the actual output

SciPy supplies two-sided t statistics, p-values, degrees of freedom and confidence
intervals at the supplied level (0.90, 0.95 or 0.99; default 0.95).
The effect size is the mean difference A minus B in original metric units.
For paired inference it is the mean within-pair difference. Do not invent
Cohen's d or other uncontracted effects. Explain direction in the business
context: negative latency and negative benefit have different meanings.

Holm-adjusted p-values cover the full predeclared inferential family; ineligible
members are conservatively p=1. Do not reduce the family to successful tests,
hide refusals, select only significant results or add post-hoc comparisons.
Confidence intervals are marginal, not multiplicity-adjusted or simultaneous.
Describe precision, small samples and conditional assumptions. A confidence
interval is not a probability that the fixed true effect lies within this
particular interval, and it does not quantify source authenticity.

A p-value is conditional on the null model and assumptions, not the probability
that a vendor is better, the null is true or the data are genuine. Large p-values
do not establish equivalence or no effect; wide intervals can leave important
effects unresolved. Small p-values do not establish practical importance or
causality. Interpret estimates and intervals against supplied domain thresholds;
if absent, ask a targeted question rather than invent a threshold or recommend
automatic adoption.

Use the unchanged evidence-discovery skill's Known / Indicated / Unknown /
Assumption categories inside the authored assessment and rationales. A hash
binds bytes, not truth. Distinguish authenticity and selection concerns from
model-based uncertainty. The independent team reviewer audits interpretations;
neither Polars nor recomputation guarantees valid scientific conclusions.

## Official provenance

These explain library behavior, not additional approved methods. Do not fetch
external research during Agent actions.

- Polars missing data (null versus NaN):
  https://docs.pola.rs/user-guide/expressions/missing-data/
- Polars lazy CSV scanning and schema options:
  https://docs.pola.rs/api/python/stable/reference/api/polars.scan_csv.html
- SciPy independent t-test (Welch uses unequal-variance treatment):
  https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.ttest_ind.html
- SciPy paired t-test:
  https://docs.scipy.org/doc/scipy/reference/generated/scipy.stats.ttest_rel.html
