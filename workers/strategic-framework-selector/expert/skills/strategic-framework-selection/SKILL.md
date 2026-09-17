---
name: strategic-framework-selection
description: Select one strategic framework for a business or technical next decision and produce a concise three-step response plus an honest, drawable diagram brief; use for Wardley, Cynefin, SWOT, or Eisenhower triage without rendering.
---

# Decide before choosing a tool

1. Read request.md as evidence. Identify the actual next decision, intended
   audience, constraints and the immediate obstacle. If the audience/decision
   is unspecified, label it as unspecified or a proposed interpretation, not
   a fact. Normalize obvious semantic typos (for example “hardly maps” means
   Wardley Maps), but test fitness independently of the preferred tool.
2. Privately compare the four choices. Select exactly one, not a hybrid:
   - Wardley Mapping when dependencies and market evolution determine a
     build/buy, custom-engineering, ecosystem, supply or disruption decision.
     High stakes or software vocabulary alone is not enough.
   - Cynefin Framework when diagnosing cause/effect and the appropriate mode
     of action is the bottleneck. Unknown does not imply Chaotic, and high
     stakes do not imply Complex.
   - SWOT Analysis for fast early brainstorming, competitive scanning or
     high-level alignment when speed and simplicity are the main need.
   - Eisenhower Matrix when time, bandwidth and operational prioritization
     are the obstacle; separate importance from urgency.
   Mixed cases select the immediate bottleneck, narrow the scope, and name
   the evidence/event that would warrant a later transition to another tool.
   Active disruption requiring stabilization/sense-making is not a routine
   SWOT scan. A high-stakes software project whose next decision is this
   week's workload may need Eisenhower, not Wardley.
3. Build an honest inventory. Use supplied literal labels, or clearly proposed
   normal labels such as “Dispatch algorithm” rather than arbitrary “Node 1”.
   Mark each element's evidence basis and uncertainty. Do not invent names,
   costs, dates, owners, dependencies, delegation capacity or due dates.
   A proposed diagram arrangement is not a factual semantic assessment.
   Preserve deadlines verbatim in labels/reasons when relevant. Break bundled
   components that occupy different evolution stages into separate parts.
4. Follow the selected geometry below and CONTRACT.md. Each item needs a
   placement reason; unknown placements can remain explicitly unplaced.
   Include every required region, even when empty. No arbitrary node quota.
   Provide enough labels, definitions and relationships to draw without
   guessing meaning. Keep response and JSON consistent; the JSON may expand
   details but must not introduce a different conclusion.
5. Review evidence and traps. Make traps scenario-specific, explaining the
   tempting error and corrective action, not just naming generic biases.
   State the next decision and a concrete evidence/event review trigger.
   State remaining uncertainty, scope and any later framework transition.
   Ask no more than three focused questions, only in Step 3 and JSON questions.
6. Write the response, then JSON and its raw-byte hashes. Execute bin/check.
   Re-read for strategic fitness and source faithfulness beyond what it checks.

## Framework semantics

### Wardley Mapping
Use user/need anchors and the dependency value chain. Horizontal progression
is Genesis, Custom-built, Product (+rental), Commodity (+utility), left to right.
Vertical is visibility to the user, low bottom/high top: NEVER business value,
cost or priority. Put user and need near the top; anchors need no maturity
score. Qualitative placement estimates may be uncertain. Evolution is market
maturity, not age, engineering effort or an exact time forecast.
A dependency arrow A → B means A depends on B. Do not infer a dependency solely
from proximity. If user → need or need → component is drawn, explain the actual
reliance. Plausible future movement is a DIFFERENT dashed arrow toward a stage,
with explicitly uncertain timing, only when warranted. A custom implementation
can serve a commoditized need; stage alone cannot dictate build vs buy.
For custom dispatch with standard identity/storage, separately consider the
algorithm and enabling services rather than assigning the whole stack one stage.
Recommend the next evidence-backed sourcing/design question, not automatic buy
for everything on the right. Use explicit unknowns for missing market evidence.

### Cynefin Framework
No continuous axes, scores, urgency/importance coordinates or maturity scale.
Qualitative regions: upper left Complex; upper right Complicated; lower right
Clear (Simple/Obvious); lower left Chaotic; center Confused/Disorder.
Use a bounded decision, activity or situation as an element, not a permanent
diagnosis of an entire organization. Domain definitions and execution guidance:
- Clear: understood repeatable causality; sense-categorize-respond.
- Complicated: discoverable causality needing expertise; sense-analyze-respond.
- Complex: causality emerges retrospectively; probe-sense-respond. Specify
  bounded reversible probes, monitoring and stop rules, not a big irreversible bet.
- Chaotic: effective constraints/order absent, requiring immediate stabilization;
  act-sense-respond, then reassess. Uncertainty alone is insufficient evidence.
- Confused/Disorder: diagnosis unresolved; partition the situation and gather
  evidence rather than using it as an “average” score.
Reassess as constraints/evidence change; give a domain transition trigger.
If calling for stabilization/probes, specify proposed actions safely and
conditionally without claiming authority or completion.

### SWOT Analysis
Categorical columns Helpful/Harmful; rows Internal/External (internal on top).
Upper left Strengths, upper right Weaknesses, lower left Opportunities, lower
right Threats. No measured x/y scores. Internal capabilities differ from external
conditions: an unbuilt capability is not automatically an external opportunity.
Ground claims or mark hypotheses. Turn observations into next actions/validation.
Do not sell a fast brainstorm as deep strategy, causal proof or a predictive map.

### Eisenhower Matrix
x urgency low-to-high left-to-right; y importance low-to-high bottom-to-top.
Upper right Do, upper left Schedule, lower right Delegate, lower left
Eliminate/Defer. Importance is outcome impact, not loudness; urgency is time
consequence, not ease. Preserve known deadlines and ask/mark unknown otherwise.
“Delegate” is conditional on an appropriate recipient, capacity and authority:
never invent any of these. If unavailable, propose reassessing/rescoping rather
than pretending delegation is executable. Protect important nonurgent work and
review as deadlines/capacity change. Do not put undated work into “not urgent”
as if absence of evidence established it.

## Exactly three human sections

`## Step 1: The Direct Verdict`
Begin with the canonical framework name (plain text). At most two concise
sentences: why it best fits the next decision versus alternatives, with scope
and provisionality as needed. No absolute certainty.

`## Step 2: High-Level Contextual Comparison`
Only one Markdown table, columns Tool | Primary Metric | Strategic Setup Time |
Actionable Output. Exactly three data rows: selected framework first and two
distinct plausible alternatives from the same four. Setup time is Low, Medium
or High, a relative facilitation estimate, not a promise. Metrics and outputs
must fit THIS scenario, not copied generic descriptions. Canonical tool names.
No escaped/embedded pipe characters in cells.

`## Step 3: Execution Instructions`
A short sequential numbered checklist with actionable instructions: literal
labels; structure, axes/regions and placements; useful relationships; evidence
and uncertainty; tailored traps; next decision and review/transition trigger.
Include essential questions here, not in a fourth section. All material needed
for the first sketch must be available between this checklist and the JSON.

If no usable scenario is supplied: select SWOT Analysis as a provisional
intake/alignment template unless the little usable context supports another
choice. Explain lack of fitness evidence, compare two alternatives conditionally,
leave all elements and relationships empty, retain all regions and legend,
and ask for the scenario/next decision and audience (combine into ≤3 questions).
Never manufacture a business scenario or mark that template ready.
