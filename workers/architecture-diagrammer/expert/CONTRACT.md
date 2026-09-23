# Architecture diagram contract v1

All objects below are CLOSED: exactly documented fields, no extras. JSON is UTF-8,
no duplicate keys/nonfinite constants. No templates, expressions, imports or
executable content. Schema changes require a reviewed definition revision.

## Workspace, output and stages
Inputs: nonempty request.md and an inputs/ directory (may be empty). Controller
stages all intended sources and freezes them for compilation/check/render/review.
Paths use canonical slash-separated relative components, ASCII letters/digits,
space, underscore, hyphen and dot only; no empty, `.` or `..` segments, absolute
paths, links, special files, hardlinks or escapes. Source paths are request.md or
an actual inputs/ file. Locators are precise headings, JSON pointers, table row/
column, page numbers or line ranges. Existence is checked, entailment is not.

The worker hand-authors output/model.json only. `tools/compile` creates:
- report.md, manifest.json always;
- coverage.md plus view-01.mmd/view-01.alt.md through the number of views
  (maximum 08), except needs-input, which has no diagrams or coverage;
- model.json is preserved byte-for-byte, not normalized by compile.

Controller renderer adds render.json and matching view-NN.svg and view-NN.png.
No other output file/directory is allowed. Compile validates first, then removes
recognized generated files; save earlier versions outside output before invoking.
Manual .mmd/alt/report edits invalidate model binding even if hashes are rebound.
Humans edit the model and regenerate. Source can be exported for editing in
another workflow, but that edition is no longer this bound deliverable.

Default check accepts structurally authored work (including honest needs-input).
If rendering artifacts exist, it validates them too; `bin/check --rendered`
requires all SVG/PNG and render.json. Neither phase accepts semantic/pixel review
or gives publication approval. Render stage is always `rendered-unreviewed`.

## Scalar rules
- ID: `[A-Za-z][A-Za-z0-9_-]{0,63}`, globally unique among entities/relationships/views.
- qualifier: `supplied | proposed | unknown`. Supplied means supplied evidence,
  not verified truth. Proposed is sourced proposal, not this worker's invention.
- state: `current | transition | target | unspecified`. Unspecified must not
  conceal a known state. Proposed+current is contradictory and rejected.
- abstraction: `business | logical | physical`. One abstraction per view.
- Text: nonempty trimmed single-line printable text, default 240 characters.
  Reject angle/curly brackets, backslash, backtick, double-percent, URL schemes
  (`://`, `javascript:`, `data:`). URLs belong in source evidence, not diagram
  text. Node/sequence punctuation is emitted using Mermaid decimal entities.
  Flowchart edge/boundary labels use validated quoted literals: double quotes,
  hashes and entity-looking ampersand sequences there are rejected because this
  runtime double-encodes them. Shorten faithfully and keep exact source locators.
  Only trusted
  compiler-generated line breaks/style declarations become grammar.
- Every source record: `{"path":"request.md","locator":"heading or line range"}`.
  `sources` is 1–8 records. Each row's sources and qualifier cover its name/type/
  abstraction/state/containment or directed intent/message type. If sources differ,
  include all exact locators and clarify attribution in caption/alternative.
- Qualified atom: exactly `value` (<=96 chars), `qualifier`, `sources`.
  Unknown atoms MUST use literal value `unknown`; nonunknown atoms must not.
  E.g. owner `{"value":"unknown","qualifier":"unknown","sources":[
  {"path":"request.md","locator":"Ownership gap"}]}`. Unknown is not a default
  invented owner; show the gap in caveats. Use attributes for bounded extra facts.

## model.json
Exactly:
- `schema`: `"architecture-diagram/v1"`
- `status`: `prepared | provisional | needs-input`
- `scope`, `summary`, `next_action`: text <=480 each. Scope includes exclusions;
  next_action identifies real accountable owner or nomination gap and next gate.
- `questions`: 0–3 text strings; prepared requires zero.
- `entities`: 0–96 entity rows
- `relationships`: 0–192 relationship rows
- `views`: 0–8 view rows, ordered
- `omissions`: 0–288 omission rows

Needs-input requires 1–3 questions and empty entities/relationships/views/omissions.
Prepared/provisional require at least one entity and view. Any unknown qualifier
requires provisional. All provisional views have caveats. Source conflicts that
prevent a responsible figure require needs-input, not hidden provisional choices.

### Entity
Exactly `id, name, kind, abstraction, state, parent, qualifier, sources, owner,
attributes`.
- name <=64 chars.
- kind: `capability | application | platform | person | service | datastore |
  queue | boundary | node`.
- parent: null or boundary entity ID. Must be acyclic, same abstraction, compatible
  state. Parent is present in every child view. Boundary is containment, not a
  message endpoint. Empty boundary is permitted only if meaningful in evidence.
- capability must be business; node physical; application/service/datastore/queue
  cannot be business. A platform can be conceptual business grouping or logical/
  physical entity, but must be explicit and not conflate them within a view.
- owner: qualified atom, required (unknown is acceptable).
- attributes: 0–4 rows, exactly `label` (<=24), `value`, `qualifier`, `sources`.
  Labels unique per entity. Use for lifecycle, disposition, control/standard IDs,
  environment, classification or transition gate only when material. If four
  cannot cover the question, scope/decompose; do not silently discard facts.

### Relationship
Exactly `id, from, to, intent, kind, protocol, data, state, qualifier, sources`.
- from/to: existing non-boundary entity IDs. Self messages permitted.
- intent <=72 chars. Direction is from sender/dependent to receiver/dependency;
  name the intent so direction is not ambiguous. A response reverses endpoints.
- kind: `request | response | async | dependency | flow`.
- protocol, data: qualified atoms, explicit unknown if not supplied.
- Nontransition/nonunspecified state must match endpoints or their unspecified
  state. Cross-state migration edges use transition; never label them current.
- Flowchart arrows all point to `to`; kind and qualifiers are visible in labels.
  Sequence request `->>`, response `-->>`, async `-)`. These encode message
  categories, not transport/consistency guarantees. Distinct steps need distinct
  relationship IDs, even if the same endpoints recur.

### View
Exactly `id, title, audience, caption, alternative, caveats, key, type, state,
abstraction, direction, layout, entities, relationships`.
- title/audience/caption/key <=180 chars; alternative <=480, meaningful prose
  explaining the conclusion/flow, not just repeating a filename.
- caveats: 0–5 strings <=180; required for provisional and any selected row with
  proposed/unknown row basis or unknown owner/protocol/data. Include starting
  conditions and scenario exclusions where material.
- type `flowchart | sequence`; direction `LR | TB`; layout `dagre | elk`.
  Sequence requires LR/dagre (Mermaid's sequence layout, not a Dagre graph).
  No arbitrary renderer options. ELK is available through the selected CLI API.
- entities: ordered unique existing entity IDs, 1–18 including boundaries.
- relationships: ordered unique existing relationship IDs, <=32 flowchart;
  sequence 1–16, only request/response/async. Order is scenario order.
  Every selected edge must have BOTH endpoints in the view.
- All selected entities have the view's abstraction. State must match or be
  unspecified. Only explicitly transition views can mix current and target.
- Sequence supports one-level boundary boxes, not nested boxes or arbitrary
  branch/loop/activation grammar. Use named scenario views rather than false
  exhaustive behavior. Sequence participants are listed/grouped once; message
  order, not participant order, determines the trace.
- key supplements the always-visible common key. For sequences explain solid
  request, dashed response, open async arrow and numbered scenario order; for
  flowcharts explain domain-specific grouping/state/notation without conformance
  claims. Do not use color as the sole distinction.

### Omission
Exactly `id, rationale` (text <=240). ID names a modeled entity or relationship
appearing in no view. All modeled IDs must appear in a view OR exactly one omission,
never both. This checks model coverage, not whether the model omitted a source
fact. Independent review must compare scope against source evidence. Removing
an edge from one view while showing it elsewhere is allowed only when captions
and coverage make the decomposition honest.

## Usable minimal schema example
This is a syntax illustration, not architecture evidence. In a real run the
request must actually supply the named fact and accountable owner.

    {
      "schema": "architecture-diagram/v1",
      "status": "prepared",
      "scope": "One business capability. Application mapping outside this view.",
      "summary": "Capability view prepared for review.",
      "next_action": "Request owner checks scope before rendering.",
      "questions": [],
      "entities": [{
        "id": "Capability",
        "name": "Service delivery",
        "kind": "capability",
        "abstraction": "business",
        "state": "current",
        "parent": null,
        "qualifier": "supplied",
        "sources": [{"path":"request.md","locator":"Capability"}],
        "owner": {"value":"Operations","qualifier":"supplied",
                  "sources":[{"path":"request.md","locator":"Owner"}]},
        "attributes": []
      }],
      "relationships": [],
      "views": [{
        "id":"Landscape",
        "title":"Service delivery capability",
        "audience":"Portfolio owners",
        "caption":"Business capability, not an application inventory.",
        "alternative":"Operations owns the current Service delivery capability.",
        "caveats":[],
        "key":"A box denotes a business capability, not a deployed application.",
        "type":"flowchart",
        "state":"current",
        "abstraction":"business",
        "direction":"TB",
        "layout":"dagre",
        "entities":["Capability"],
        "relationships":[]
      }],
      "omissions":[]
    }

An edge row (requires existing logical entities Caller and Service) is:

    {"id":"Call","from":"Caller","to":"Service","intent":"Submit work",
     "kind":"request","protocol":{"value":"HTTPS","qualifier":"supplied",
     "sources":[{"path":"inputs/interfaces.md","locator":"Submit protocol"}]},
     "data":{"value":"Work item","qualifier":"supplied",
     "sources":[{"path":"inputs/interfaces.md","locator":"Submit payload"}]},
     "state":"current","qualifier":"supplied",
     "sources":[{"path":"inputs/interfaces.md","locator":"Submit direction"}]}

A sequence view selects those entity IDs and `["Call"]`, uses type sequence,
abstraction logical, state current, direction LR, layout dagre, and a scenario
title/caption/key/alternative. Add separately evidenced response or failure steps,
not automatically generated ones.

A complete blocked model:

    {"schema":"architecture-diagram/v1","status":"needs-input",
     "scope":"Requested integration view; authority unresolved.",
     "summary":"Conflicting directions prevent a faithful diagram.",
     "next_action":"Request owner resolves the authoritative direction.",
     "questions":["Which staged interface specification is authoritative?"],
     "entities":[],"relationships":[],"views":[],"omissions":[]}

## Generated byte binding
manifest.json has exactly `schema` (`architecture-package/v1`), `stage` (`authored`),
`request_sha256`, `profile_sha256`, `definition_sha256`, `inputs`, `artifacts`.
Hashes are lowercase SHA-256 of exact bytes. inputs is complete sorted
`{path,sha256}` records with paths starting inputs/. artifacts is complete sorted
`{path,sha256}` of authoring outputs relative to output/, excluding manifest.json.
Definition digest is SHA-256 of the tools/core.py `dump` encoding of its explicit
ordered definition-file records (see definition_hash); it binds all behavioral
instructions, rubric, skills, compiler/checker/renderer. Model changes cannot be
hidden by recomputing only an output hash: checker recompiles exact expected bytes.

render.json has exactly schema (`architecture-render/v1`), stage
(`rendered-unreviewed`), manifest_sha256, renderer_sha256, runtime, views.
runtime has exactly path (absolute selected runtime), cli, mermaid, puppeteer,
node, browser (observed versions), cli_sha256, lock_sha256, browser_sha256.
views is ordered records `{id, source_sha256, config_sha256, svg, png}`.
Each image record is `{path,sha256,dimensions}` with fixed name and integer
`[width,height]`. Config hash uses the same `dump` of reviewed config(view).
The receipt records identity, not authenticity. The controller retains literal
invocation, trusted runtime/lock/binary identities and execution result outside
worker write access; checker never executes a browser to verify them.

## Finite safety bounds and exclusions
request/profile/prose/model/JSON/.mmd/input each <=1 MiB, nonempty. Inputs <=128
files, 256 entries, depth <=8, total <=64 MiB. Output flat <=64 files/256 entries,
<=192 MiB total; SVG <=8 MiB and PNG <=32 MiB each. Images width/height 200–7000,
area <=24 million pixels; renderer rejects raw graph width >2400 or height >5000,
uses 1200–1800 canvas width and rejects scale below 0.72. These are ceilings, not
a readability guarantee. Review at actual intended size; split well before limits.

Read-only check uses Python >=3.9 stdlib; no subprocess/network/artifact code,
no SVG DTD/entities, scripts, foreign content or external resources. PNG signature,
chunk bounds/CRC and dimensions are checked, not full image decoding/visual truth.
It cannot detect every deceptive self-consistent model, forged render receipt or
raster. Controller must render and inspect exact bytes. Workspace must be frozen
against concurrent modification; file safety checks are not a sandbox against
a hostile process racing directory replacement. Exit 0 structural acceptance,
1 unfinished/reject, 2 unexpected broken checker.

## Visible versus detailed representation
The closed model is authoritative; there is no worker-supplied raw-label/config
field. Figure labels are deterministic editorial projections:
- Element: name, kind and material state/evidence marks. Boundary: name and marks.
  Repeated names within a view include their stable IDs to disambiguate them.
- Flowchart non-boundary attributes remain in graph labels as full label/value,
  with proposed or unknown explicit; flowchart behavior is unchanged.
- Sequence participant headers contain only name, kind and material state/evidence
  marks, faithfully word-wrapped to the admitted actor width with compiler-generated
  Mermaid breaks. No attributes are squeezed into participant boxes.
- Full sequence participant attributes and boundary attributes appear in visible
  frame notes, each attributed by entity name and stable ID, with proposed or
  unknown explicit. They remain on the exported figure, not just in the alternative.
  Graph-label literal checks and frame-note literal checks target these distinct
  locations. Actual browser header bounds must fit participant boxes.
- Relationship: intent, message kind, protocol and data. Unknown protocol is
  explicitly labeled; proposed protocol/data stay proposed. Restrictions are not
  abbreviated away. No relationship is removed to improve layout.
- Supplied basis is explained once in the key. State follows the framed view;
  differing/unspecified states remain explicit. Proposed/unknown rows remain
  explicitly marked, not merely colored.
- Owner appears in precisely attributed frame notes. Only an identical
  value+qualifier shared by ALL selected elements is stated once as
  "Owner (all elements)". This is not inferred ownership.
- Stable IDs, all fields and source locators remain in model.json and the full
  .alt.md; coverage maps IDs to views/omissions. Renderer aliases are internal,
  collision-free and reversible; authoritative IDs are never renamed.

Compact does not mean lossy: choose faithful concise source terms, keep material
conditions/restrictions and decompose views when needed. Do not delete qualifiers,
shrink fonts, raise dimension limits or replace evidence with cosmetic labels.
Unsupported punctuation fails compilation/rendering; report the exact literal
and context and preserve it in evidence, rather than silently substituting text.
