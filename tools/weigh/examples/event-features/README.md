# Extract small features for offline experiments

This example is one Weigh request, not an experiment runner. Its supplied
synthetic candidate illustrates three independent observations: the reason
given for a route, a claim that tests ran, and expressed uncertainty. Each
question sees the supplied candidate directly; no question needs another answer.
The candidate's claims are data, not trusted instructions or verified facts.

With Weigh 0.2.0+, jq, an explicitly selected Decisions model and a private
credential environment, run from this directory. Choose an external directory
for evidence:

```sh
record run -f "$EXPERIMENT_DIR/features.record.jsonl" -- \
  weigh -m "$WEIGH_MODEL" < request.json > "$EXPERIMENT_DIR/features.json"
```

Only after the command succeeds, extract numeric features for analysis:

```sh
jq '{
  keyword_basis: .answers.decision_basis.probabilities.keyword,
  claims_tests_ran: .answers.claims_tests_ran.value,
  stated_uncertainty: .answers.stated_uncertainty.value,
  model: .model
}' "$EXPERIMENT_DIR/features.json" > "$EXPERIMENT_DIR/row.json"
```

This call costs provider usage. No numerical answers in this example are
asserted in advance. Retain the complete result, question definitions and Record
receipt; the reduced row is only a convenience for analysis. Run `record check`
on the receipt to verify it, or `record replay` to recover the output offline.

For actual episodes, select a small relevant excerpt first and replace `state`
with that evidence. Bind the source receipt and excerpt hash to the row in an
external manifest. Keep deterministic facts separate: count successful test
invocations from event records in code. A high `claims_tests_ran` value measures
what the candidate says; it does not establish that tests ran or passed.

The experiment composition remains ordinary files and filters:

```text
selected Record/Trail evidence → small state + frozen questions → Weigh features
                                                                     ↓
independent labels → analysis → Hire proposes one change → Agent runs fresh cases
                                                                     ↓
                                 deterministic scorer → keep/discard → new release
```

A useful first experiment compares a deterministic baseline with the same
baseline given these features. Freeze questions and candidate selection before
the holdout, use matched run limits, retain failures and costs, and score actual
worker outcomes. Features that agree with a plausible story but do not improve
fresh outcomes should not be promoted. Preserve independent final checks; a
proposed checker must not grade its own promotion. Feed Hone only real checked
recoveries that qualify under its existing contract.

For overlapping failure types, use one probability question per observation
rather than forcing one diagnosis. A relative choice among fixes cannot prove
that any fix helps. If a later experiment needs ranking, separately evaluate
whether each candidate applies; do not invent a universal confidence cutoff.

This follows TypeSafe's [AutoResearch feature discovery pattern](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery)
and [atomic-question guidance](https://docs.typesafe.ai/primitives/noul).
Their recipe uses a generative proposer, Jev features and external evaluation.
Weigh supplies the feature step; question generation, statistics and promotion
stay with the caller. See the [small worker experiment](../../../../examples/experiment-engine/README.md)
in a Bench checkout for an existing trial/scoring composition.
