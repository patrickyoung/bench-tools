# Deterministic calibration and heldout evaluation

`evaluate.py` compares an explicit proposed checker setting with a frozen
baseline. It can rescore recorded criterion results at different thresholds or
with a criterion/check removed. It reports calibration results for those
counterfactual variants, then evaluates only the explicitly selected baseline
and proposal on heldout cases. It never chooses a setting from evaluation data,
calls a model, runs a checker, edits policy, or retains a lesson.

The filter uses only Python's standard library. Keep requests, labels, source
inventories, run records and result files in an explicitly selected external run
directory. The surrounding [workflow](README.md) composes Record, optional Weigh
diagnosis, Hire/Ply, this evaluation and Hone through public commands.

```sh
python3 evaluate.py --input /selected/run/evaluation.json \
  > /selected/run/evaluation-result.json

python3 evaluate.py --input /selected/run/evaluation.json --gate \
  > /selected/run/gate-result.json
```

`--input -` reads the one request from stdin. Stdout is one JSON report;
diagnostics go to stderr. Without `--gate`, any valid report exits 0, including
one that blocks a proposal. With `--gate`, `supported` exits 0 and every other
valid decision exits 1. Invalid invocation, invalid input or unreadable input
exits 2 without a result. Output write failure exits 1 and may leave partial
bytes; check both status and the result. Gate exit 0 is evidence for the caller's
review and subsequent authorized workflow, not an automatic policy edit.

## Version-1 request

Every object rejects unsupported fields. Required top-level fields are
`version`, `criteria`, `settings`, `baseline`, `proposal`, `proposal_basis`,
`constraints` and `cases`. Optional fields are `sweeps`, `ablations` and
`repair_trials`.

| Field | Contract |
| --- | --- |
| `version` | Integer `1`. |
| `criteria` | Nonempty list of `{ "id": "content", "check": "semantic" }`. Criterion IDs are unique. A check can contain multiple criteria. |
| `settings` | Explicit frozen settings in the format below. |
| `baseline`, `proposal` | IDs of explicit settings. Generated sweep/ablation IDs cannot be selected here. |
| `proposal_basis` | `input` or `calibration`: the caller attests that the proposal was frozen before reading evaluation outcomes. `evaluation` is invalid. |
| `constraints` | Frozen coverage and acceptance limits in the format below. |
| `cases` | Explicit independently labeled fixed candidates, with recorded scores for one or more profiles. |
| `sweeps` | Optional list such as `[{"criterion":"content","values":[1.5,2,2.5]}]`. Each value creates a separate change to the baseline; there is no Cartesian product. |
| `ablations` | Optional list such as `[{"criterion":"clarity"},{"check":"style"}]`. Each independently removes the selected criterion or check from the baseline. |
| `repair_trials` | Optional independent reviews of actual outputs from paired fresh repair runs; described below. |

A setting has exactly these fields:

```json
{
  "id": "proposed",
  "profile": "revised-checker-observations",
  "source_sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "thresholds": {"content": 2, "format": 1},
  "drop_checks": [],
  "drop_criteria": []
}
```

`source_sha256` identifies the caller's frozen implementation bundle or source
inventory, including the worker skill and checks relevant to the comparison.
Pin the actual selected files outside this request; a name such as `latest` is
not a source pin. `profile` selects observations from each case. A changed skill
or checker that changes observations needs a separately collected profile.
Different thresholds on the same recorded scores can share a profile.

`thresholds` must name every declared criterion, including dropped ones.
Scores and thresholds retain their caller-selected native numeric scale; the
filter does not normalize them or reinterpret provider confidence as
correctness. A known criterion passes when `score >= threshold`. All active
criteria must pass. Convert any lower-is-better metric explicitly before
creating the request and document that transformation with the selected
evidence. No generated setting changes source code.

`drop_checks` and `drop_criteria` are unique declared IDs. Dropping a check
removes all its criteria. An empty active set cannot pass. A required check
retains at least one active criterion; list each essential criterion separately
under `required_criteria` to preserve all of it.

Constraints have these required fields:

```json
{
  "required_criteria": ["content", "format"],
  "required_checks": ["semantic", "structural"],
  "min_evaluation_cases": 10,
  "max_false_pass_rate": 0,
  "max_false_rejection_rate": 0.1,
  "require_final_review": true
}
```

Optional `min_evaluation_accept` and `min_evaluation_reject` specify the minimum
completed independently labeled cases in each class; both default to 1 and
must be positive. Optional `min_thresholds`, such as `{"content":2}`, freezes a
threshold floor and also requires those criteria to remain active. Rate limits
are in `[0,1]`. `min_evaluation_cases` must be positive and applies both to
paired fixed-candidate cases and, when required or supplied, paired repair
trials. Select limits and coverage before looking at heldout results.

## Fixed candidates and independent labels

Every case has exactly these fields:

```json
{
  "id": "heldout-01",
  "split": "evaluation",
  "candidate_sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "evidence_sha256": null,
  "expected": "accept",
  "label_basis": "Independent reviewer checked every requirement against the supplied evidence.",
  "label_independent": true,
  "profiles": {
    "revised-checker-observations": {
      "criteria": {
        "content": {"status":"known","score":2.4},
        "format": {"status":"known","score":1}
      },
      "cost": 0.001,
      "seconds": 0.4,
      "tokens": 150,
      "turns": 0
    }
  }
}
```

`split` is `input`, `calibration` or `evaluation`. Input cases are counted but
never used in calibration or evaluation summaries. Generate proposals using
input cases and calibrate on calibration cases. Evaluation cases must remain
held out until the proposal and constraints are frozen. Related task families,
candidate revisions and repeated observations should stay in the same split;
the filter cannot detect semantic near-duplicates. It rejects duplicate case
IDs and repeated candidate/evidence hash pairs within or across splits.

Both profiles judge the same hashed candidate with the same selected evidence.
`evidence_sha256` is a digest or `null` for no additional evidence. Hashes are
lowercase hexadecimal SHA256 strings; the repeated letters above are
illustrative placeholders. The filter reads no candidate or source files and
does not authenticate caller-provided hashes.

`expected` is `accept`, `reject` or `unknown`. `label_basis` identifies the
independent reference used to make the label. Labels must come from a suitable
independent reviewer or reference evidence. Agreement between checkers,
agreement with the proposal, model confidence and success reported by the
worker are not ground truth. `label_independent` is a required boolean caller
attestation. False attestations and unknown labels remain visible and are
excluded from accuracy denominators; they cannot supply required evidence.

Each profile result requires `criteria` and allows optional `cost`, `seconds`,
`tokens` and `turns`. Each recorded criterion is one of:

```json
{"status":"known","score":2.4}
{"status":"missing"}
{"status":"unknown"}
{"status":"error"}
```

Only `known` may have a score, and it must have one. Missing profiles or omitted
criteria are `missing`, never zero scores. When multiple states occur, the
whole checker outcome uses the precedence `error`, `missing`, `unknown`, then
the all-known pass/reject decision. Thus a rejected criterion cannot hide a
broken sibling check. Metrics keep the five states separate.

False-pass rate is accepted independently labeled rejects divided by completed
independently labeled rejects. False-rejection rate is rejected independently
labeled accepts divided by completed independently labeled accepts. Reports
include attempted and completed denominators and unknown/non-independent
label counts. Empty denominators produce `null`, not zero. A proposal cannot
be supported while any independently labeled heldout checker pair is
incomplete, or while either label class lacks required coverage.

## Actual final-output evaluation

Better judgments on fixed candidates do not establish better completed work.
When repair behavior is relevant, run the baseline and the frozen proposal on
each selected fresh heldout task through the existing worker/team entry
command. Record both executions. Independently assess the actual resulting
artifacts against the original task requirements, including any visual or
factual evidence needed for a real quality judgment.

Append a repair trial for each task:

```json
{
  "id": "repair-01",
  "split": "evaluation",
  "task_sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
  "settings": {
    "proposed": {
      "setting_sha256": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
      "artifact_sha256": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
      "checker_accepted": true,
      "final_review": {
        "quality": "accept",
        "basis": "Blind independent review of the final artifact against the task requirements.",
        "independent": true
      },
      "cost": 0.02,
      "seconds": 8,
      "tokens": 1200,
      "turns": 2
    }
  }
}
```

Include both baseline and proposal entries to provide a completed pair. Trial
IDs and task hashes must be unique. Only the explicit baseline/proposal IDs
are allowed. Review `quality` is `accept`, `reject` or `unknown`;
`checker_accepted` records the checker outcome separately. It never supplies
the independent quality judgment. An absent setting, an unknown quality, or a
review with `independent:false` leaves the pair incomplete. This version does
not describe partial failed runs as reviewed final artifacts; omit that
setting result and retain its run failure in the external Record evidence.

`setting_sha256` is required. Compute it from the exact declared criteria and
setting before running that setting:

```python
import hashlib
import json

selected = {"criteria": request["criteria"], "setting": selected_setting}
encoded = json.dumps(selected, sort_keys=True, separators=(",", ":"),
                     ensure_ascii=False, allow_nan=False).encode("utf-8")
setting_sha256 = hashlib.sha256(encoded).hexdigest()
```

The report's `hashes.settings` also supplies these digests. A preliminary
report without required repair trials will correctly be `inconclusive`.
Changing a threshold, dropped criterion, profile or source pin invalidates
older repair evidence through the setting digest. Never relabel an old run
with the new digest: run the changed setting again. The digest is an integrity
binding, not proof that the claimed executable ran.

If `require_final_review` is true, absent or insufficient repair trials prevent
support. Supplied repair trials always participate in the gate even when this
constraint is false. Any independently reviewed final-output regression, or
an increase in checker-accepted bad final outputs, blocks the proposal.

## Reading the result

The report includes the raw request hash, canonical constraints and evaluation
input hashes, every setting's digest, split counts, calibration summaries,
paired heldout decisions and paired final-output reviews. It hashes the
selected evidence rather than silently discovering more input. Save the full
request and stdout result with the Record evidence for audit and replay.

Calibration contains all explicit settings plus generated `sweep-0001` and
`ablate-0001` variants, with coverage violations shown. It does not rank or
select a winner, and no generated variant receives a heldout evaluation
summary. To select a calibration variant, copy its configuration into a new
explicit setting ID, freeze its source and constraints, and then collect fresh
heldout evaluation. Repeatedly choosing between proposals after observing the
same evaluation set turns that set into calibration data; reserve a new
evaluation split.

| Decision | Meaning |
| --- | --- |
| `supported` | Required independent paired evidence is complete, frozen coverage and rate limits hold, false-pass/false-rejection rates and required final-output quality do not regress, and there is a measured improvement. |
| `blocked` | A coverage, threshold, rate, incomplete-outcome or final-output regression violates the gate. |
| `inconclusive` | Required paired evidence, known independent labels or actual final reviews are insufficient. |
| `no_improvement` | The evidence is sufficient and constraints hold, but there is no measured improvement. |

Measured improvement can mean fewer fixed-candidate errors, better independent
final-output reviews, or lower fully measured cost, latency, tokens or worker
turns without the above quality regressions. Equal adequate quality with less
work is an improvement; it does not need to score higher on quality. The
independent quality floor, required coverage and error limits stay unchanged.

Efficiency improvements are named `lower_observed_cost_total`,
`lower_observed_seconds_total`, `lower_observed_tokens_total` and
`lower_observed_turns_total`. Token/turn improvements additionally require
complete paired checker outcomes, or complete independent final-review pairs
when repair trials are required or supplied. Comparisons use actual repair
measurements when those trials apply; otherwise their scope is fixed-candidate
judgments. All baseline and proposal usage remains visible: fewer tokens can
come with more time, for example. The gate does not require every efficiency
metric to improve. Review such tradeoffs explicitly. A `blocked` report can
still list an efficiency metric that improved; the full quality gate controls
the decision. None of these metrics alone claims higher output quality.

## Optional measured effort

Profile and repair-setting results accept optional `cost`, `seconds`, `tokens`
and `turns`. Cost and time are nonnegative measured numbers. Tokens and turns
are nonnegative JSON integer counts at most `1e12`; fractional counts, booleans
and strings are invalid. Missing fields or explicit `null` mean unavailable,
never zero. Measurement is optional; collect the metrics relevant to the
proposed benefit rather than inventing data for every workflow.

Use the same counting scope on both paired sides and retain its definition with
the run evidence. `tokens` counts all provider input and output tokens in the
selected profile/run, including generator, judge and fallback calls. Use each
provider's reported accounting; do not double-count cached/reasoning subtotals
already included in its input/output totals. If a required stage's usage is
unavailable, the total is `null`. Different models can tokenize differently;
these counts measure reported usage, not equal units of semantic work.

`turns` counts worker/generator assistant submissions, including attempts and
repairs. It excludes startup, pre-checks and verifier-only calls. A fixed-case
judge-only profile can have zero worker turns; a successful pre-check-only run
can also have zero turns when its actual final artifact is independently
reviewed. A failure to run is not evidence of a zero-effort successful job.
Retain judge/fallback calls in token, cost and latency accounting even when they
are not worker turns. Do not mix command count with this turn definition.

Reports add `tokens_total`, `tokens_known_subtotal`, `tokens_unknown`,
`turns_total`, `turns_known_subtotal` and `turns_unknown` to each usage summary.
Unknown counts are numbers of selected observations missing that measurement.
The known subtotal may be zero while the complete total remains `null`; a total
is emitted only for a nonempty set whose measurements are all known. Existing
cost/time summaries preserve the same distinction. Median and nearest-rank p95
latencies explicitly describe only known samples.

Ablations and threshold sweeps do not fabricate cost, latency, token or turn
savings for an unexecuted configuration: their calibration measurements refer
to the reused recorded profile. Rescoring cannot establish fewer repair loops.
Run the proposed setting on fresh matched jobs to measure actual savings.

Guard against adaptive overfitting even in small experiments. Changing only
names, IDs or dates does not establish generalization; use genuinely different
tasks, templates or workbooks, including cases where a proposed lesson should
not apply. A case reused after its feedback guides a change is development
data. Checks, cutoffs, routing and cost/effort objectives can all be overfit.
Freeze the independent quality target and selected efficiency metrics before
comparison, and keep unrelated quality regressions visible despite savings.
The current pilot consists of targeted family probes, not broad transfer proof.
Require repeatability and appropriate representative coverage before expanding
that claim; this is not a mandate for costly benchmarks or a universal
statistical test on every routine change.

These rules enforce observed no-regression checks, not statistical confidence
or universal correctness. A tiny paired sample is useful executable evidence
but cannot establish generalization. Expand independent evaluation according
to the task's cost and failure risks before promoting generalized source or
retaining a lesson through Hone.

## Bounds and offline checks

The request is bounded to 8 MiB, 64 JSON nesting levels, 10,000 fixed cases,
10,000 repair trials, 128 criteria, 128 total explicit/generated settings and
1,000,000 case-setting combinations. IDs are nonempty UTF-8 of at most 256
bytes without control characters. Label/review bases are nonempty UTF-8 of at
most 8,192 bytes. All numbers must be finite and have absolute value at most
`1e12`. Duplicate JSON keys, unsupported fields, malformed Unicode, booleans
used as numbers and non-finite numbers are rejected before a result is written.

```sh
python3 -m unittest discover -s tools/weigh/examples/improve-checks \
  -p test_evaluate.py -v
```

The synthetic tests include a threshold that improves fixed judgments but
worsens actual final outputs, fresh paired improvement, stale setting-digest
rejection, missing/unknown/error outcomes, attempted/completed denominators,
independent-label exclusion, coverage/floor enforcement, calibration isolation,
incomplete costs and effort counts, same-quality efficiency improvements,
visible metric tradeoffs, zero-turn reviewed runs, invalid counters, duplicates,
bounded inputs and executable gate statuses.
They prove the filter's offline contract, not live model quality.
