---
name: leadership-narrative
description: Turn product strategy or checked evaluation findings into a concise leadership recommendation with evidence, tradeoffs, smallest commitment and an explicit decision ask. Use without imposing feature templates on vendor framing or synthesis.
---

# A SAFe product narrative leadership can act on

SAFe governs the narrative: trace vision -> customer need -> feature benefit ->
sequence/dependencies -> business decision. Product Management owns needs,
vision/roadmap and ART backlog/features with System Architect, RTE and Business
Owners; Product Owner/team owns team backlog and actual stories/planning.
Apply customer centricity and design thinking, considering desirability,
viability, feasibility and sustainability. The two-file package is Bench storage,
not a SAFe-mandated schema.

Write the main leadership narrative in output/response.md, normally about
250–450 words. Explicit request/team word limits take precedence; include
detailed feature descriptions only as useful and within the total bound.
This is a coherent argument, not a checklist recitation or a new deliverable.
For blocked intent, a concise needs-input explanation is better than filler.

1. Lead with **one clear recommendation** and the intended outcome. Explain
   why it matters and why now using supplied evidence; mark urgency or
   strategic fit unknown where unsupported. A requested solution or executive
   preference is not proof of customer need.
2. Connect the observed problem to the proposed behavior change and benefit
   hypothesis. Identify the decisive evidence and its limits, separating it
   from assumptions and targets. Use the same feature IDs and outcomes as the
   detail and JSON handoff; do not quietly change cohort, metric or scope in
   the executive summary. A benefit hypothesis is proposed measurable value,
   not realized value; acceptance checks feature behavior and relevant NFRs.
3. Explain the strongest relevant alternatives, including reuse or defer when
   appropriate, why the recommendation is preferable, its principal tradeoff
   and displaced work/opportunity cost. If displaced work is not known, say
   who must confirm it rather than invent another initiative or spare capacity.
   For a broad breakdown, explain how cohesive features and their justified
   sequence support vision/roadmap, with dependencies and unresolved decisions.
   For one feature, keep its ID/name, context, hypothesis and acceptance with
   a concise business narrative; no mandatory stack of separate documents.
4. Recommend the **smallest commitment** justified now. State readiness in prose:
   recommended validation, build or refinement stage, and what prevents the
   next stage. Build advice does not assert PI fit or approval. An experiment
   can be the right commitment even when a feature is well described.
5. Close with an **explicit decision/ask**, proposed accountable owner pending
   confirmation where needed, measurable next review and advance/change/stop
   rules. Use a supplied date or an evidence/event trigger, not an invented
   deadline. When baseline/threshold is unknown, make establishing it and
   agreeing a target a bounded next action before benefit-based expansion.
   Plan a review of actual customer/business outcomes after release, preserving
   attribution limits and revising the benefit hypothesis when evidence changes.

A reader should know what to decide, what not to commit to yet and what evidence
could change the recommendation. Do not fabricate funding, dates, ROI,
stakeholder agreement, estimates, capacity, owners' acceptance or causal
certainty. "Leadership should align" and "continue research" are not actionable
asks without responsibility, evidence and a consequence for the decision.

Roadmaps forecast future work; distinguish supplied current commitments from
later forecasts. Relevant PI Planning input supplies vision, feature priorities,
dependencies, enablers/NFRs and WSJF preparation, not unilateral team promises.
Teams create PI Objectives during PI Planning. PM provides context or clearly
illustrative proposals only; never assert PM commits objectives, capacity,
business-value scores or dates.

## Keep modes and handoffs intact

- In strategy, feature sequencing is a recommendation in prose, not vendor
  selection. Keep details.selection not-assessed/null. Use existing summary,
  recommendation, desired_outcome, decision_rule, evidence_plan,
  delivery_handoff and next_actions to carry consistent decision and handoff
  meaning, not a new narrative or readiness schema.
- In evaluation, recommend a decision framework and diligence commitment, not
  a winner. Preserve explicit candidate scope, weights and gates. Alternatives
  outside that scope are human scope questions only.
- In synthesis, the one recommendation must respect the checked comparison's
  status/candidate, including conditions, or defer/null with revision findings.
  Preserve analyst limitations and refusal; never rescore or switch candidates.
  A concise narrative does not justify dropping material statistical caveats.
- Do not apply the feature definition checklist to vendor evaluation/synthesis.
  Use relevant outcome, tradeoff and decision principles, keeping the existing
  framework rationale and team handoff responsibilities.

Before finalizing, use QUALITY.md as a manual review rubric, not proof that a
semantic judge ran. Correct internal contradictions and unsupported claims.
The unchanged bin/check validates the package envelope/schema and arithmetic,
not the persuasiveness, truth or usefulness of this narrative.
