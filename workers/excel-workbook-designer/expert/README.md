# Excel workbook designer and editor

One independently reusable Agent/Cage worker designs friendly, summary-first
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
mode. Do not apply creation's minimum mutation count or control/dashboard design
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
Before planning, the worker invokes `tools/inspect-workbook` using the contract's
syntax and reads the resulting input inventory, values/formulas, exact headers,
feature information and actual before PNGs when image inspection is available.
It never reconstructs the workbook from a prior spec or memory.

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
LibreOffice may be supplied by the host for separate engine evaluation, not a
replacement authoring API. Microsoft Excel availability is operator-dependent.
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
"$AGENT_HOME/bin/check"
```

For operator reruns of these deterministic commands, from that same workspace,
use `"$EXPERT/tools/render"` then `"$EXPERT/bin/check"` with the same host-selected
environment. Do not run them in the reusable definition directory or invent flags.
`bin/check` exit 0 accepts mechanically, 1 means unfinished, another status means
the check is broken. Agent exit statuses follow Agent's public manual.
A definition-only inspection is `agent check -C "$WORKSPACE" "$EXPERT"` with
an existing workspace; it is not a workbook evaluation.

## What good looks like
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
This revision authors only `AGENTS.md`, `README.md`, `PROVENANCE.md`,
`skills/workbook-editing/SKILL.md` and `skills/common-workflows/SKILL.md`.
The creation skill is preserved. The host owns both contracts and all deterministic
implementation. Missing capabilities require a separately reviewed host adapter
and external cases, not generated runtime code. One worker is sufficient:
domain interpretation belongs in skills, transformations/checking in trusted tools.
The dated 2026 evidence is directional, not a universal workflow ranking or proof
of this worker's quality; see provenance. No new evaluations are claimed.
