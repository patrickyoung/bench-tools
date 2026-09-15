---
name: weighted-comparison
description: "Use to compare technical candidates against business needs with anchored weighted criteria, mandatory gates, scoped evidence, visible unknown coverage and sensitivity-aware recommendations."
---

# Weighted comparison

Use with the installed definition's authoritative `CONTRACT.md`. This skill
adds judgment, not a scoring engine, source collector or new schema.

## Frame and design

- Translate the supplied business case into the decision to make, relevant
  users/capabilities, constraints and horizon. Use business-outcomes and
  product-capabilities where relevant. Do not invent a baseline, target or
  alternative's capabilities. Discuss reuse or defer when relevant to the
  narrative without adding unsupplied matrix candidates.
- Honor supplied criterion IDs, names and weights exactly; preserve supplied
  reasons and anchors. Do not reweight to favor a preferred candidate. Missing
  reasons or anchors must be explicitly defined. Invalid supplied definitions
  need host correction, not quiet normalization.
- Without supplied criteria, propose 3–12 material, non-overlapping dimensions,
  with positive weights summing to 100 (contract tolerance 0.000001).
  Explain outcome relevance, tradeoffs and stakeholder assumptions in
  weight_rationale/assumptions. Avoid counting the same benefit in feature,
  productivity and cost criteria. Mark weight_basis correctly.
- For each criterion define all six 0–5 anchors as observable, criterion-specific
  conditions, higher always better. Use comparable scope and meaningful
  distinctions, not just "poor" through "excellent". For cost or risk, lower
  cost/risk must map to better scores. Do not invent numeric thresholds as
  stakeholder requirements: label proposed thresholds as assumptions. Define
  anchors before examining which candidate wins.
- Mandatory gates are separate from compensating weighted criteria. Preserve
  every supplied gate ID and assess every candidate. Do not invent binding
  gates from manager suggestions or silently relax supplied constraints.
  If a narrative hard constraint lacks a usable gate definition, surface it
  for host clarification and do not recommend a violating option.

## Evidence and comparability

For each cell, inspect applicable source segments and counterevidence. In its
rationale identify the evidence category and scope:

- Observed proof: supplied test, demonstration or measured result, limited to
  its actual environment, workload and observation period.
- Documented statement: published specification, terms or documentation; not
  automatically a measured operational result.
- Seller assertion: sales statement or unverified claim; attribute it and
  require corroboration for conclusions needing demonstrated performance.
- Roadmap: future intent, not current capability or a present gate pass.
- Human preference: decision input, not evidence that a vendor has a feature.

Evidence strength is contextual, not a mechanical source hierarchy. Record
recency, retrieval date versus content date, extraction limitations and scope.
Do not claim freshness from retrieval alone. A quotation's presence does not
prove that it entails the proposed score. Confidence high/medium/low describes
the actual evidence basis and limitations; do not invent probability values.

Match offering, version, plan/tier, deployment mode, region and availability.
Do not transfer an enterprise-tier feature or future release to a different
candidate plan. If supplied scope is ambiguous, record the gap. Shared sources
can support multiple candidates only where their content actually applies;
otherwise refs must identify a source listing that candidate.

Use finops for economics. Compare currency, pricing date, tax treatment, billing
period, seat/usage units, scale, commitment, discounts, included support and
service levels on a common supplied basis. Expose license, operations,
migration, integration, egress and exit omissions where material. Distinguish
list price, quote, estimate and observed cost. Show assumptions and traceable
arithmetic in rationales when converting compatible supplied values. Do not
invent exchange rates, workloads, discounts or total costs to force a ranking.
A partial price is not TCO; an unknown cost is not free.

## Score and gate judgments

Use exactly one cell for every candidate/criterion:
- supported: an integer 0–5, with refs sufficient for the chosen anchor and a
  rationale explaining fit and limitations. Zero needs affirmative failure
  evidence. A low-confidence supported score still needs actual support.
- unknown: null, with a concrete missing-evidence explanation. Cite relevant
  context when available; do not quote unavailable content.
- conflicting: null, with at least two refs and an explanation of the
  unresolved contradiction. Never average incompatible claims.

Investigate whether apparent contradictions concern different dates, versions
or plans. If evidence resolves one, explain why that source applies; otherwise
retain the conflict. Do not cherry-pick favorable sales claims or treat
repetition of one claim across sources as independent corroboration. A seller
claim alone cannot satisfy an anchor requiring observation. Score only what
the applicable evidence establishes, not the best plausible interpretation.

Each ref has only source_id, locator and quote. Copy a verbatim substring of
at least eight characters from that exact segment; do not insert ellipses,
paraphrases or invented citations. Quotes should be sufficient to understand
the claim in context. Inspect unavailable status and limitations before use.
Images omitted by extraction require supplied transcript or visual/OCR
follow-up through the host, never an assumed visual inspection.

For each candidate/gate use pass, fail or unknown. Pass/fail require direct
applicable refs. Unresolved contradiction is unknown. Gates cannot be traded
away by a high weighted result.

## Interpret the renderer, decide and hand off

Let compare.py calculate; do not manually replace its outputs:
- Known points = sum(weight * score / 5) over supported cells.
- Lower bound = known points; upper bound adds unresolved weights.
  Assigning zero to missing evidence is only a conservative bound, not a score.
- Coverage is the sum of supported weights, expressed as a percent.
  Known-only fit = 100 * known points / coverage, null with zero coverage.
  Always discuss fit alongside coverage and bounds. No renormalized overall
  score and no confidence multiplier may hide missing evidence.
- Candidates are excluded for any failed gate, conditional for an unknown
  gate, otherwise eligible. Rank eligible before conditional before excluded,
  then by lower bound, retaining explicit shared ranks for exact ties.
- Sensitivity perturbs each weight by ±20% relative and renormalizes to 100;
  it reports conservative-score leaders among eligible candidates. Explain
  leadership changes, ties or no eligible candidates. Stability under this
  limited test is not robustness to all weights, evidence gaps, correlated
  changes or alternative assumptions. Bounds are not probability intervals.

In decision_summary and recommendation.rationale write a concise business
narrative: decision, strongest tradeoff, gate result, relevant alternatives,
coverage/bounds, evidence weaknesses and sensitivity implications. Explain
close calls or divergence between rank and proposed choice. A higher lower
bound may reflect evidence availability, not inherently superior capability.
Overlapping bounds leave the decision exposed to new evidence; do not claim
a proven winner from one score. Proposed weights require stakeholder review.

Use recommend only for a defensible supplied candidate with nonzero coverage
and no failed/unknown gate. Use conditional for a selectable candidate needing
explicit diligence or approval; never select a failed-gate or zero-coverage
candidate. Use defer with null candidate_id when no defensible choice exists.
A recommendation remains advice for independent and human review.

List assumptions separately from facts. Use gaps for missing, unavailable and
conflicting evidence and decisive diligence: candidate_id (or null if shared),
a focused question, role owner and decision impact. Use evidence-discovery to
propose the smallest discriminating test with acceptance/stopping rules when
useful. Prioritize what could change eligibility or the decision rather than
demanding an exhaustive questionnaire. Do not invent named owners or promise
that a test has run. New acquisition is a host/human follow-up.
