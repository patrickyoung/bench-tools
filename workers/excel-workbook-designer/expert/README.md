# Excel workbook designer

One independently reusable Agent/Cage worker designs friendly, summary-first
workbooks from selected CSV/JSON, semi-structured records and notes. It authors
declarative logic and guidance; the host's Artifact Tool renderer owns production.
It adds workbook schema, control, formula, update and evaluation expertise without
a second model loop, analytical engine or team.

## Inputs and outputs
Use the exact schemas and fields in `CONTRACT.md`. The runtime workspace contains:
- `request.json`: `schema:"bench.workbook-request/v1"`, `title`, `brief`, `audience`,
  `inputs:[{path,sha256}]`, `required_metrics`, `required_controls`; optional
  `context`, `constraints`, `as_of`, `engine`.
- Only explicitly selected regular CSV/JSON/MD/TXT files under `inputs/`.
  Hash actual bytes, not normalized text or reserialized JSON. Pre-extracted
  documents must carry supplied filename/page provenance.

The worker authors only `output/spec.json` (`bench.workbook-spec/v1`) and
`output/guide.md`. The trusted renderer produces `output/workbook.xlsx`,
`output/qa.json` and sheet previews. Keep the spec, original production inputs,
guide and actual QA evidence together in the operator's run records, not inside
this reusable definition. Source changes require a newly bound request.

The workbook is macro-free XLSX targeting Microsoft 365 desktop/web. Tables and
filters, validation, formula controls, conditional formatting, panes and cell-bound
bar/line charts are supported. PivotTables/pivot export, slicers, Power Query,
checkboxes, VBA, external connections and dynamic-array export are not.
A list selector is a list selector, not a slicer. Native What-If Data Tables,
custom scripts and other undocumented features are not promised.

## Run locally
Prerequisites: Agent and its configured Ask/Ply/Brief/Cage/Record companions,
operator-selected model/credentials, and the host-supplied trusted `CONTRACT.md`,
`tools/render`, `bin/check`, `tools/` and `lib/` implementation. The host selects
`WORKBOOK_NODE`, `WORKBOOK_NODE_MODULES` (containing `@oai/artifact-tool`) and
`WORKBOOK_PYTHON`, with dependencies outside the writable workspace.
LibreOffice is available to the host for separate engine evaluation, not a
replacement authoring API. Microsoft Excel is not installed on this host.
The operator must admit the trusted tools and their dependencies under Cage.
The worker cannot fix denied permissions by disabling confinement.

To bind a new explicit input selection without hand-writing hashes:

```sh
"$EXPERT/tools/prepare-request" --brief /absolute/brief.md \
  --input /absolute/data.csv --input /absolute/notes.md \
  --out /absolute/new-case --title "Operations review"
```

This copies selected files and emits new-case/request.json. Existing destinations
are rejected. The brief should name the workflow, intended readers, desired
controls, important definitions, reporting date and update expectations. It may
request any supported layout; the three evaluation domains are not templates.

With absolute paths selected by the operator, use an existing workspace separate
from and non-nested with the definition, containing the request and selected inputs:

```sh
agent run -C "$WORKSPACE" -evidence "$EVIDENCE" "$EXPERT" -- \
  "Design the workbook specified by request.json from its selected inputs." \
  >"$STDOUT_LOG" 2>"$STDERR_LOG"
status=$?
printf 'agent exit=%s\n' "$status"
```

Keep stdout, stderr, exit status, artifacts and controller evidence. `$EXPERT`
points to this installed definition; logs/evidence are operator-selected outside
the definition. This same definition can receive a bounded assignment in a
separate operator-invoked Agent run; no nested invocation is performed by the
worker. No network listener, MCP adapter, scheduler or provider client is needed.

Within an admitted runtime workspace the worker invokes:
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
This adaptation changes only `AGENTS.md`, this README, `PROVENANCE.md`, `LICENSE`
and `skills/workbook-design/SKILL.md`. The host owns contract and deterministic
implementation. Extend this worker's design guidance only within that contract;
a missing renderer feature needs a separately reviewed host capability and
external cases, not another worker or generated runtime code. See provenance for
what was reused and what was deliberately excluded.
