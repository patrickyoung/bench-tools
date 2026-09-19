# Comparison specialist

Reusable middle member of a fixed comparison team:

1. Unchanged Product Owner manager supplies optional planning advice.
2. This specialist analyzes the normalized evidence packet and renders a
   traceable comparison.
3. A separately invoked unchanged Product Owner reviewer judges evidence
   entailment, business reasoning and completeness, returning proceed,
   revise or hold through the host's existing team process.

There are no nested agents, listeners, scheduler or new runtime here. The host
owns team handoffs and integration. The copied business-outcomes,
product-capabilities, finops and evidence-discovery skills remain unchanged;
weighted-comparison supplies the missing scoring and decision discipline.
This definition contains no current business case or vendor evidence.

## Inputs, prerequisites and invocation

Supply an existing isolated workspace with immutable `packet.json` prepared
by the team's deterministic helper, and optionally `planning.md`. The packet
must conform to `CONTRACT.md`: schema bench.comparison-packet/v1, original job,
candidate identities and normalized sources with exact source segments,
provenance, hashes, availability and extraction limitations. Planning is
advice, not vendor evidence. Supply actual accessible packet bytes to each
invocation, not a bare path offered as if its contents had been read.

Required: public Agent and its configured model, Python 3, the host-implemented
`tools/compare.py`, and the host-implemented executable `bin/check`. Definition
and checks stay outside mutable workspaces. Use absolute paths for DEFINITION,
WORKSPACE and EVIDENCE; EVIDENCE is an explicit operator-controlled location
outside the reusable definition. With those shell variables set, the Unix or
separate subagent invocation is:

    agent run -C "$WORKSPACE" -evidence "$EVIDENCE" "$DEFINITION" -- \
      "Compare the candidates in packet.json under CONTRACT.md; use optional planning.md as advice and produce checked outputs."

Keep the default network-disabled action boundary; no -net or -no-cage.
The controller independently invokes manager/reviewer with their explicit
inputs and dependencies. This specialist never invokes them itself.
Preserve stdout, stderr, exit status and the five outputs under the chosen
evidence root. Do not release a failed/unknown specialist attempt as an
accepted contribution; review and final integration remain explicit host
responsibilities.

## Exact analysis interface

Use `CONTRACT.md`, section beginning **“analysis.json is JSON with exactly:”**,
as the exact JSON shape reference, not a loose illustrative schema. It specifies
all required keys, nested records, enums, null behavior and citation restrictions.
The required top-level keys are:

- schema, packet_sha256, decision_summary;
- weight_basis, weight_rationale, criteria;
- cells, gates, recommendation, assumptions, gaps.

The schema literal is `bench.vendor-comparison/v1`. packet_sha256 binds exact
input bytes. Criteria contain id/name/weight/reason/anchors (keys "0"–"5").
Cells contain candidate_id/criterion_id/score/status/confidence/rationale/refs.
Gates contain candidate_id/gate_id/status/rationale/refs.
Refs contain source_id/locator/quote. Recommendation contains
status/candidate_id/rationale; each gap contains
candidate_id/question/owner/decision_impact. Assumptions are strings.
Do not add a homemade evidence-strength field: put the category and scope in
rationales. Do not put aggregate scores in analysis: the renderer owns them.

From WORKSPACE, after writing output/analysis.json:

    python3 "$DEFINITION/tools/compare.py" render
    "$DEFINITION/bin/check"

The only files in output are analysis.json and the generated matrix.json,
report.md, matrix.csv and evidence.md. See CONTRACT.md **Deterministic scoring**
for exact arithmetic, eligibility, ranking, tie and sensitivity semantics.
Never modify generated files to reconcile them with preferred prose.

## Validation and bounded examples

`hire verify expert` is structural validation only. It does not run generated
checks, a model-backed comparison or the team reviewer.

The host's read-only runtime check validates schema, exact packet binding,
complete candidate/criterion/gate coverage, weights, source identity and quote
containment, gate selection rules, formulas and exact rendered bytes. It
cannot prove source truth, quote entailment, extraction completeness or business
judgment. Human and independent reviewer scrutiny are still required.

Reproducible examples for an operator using a fresh supplied packet, outside
this definition (not evaluations performed by this authoring task):

- Positive: use that packet's real IDs, all criteria and gates; score only
  supported anchors with applicable exact quotes. Mark unresolved cells null,
  record their gaps, and use defer/null if no choice is defensible. Render,
  then run bin/check. A contract-conforming uncertain comparison can pass;
  a fabricated complete one should not be preferred.
- Negative: in a disposable copy of an otherwise accepted workspace, alter
  packet_sha256 in analysis.json to a different digest and run bin/check.
  It must reject input mismatch. Separately, selecting a candidate with a
  failed gate must reject; missing evidence represented as zero without refs
  must reject. Recreate each test from the accepted copy rather than stacking
  failures. Keep tests and receipts outside expert.

This authoring task runs structural verification only; it launches no live
evaluations and stores none here. The definition includes the deterministic renderer and checker. Fresh model
trials, case verdicts and independent evaluation limits are recorded outside
this definition in the solution EVALUATION.md.
