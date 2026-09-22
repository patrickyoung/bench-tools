# Improve a run with tested, reviewable changes

Use this procedure for “Improve this run,” “make this check useful,” or a request
to propose improvements to a worker's skills, rubric, feedback or thresholds.
Use the current task, worker and source selection; ask for a target only if it
remains ambiguous. Inspect and run the bounded tests already authorized by the
request. Do not ask the user to choose tools when the existing setup answers it.
An analysis-only request never admits a change.

This improvement workflow leaves the selected source unchanged while preparing
suggestions. “Apply 1” or “Apply all” accepts the identified reviewed changes.
It does not impose a new approval step on ordinary edits the user has already
asked you to make. An explicit request to implement a specific correction keeps
its existing authorization; explain what changed and how it was tested.

## Run a bounded experiment with Improve

Before creating the experiment, define its use-case acceptance contract as
described in [Test a focused proposal](#test-a-focused-proposal). The creating
application proposes that policy from the user's intended outcome and records
its rationale. It does not inherit a global cost/accuracy threshold from a demo.

For log-driven experiment creation, inspect the reusable
`workers/experiment-researcher` entry in the selected checkout. It uses Trail
to investigate explicitly selected archives and returns a cited hypothesis and
an `experiment.json`, or an honest needs-input/no-experiment result. The model
writes only `research.json`; its local `bin/check --assemble` derives the plan
from the trusted template and checked research. Default checking is read-only. The caller
supplies the reviewed template, independent cases and judge, models and bounds.
The researcher preserves those choices and stops after `improve -n`; running
the experiment remains a separate existing command. Its checked citations
establish recorded text identity, not hypothesis quality. Follow its README
for the trusted request and external workspace contract.

The independent `improve` command turns one source-change experiment into a
repeatable Unix filter. Read `tools/improve/README.md` in the selected checkout.
Its JSON specification names a clean source tree, exact mutable existing files,
development and reserved cases, literal proposer/trial/judge commands, declared
dependency files and limits. `improve -n` validates without execution;
`improve -o NEW_DIRECTORY` runs the admitted experiment. Use the source-matched
installer to install `improve` explicitly if it is absent from the builder setup.

Reuse the router example only as a starting adapter or synthetic process test.
Select real independent labels and a judge matching the task. Hire can author
prompt/skill/subagent/check changes; Agent executes workers; Record captures the
process boundaries; Ask replay supplies model usage and stop evidence. Preserve
unknown costs, including retries and missing usage. Never confuse a worker's
format-check pass with independent job correctness.

One invocation allows one proposal, fresh matched development comparisons and
one reserved comparison after development passes. No second proposal follows
holdout feedback in that experiment. The result preserves unsuccessful attempts
or exports exact supported files with a manifest. Review and promote under the
existing authorization; the command never writes the selected source or
publishes it. `improve verify DIR` checks retained evidence without inference.
Weigh is optional research composition, not a new live-run dependency. These
independent trials do not create a qualifying Hone recovery by themselves.

## Establish what failed and what must stay true

Keep the original source pin, run, candidate, check outputs and independent
final review. Follow [setup](setup.md) and [library](library.md) to select the
actual definition. Create a separate authoring copy; keep cases, expected
answers, model sessions, proposed diffs and the comparison runbook outside the
reusable source. Record the exact files and hashes that define each comparison.

Distinguish a deliverable defect from missing evidence, wrong applicability,
an incomplete rubric, misleading feedback, a cutoff problem or broken execution.
A failed process is not low confidence. A checker pass is not an independent
quality label. Inspect the final artifact, including actual rendered media when
necessary, before deciding what to change.

Optional helpers live at `tools/weigh/examples/improve-checks/README.md` beneath
the selected `BENCH_SOURCE`; read that README and each selected helper's help.
Weigh is off unless `BENCH_WEIGH=1`; unset, empty or `0` keeps it off. Explicit
Ask and deterministic rules remain available without the Weigh service.
`triage.py` and `select-fix.py` need an explicit backend and model for inference;
they need no additional live-call flag. The semantic-check batch evaluator
retains its explicit `--live` opt-in. A selected live Weigh path needs provider
access; an unavailable required judgment fails without passing the candidate
or switching backends. Preserve existing dependencies during unrelated edits.
`triage.py` can ask explicitly selected Weigh or Ask to suggest an investigation
from one supplied snapshot. Its output is a hypothesis to test, not a diagnosis,
recovery witness or permission to change policy. Direct inspection can suffice.
Use existing credential wrappers and recorded public commands; do not introduce
a provider client, scheduler or agent loop.

## Select the next repair when judgment helps

Use ordinary rules for a repair established by inspected facts. For a remaining
semantic choice, `select-fix.py` in the same example directory lets optional
Weigh or Ask select one ID from the caller's allowed actions. Supply the task,
exact candidate, selected evidence and check findings together: a check pass
alone does not justify keeping an artifact, and a rejection alone does not prove
it needs editing. A trusted rule result can select an action without inference;
an undecided rule leaves the choice to the explicitly configured model.

The helper records one choice and never executes it. Bind the result to the
exact snapshot, map the ID to reviewed literal argv, and use the existing
runner's action boundary and remaining limits for at most the selected repair.
Keep the runner model separate from the selector model. Reject malformed or
failed selection instead of silently retrying or switching backends. Re-run
the unchanged independent final check even when the choice keeps the candidate.
A valid selection is not acceptance. Check/rubric changes still follow the
tested proposal flow below; a fix selection is not a Hone recovery witness.

Read `select-fix.py --help` and the example README for its input, rule binding
and dispatch contracts. Evaluate routing as its own proposal against simple
rules and, when useful, an explicitly selected Ask selector using the same
allowed actions and fresh cases. Measure the whole path, including selection,
repair and final checks; a faster selection can still add cost or tokens.

## Test a focused proposal

The application creating the experiment owns its acceptance contract. Derive
and propose criteria from the user's use case: who acts on the result, what a
qualified outcome means, critical failure consequences, the value of better
quality and the operating budget. Reuse an applicable established policy when
one exists. Explain the basis for each threshold and acceptable trade-off;
do not ask users to invent percentages when their business context supplies
the answer, or invent business values when it does not. Resolve material
missing context before admitting a paid evaluation.

Include the selected policy and rationale in the experiment before testing,
with case coverage, an independent rubric and scoring method, and a defined
accept/reject/insufficient-evidence action. Define the exact promotion target,
configuration and rollback pin before deployment. A policy may accept higher
quality at greater cost, or adequate quality at lower cost; not every metric
must improve. Price premiums, savings targets and quality floors are use-case
choices, never toolkit defaults. Keep critical requirements separate from
preferences. A changed policy starts a new experiment; it cannot convert an
old holdout failure into a prospective pass.

The caller-owned `tools/improve/examples/acceptance/` adapter in the selected
checkout provides deterministic policy preflight and an existing-protocol
judge. The creating application supplies its policy; the adapter only applies
the frozen rule. Its `spec.py --experiment DRAFT --policy POLICY` command binds
that policy to the app's draft, checks matching coverage and selects the judge
without running the experiment. The trial/scorer must implement the example's
qualified-outcome score contract. Validate it before paid execution because Improve's generic
`-n` does not interpret application settings. For the experiment researcher,
put the reviewed policy in its trusted template: its assembler preserves the
gate and the worker cannot rewrite that gate while researching logs. Missing
policy context remains `needs_input`, not a fabricated release decision.

Use Hire when expertise or check wiring needs authoring, and the existing Agent
or Ply entry command for runs. State one concrete change to test. Preserve the
original check and definition as a baseline; do not let either worker modify its
checker or independent evaluation contract.

Freeze that independent evaluation contract before testing: required outcomes,
criterion/check coverage, independent label rules, acceptable false-pass and
false-rejection rates, and required final-output review. It stays unchanged
while the proposed skill, check, feedback or threshold changes. Retiring a check
requires showing that required coverage remains and that its removal helps;
more accepted candidates alone is not an improvement.

Use separate calibration and held-out cases. `evaluate.py` is an offline filter
for supplied recorded observations and independent labels; it does not collect
new model results or select a winner from held-out scores. Calibration sweeps
and ablations can suggest a candidate. Freeze that candidate before evaluating
it. A changed question, model or skill needs new observations where behavior
changes; reusing old scores cannot simulate a new judge or worker.

Report false passes per independently rejected case and false rejections per
independently accepted case, with counts and denominators. Preserve missing,
uncertain and execution-error outcomes. Then compare fresh matched jobs using
the same inputs, generator, limits and other settings. Judge final outputs with
the unchanged independent check/review, recording repaired defects, new defects,
unfinished runs, elapsed time, total measured cost, token usage and worker turns
when relevant to the proposed benefit. Count all provider stages in usage and
cost; count worker submissions, including repairs, consistently on both sides.
Keep unavailable measurements unknown. A fixed-candidate score
comparison or offline fixture cannot establish improved completed work. Follow
[evaluation](evaluate.md) and [semantic checks](semantic-checks.md) for details.

Check transfer on genuinely different tasks, templates or workbooks, including
cases where the lesson should not apply. Changing only names, IDs or dates is
not evidence of generalization. Once evaluation feedback guides an edit, that
case is development data; reserve new cases for the next evaluation. Even a
small adaptive search can overfit the checker, threshold, routing or efficiency
objective. Keep the independent quality target and selected efficiency metrics
fixed, and do not hide unrelated quality regressions behind fewer tokens or
loops. Require repeatable gains and representative coverage before making broad
transfer claims. Label a pilot confined to one task family as targeted family
probes. Match the amount of testing to the change and claim; routine edits do not all need an
expensive benchmark or a universal statistical test.

Keep unsuccessful experiments. Report “didn't help” when evidence supports that
conclusion, and “needs evidence” when required cases or final reviews are missing.
Do not lower the independent acceptance bar to make an experiment look successful.
Equal adequate quality with fewer tokens, fewer loops, less time or lower cost
can justify an improvement; higher quality is not required. Keep the quality
floor unchanged and state tradeoffs between efficiency metrics. Collect relevant
measurements rather than requiring every metric for every workflow. Offline
rescoring or removing a check from a list cannot establish fewer real turns.
If suggestions overlap or depend on one another, test and freeze their combined
result before offering “Apply all”; separate successes do not prove the union.

## Transfer a verified method between models

Select the teacher and runner explicitly for the task. These are roles, not
model names, size requirements or defaults; Astra as teacher and Luna as runner
is one example pairing. Compare three conditions on fresh matched tasks under
the same independent check: teacher baseline, runner baseline, and runner with
the proposed method. The teacher's answer is not a gold label, and it need not
be the best model for every case. Independently verify useful recoveries and
examples before transferring them through Hire or qualifying Hone lessons.
Transfer an explicit procedure, skill, small checked examples or deterministic
helper; do not claim to transfer hidden reasoning or model weights.
Keep the reusable method independent of model identity and select the model at
invocation. Keep teaching examples separate from evaluation tasks.

Freeze the method first. Then test an explicit fallback/escalation policy for
remaining hard cases as a separate proposal. Explicitly select the escalation
model; it need not be the teacher. Optional Weigh can suggest which cases need
escalation, but calibrate routing on independent labeled data and retain the
final check. Report quality against the teacher and the untaught runner, total
cost, latency, token use and worker turns where relevant, including escalation,
and the number escalated over all attempted tasks. “Comparable quality” needs
a stated independent tolerance and representative evidence, not a higher acceptance rate.
Present the tested transfer or escalation change through the same numbered
review-and-apply flow below.

## Present a small set of numbered suggestions

For each suggestion state the concrete change, why it addresses the observed
problem, measured benefit with denominators, tradeoff, evidence links and tested
scope. Explicitly distinguish checker discrimination from final-output testing.
Name anything still untested. Prefer a few useful suggestions over a long list
of plausible improvements. Include what was tried and did not help.

The following is a fill-in card example, **not observed results**:

1. **Distinguish historical import notes from current warnings.** Change the
   warning criterion to preserve an explicitly historical note while checking
   current issues separately. This addresses the inspected stale-warning error.
   **Benefit:** false passes `[before]/[bad held-out cases]` to
   `[after]/[bad held-out cases]`; false rejections `[before]/[good cases]` to
   `[after]/[good cases]`. **Efficiency and tradeoff:** `[measured tokens/turns/time/cost benefit and
   any regression in another metric]`.
   **Final-output testing:** `[tested on N fresh matched jobs / not yet tested]`.
   **Status:** `[tested improvement / needs evidence / did not help]`.
   **Evidence and exact change:** `[comparison record and reviewed diff]`.

Tell the user which tested suggestions can be applied and how to select them,
for example “Apply 1” or “Apply all.” Do not present a needs-evidence experiment
as ready for adoption. Leaving everything unchanged is a valid outcome.

## Bind acceptance to the exact reviewed change

The harness keeps the number-to-proposal-digest mapping in the external runbook,
with target source fingerprints, evidence, tested scope and any combined
proposal. The user need not copy hashes. Keep displayed numbers stable while
review is pending; a changed proposal needs a new review identity.

For source changes, the optional `review.py` helper fingerprints explicit files,
prepares a new proposal directory, shows it, and verifies it against the reviewed
digest and current source. It never applies changes. After acceptance, its
`verify` output supplies literal Git `apply_argv`; execute that argv through the
existing Git command, without shell evaluation. Verify the resulting source
fingerprint and run the relevant checks. Do not bypass a stale-source refusal,
rewrite hashes, silently rebase the patch or substitute newly generated wording.
Changed source, proposal or evidence requires a fresh comparison and review.

For an actual checked recovery, use [Hone's exact lesson procedure](learn.md).
Hone 0.3.0 can recognize rejected and changed accepted assistant candidates only
when complete replay-verified Ply v2 receipts bind both inputs and outputs under
the same recorded checker identities. An empty initial pre-check cannot identify
rejected candidate bytes read privately by a checker. Recorded command hashes
do not prove unchanged rubric, executable or transitive files. Inspect that
independent evidence too; never fabricate the missing recovery. Hone words a
supported procedural correction, not a lesson to weaken checks or cutoffs.
Prepare/show preserve the exact lesson for acceptance; admit applies those bytes.
A nonqualifying run may still motivate a separately justified Hire amendment.

After the selected changes are applied, report the actual applied files and
validation. Follow [library promotion](library.md) for any separately authorized
commit, source pin or export update. Existing exports do not update themselves.
Keep proposed, tested, applied and published states distinct.
