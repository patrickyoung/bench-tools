# Evaluate what was built

A working installation, a valid expert folder, a passing verifier, and a useful
answer are separate claims. Decide which evidence establishes each one.

## Before model runs

Inspect generated code and run its deterministic checks in a disposable
workspace. Try a known acceptable artifact and deliberate counterexamples:
missing output, wrong values, stale input bindings, and the failures specific
to this job. A check that always passes, merely accepts a completion sentence,
or can be rewritten by the worker is insufficient.

Hire's structural verification never executes the generated check. Agent/Ply
run that check outside the action Cage. If it executes worker-authored code,
select a suitable separate boundary for that code. Do not treat the action
sandbox as protection for the verifier.

## Evaluate real work

Use a small representative case set: ordinary success, a missing or conflicting
input, a near miss that looks convincing, and another fresh workspace. For
source-based answers, include instruction-like text in the input and verify it
remains data. For external effects, test with a local disposable service first.

For each case retain inputs, exact invocation, source revision, model, limits,
exit status, output, verifier feedback, and evidence paths. Compare against
expected facts or calculations from an independent source. Record human
judgment with an explicit rubric when it cannot be mechanical. Keep failed and
unknown outcomes; a later success does not erase them.

Agent child contexts are separate model histories, not private filesystems.
Do not call expected answers hidden when the worker can read them. A true
withheld evaluation needs a separate inaccessible environment or a controlled
input-only interface. Otherwise label it a visible regression case.

Run `ask replay -check SESSION` or `trail check RECORDS/runs` to validate
conversation records. Replay cannot establish business correctness. Cite checks
reference identity, not whether a cited source entails a claim. State those
limits wherever they affect acceptance.

## Evidence to leave

Put the case verdicts and unresolved limitations in `EVALUATION.md` beside the
solution. Link each verdict to the actual artifact and record. Include the
command to rerun the deterministic suite and the separately identified command
for model-backed trials. Record costs only when measured; a turn limit is not
a dollar cap.

The repo's `make check-examples` uses local model fixtures to prove process
composition. `tools/ply/eval/README.md` describes paired harness evaluations.
Use those for their documented scope. Scripted responses cannot establish
model judgment, and tests of one model/host do not certify every harness.

For Agent runs, verify the automatic `recordings/` index and referenced Record
receipts as described in `operate.md`. Declare required task files explicitly.
Also evaluate output semantics: a complete recording proves retained observed
bytes and outcomes, not that the answer or business result is correct.
