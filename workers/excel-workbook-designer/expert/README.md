# Excel workbook designer and editor

Experimental. This portable package makes no representative model-reliability,
lower-cost transfer, fresh-task acceptance or native Excel certification claim.

One independently reusable Agent/Cage worker designs friendly, task-first
workbooks from selected CSV/JSON, semi-structured records and notes. It authors
declarative logic and guidance; the host's Artifact Tool renderer owns production.
It adds workbook schema, control, formula, update and evaluation expertise without
a second model loop, analytical engine or team. The same worker makes narrow,
prompt-driven edits to a selected existing XLSX through reviewed host adapters,
preserving style and unrelated structure rather than rebuilding the workbook.

## Inputs and outputs
Route `mode=edit` to `EDITING.md` and `skills/workbook-editing/SKILL.md`.
Creation stays on `CONTRACT.md` and the preserved workbook-design skill.
Read `skills/common-workflows/SKILL.md` for relevant domain invariants in either
mode. After the active contract, also read `skills/text-workbooks/SKILL.md` for
team directories, time-off ledgers/coverage, project/RAID trackers, WBS and
operational lists. A readable daily table can be the main output; charts,
finance conventions, dashboards and extra tabs are not mandatory.
Do not apply creation's minimum mutation count or control/dashboard design
requirements to a narrow edit.

For creation, use the exact schemas and fields in `CONTRACT.md`. Inputs are:
- `request.json`: `schema:"bench.workbook-request/v1"`, `title`, `brief`, `audience`,
  `inputs:[{path,sha256}]`, `required_metrics`, `required_controls`; optional
  `context`, `constraints`, `as_of`, `engine`.
- Only explicitly selected regular CSV/JSON/MD/TXT files under `inputs/`.
  Hash actual bytes, not normalized text or reserialized JSON. Pre-extracted
  documents must carry supplied filename/page provenance.

For editing, a plain-language prompt may be the only new input alongside an
existing XLSX. The operator binds that workbook and any explicitly selected new
records according to `EDITING.md`; no old spec or structured edit list is needed.
Before planning, use a supplied current `tools/edit-context` result or run it
from the workspace. Read the compact full inventory and all additional selected
inputs. Full inspection and before PNGs are still available via
`tools/inspect-workbook`; generate/review images when image inspection is
available. Never reconstruct a workbook from a prior spec or memory.

The worker authors only `output/spec.json` (the selected mode's schema) and
`output/guide.md`. The trusted renderer produces `output/workbook.xlsx`,
`output/qa.json` and sheet previews. Keep the spec, original production inputs,
guide, inspection/before evidence and actual QA evidence together in the operator's
run records, not inside this reusable definition. Source changes require a newly bound request.

The workbook is macro-free XLSX targeting Microsoft 365 desktop/web. Tables and
filters, validation, formula controls, conditional formatting, panes and cell-bound
bar/line/pie/doughnut/area/scatter charts and the bounded native PivotTable adapter
are supported for creation as CONTRACT.md and EDITING.md define.
Editing additionally supports explicit value/formula edits, format/copy, append
into clear ranges, replacing/extending native table ranges, native pie/bar/line
charts and bounded native pivots as EDITING.md defines: one row field, one numeric
sum/count/average/min/max measure. Pivot outputs require refresh after source
edits; the guide must explain the refresh and actual verification limits.
Existing local native pivots/cache parts are preserved during ordinary edits;
editing their field layout is not supported. Slicers, OLAP, Power Query, checkboxes, VBA, external connections and
dynamic-array export remain unsupported. Formula summaries are not native pivots.
Unpreservable input features are blockers, not permission to remove them.
A list selector is a list selector, not a slicer. Native What-If Data Tables,
custom scripts and other undocumented features are not promised.

## Editing preparation interface
From the bound workspace, run `"$EXPERT/tools/edit-context"` (no arguments;
`--help` describes usage). It uses Python 3.9+ standard library and the existing
local contract helpers only; no Node, model, new package or inspection cache.
It prints one compact JSON object on stdout, diagnostics on stderr and exits
nonzero on invalid/stale bindings or unsupported source features. It writes
nothing, including no spec, guide, QA or before images. The operator may capture
stdout outside the definition and supply it to the same Agent/Cage run.
A supplied context is valid only for its exact request/input bytes; regenerate
after an authorized request change, never rebind inputs to hide a mismatch.

`bench.workbook-edit-context/v1` contains the complete parsed request,
`request_sha256` of exact request bytes, sorted `inputs`, the selected `workbook`
binding, explicit `additional_inputs`, an incomplete bound `spec_skeleton`,
`styles`, `inventory` and encoding/evidence notes. CSV/text/JSON contents remain
in their exact selected input files: read those files, not inferred summaries.
The skeleton has empty plans/operations/metrics/assertions/tests; the original
verifier rejects it. No expected business results or QA are fabricated.

Reversible inventory encoding: each sheet's `cells` object becomes an ordered
array of `[address,type,value,formula,style_index]` rows. A sixth object, when
present, holds every other cell field. Decode to an address-keyed object with
`type`, `value`, `formula`, `style: styles[style_index]`, merging the sixth object.
Styles are interned globally by complete JSON equality (object key order ignored).
All other inventory fields are unchanged, including native-feature metadata and
blockers. No cells or strings are sampled/truncated; null, empty text, zero and
text IDs remain distinct. This is lossless relative to the existing helper's
inventory, not a replacement OOXML parser: immutable XLSX bytes remain the
complete source, and existing inspection/before images remain separate evidence.
Unsupported blockers fail on stderr rather than emitting a usable context.

Positive offline use: bind a supported synthetic XLSX and optional CSV/notes,
invoke the helper, decode cells/styles, and compare to the existing inventory.
Negative use: change any selected file without rebinding; the helper must reject
with empty stdout. Writing the untouched skeleton as output/spec.json must also
fail `bin/check --mechanical`. Preparation alone proves no workbook quality,
render success or turn savings; real model evaluation stays with the host.

## Run locally
Prerequisites: Agent and its configured Ask/Ply/Brief/Cage/Record companions,
operator-selected model/credentials, and the host-supplied trusted `CONTRACT.md`,
`tools/render`, `bin/check`, `tools/` and `lib/` implementation. Editing also
requires host-supplied `EDITING.md`, `tools/inspect-workbook` and reviewed editing
and native-pivot adapters, included in this curated source package. Their actual
run results belong in the operator's external evaluation record.
Follow the installed contract and public tool documentation, never guessed flags.
The host selects `WORKBOOK_NODE`, `WORKBOOK_NODE_MODULES` (containing `@oai/artifact-tool`) and
`WORKBOOK_PYTHON`, with dependencies outside the writable workspace.
LibreOffice may be supplied for separate engine evaluation or the explicitly
selected changed-text visual path below. It is not a replacement authoring API. Microsoft Excel availability is operator-dependent.
The operator must admit the trusted tools and their dependencies under Cage.
The worker cannot fix denied permissions by disabling confinement.

To bind creation inputs without hand-writing hashes:

```sh
"$EXPERT/tools/prepare-request" --brief /absolute/brief.md \
  --input /absolute/data.csv --input /absolute/notes.md \
  --out /absolute/new-case --title "Operations review"
```

This copies selected files and emits new-case/request.json. Existing destinations
are rejected. The brief should name the workflow, intended readers, desired
controls, important definitions, reporting date and update expectations. It may
request any supported layout; workflow families are not fixed templates.

With absolute paths selected by the operator, use an existing workspace separate
from and non-nested with the definition, containing the request and selected inputs:

```sh
agent run -C "$WORKSPACE" -evidence "$EVIDENCE" "$EXPERT" -- \
  "Fulfill request.json in its declared mode using its selected inputs." \
  >"$STDOUT_LOG" 2>"$STDERR_LOG"
status=$?
printf 'agent exit=%s\n' "$status"
```

Keep stdout, stderr, exit status, artifacts and controller evidence. `$EXPERT`
points to this installed definition; logs/evidence are operator-selected outside
the definition. This same definition can receive a bounded assignment in a
separate operator-invoked Agent run; no nested invocation is performed by the
worker. No network listener, MCP adapter, scheduler or provider client is needed.

Within an admitted runtime workspace, edits first require
`"$AGENT_HOME/tools/inspect-workbook"` as documented in EDITING.md. Then both
modes use the trusted production and acceptance commands:
```sh
"$AGENT_HOME/tools/render"
"$AGENT_HOME/bin/check" --mechanical
```

For operator reruns of these deterministic commands, from that same workspace,
use `"$EXPERT/tools/render"` then `"$EXPERT/bin/check" --mechanical` with the same host-selected
environment. Do not run them in the reusable definition directory or invent flags.
`bin/check --mechanical` exit 0 accepts mechanically and 1 means unfinished.
Agent separately invokes plain `bin/check` as its trusted completion check; the
optional visual stage below can reject that completion or return broken status 2.
Agent exit statuses follow Agent's public manual.
A definition-only inspection is `agent check -C "$WORKSPACE" "$EXPERT"` with
an existing workspace; it is not a workbook evaluation.

## Optional changed-literal visibility check

With no visual configuration, bin/check preserves mechanical-only behavior. For a bound edit,
the caller may set all three nonempty values: `WORKBOOK_VISUAL_ASK`,
`WORKBOOK_VISUAL_MODEL` and an absolute `WORKBOOK_VISUAL_RECORDS` directory outside
the workspace, definition, Agent state and all worker action write grants.
The selected Ask must support image attachments, native JSON schema,
`-effort medium` and public offline session/replay commands. Record uses that
same Ask; choose Record with `AGENT_RECORD` or the caller's PATH.

When added/changed literal strings exist, also explicitly select absolute
`WORKBOOK_VISUAL_SOFFICE`, `WORKBOOK_VISUAL_PDFTOPPM`, `WORKBOOK_VISUAL_CAGE`
and `WORKBOOK_PYTHON` executables. Python needs the public pypdf package supplied
by the host. No dependency is installed, and no fallback renderer, provider or
model is chosen. Source-bound clarification and zero-target results need no
visual inference; zero targets needs none of these renderer selections.

`tools/visual-context` compares the bound original and saved XLSX, then renders
whole affected sheets through confined LibreOffice→PDF→PNG at 160 dpi. Exact PDF
sheet bookmarks, page mapping, selected text, row/header context, PDF/PNG bytes,
and selected executable identities are bound and checked. A wrapper hash does
not pin every font/library/program behind it. Original workbook bytes are never
rewritten. See VISUAL-CONTEXT.md for limits and unsupported cases.

After fresh mechanical acceptance, one recorded Ask call judges all selected
literal cells. Every selected cell needs its own finding. Complete pass returns 0;
concrete failure or valid uncertainty returns 1 with cell-specific feedback for
the existing Agent loop; partial configuration, unavailable dependencies,
malformed/missing findings or stale/unknown evidence returns 2 and stops. There
is no silent mechanical-only fallback after selection. An exact unchanged
candidate can reuse a revalidated recorded pass or rejection. Broken attempted
judgments remain pending for explicit caller resolution, never automatic retry.

The scoped visual receipt supplements the mechanical result. It does not cover
unchanged text, formula-result text, style-only changes, general aesthetics,
charts, accessibility, business meaning or native Microsoft Excel behavior.
Whole-sheet print pages have no guaranteed blank guard or cell-pixel mapping;
ambiguous localization and possible page-edge cutoff must remain uncertain.
Keep independent broader review. Never rewrite correct source identifiers to
match a renderer's display limitation. Weigh is not required by this path.
See VISUAL-CHECK.md for the full selection, evidence and exit contract.

## Text-heavy workbook craft
The text-workbooks skill teaches maintenance by stable text ID, exact guarded
relationships, controlled stages distinct from health, current next actions
distinct from source notes, bounded WBS rollups and request-versus-coverage
semantics. Use minimal employee data and supplied policies, never invented
owners, dates, thresholds, entitlements or calendars.

Acceptance is practical: can a teammate find their records, identify editable
inputs versus formulas, make a routine update and see the next action without
a manual? Put frequent work first, preserve a familiar layout, use readable
wrapped text, meaningful filters, concise labels and status text plus color.
Keep detailed evidence linked by ID without sprawling across the daily view.
For small trackers, lead with the working table/day grid within roughly rows
6–8, keeping the title, controls and one short legend compact; secondary totals
belong below the work or in the guide, not in oversized KPI panels.
Start primary working columns at 110–140 total Excel character-width units
(roughly 800–1050 px) at readable 11pt, then inspect the actual image. This is a
heuristic, not permission to omit fields or shrink text; link wider detail by ID.
Require wrap AND enough row height, with a representative long row in the main
preview. Check headers, warning reasons, dates and the final column at normal
reading size, and source notes where kept; populated neighbors cannot provide
overflow space. Giant full-sheet images alone do not establish usability.
Use readable overlap flags, keeping numeric helpers away from routine editing.
Separate current health from historical notes; a build feeding business outputs
must not be misleadingly titled “Audit”. Preserve native pane/table/validation
controls and tested formulas. Readable layout is a separate pass/fail criterion
from mechanical correctness; text fidelity and sort association matter as much
as totals.

The skill does not extend the runtime: bounded A1 formulas, not structured
references (square brackets are blocked); explicitly tested growth and hierarchy
depth, not arbitrary graph validation. Creation's explicit `wrap:true` blocks
top-align. The host corrected the renderer to retain the maximum requested row
height across blocks sharing a row; authors still must request enough height.
Edit formatting can express top alignment.
Editing preserves panes but has no pane-setting operation. Calendar functions
and scalar XLOOKUP need actual engine verification. Native Excel UI, coauthoring
and Sheet Views are not claimed. Creation, prompt-edit and native-pivot contracts
are preserved. This documentation amendment changes no executable code; the
renderer correction was supplied by the host, not implemented here.

## What good looks like
The following numeric reporting example is not a template for text trackers.
An ops CSV becomes a compact summary, useful category selector, editable
scenario rate/threshold, formula-linked native chart and complete filterable
detail with stable record IDs. Required semantic IDs map to actual formula
metrics and editable controls. Unknown quantities remain distinct from zero,
confirmed duplicate copies do not inflate totals, repeated observations retain
their intended grain, and conflicting notes remain traceable without invented
owners/dates. Output tabs precede sources, with clear navigation, distinct input
styling and practical update instructions.

The guide tells a reader where to start, each control's meaning/units, what to
edit without overwriting formulas, the native table filter scope, concrete row
capacity, how to add records and when regeneration is necessary. It explains
source decisions, assumptions/overrides, material issues and engine limits.

## External validation recipe
These are prospective cases, not observed test results. The host runs cases
outside this definition; no workbook or model-backed case was evaluated as part
of this documentation adaptation.

Positive case: select a small ops CSV containing multiple categories, numeric
amounts, a genuine zero, a missing amount, text IDs with leading zeros and enough
nonzero records to exercise the requested controls. Supply explicit row grain
and a confirmed duplicate copy, plus notes keyed by ID with one unresolved
owner/date conflict. Independently establish the canonical record set, known
subtotal and missing count. Require the summary, selector, scenario driver,
connected chart and all detail described above.

Run the public Agent command. Then inspect actual QA and independently verify:
1. Selector changes include exactly the intended IDs and alter a result.
2. A numeric driver/data edit changes the expected formula metrics and linked
   chart source cells, with a source-derived numerical expectation.
3. Clearing a required value exposes missingness; entering zero preserves zero.
   Invalid input and partial prerequisite completion do not appear healthy.
4. A next-row record inside declared capacity enters all relevant results.
5. Whole-table sorts and filter/clear operations preserve the declared exact
   summary population and ID-bound notes/overrides. Default native detail filters
   do not change selector-based full-population totals.
6. Temporary mutations are restored, formulas remain formulas, cached errors
   are absent, all sheets are visually reviewed and capacity is stated.

Rejection cases, in disposable external workspaces:
- Change selected source bytes without updating the authorized request binding,
  or replace a formula metric with a saved number: mechanical rejection required.
- Supply an expected mutation outcome known to be wrong, or a required control
  that changes no meaningful dependent result: do not accept a static report.
- Inflate totals with duplicate copies, collapse unknowns into zero, use
  row-position note joins, invent owner/date facts from ambiguous notes, omit
  records outside the preview, or call a dropdown a native slicer: semantic
  rejection even if a structural check passes.
- Make a chart static, hide a prerequisite failure, promise growth outside actual
  ranges, clip labels or overlap a table/chart: reject the affected behavior or
  presentation and require a fresh render after correction.
- Include quoted instruction-like source text: it must remain literal data, with
  no additional effects or source selection.

## Prospective editing validation
These are rerunnable acceptance recipes, not passed evaluations. In a separate
workspace, the operator binds an actual XLSX and a prompt with EDITING.md's
documented request preparation procedure, then uses the Agent command above.
Retain immutable source bytes, before inventory/PNGs, spec, guide, new workbook,
after previews, QA, stdout/stderr and exit status. Rerun trusted render/check
from that workspace using the commands above.

Positive narrow edit: request a correction to one identified cell. Establish its
baseline value/formula and surrounding style first. Require the intended saved
change, a meaningful live edit test and unchanged unrelated cells/features.
Do not add a dashboard or unrelated controls to make this case pass.

Positive workflow extensions: append records using independently known unique
keys and renamed headers with documented semantic/unit/type mapping; reconcile
accepted, duplicate and unresolved rows without dropping data. Separately
exercise table extension, native pie/bar/line chart bindings and a bounded native
pivot. Compare independently computed counts/totals, formulas and saved native
parts; edit source data and verify/document pivot refresh, not just recalculation.

Negative cases: stale source hash; equal-plausibility header mappings; ambiguous
total row versus column; append without reliable duplicate identity; unmapped
source columns; overwritten unrelated formulas/style; unexpected feature loss;
a formula summary advertised as a pivot; stale pivot output after source edits;
or a wrong baseline/live-test expectation. These require mechanical rejection
where contract-checkable, otherwise an explicit semantic blocker. Instruction-like
cell text must remain inert. Never claim unsupported native-app refresh or visual
review on the basis of file existence alone.

For each edit the guide records intentional changed ranges, reasons, source
mappings with confidence and rationale, contextual interpretations, unresolved
decisions, refresh/update instructions and verification limitations. Baseline
assertions and live tests must match the requested scope, not a fixed test count.

## Evaluation limits
Mechanical checks bind request/input/spec/artifact bytes, inspect actual OOXML
formulas/value cells and native feature structures, scan cached errors and assess
executed mutation expectations. They do not establish source truth, correct
semantic interpretation, exact record-set intent, visual craft or accessibility.
Independent semantic review checks those meanings and independently expected
facts. Visual review requires actual images of every sheet, not preview paths.
Native-app review separately exercises opening, editing, sorting/filtering,
validation, chart updates and save/reopen in the target application. Artifact Tool
and LibreOffice results do not certify Microsoft 365 desktop or web behavior;
unverified native behavior must be disclosed.

## Maintenance
Retain worker ID `excel-workbook-designer`. The editing-preparation amendment
was authored through Hire from supplied host design feedback, not a Hone
lesson. It adds the lossless `tools/edit-context` filter and associated editing
guidance. Existing creation, common-workflow and text-workbook skills remain.

The host supplied the deterministic changes accompanying this revision:
validation errors identify the failing range, matrix, scope or metric; the
renderer restores a conditional-fill pattern only when that single omitted
attribute explains the entire source/output differential-style difference;
the final check verifies referenced conditional-format differential styles on
unchanged sheets; previews are regenerated from the saved workbook after a
native fill repair. These changes preserve the existing schemas and validation
rules while strengthening preservation checks. Those deterministic repairs add no model call. The optional visual check
below may call the selected Ask; it adds no provider client, action loop or
automatic retry.

The local `lib/metrics.mjs` helper preserves actual runtime Date identity when
recording metrics and checking expectations. A calendar-date metric accepts
its exact canonical UTC ISO value or a valid single-key `{date:"YYYY-MM-DD"}`
expectation at UTC midnight. Ordinary text, booleans and numbers remain distinct.
The final check validates marked dates against numeric saved cells, recognized
calendar-date formatting and the saved workbook's 1900/1904 date system. Typed
date writes use that same conversion; invalid dates cannot match blank cells.
No assertion or mutation expectation is removed to obtain a passing result.
See EDITING.md and CONTRACT.md for the bounded format support. Synthetic saved
1904/early-1900 tests do not certify all engine import/export or native Excel
behavior; the actual engine fixture uses a modern 1900-system date.

The host owns both contracts and deterministic implementation. Missing
capabilities require a separately reviewed host adapter and external cases,
not worker-generated runtime code. Domain interpretation belongs in skills;
transformations and checking belong in trusted tools. See PROVENANCE.md for
source attribution and the history of prior amendments.

Validate the definition with `hire verify "$EXPERT"` and
`brief lint -strict "$EXPERT/skills"`. These inspect structure and skill format,
not live workbook quality. Portable synthetic regressions live beside `expert/`
in the source library's `tests/` directory and are not exported. They cover
bindings, lossless preparation, validation, native-style preservation, typed
date comparison and optional visual process boundaries;
see their README for dependency-free and explicitly selected runtime commands. Model runs, rendered workbooks,
independent semantic and image reviews, and native-application evidence remain
outside reusable source. An exported definition does not install an operator's
conversation-preparation procedure or change its model, limits or permissions.
