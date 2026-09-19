# Optional semantic checks

This example turns a fixed rubric into an ordinary executable check. It can use
Ask alone, or Weigh with an explicitly selected Ask fallback. Weigh is optional;
neither the checker nor Bench silently chooses a paid backend on your behalf.
Its Weigh model path is off by default: set `BENCH_WEIGH=1` to enable it.
Unset, empty or `0` disables it; other values are errors when that path is
reached. This caller convention does not disable the standalone `weigh` command.

`check.py` is application example code, not a new verifier runtime. Copy and
adapt it with the rubric for the application that owns acceptance. Keep the
checker, rubric, thresholds and evidence directory outside the candidate's
writable workspace. Run mechanical checks first: existence, decoding, schema,
tests and exact calculations require no semantic inference.

## Ask-only path

Requires Python 3.9+, Ask, Record, a configured provider and a schema-capable
model. It works without Weigh installed or OpenRouter credentials:

```sh
python3 check.py --backend ask --model "$CHECK_MODEL" \
  --rubric rubric.json --evidence evidence.txt --candidate /work/candidate.txt \
  --records /controller/checks
```

The example rubric concerns a fictional cancellation policy. `cases.json`
contains visible, curated good/bad/near-miss candidates. These are regression
examples, not evidence of any model's accuracy or held-out calibration.

The rubric is deliberately narrow and does not certify overall policy fidelity.
For example, changing "notice must arrive" to "provide notice" can lose a source
condition, and a rule against refund promises can miss overbroad refund exclusions
or unsupported billing assurances. Independently review final outputs for these
and other untested requirements before relying on an adapted rubric.

## Weigh path

Requires Weigh and access to OpenRouter's Decisions API. Supply thresholds you
have evaluated for this exact rubric, question type and pinned model. There
are no threshold defaults. The following variables must be selected explicitly:

```sh
BENCH_WEIGH=1 python3 check.py --backend weigh --model openrouter/typesafe/jev-1.13 \
  --accept-at "$ACCEPT_AT" --reject-at "$REJECT_AT" \
  --fallback-model "$STRONG_CHECK_MODEL" \
  --rubric rubric.json --evidence evidence.txt --candidate /work/candidate.txt \
  --records /controller/checks
```

Each requirement becomes a Choice over `satisfied`, `violated` and
`insufficient_evidence`. All questions share one candidate/evidence snapshot.
Any sufficiently supported mandatory violation rejects; acceptance needs every
criterion's satisfied probability to reach the selected threshold. The checker
does not average hard requirements or multiply dependent probabilities.

An ambiguous valid result invokes Ask only when `--fallback-model` was supplied.
That model independently judges the same full rubric. With no fallback,
unestablished compliance is a rejection with evidence-needed feedback. Service
errors, missing credentials or missing executables are broken checks, never
acceptance or a reason to spend on an unselected fallback. A reported Weigh
confidence number is not used as a calibrated probability of correctness.

Turning Weigh off does not waive a configured semantic requirement. When its
model path is reached while disabled, the check exits 2 with no verdict and no
dependency calls, including the selected Ask fallback. Choose and validate a
different checker explicitly. Ask-only checks ignore `BENCH_WEIGH`; the cheap
missing-candidate rejection still works with Weigh disabled.

Both backends return JSON with a verdict, per-criterion judgments, fixed repair
messages, selected models, input hashes, elapsed time and recording paths. The
semantics are the existing Ply checker contract: **0 accepts; 1 rejects with
feedback; 2 means the check broke.** Weigh's successful-inference status alone
never decides acceptance. A missing or empty candidate rejects before any model
call or evidence-directory creation, including Ply's initial pre-check.

The caller selects the records parent; the checker may create it and creates a
private unique child per invocation. Record retains the actual request/result
streams and status. Ask judgments also retain the complete Ask session. A file
candidate, evidence or rubric changed during a check cannot pass. These bindings
do not confer filesystem isolation; select that boundary separately.

## Put it in an existing check

Use an operator-owned wrapper as the existing `ply -check` executable or Agent's
`bin/check`. For example, a wrapper checks an output file and then invokes the
example using explicit paths and selected configuration:

```sh
#!/bin/sh
test -s candidate.txt || { echo 'Write candidate.txt first.'; exit 1; }
exec python3 /operator/semantic-check/check.py \
  --backend ask --model "$CHECK_MODEL" \
  --rubric /operator/semantic-check/rubric.json \
  --evidence /operator/semantic-check/evidence.txt \
  --candidate candidate.txt --records /controller/checks
```

The wrapper can select the Weigh path above instead. `check.py` also accepts
candidate bytes on stdin when `--candidate` is omitted. Do not pipe a failed
Weigh invocation through a shell expression that turns infrastructure failure
into ordinary rejection or success.

## Measure judgment and finished-output quality separately

1. Freeze the rubric, source evidence, selected model IDs and question form.
   Create independently labeled held-out cases: acceptable paraphrases, omitted
   requirements, plausible wrong answers, contradictions, injected instructions,
   and missing evidence. Separate calibration cases from evaluation cases.
2. Compare a small Ask model, the current stronger checker, Weigh alone, and
   Weigh with explicit escalation. Use the same candidates and labels. A larger
   model's agreement is a comparison, not ground truth. Select thresholds by
   false-pass/false-rejection tradeoffs, then freeze them before held-out tests.
3. Run actual paired Ply/Agent jobs with the same generator, starting artifact,
   task, limits and deterministic gates. Change only the selected checker. Keep
   every attempted candidate and repair message. Independently review the final
   outputs with an evaluator that is not told which checker produced them.
4. Compare final-output correctness and visual quality, false acceptance,
   unnecessary repair, repair turns, exhausted budgets, total latency and cost.
   Better scores or fewer model calls alone do not establish better outputs.
   Repeated attempts may exploit a probabilistic checker; include those cases.

`evaluate.py` implements step 2. It requires `--live`, even for a compatibility
fixture endpoint, and creates a new external results directory. Select profiles
in a JSON document like this, replacing the model and threshold placeholders:

```json
{
  "version": 1,
  "profiles": [
    {"name": "small", "backend": "ask", "model": "PROVIDER/SMALL_MODEL"},
    {"name": "strong", "backend": "ask", "model": "PROVIDER/STRONG_MODEL"}
  ]
}
```

A Weigh profile uses `backend: "weigh"`, `model: "openrouter/typesafe/jev-1.13"`,
numeric `accept_at` and `reject_at` fields, and optionally `fallback_model`.
Set `BENCH_WEIGH=1` in the evaluator's environment when selecting such profiles;
it is inherited by each checker. Ask-only evaluations need no opt-in.

```sh
python3 evaluate.py --live --profiles /operator/profiles.json \
  --cases /operator/held-out-cases.json --rubric rubric.json \
  --evidence evidence.txt --output /results/new-comparison
```

The supplied `cases.json` demonstrates the case format. The evaluator retains
all checker output, records and model sessions, and reports false passes,
false rejections, infrastructure errors, escalations and p50/p95 elapsed time.
Error rates are conditional on completed judgments, with attempted/completed
counts for each label; an unavailable rate is `null`, not zero. Cost is summed
only where the providers reported it; missing cost remains unknown. The `--live` flag does not supply credentials or validate their access.
This fixed-candidate evaluator does not simulate repairs or claim loop gains.
Use the existing Ply loop for step 3, rather than writing a second agent loop.

## Visual and other non-text outputs

Use the [visual observation recipe](../visual-check/README.md) for actual images.
It retains the observed images and rejects changed images or incomplete visual
observations before calling this checker. Jev currently judges text only; its
probabilities cannot recover defects omitted by the vision observer. Compare the
whole vision-plus-Weigh path with direct vision judgment, including observer
cost and missed defects. For audio/video, select a capable transcription or
perception tool and preserve timing/coverage; textual summaries cannot certify
unobserved motion, timing, sound quality or visual detail.

## Offline checks

```sh
python3 -m unittest discover -s . -p 'test_*.py' -v
```

These test process/policy behavior with explicit fixtures. The repository's
`scripts/check-weigh.py` separately tests real Weigh, Ask, Record and Ply process
composition against local wire fixtures. Neither kind of check demonstrates
live Jev accuracy or an improvement in finished outputs.
