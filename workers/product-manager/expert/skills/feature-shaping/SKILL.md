---
name: feature-shaping
description: Shape evidence-led product features and minimum useful slices in strategy mode, with benefit hypotheses, verifiable acceptance, outcome measures and PO/team refinement handoffs. Use for feature definition or sequencing, not vendor selection or committed sprint planning.
---

# Shape SAFe ART backlog features

SAFe governs feature purpose and role boundaries. Product Management owns
customer/market needs, vision/roadmap and ART backlog/features in collaboration
with System Architect, RTE and Business Owners; Product Owner/team owns team
backlog and actual stories/planning. Use customer centricity and design thinking,
not an external product framework or mandatory document hierarchy.

Use with product-management and CONTRACT.md. Feature shaping is a **strategy**
use case. Put narrative and feature detail in output/response.md; existing JSON
holds the decision, outcomes, evidence plan and delivery handoff. Do not add
feature arrays, readiness fields or outputs. Feature sequencing never changes
selection: keep status not-assessed and candidate_id null. Do not turn features
into vendor candidate IDs or vendor criteria. Preserve any supplied scope.

## Find the problem worth solving

Distinguish the vision and business outcome (desired change for a population),
customer need (evidenced problem in a situation), and proposed solution. A requested portal, automation or dashboard is not itself an
outcome. Constructively acknowledge its intent, trace it to the evidenced
problem and compare a smaller intervention, reuse or defer where relevant.
Do not merely reject a solution-first brief or invent a need to justify it.

Cite actual source IDs/locators for the current problem. Separate observed
facts, indications, hypotheses and unknowns. If intent is known but evidence
is thin, recommend validation rather than pretend build readiness. If customer,
problem or outcome intent is genuinely unknown, follow AGENTS.md's needs-input
rule. Record a benefit hypothesis: for this user in this situation, this change
should enable this behavior and improve this outcome because of this mechanism.
Name the assumption that could break that causal chain.

## Describe selected features proportionally

Retain supplied stable feature IDs, or assign neutral IDs such as F-01 and
keep them stable through narrative, detail and the JSON handoff. Define only
the features useful to the decision; one compact paragraph may suffice for a
single-feature request: provide the feature and concise business narrative.
The SAFe starting point for each feature is **ID/name, context, benefit
hypothesis and acceptance criteria**. The benefit hypothesis proposes a
measurable customer/business benefit; it is not realized value. Acceptance
describes correct functionality and relevant NFRs.

For broader breakdowns, provide a cohesive feature set and justified sequence
tied to vision/customer need/business outcome and the product/solution roadmap,
with readiness and unresolved decisions. Use a roadmap forecast only when
helpful; distinguish supplied current commitments from future forecasts.
ART Kanban manages feature flow; report actual flow state only if supplied.

Add only relevant supporting detail from the prompts below, proportionate to
the decision, not a custom mandatory artifact stack or checklist:

- User/consumer and triggering situation or job.
- Current problem, its evidence and material evidence gaps.
- Changed behavior and expected user/business value; the benefit hypothesis.
- Scope and non-goals, including who or what is deliberately excluded.
- A vertical **minimum useful slice**: a narrow end-to-end task yielding usable
  value or decision-changing learning, not just a UI, database or integration
  layer. State any enabler needed to make it usable.
- Observable feature acceptance: preconditions, action and expected observable
  result; important negative/failure conditions and safe recovery. Cover the
  salient unauthorized, invalid, unavailable or partial-failure paths rather
  than boilerplate. "Easy to use", "API built" or "tests pass" is insufficient.
- Outcome success measure, unit, population, observation window, collection
  source/method and proposed measure owner. State baseline and target status
  independently: supplied with locator, explicitly proposed pending agreement,
  or unknown. Do not manufacture numbers to fill a definition.
- Dependencies, enablers and relevant NFRs/guardrails; distinguish a necessary
  prerequisite from a preferred ordering. Unknown thresholds need validation.
- Highest-risk assumptions and the next validation that changes a decision:
  evidence to collect, proposed owner and proceed/change/stop implications.

Feature acceptance answers **does it work**; an outcome measure answers
**does it matter**. A successfully completed workflow does not prove reduced
support cost or better retention. Link proxy measures to the hypothesized
benefit, preserving attribution limits and possible adverse effects.

## Scope and sequence without invented commitments

Use SAFe terms with their intended purpose. A feature describes stakeholder value and is normally
sized for one ART within a PI; a capability is broader and can span ARTs,
refining into features. An epic is a larger investment hypothesis needing
further analysis/decomposition, not just a big feature label. An enabler
supports future value (for example architecture, exploration or infrastructure);
make its consuming feature/outcome and acceptance explicit. Do not force every
request into epic/capability/feature/story levels. A durable product capability
is not automatically a formally scoped SAFe capability.

No claim that work fits a PI, team capacity or delivery date without supplied
team evidence. Unknown fit is a refinement question. If useful, offer a few
illustrative story-sized vertical slices linked to the feature ID, explaining
the user task and observable result. These are PO/team refinement input only,
not invented sprint stories, a committed backlog, estimates or acceptance by
the team. The PO and team own actual story refinement and planning.

Sequence by qualitative value, risk, learning, dependencies and displaced work
unless comparable supplied/agreed relative WSJF inputs exist. Only then show
relative cost of delay (value + time criticality + risk reduction/opportunity
enablement) divided by positive relative job size/duration. State input sources,
scale comparability, assumptions, ties and sensitivity to plausible changes;
if ranges are not agreed, state which input/order would need testing rather
than invent a range. Mandatory requirements and hard dependencies are not
overridden by a quotient. Without inputs, do not generate scores or treat
unknown size as zero; give qualitative rationale and request team estimates.
WSJF is not ROI, vendor scoring, or funding authority.

## Discovery and handoff

Use SAFe **desirability, viability, feasibility and sustainability**, customer
centricity and design thinking to guide discovery. Choose the
decision-changing risk, not a generic call for more research. For example, a
task observation tests the problem, a prototype task test probes usability,
a bounded technical spike probes feasibility, and an accountable cost/support/
lifecycle review probes viability and sustainability. Tie the test to the feature ID, evidence
needed, guardrails and the decision it can reverse. Do not claim those tests
ran, statistical sufficiency, or architecture approval.

Hand off selected IDs/outcomes, unresolved assumptions, acceptance and team
refinement needs in existing details.delivery_handoff and next_actions.
Include relevant PI Planning input: product vision, feature intent/priorities,
dependencies, enablers/NFRs, evidence and unresolved decisions. Teams create PI
Objectives during PI Planning; provide product context or clearly illustrative
proposals, never PM-committed objectives, capacity, business-value scores or
dates. Use leadership-narrative for the business decision and explicit ask;
state validation/build/refinement readiness in prose, not a schema extension.
Review actual outcomes after release against the hypothesis, with attribution
limits; acceptance alone cannot establish realized customer/business benefit.
