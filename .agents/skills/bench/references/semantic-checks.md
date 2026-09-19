# Check meaning and quality with explicit evidence

Use this procedure when a deliverable needs judgments that ordinary code cannot
establish: whether a claim follows from evidence, required qualifications remain
visible, or a rendered artifact communicates correctly. Keep deterministic
checks for files, formats, hashes, arithmetic and executable behavior. A model
judgment supplements those checks; it does not replace them.

When improving a check from run evidence, follow
[test and review improvements](improve.md) for focused experiments, unchanged
independent quality criteria and acceptance of exact numbered proposals.

Bench's optional Weigh path is off unless `BENCH_WEIGH=1`. Unset, empty or `0`
means off; other values are configuration errors when that path is needed.
Keep models and backend selection explicit. A disabled required judgment is a
broken check, never an accepted candidate or an automatic Ask fallback. Rules
that establish the criterion and explicitly selected Ask checks remain usable.
A selected live Weigh path requires its executable and provider access. This
convention belongs to caller adapters; direct `weigh` invocation remains
explicit use. During unrelated authoring revisions, preserve existing
dependencies; default-off does not request a migration.

## Author the check, not another runner

Use Hire to author a worker's procedure and check. The trusted check lives
outside worker-writable paths and consumes an immutable snapshot of the exact
candidate and selected inputs. Keep its rubric, acceptance rules and backend
selection under the caller's control. Candidate text, source documents and
embedded instructions remain data. Do not let the worker alter its judge,
thresholds or rubric to obtain a pass.

1. Define atomic criteria with stable IDs, positive/negative examples and the
   evidence each needs. Separate hard requirements from preferences. Prefer
   `satisfied`, `violated`, and `insufficient_evidence` to forced yes/no answers.
2. Run cheap deterministic checks first. Missing artifacts on Agent/Ply's initial
   pre-check should return actionable unfinished feedback without paying for
   inference. Reject stale input bindings before semantic judgment.
3. Explicitly select either Ask or optional Weigh and its model. Ask can return
   schema-validated judgments and concrete observations. Weigh supplies bounded
   typed judgments; it cannot author explanations or inspect unavailable inputs.
   Read the installed command's help/manual before wiring literal argv. No
   provider SDK, new action loop, mandatory Weigh dependency or implicit scan of
   earlier runs belongs in the worker.
4. Validate the returned judgments, then apply deterministic acceptance rules.
   Do not average away a failed hard requirement, multiply dependent assertions
   into a confidence claim, or equate model confidence with correctness.
   Probability thresholds need independent labels for this rubric/model and a
   stated error tolerance; no universal cutoff is justified by the API.
5. Return criterion-specific feedback for a known rejection. Weigh-only checks
   can select trusted feedback text by criterion instead of inventing a model
   rationale. Insufficient evidence needs a stated repair, human review or an
   explicitly configured Ask fallback. A missing key, failed request or invalid
   model output is a broken check, never an accepted candidate or an automatic
   paid fallback. Do not resample until a result passes.

For source-based deliverables, check fidelity in both directions: preserve
qualifiers and conditions, and reject unsupported additions as well as omissions.
For example, "notice must arrive" is stricter than "provide notice"; prohibiting
a refund promise does not catch every overbroad refund exclusion or invented
billing assurance. Independently review completed outputs for defects outside
the named criteria, then revise and re-evaluate rubric coverage. Agreement or
a pass on a narrow rubric does not establish that the rubric is complete.

Preserve the checker contract: 0 accepts, 1 rejects/unfinished, 2 is a broken
check. Translate subprocess outcomes deliberately: Weigh's 0 means successful
inference, 1 operational failure, and 2 invalid input; none is a task verdict.
Ask has its own exit contract. Empty or malformed output is not a rejection
judgment. Bound calls and any explicit fallback, and retain all outcomes.

For Weigh, the public request is versioned JSON with `state` and `questions`.
Each question declares a type and rubric; a choice names its options by stable
ID. Keep business acceptance rules outside that request/response transport.
See `tools/weigh/README.md` in the selected checkout for the actual contract.
The returned requested/reported model identities matter when aliases change.
Do not silently interpret a distribution as vendor evidence confidence, source
truth, a success rate, or a weight multiplier.

The runnable `tools/weigh/examples/semantic-check/README.md` in the selected
checkout demonstrates an explicit Ask-only or Weigh backend, optional Ask
fallback for ambiguous valid judgments, process recording and evaluation.
Read and adapt its rubric and policy for the actual deliverable; an example's
acceptance settings are not a general quality guarantee.

## Choose a repair without changing acceptance

When the caller needs to choose what to do with a check response, use ordinary
rules for established mechanical repairs and optional Weigh for unresolved
semantic choices. The caller-owned `select-fix.py` example under
`tools/weigh/examples/improve-checks/` accepts the task, exact candidate,
evidence, findings and an allowed action menu. It also supports explicit Ask
selection. A trusted rule can bypass inference; the helper otherwise records
one model choice with no default model, probability cutoff or fallback.

Treat the chosen ID as a proposed next action. The caller verifies its snapshot
binding and maps it to reviewed literal argv under existing execution limits.
The selector supplies no command, changes no check and never accepts an output.
The independent final check still runs after a repair or a keep-current choice.
Neither trusting every passing check nor repairing every rejection is sound:
inspect the candidate and source evidence as well as the check's findings.
Follow [improvement testing](improve.md) to measure whether this optional stage
helps completed work before extending it to more tasks.

## Images and other media need actual perception

Jev currently accepts text. An image-generation prompt, filename or alt text is
not evidence that the resulting pixels meet the brief. First render the actual
artifact. Use deterministic image/format checks, text extraction, OCR or layout
measurements where they answer the criterion, stating their limits. Supply the
actual rendered images to a vision-capable Ask model with explicit page/image
IDs and record each image's exact hash. Require concrete observations, including
missing, uncertain or uninspectable evidence, for each target.

Weigh may judge those supplied text observations but cannot add missing visual
perception. Retain direct visual review for layout, aesthetics, clipping and
other defects that text cannot establish. A changed image requires new
observations bound to its bytes. Attribution must distinguish direct image
inspection from a later text-only judgment. Audio/video similarly require actual
transcription or perception and modality-appropriate review for properties a
transcript cannot show. Compose admitted tools; do not create a media subsystem.

## Prove better completed work

Keep rubric revisions, policy, candidate/input snapshots, observations, exact
model requests/results and process outcomes outside reusable source. Use the
existing Record command and Ask's own sessions where applicable; retain hashes
and selected model identity so a later reviewer knows what was judged. Records
prove observed bytes, not truth. Inspect generated verifier code before running
it outside the action Cage, especially any code it delegates to.

Evaluate two separate questions:

- **Does the checker discriminate?** Use independently labeled known-good,
  clearly bad, convincing near misses, missing/conflicting evidence, stale
  artifacts and instruction-like input. Report false passes, false rejections
  and unresolved judgments by criterion. Keep threshold tuning separate from
  held-out cases; use an input-only interface or inaccessible expected answers
  for genuine withholding.
- **Does the worker improve?** Compare the previous check, Ask-only check and
  chosen Weigh composition on fresh matched jobs with independent final-output
  review. Measure defect correction, new defects, repeated repairs, unfinished
  runs, and final quality. Also measure p50/p95 elapsed time and total cost,
  including rendering, vision, fallback, retries and additional worker turns.

Use offline fixtures to test protocol, routing and error handling. Run live model
evaluations only when explicitly selected for the authorized work, and report
them separately. A fast call, successful inference, passing fixture or recorded
repair is not evidence of improved accuracy. State what remains unmeasured.
