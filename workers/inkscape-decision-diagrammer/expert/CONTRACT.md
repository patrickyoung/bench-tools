# inkscape-decision-diagrammer contract

## Inputs and trust
Only `request.json` and optional `selector/{diagram-brief.json,request.md,response.md}`
are job input. UTF-8 regular files, no symlinks (including ancestors). Request
≤65,536 bytes; required `brief` nonblank string ≤48,000 characters. Only other
keys: integer width/height in [640,4096], defaults 1600/1000; boolean dark;
nonblank owner/date/palette strings ≤2,000 characters. Duplicate keys, NaN,
Infinity, unknown fields and executable input fields are rejected.
“Run …” inside brief or a label is retained data, never executed.
Dark explicit true/false wins; else case-insensitive dark-mode/dark mode,
dark-deck/dark deck, slide/slides triggers it.

Selector directory, if present, contains exactly all three files. JSON/request
≤1 MiB, response ≤256 KiB. `references/selector-contract.md` is the original
upstream contract. This consumer validates schema, exact shape, axes/regions,
IDs/references/uncertainty and raw SHA-256 bindings. It retains human response
as bound evidence; it does not regrade that upstream worker's editorial
heading/word-count contract or certify prose/JSON parity. Read all evidence.
Malformed input is unfinished; a valid needs-input handoff becomes a complete
semantic conflict report. A new request conflicting with selector evidence
must not silently override it.

## Output package
All output files are immediate children of the real directory `output/`.
No subdirectories/symlinks, ≤24 files, each ≤64 MiB, total ≤200 MiB.
SVGs additionally ≤16 MiB; JSON and notes ≤1 MiB.
Required rendered package:
- diagram.inkscape.svg: editable live text, named layers, grouped logical nodes.
- diagram.svg: native Inkscape plain export, zero live text, no raster.
- diagram.png: native exact requested pixels, final-export render.
- layout.py: per-job deterministic stdlib geometry; never invoked by checker.
- diagram-plan.json: schema below.
- design-notes.md: decision, frame, evidence, choices and review limits.
- render.log: literal argv, stdout/stderr/outcomes, all real query rows.
- render.json: native identity, successful command sequence, queried bounds,
  current raw input + every pre-receipt output SHA-256.
- result.json: bound status, worker identity, exact file inventory and hashes.
Dark adds diagram-dark.inkscape.svg, diagram-dark.svg, diagram-dark.png, only
when triggered. Master XML structure/coordinates/text/typography must be equal;
only semantic paint/opacity tokens may differ. Independently check each variant.
Optional layout-inputs.json contains the frozen non-executable data needed by
layout.py. No arbitrary filenames or code imports.
`finish --previews` adds thumbnail.png (native 320-wide render) and grayscale.png
(deterministic integer Rec.709 luma RGBA conversion), plus dark-thumbnail.png /
dark-grayscale.png for dark. They are support artifacts, not substitutes for
native core exports, and are independently checked.

Notes require these headings:
`## Decision`, `## Framework`, `## Axes and regions`, `## Items and evidence`,
`## Audience and medium`, `## Omissions`, `## Style`, `## Review`.
Include specific source pointers/assumptions and uncertainty; all axis
directions/actions; unknown tray rationale; cuts/clusters with membership;
quantitative units/formulas; grid exceptions; tokens/type/contrast surfaces;
owner/date (unspecified if absent); remaining human review tasks. Include
“Visual review pending”. Do not infer viewing from CLI checks.

## Geometry plan v1
JSON schema string: `inkscape-decision-diagrammer.plan/v1`.
All top-level keys required, no extras:
- schema; framework (canonical spelling from framework reference); grammar
  (nonblank actual rules/evidence); decision; audience; axes (nonblank exact
  meaning/direction in this decision).
- regions: ordered list of `{id,label,box,meaning}`. `box=[x,y,width,height]`
  is a positive rectangular ALLOCATION, not a mandate to draw a box. Retain
  empty regions. For Cynefin the actual boundary is soft, allocation can
  still be rectangular. For BCG IDs stars,cash-cows,question-marks,dogs.
- tray: null or box outside classified regions, explicitly labeled.
- visibility_bands: Wardley `{high:box,medium:box,low:box}`, vertically ordered,
  elsewhere `{}`. These are layout bands, not invented numeric measurements.
- items: `{id,label,source_ids,evidence,assumption,region,visibility,x,y,
  group,mark,label_id}`. ID `[a-z][a-z0-9_-]{0,63}`. Nonempty source_ids and
  evidence, boolean assumption (prominently label if true). Region semantic ID
  or null (tray); Wardley visibility high/medium/low/unknown, elsewhere null.
  x/y are actual mark bounding-box center in final units, not guessed label
  bounds. group,mark,label_id are unique meaningful SVG IDs; group must be
  directly in marks layer, containing mark AND live text. With selector,
  source coverage is exact and single-source labels/placements literal.
- relationships: selector relationship grammar verbatim, including ID, from,
  to, type, direction, meaning, style, uncertain and timing. Sources are source
  IDs. No invented relationships. Each has an actual path with its ID.
  Wardley dependency paths use absolute M/L/Q/C, attach to a rect/circle mark,
  downward source→target, solid local marker-end. No transformed dependencies
  or marks; use explicit coordinates. Movement dashed, uncertain timing.
- omissions: list `{source_ids:[...],reason}`; no displayed source also omitted.
  Clusters use items.source_ids, evidence/reason in notes. Each source exactly
  once across shown/omitted. Connected selector items cannot be clustered/cut.
- selector: complete validated selector object as DATA, else null.
- math: `{}` unless applicable; arithmetic contracts below.
- geometry: `{anchors,contains,disjoint,overlaps,contrast}`.
  anchors list `{id,x,y}` computed structural anchors divisible by 8.
  contains list `{inner,outer,padding}` SVG IDs; native text extents must fit.
  disjoint list `[id1,id2]`; native bounds must not intersect.
  overlaps list `{ids:[id1,id2],reason}` only for intentional SEMANTIC overlap
  (e.g. overlapping area bubbles), not a bypass for colliding labels.
  All independent live-text pairs are checked automatically; parent/child
  containment isn't a pairwise collision. No blanket mark collision test.
  contrast list `{foreground,background,paint:"fill"|"stroke"}` covers every
  live text and meaningful mark. Actual inherited SVG paint, native bounds,
  draw order and declared opaque rectangular background are checked. Flat
  explicit #rrggbb surfaces avoid ambiguous compositing. Place labels outside
  bubble/curve regions where necessary; use the actual containing card/page.
  Curved shapes/occlusion still need human review. Do not falsely declare the
  page as surface when another fill is underneath the label.
- style: `{font,sizes,weights,margin,tokens,semantic_channels}`. font e.g.
  `"Arial, sans-serif"`; ≤3 sizes, min11; ≤2 weights; scaled margin.
  tokens maps semantic name to `{light:"#rrggbb",dark:"#rrggbb"}`. Include
  neutral ramp/accent. semantic_channels is a list of descriptions for
  extra meaningful hues and their non-color redundant encoding.

Plan values are drawing proposals, not observed quantities. Checker verifies
declared mark centers and labels against actual SVG/native bounds, formula
arithmetic and source bindings. It cannot recover the author's intent from
numbers or prove a source claim. Unit/format errors are not license to guess.

## Framework arithmetic (required keys when applicable)
Numbers finite, only supplied or explicit assumptions; evidence strings on
every numerical input row. Tolerance 1e-7 absolute/relative, never normalization.
IDs below correspond to plan items. Show the relevant numbers/units in SVG too.
- Decision tree: `math.tree={root,nodes}`. Each node
  `{id,kind:"decision"|"chance"|"terminal",branches,value,evidence}`.
  Branch `{to,probability,evidence}`; probability null on decisions. Terminal
  branches empty; value payoff or null if no EV claim. Chance probabilities
  each [0,1], sum1; nonnull chance value exactly sum(p×child.value), all child
  values then required. Decision value null; policy/choice rationale separately.
  Connected unique-parent tree, max4 node levels, ≤16 terminal leaves.
- BCG: `math.bubbles={unit,denominator,growth_threshold,share_threshold,scale,entries}`.
  Entry `{id,revenue,radius,share,growth,category,evidence}`.
  Positive revenue/radius; πr²=scale×revenue; radius checked against actual
  circle. Share/growth threshold comparisons include equality in “high”.
  Category/region and conventional orientation checked.
- Options × criteria: `math.matrix={options,criteria,totals,score_min,score_max,
  weight_basis}`. options list ≤5 names. Each criterion `{label,weight,scores,
  weighted,evidence}`, ≤8 rows, arrays in option order. Nonnegative weights,
  positive sum; weighted[j]=weight×scores[j], totals sum contributions; scores
  in declared inclusive range. No automatic normalization or default weights.
- MoSCoW: `math.scope={unit,entries,bands,total_count,total_effort}`.
  Entry `{id,band,effort,evidence}`, nonnegative effort. bands has exactly
  must/should/could/wont, each `{count,effort}`. Counts/effort sum correctly;
  item regions match, Won't is included in total scope accounting.
- RACI / DACI: `math.accountability={mode,roles,rows}`, mode RACI or DACI.
  Roles unique names in column order. Row `{id,cells,evidence}`; cells a
  single code from mode's letters or "" per column. Exactly one A per row,
  ≤12 rows. A means Accountable (RACI), Approver (DACI).
- Force-field: `math.forces={scale,unit,entries}` with
  `{id,weight,extent,evidence}`; positive weight/scale, extent=scale×weight.
  State whether extent is arrow length or width in grammar/notes.

## Source and output binding
After writing notes, frozen layout-inputs and plan, layout.py hashes the actual
bytes of each; hashes itself; and includes current raw request/selector hashes.
Put the flat map `{workspace_relative_path:lowercase_sha256}` in a direct SVG
`<desc id="source-bindings">JSON</desc>` in each master. XML-escape through an XML
library. Keys include request.json, optional all selector files, and
output/layout.py, output/diagram-plan.json, output/design-notes.md, optional
output/layout-inputs.json. No masters/receipts in this map (no circular hash).
The retained geometry inputs may store the frozen input bindings so a copied
output directory can reproduce masters without reaching into the old workspace.
Checker recomputes from ACTUAL current files and compares the embedded master
map, independently of self-reported render/result hashes. It never executes a
script to “prove” correspondence. Adversarial editing of every source and every
binding is not cryptographic attestation; operator Cage rerun proves reproduction.

## Semantic conflicts and exit/status semantics
For diagnosed semantic conflicts, output contains EXACTLY:
- conflicts.json: `{schema:"inkscape-decision-diagrammer.conflicts/v1",
  conflicts:[{kind,evidence,question},...]}` (1–20 specific entries).
  kind contradiction/missing-evidence/incompatible-framework/selector-needs-input.
- design-notes.md: exact evidence and question strings, context, why rendering
  would mislead; ≥120 characters. No fictional diagram.
- result.json: generated by finish --needs-input with request/selector hashes,
  exact file list and notes/conflict bindings. No render.json/log or SVG/PNG.

Result schema `inkscape-decision-diagrammer.result/v1`, worker same ID.
Status `visual-review-pending` means mechanical rendered package; `needs-input`
means only a bound conflict report. accepted_diagram is always false;
visual_review pending/not-rendered respectively. Result binds every other output,
including render.json (which binds the preceding artifacts/log); cannot hash itself.
`bin/check` ignores stdin. Exit0 accepts the appropriate PACKAGE contract, not
a diagram/design verdict. Exit1 is incomplete/rejected or unavailable native tool;
other status is a broken check. Finish exit0 means it produced a bound package;
exit1 failure removes prior result/render receipts so stale success cannot pass.
Semantic conflict requires actual evidence, not a renamed executable failure.
Agent's own public exit statuses are distinct: 0 checked, 2 unfinished, etc.;
read current `agent help`. A report accepted by Agent can still be needs-input.
