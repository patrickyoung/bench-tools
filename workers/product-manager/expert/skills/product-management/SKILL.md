---
name: product-management
description: Frame product strategy, evaluation criteria and decision-changing discovery, or synthesize checked specialist evidence into an executive decision brief. Use for feature shaping, platform value, customer outcomes, strategic fit, viability, feasibility questions and lifecycle tradeoffs; not team sprint planning or vendor scoring.
---

# SAFe Product Management decision judgment

Follow the owning expert's CONTRACT.md; this skill introduces no output fields.
SAFe governs the role and deliverables; choose only steps and supporting skills
relevant to the bounded decision. Product Management owns customer/market needs,
vision/roadmap and ART backlog/features, collaborating with System Architect,
RTE and Business Owners. Product Owner/team owns team backlog and actual
stories/planning. The two-file package is Bench storage, not a SAFe schema.

## Explicit supporting-skill routing

- For feature definition/decomposition or sequencing, use **strategy** and
  read skills/feature-shaping/SKILL.md. Keep selection not-assessed/null;
  sequencing is not vendor selection. Feature detail belongs in response.md,
  with decision/outcomes/handoff in existing JSON fields, never new schema.
- For internal/developer platform strategy, also read
  skills/platform-product/SKILL.md: start from one evidenced consumer journey
  and select relevant platform concerns, not a mandatory portfolio catalog.
- Read skills/leadership-narrative/SKILL.md to integrate one recommendation,
  alternatives, tradeoffs, smallest commitment, explicit ask and review rules.
  In evaluation/synthesis apply its decision principles without feature templates.
- Use evidence-discovery and business-outcomes for learning/benefit design;
  product-capabilities for broader capability/lifecycle questions and finops
  for consequential economics. They do not require exhaustive inventories.
- Manually review QUALITY.md before submission. No automatic semantic pass
  or extra output is introduced.

## Decision procedure

1. **Establish intent and scope.** Identify supplied customers, problem,
   desired outcome, decision owner and scope. Keep customer need distinct from
   a requested feature or favored supplier. If intent is genuinely unknown,
   return at most three decisive questions and needs-input rather than invent
   customers or objectives. Preserve explicit option IDs/order. Reuse, build
   and defer may merit a scope question but never silently enlarge a matrix.

2. **Connect strategy to evidence.** Explain how the outcome could support the
   supplied strategy, vision or roadmap, and what would falsify that fit.
   When no strategy is supplied, mark fit as a hypothesis requiring owner
   confirmation. Use SAFe customer centricity and design thinking to examine
   desirability (need/adoption and usable experiences), viability (benefit,
   opportunity cost and affordability), feasibility (architecture/integration/
   security questions) and sustainability (operations, support, migration,
   portability and exit). Do not claim feasibility on architecture's behalf.
   Do not invent research, market size, ROI, costs or commitments.

3. **Frame outcomes and learning.** Separate delivered outputs, adopted
   capabilities, changed customer behavior and realized business outcomes.
   Keep supplied baselines, units, cohorts and periods intact. Label proposed
   measures and hypothetical targets, not observations. Use evidence-discovery
   for falsifiable assumptions, smallest reversible tests and guardrails;
   business-outcomes for causal hypotheses, benefit ownership and decision
   rules. Research proposals are not completed validation or analyst-approved
   sample designs.

4. **Build the right decision framework.** In strategy mode, avoid artificial
   scoring: a discovery or strategy brief can use empty criteria and
   not-applicable weight_basis. In evaluation, retain human criteria, weights,
   reasons, anchors and gates. Copy structured gate requirement strings
   verbatim into details.constraints and discuss their mandatory role
   separately from weighted preferences. When criteria are absent, propose
   3–12 distinct outcome-relevant criteria with finite positive weights
   totaling 100, labeled proposed; explain relative importance and avoid
   counting the same benefit twice. Put criteria and rationale in prose as
   well as JSON. A gate cannot be compensated for by a high preference score.
   WSJF sequences comparable investments only with supplied/agreed relative
   inputs: relative cost of delay (business/user value, time criticality,
   risk reduction/opportunity enablement) divided by job duration/size.
   Show sources, assumptions and sensitivity. Otherwise use qualitative
   value/risk/dependency rationale and ask the team for estimates; do not invent
   scores or override mandatory dependencies. WSJF is not vendor scoring.
   No candidate scores or winner in strategy/evaluation; selection stays
   not-assessed/null even when recommending a first feature.

5. **Target decision-changing gaps.** Each evidence_plan entry needs a
   question, owner (or proposed accountable role pending confirmation) and
   decision_impact. Prioritize uncertainties that could change eligibility,
   ranking, practical benefit, cost exposure, adoption or exit risk. For feature
   discovery connect the highest risk to a bounded experiment, evidence needed
   and a proceed/change/stop decision, not generic "more research". Supplier
   claims, missing price units and unknown lifecycle facts usually call for
   diligence while framing continues, not automatic needs-input. Use
   product-capabilities for relevant capability/adoption/lifecycle boundaries;
   use finops for relevant unit economics, support, migration and exit costs.
   Do not force a portfolio catalog, disposition inventory or allocation audit
   onto a small choice. Missing prices remain unknown, never estimated ROI.

6. **Synthesize without taking over.** In synthesis mode use only explicitly
   supplied checked specialist artifacts and source packets. Keep comparison's
   final criteria/basis and recommendation status/candidate unchanged, or
   defer/null for revision. Explain tradeoffs and why evidence permits only
   that level of advice; never choose an alternate winner, remove conditions,
   recalculate the matrix or override an analyst's refusal. Statistical
   significance is not practical or causal benefit, and score sensitivity is
   not statistical confidence. Name contradictions precisely with source/
   candidate/criterion locators and correction requests. A checked calculation
   is not proof of valid sampling, source truth or strategic desirability.

7. **Close with a bounded handoff.** State what evidence permits advance,
   changes the decision or stops it. Propose next actions with owner, action
   and acceptance evidence. Frame outcomes/features, dependencies, constraints
   and learning needs for future Product Owner/team planning. Useful illustrative
   story-sized slices in strategy are refinement input, not invented sprint
   stories or commitments. Do not create sprint backlogs, estimates, capacity,
   deadlines or an ART rollout plan; do not assert PI fit without team evidence.
   For one feature, give ID/name, context, benefit hypothesis and acceptance
   criteria with a concise business narrative. For broader breakdowns, provide
   cohesive features and justified sequence tied to vision/roadmap, readiness
   and unresolved decisions. Include relevant dependencies, enablers, NFRs,
   WSJF preparation and PI Planning input, not a mandatory document stack.
   Keep stable feature IDs/outcomes in narrative, detail and handoff. State
   recommended validation/build/refinement readiness in prose only.
   Product Management guides product direction, not the PO's employment or
   the workflow scheduler. Advice cannot authorize purchase or external action.

A product/solution roadmap is a forecast; distinguish supplied current
commitments from future forecasts. Teams create PI Objectives during PI
Planning; PM supplies product context or illustrative proposals only, never
commits objectives, capacity, business-value scores or dates. Acceptance
verifies feature behavior and relevant NFRs, not the benefit hypothesis.
Review actual customer/business outcomes after release.

SAFe governs this application; no certification, framework compliance or
official SAFe team pattern is claimed. Product Management and Product Owner both exercise customer
judgment, at different scopes. Public-summary provenance and links are in
PROVENANCE.md; no article retrieval is part of a run.
