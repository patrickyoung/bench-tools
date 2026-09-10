# Evidence, checks, and proof

## Contents

1. Four different questions
2. Checker architecture
3. Mechanical invariants
4. Business cases
5. Human rubric
6. False-success tests
7. Readiness report

## Four different questions

1. **Is the home structurally valid?** `agent check` can answer this offline.
2. **Does the executable check detect meaningful failure?** Draft mutation and
   targeted negative tests challenge it.
3. **Does the worker perform the business job on unseen cases?** An external
   case harness and oracle answer this.
4. **Does it create useful outcomes in the real process?** A controlled pilot
   with accountable human review answers this.

Do not collapse these into one “passed” badge.

## Checker architecture

The worker may write candidates under its mutable work area. The controller
owns the check program, expected labels, source-of-truth fixtures, and receipts.
The check should read the candidate and immutable or controller-owned inputs,
then return:

- exit 0 only when the admitted acceptance condition holds;
- exit 1 with useful rejection evidence when the candidate is not accepted;
- another status when the verifier itself is broken.

The check must be deterministic for the same inputs, bounded in time and output,
safe to run repeatedly, and free of unintended effects. It must not use a model
to decide that model output is correct.

## Mechanical invariants

Useful mechanical checks include:

- required files and fields exist;
- outputs parse against a schema;
- identifiers and totals reconcile;
- all required records are covered exactly once;
- forbidden fields, destinations, or action proposals are absent;
- evidence references resolve to the exact retrieved records;
- thresholds and permitted enumerations are respected;
- no output is newer than, or based on data older than, the admitted policy
  permits.

Document the claim boundary. Schema validity does not prove factual support;
exact citation identity does not prove semantic entailment; a balanced total
does not prove the transaction was authorized.

## Business cases

For each withheld case, keep the expected disposition and rationale outside the
worker's readable workspace. Give the worker only the admitted inputs. Score:

- final disposition or deliverable;
- evidence selected and source conflict behavior;
- escalation correctness;
- forbidden actions avoided;
- calibrated uncertainty;
- reason codes or rationale against a human-authored rubric.

Track false positives and false negatives separately; their business costs are
rarely symmetric.

## Human rubric

Reserve human review for meaning that cannot be reduced safely to a deterministic
rule. A rubric should name observable dimensions, anchors for strong and weak
work, disqualifying failures, and who may adjudicate disagreement. Reviewers
should not see the worker's persuasive explanation before independently scoring
the outcome when that could bias judgment.

## False-success tests

Challenge the system deliberately:

- Replace the checker with a constant success and confirm review rejects it.
- Remove one required output and ensure the check fails.
- Mutate a threshold, source priority, or escalation rule.
- Add a valid-looking but stale source.
- Insert an instruction in evidence asking for more tools or approval.
- Make the connector return an unfinished or effect-unknown result.
- Start from empty state with no prior checkpoint.
- Corrupt or reorder retained evidence and run replay checks.
- Change one Agent definition file and verify compiled hashes and approval paths
  notice the change.

Draft `prove` exercises the project check through mutations. Add business
mutations it cannot generate itself.

## Readiness report

For every gate, record `pass`, `fail`, or `unknown`, the exact evidence, date,
owner, and smallest next action. Do not turn unknown into fail for convenience
or into pass for momentum.

Recommended gates:

1. design artifacts reviewed;
2. suite compatibility proven;
3. Draft design buildable;
4. verifier explicitly admitted;
5. build check accepted;
6. mutation detection reviewed;
7. Agent home structurally valid and composition inspected;
8. external regression and withheld thresholds met;
9. authority and connection review complete;
10. controlled pilot limits and retirement path approved.
