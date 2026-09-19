---
name: improving-checks
description: Propose and test focused skill or checker improvements when a run reveals false acceptance, unnecessary rejection, poor feedback or a possible threshold problem; preserve independent quality criteria and exact reviewed changes.
---

# Turn run evidence into tested improvement suggestions

Use the selected run, expert and authoring workspace. Ask for a target only if
it cannot be resolved from the request. Hire authors expertise and checks;
existing Agent/Ply commands run the expert. Keep the original definition and
run intact, make proposed edits in a separate authoring copy, and keep test
cases, labels, raw records and proposals outside the reusable definition.

For a conversational request to improve or suggest improvements, inspect and
perform authorized bounded tests, then present numbered changes for acceptance.
An analysis-only request never admits a change. “Apply 1” selects the exact
previously reviewed proposal. This convention does not add a new permission
step to a specific implementation change the user has already authorized.
Use the existing setup and tools without asking needless backend questions.

## Fix the right thing and preserve the independent test

Inspect the candidate, source evidence, checker output and independent final
review. Separate output defects from missing evidence, criterion applicability,
rubric coverage, feedback, calibration and execution failures. Low confidence
alone does not prove a threshold problem; a failed call supplies no judgment.
Optional Weigh/Ask triage proposes a hypothesis, never a causal finding or an
authorization. Keep an Ask-only workflow usable without Weigh.

For choosing the next fix, use deterministic rules where inspected facts settle
the action. Optional `select-fix.py` in the Bench example named below records a
Weigh or Ask choice from an explicit caller-allowed menu using task, candidate,
evidence and check findings. A trusted rule can bypass inference. Its selected
ID carries no command or acceptance authority: bind it to the exact snapshot,
dispatch reviewed literal argv through the existing runner within its remaining
limits, and run the unchanged independent final check even for no change. Select
the repair model separately. No default model, automatic fallback, threshold
adjustment or checker edit belongs in this route. A check pass alone is not a
sufficient rule for keeping an artifact. Compare whole-path quality, time,
tokens and cost against rules before claiming the selector improves a workflow.

Before testing, freeze required outcomes, criterion/check coverage, independent
label rules, acceptable error rates and final-review requirements. Preserve this
independent evaluation contract while editing the skill or checker it evaluates.
Use supplied facts or independent review for labels; never relabel cases from
the proposed check's own verdicts. The worker cannot rewrite its judge.

Author one focused change. Calibrate cutoffs or test removing a redundant check
on separate calibration cases. Freeze the proposed setting before held-out
comparison. Removing a check must preserve required coverage and improve the
measured result; more passes alone do not establish better work. A changed
question, model or skill may require new observations, not rescoring old ones.
Report false passes over independently bad cases and false rejections over good
cases, retaining counts, denominators, missing evidence and execution failures.

Compare fresh matched jobs with the same inputs, generator and limits. Keep the
independent final-output check/review unchanged. Record corrected defects, new
defects, unfinished runs, time, total measured cost, token use and worker turns
as relevant to the proposed benefit. Count all provider stages in usage/cost
and worker submissions including repairs in turns; keep unavailable counts
unknown. Equal adequate quality with fewer tokens, fewer loops, less time or
lower cost is valuable without a quality score increase. Preserve the quality
floor and report efficiency tradeoffs; not every workflow needs every metric.
Counterfactual rescoring cannot establish fewer real worker turns.
State when only fixed candidates or protocol fixtures were tested; neither proves improved final work.
Use the `checking-deliverables` skill when authoring a semantic or visual check.

Guard against overfitting: renamed IDs, people or dates do not establish
transfer. Include genuinely different tasks, templates or workbooks and cases
where the lesson should not apply. Evaluation cases used to guide another edit
become development data; retain fresh cases for evaluation. Small adaptive
searches can overfit checks, thresholds, routing and efficiency objectives.
Freeze the independent quality target and selected efficiency metrics; fewer
tokens or loops cannot hide unrelated quality regressions. Seek repeatable
gains across representative cases before broad claims. Describe a pilot limited
to one task family as targeted family probes, and keep validation proportional to the claim
rather than requiring an expensive benchmark for every routine edit.

## Transfer a verified method between models

Select a teacher and runner for the task; these are model-agnostic roles, not
fixed models, size requirements or defaults. Astra and Luna are one example
pairing. Compare teacher baseline, runner baseline and taught runner on fresh
matched tasks under the same independent check. The teacher's output is not
ground truth, and it need not be best for every case. Verify the procedure and
examples, then transfer explicit skills, checked few-shot examples or
useful deterministic helpers through Hire or qualifying Hone lessons. Do not
claim to transfer hidden reasoning or weights. Keep method instructions model
independent; model choice belongs to the invocation. Withhold evaluation tasks
from teaching examples.

After freezing the method, test fallback/escalation for remaining hard cases as
a separate proposal, explicitly selecting an escalation model that need not be
the teacher. Optional Weigh routing needs calibration on independently labeled
cases and cannot replace the final check. Report quality relative to
both baselines, total cost/time and relevant token/turn counts including
fallback, plus escalation count over all attempted jobs. Comparable quality
requires an explicit independent error tolerance. Use the same numbered acceptance flow for the tested changes.

## Produce concrete proposals, not automatic policy updates

Return a few numbered suggestions, each with exact change, reason, measured
benefit and denominator, tradeoff, evidence and status: tested improvement,
needs evidence, or did not help. Explicitly state whether fresh final outputs
were tested. Keep failed experiments visible. Test overlapping or dependent
suggestions together before presenting their combination as “Apply all.”

The caller's external runbook maps each displayed number to the exact proposal
digest, source fingerprint and retained evaluation. After acceptance, apply only
those reviewed bytes. A stale source, changed proposal or changed evidence needs
fresh review; do not repair hashes or silently rebase an accepted patch. Recheck
the resulting bytes and relevant contracts. Report proposed, tested and applied
states separately; publishing or replacing a pinned export is a separate action.

If the caller has selected a Bench source checkout, read
`tools/weigh/examples/improve-checks/README.md` beneath its `BENCH_SOURCE` for
optional public-command examples. `triage.py` supplies hypotheses; `evaluate.py`
compares frozen observations, calibration/held-out splits and final reviews;
`select-fix.py` supplies one recorded next-action ID without executing a repair;
`review.py` fingerprints, prepares, shows and verifies a proposal without
changing the selected source. Its verified literal Git argv remains a caller
operation. These helpers are optional, not a new Hire runtime or provider client.

Hone can prepare an exact lesson only from a qualifying recorded recovery.
Hone 0.3.0 also recognizes changed rejected/accepted assistant candidates bound
to complete replay-verified Ply v2 receipts under the same recorded check
identities. Empty pre-check stdin cannot identify a rejected candidate loaded
by a private checker. Equal recorded command hashes do not establish unchanged
rubrics, executable bytes or transitive files. Independently inspect those
claims. Never manufacture a recovery or teach weakening the check as its fix.
Use prepare/show/admit for exact lesson bytes; otherwise author a separately
justified amendment through Hire and identify it honestly.

Keep expertise in skills and deterministic checks in ordinary programs. Do not
add a policy engine, scheduler, provider SDK, daemon, persistent ranking store
or another agent loop to implement this workflow.
