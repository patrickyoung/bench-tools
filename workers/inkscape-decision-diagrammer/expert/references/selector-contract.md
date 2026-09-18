# bench.diagram-brief/v1

Only job input: regular, non-symlink UTF-8 `request.md`, 0–1,048,576 bytes.
Never edit it. No usable scenario is an intake case, not grounds to invent one.
Artifacts: regular, non-symlink UTF-8 `output/response.md` (1–262,144 bytes) and
`output/diagram-brief.json` (1–1,048,576 bytes). `output` must be a real directory,
not a symlink. The checker reads only these fixed paths and ignores stdin.
There are no path/URL execution fields. JSON object keys are unique; NaN,
Infinity and unknown fields are forbidden. IDs match `[a-z][a-z0-9_-]{0,63}`.

## Top-level object (all and only these fields)
- `schema`: `"bench.diagram-brief/v1"`.
- `request_sha256`, `response_sha256`: lowercase 64-hex SHA-256 of the raw
  corresponding file bytes, including whitespace/newlines, not decoded text.
- `framework`: exactly `"Wardley Mapping"`, `"Cynefin Framework"`,
  `"SWOT Analysis"` or `"Eisenhower Matrix"`.
- `status`: `"ready"`, `"provisional"` or `"needs-input"`. Ready is handoff
  ready for review, not certified correct or already rendered.
- `title`, `audience`, `decision`: nonblank strings. Label absent audience or
  proposed decision explicitly; do not invent identities.
- `uncertainty`: object with only `assumptions` and `unknowns`, each an array of
  nonblank strings. These distinguish assumed facts from missing evidence;
  empty arrays are permitted when justified. Questions do not replace unknowns.
- `questions`: array of 0–3 nonblank focused question strings, also in Step 3.
- `layout`: object defined below.
- `elements`: array of element objects defined below, no arbitrary count quota.
- `relationships`: array defined below; `[]` is valid.
- `legend`: nonempty array of nonblank strings explaining evidence/uncertainty
  notation and any arrow semantics; say that layout is proposed. Explain the
  unplaced tray if used.
- `traps`: nonempty array of nonblank scenario-specific trap/corrective-action
  strings. For intake, describe conditional pitfalls rather than invented facts.
- `review`: object with only nonblank `next_decision`, `trigger`, `scope` and
  `transition` strings. Transition names the later tool and its condition, or
  explicitly says no transition currently warranted and why.

Needs-input requires empty elements AND relationships, at least one unknown
and one question. Its prose verdict/comparison is explicitly provisional,
with an unpopulated template. Ready requires at least one element. Uncertain
placements must not silently become known when changing status.

## Layout
All layouts have exactly `orientation` (`"landscape"`), `axes`, and `regions`.
Layout/position choices are drawing proposals, not observed facts. Regions
are an array in the order below, containing exactly `id`, `label`, `position`,
`definition` (a nonblank original explanation). IDs/labels/positions are literal.
The definitions must explain meaning, not merely repeat the label.

### Wardley Mapping
Axes exactly:
    {"mode":"qualitative",
     "x":{"label":"Evolution","direction":"left-to-right",
          "values":["Genesis","Custom-built","Product (+rental)","Commodity (+utility)"]},
     "y":{"label":"Visibility to the user","direction":"bottom-to-top",
          "values":["Low","High"]}}
Regions (`id | label | position`):
- genesis | Genesis | column-1
- custom-built | Custom-built | column-2
- product | Product (+rental) | column-3
- commodity | Commodity (+utility) | column-4

Definitions describe market maturity, never age or build/buy prescriptions.
y is visibility, never business value/priority/cost; no invented numeric scores.

### Cynefin Framework
Axes exactly: `{"mode":"no-numeric-axes"}`. This means qualitative regions,
not hidden continuous dimensions.
- complex | Complex | upper-left
- complicated | Complicated | upper-right
- clear | Clear (Simple/Obvious) | lower-right
- chaotic | Chaotic | lower-left
- confused | Confused/Disorder | center

Definitions include causal/constraint diagnosis and the corresponding action:
probe-sense-respond; sense-analyze-respond; sense-categorize-respond;
act-sense-respond; or partition/diagnose unresolved situations, respectively.

### SWOT Analysis
Axes exactly:
    {"mode":"categorical",
     "x":{"label":"Effect","direction":"left-to-right","values":["Helpful","Harmful"]},
     "y":{"label":"Origin","direction":"top-to-bottom","values":["Internal","External"]}}
- strengths | Strengths | upper-left
- weaknesses | Weaknesses | upper-right
- opportunities | Opportunities | lower-left
- threats | Threats | lower-right

Definitions distinguish internal capability/limitation from external condition.

### Eisenhower Matrix
Axes exactly:
    {"mode":"qualitative",
     "x":{"label":"Urgency","direction":"left-to-right","values":["Low","High"]},
     "y":{"label":"Importance","direction":"bottom-to-top","values":["Low","High"]}}
- do | Do | upper-right
- schedule | Schedule | upper-left
- delegate | Delegate | lower-right
- eliminate-defer | Eliminate/Defer | lower-left

Definitions describe urgency/time consequence and importance/outcome impact.
Delegate is conditional on recipient/capacity/authority. No numeric scoring.

## Elements
Each has exactly:
- `id`: stable semantic ID, unique across elements, regions and relationships;
  keep it stable across revisions of the same scenario.
- `label`: literal nonblank text for the diagram, normal human labels.
- `role`: Wardley uses `"user"`, `"need"` or `"component"`; others use `"item"`.
- `placement`: object with `region` (region ID or null). Wardley ALSO requires
  `visibility`: `"high"`, `"medium"`, `"low"` or `"unknown"`; no other fields.
  Null means an explicitly unplaced item outside the classified regions.
  Wardley user/need anchors may have null maturity without being uncertain;
  all other null placements require `uncertain: true`. Unknown visibility
  also requires uncertainty. A populated Wardley map includes user and need
  anchors (descriptive “User (unspecified)” is allowed if marked unknown).
- `basis`: `"supplied"`, `"inferred"`, `"proposed"` or `"unknown"`.
- `evidence`: nonblank short quotation or faithful input pointer/paraphrase,
  or an explicit explanation of absent evidence/proposed design.
- `reason`: nonblank placement rationale, including any conditions.
- `uncertain`: boolean; unknown basis requires true. Supplied existence does
  not prove placement, so a supplied element may still be uncertain.

No absolute coordinates: qualitative region plus Wardley visibility is enough
to place items without inventing measured scores. Resolve collisions visually,
not by introducing new semantics. Unplaced items are not assigned fake regions.

## Relationships
Each has exactly `id`, `from`, `to`, `type`, `direction`, `meaning`, `style`,
`uncertain`, and `timing`.
- `id` follows the same global uniqueness rule. `from` is an element ID.
- `type`: `"dependency"`, `"movement"` or `"association"`.
- `direction`: `"source-to-target"` or `"undirected"`.
- `meaning`: nonblank evidence-grounded meaning, or marked inference/proposal.
- `uncertain`: boolean.
- `style`: `"solid"` or `"dashed"`.
- `timing`: null except movement, where it is exactly `"uncertain"`.

Dependency: Wardley only, `to` is a DIFFERENT element ID, solid,
source-to-target, meaning **from depends on to**. Anchor dependencies require
evidence/reason just like component dependencies.
Movement: Wardley only, `from` is a component, `to` is a different evolution
region ID, dashed, source-to-target, uncertain true with uncertain timing.
It is a proposed possible future stage, NOT a dependency or forecast.
Association: `to` is a different element ID; direction can be either;
solid/dashed as explained in legend, with timing null. In Wardley do not use
association to conceal a dependency. Do not invent connections for matrices.

## Human response validation
Exactly the three headings from the skill, in order, with no preamble or other
headings (including setext headings or standalone separators). Verdict starts with the plain canonical framework name, at most two
concise sentences. Step 2 contains only the four-column Markdown table:
Tool | Primary Metric | Strategic Setup Time | Actionable Output.
Exactly three distinct tool rows, selected first, each time Low/Medium/High.
response.md contains at most 400 whitespace-delimited words, counting headings,
numeric markers and the table (including standalone Markdown tokens). Aim for
250–350 words, not padding; there is no minimum word count.
Step 3 contains 4–9 sequential `1.`, `2.`, … numbered checklist steps; a
`needs-input` intake may use 3–9. Each step contains at most 45 whitespace-delimited
words excluding its numeric marker. All text after that marker up to the next
step or end of section, including wrapped continuations, counts toward that
step. Counts use Python's Unicode whitespace `str.split()` semantics.
Give each step one clear action, combining only tightly related actions.
No fourth appendix. Do not put other Markdown tables in the response.

Preserve the decision, correct axes/quadrants, key literal labels and placements,
scenario-specific traps, genuine uncertainty and review trigger. Keep exhaustive
elements, evidence, edge definitions, uncertainties and drawing directions in
diagram-brief.json rather than repeating them in prose. Use ordinary language.
Do not invent process blockers: a suitable helper's supplied agreement supports
a proposed delegation unless a concrete conflict exists. Retain real unknowns
without burying the plan; the worker never performs the handoff.

The checker enforces file safety, shape, identity bindings, geometry, references,
response structure, total/per-step word ceilings and checklist length. It does
not infer whether prose is true, punchy,
unbiased, contextual, or exactly two linguistic sentences; it cannot certify
strategy, evidence sufficiency, semantic parity of prose/JSON, or drawing quality.
Operator review uses the behavioral rubric in README.md.
