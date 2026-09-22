# Accept improvements for a specific use case

This caller-owned judge compares qualified outcomes and total provider cost.
It can accept higher quality at higher cost, or adequate quality at lower cost.
It has **no default quality floor, price premium, savings target or deployment
budget**. Supply a policy for the intended use case before admitting a run.
The Improve protocol and the original router judge are unchanged.

The judge is deterministic arithmetic over independently supplied observations.
It does not score meaning, infer business value, run models or deploy source.
Python 3.9+ and its standard library suffice.

## Establish the decision before collecting results

The application that creates experiments defines this policy from the user's
use case. The user supplies the intended outcome and decision context; the app
proposes the measurable criteria, explains their basis, and resolves any material
missing facts. It should not hand the user an unexplained table of numeric knobs.
Keep the creator's policy and scorer independent of the candidate being tested.

The creator can start with [policy.template.json](policy.template.json) outside reusable source.
The unresolved template deliberately fails validation. Define:

- **Use case and scope:** who uses the result, what decision it supports, and
  whether an independent person reviews it before acting.
- **Qualified outcome and critical failure:** an independent rubric, including
  completion, factual correctness and any required abstention. More `ready`
  reports are not inherently better research.
- **Policy basis:** why the quality floor, tolerated regressions, minimum useful
  gain and spending limits fit that deployment. Derive these from service
  requirements, error consequences, avoided work and the operating budget.
  Configurability alone does not justify a threshold.
- **Evaluation coverage:** the actual case IDs, categories and repeats selected
  before the run. State the intended claim and evidence needed for it. This
  judge implements a finite operational gate, not a statistical significance or
  universal generalization test.

For advisory research with human review, the relevant benefit may be less review
time or more useful findings within a per-investigation budget. For an automated
decision, error consequences may demand a much stricter quality floor and
critical-failure definition. Neither use case inherits numbers from another.
If the decision context cannot justify a rule, record `needs_policy` and resolve
the missing context before buying more evaluation results.

All policy fields are explicit:

| Field | Meaning |
| --- | --- |
| `quality_floor` | Minimum candidate qualified fraction, from 0 to 1. |
| `max_category_regression_pp` | Maximum decrease within each selected category, in percentage points. |
| `efficiency.min_savings_percent` | Positive minimum reduction in aggregate provider cost. |
| `efficiency.max_quality_loss_pp` | Maximum overall quality loss allowed for that saving; set 0 to preserve quality. Category limits and the floor still apply. |
| `quality.min_gain_pp` | Positive minimum overall qualification gain, in percentage points. |
| `quality.max_cost_increase_percent` | Maximum aggregate cost premium allowed for that gain. |
| `quality.max_cost_per_added_qualified_usd` | Optional maximum extra dollars per additional qualified job; null explicitly omits this limit. |
| `limits.max_mean_cost_usd` | Optional absolute mean provider-cost ceiling for candidate jobs. |
| `limits.max_job_seconds` | Optional elapsed-time ceiling for every candidate job. |
| `coverage.SPLIT` | `{ "cases": {"case-id": "category"}, "repeats": 2 }`; select the actual grid for each split. |

Set either benefit object (`efficiency` or `quality`) to null to disable it;
at least one must be selected. Both selected paths are frozen in advance, not
chosen after looking at holdout. A quality gain may satisfy both. Hard limits
apply regardless of which benefit is met. A candidate critical failure always
rejects; a qualified score cannot simultaneously claim a critical failure.
Lower quality for lower cost requires explicit overall and category allowances
and still must meet the deployment's quality floor.

`cost_per_added_qualified_usd` is
`max(0, candidate_total_cost - baseline_total_cost) / additional_qualified_jobs`.
It is undefined when there is no quality gain. Failed and repaired jobs remain
in both the denominator and cost totals. Unknown provider cost is not zero.
When baseline cost is observed zero, a percentage budget permits no increase;
there is no invented percentage for division by zero.

Choose how to account for human review and downstream impact in the use-case
contract; this adapter measures provider dollars only. Do not call its cost
field total business cost. Authoring and evaluator costs remain in Improve's
separate study ledger; the paired comparison excludes one-time authoring.

## Select it through the existing Improve protocol

Validate the policy **before** execution:

```sh
python3 /adapters/acceptance/judge.py --check-policy < /inputs/use-case-policy.json
```

The creating application can bind its proposed policy to its draft specification
with this deterministic adapter (no model calls or writes):

```sh
python3 /adapters/acceptance/spec.py \
  --experiment /inputs/draft-experiment.json --policy /inputs/use-case-policy.json \
  > /inputs/experiment.json
```

It requires an explicit policy, validates complete coverage against both case
splits and repeats, inserts `settings.acceptance`, selects the judge and pins
the policy/judge dependencies. It preserves the selected source, proposer,
trial/scorer, model, cases and execution limits. Unresolved context or mismatched
coverage fails before any experiment command is executed. Select this adapter
only when the independent trial produces its score contract below.

Alternatively assemble the same fields directly: copy the exact policy object
into `settings.acceptance`, select the judge as literal argv and pin the judge,
policy, scoring adapter and rubric in `dependencies`. The judge command is:

```json
{"argv":["python3","/adapters/acceptance/judge.py","--report","assessment.json"]}
```

Improve's generic `-n` does not understand application settings; it cannot
replace this policy preflight. A caller generating specifications must validate
the selected policy before allowing model calls. Then use the ordinary
`improve -n`, `improve -o NEW_STUDY` and `improve verify NEW_STUDY` commands.

The independent trial adapter returns the existing envelope with this score:

```json
{
  "version": 1,
  "score": {
    "qualified": true,
    "critical_failure": false,
    "elapsed_seconds": 12.5
  },
  "cost": 0.01
}
```

The sample numbers above only illustrate the observation format. Additional
score fields may retain review evidence. `qualified` must reflect the frozen
rubric, actual completion and independent review; a worker's self-report is not
an independent label. Time may be null if unmeasured; it prevents a decision
when the selected policy requires a time limit. Cost may be null but always
prevents this cost-based judge from accepting or rejecting on efficacy.

Every expected case/repeat must appear exactly once per arm. The judge checks
stable per-arm source digests and identical case inputs across arms/repeats.
Full coverage prevents silently dropping failures. It does not independently
replay nested evidence or establish factual truth: the trusted scoring adapter
and caller must do that through the owning public tools.

The detailed assessment records use-case rationale, canonical JSON policy/comparison digests,
source digests, category counts, measured trade-offs, satisfied paths and reasons.
`--report` creates a new file and refuses overwrite. Inside Improve, that file
is retained in the judge's own directory and covered by the study inventory.

| Assessment | Judge stdout / exit | Native Improve consequence |
| --- | --- | --- |
| `accepted` | `keep` / 0 | Continue after development; after holdout, export `supported` bytes. |
| `rejected` | `discard` / 1 | `keep_baseline`; no proposal export. |
| `insufficient_evidence` | Diagnostic / 2 | `invalid_evidence`; no efficacy verdict or export. |
| Malformed policy, mismatched inputs or missing grid rows | Diagnostic / 2 | `invalid_evidence`; fix the measurement contract. |

An incomplete engine run can separately produce `inconclusive`. Do not treat
invalid/insufficient measurement as evidence that the candidate did not help.
Do not retry until success or reclassify historical studies under a new policy.

## Run the offline process examples

The explicitly named [fixture policy](fixture-policy.json) contains arbitrary
test boundaries. It is **not a recommended policy for the researcher or any
production use case**. Scores and costs are invented; zero model calls occur.
Put built `improve` and `record` on PATH, then from this directory:

```sh
python3 spec.py --scenario quality > /tmp/quality-fixture.json
improve -n < /tmp/quality-fixture.json
improve -o /tmp/new-quality-fixture < /tmp/quality-fixture.json
improve verify /tmp/new-quality-fixture
```

Use a new destination for every run. `quality` accepts better quality at higher
cost; `efficiency` accepts equal quality with savings; `over-budget` rejects at
development; `critical-holdout` passes development and rejects at holdout;
`unknown-cost` retains invalid evidence without an export. These demonstrate
protocol decisions, not real improvement.

```sh
python3 -m unittest discover -s . -p 'test_*.py' -v
```

## Acceptance to deployment

The accepted object is the exact source/configuration and tested use case,
not a model name or an unversioned worker. A supported proposal supplies source
and evidence for the caller's existing promotion process:

1. Confirm the holdout decision and `improve verify`, review the exact diff,
   factual labels and assessment, and record the supported scope. Match the
   current baseline and proposed bytes/modes to the manifest. Source drift
   makes the proposal stale; it is not permission to force-apply it.
2. Apply the reviewed files through the owning source workflow. Run its own
   checks and public integration checks. Commit the exact candidate; extra
   behavior changes require evaluation rather than inheriting acceptance.
3. Export the committed revision and verify that the deployed definition's
   files/modes match the accepted candidate. Keep run records out of source.
4. Select that export for new jobs in the named consuming workflow. Record the
   old and new full revisions, model/provider/effort, invocation settings,
   deployment target and rollback pin. An export alone is not deployment.
5. Exercise a representative job through that consumer. Roll back to the prior
   pin if its use-case acceptance requirements fail. Keep existing jobs pinned;
   an acceptance decision does not upgrade them implicitly.

For the Bench library, use its existing source PR, worker checks and committed
export process; set lifecycle status only for the supported scope. The owning
host decides when to run promotion commands under the user's authorization.
Improve and this judge never apply, merge, deploy or schedule monitoring.

Improve v0.1 changes only existing text files. A candidate adding a helper must
use the owning component's reviewed source-change and package-evaluation path;
do not pretend it was exported by a text-only Improve proposal. A caller may
use this judge on correctly bound matched observations outside Improve, but
the assessment alone does not verify or export that package.
