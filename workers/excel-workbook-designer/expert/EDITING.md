# Prompt editing contract v1

Request uses `bench.workbook-request/v1`, with `mode:"edit"` and
`workbook:"inputs/existing.xlsx"` naming a hashed input. The brief is the user's
prompt. Additional CSV/JSON/text inputs are optional. Existing workbook bytes
are authoritative. Use the supplied current `tools/edit-context` JSON or invoke
that tool from the workspace (no arguments). Read its compact full inventory and
all additional selected files; see README.md for the reversible encoding.
It supplies bindings and an EMPTY spec envelope, not a completed edit or QA.
Read this exact contract, author the first complete spec/guide, then render/check
and correct concrete failures; do not rediscover directories or dump lib code.
Full `inspection/workbook.json` and before PNGs remain available through
`tools/inspect-workbook`; generate/review before images when image inspection
is available. Native-feature limitations are blockers, never permission to
discard content. Ordinary shell writes of JSON/Markdown are allowed, not new
runtime programs. A supplied context must match current request/input hashes.

Write output/spec.json using:
```
{schema:"bench.workbook-edit/v1",request_sha256,inputs:[{path,sha256}],
 purpose,interpretation,changes:[{sheet,range,reason}],
 operations:[...],metrics:[{id,sheet,cell}],
 assertions:[{metric,value,tolerance?}],tests:[{name,edits:[{sheet,cell,value}],expect:[{metric,value,tolerance?}]}],
 previews:[{sheet,range}],mappings:[{source,column,target,confidence,reason}],
 issues:[],limitations:[],pivots:[]}
```
All inputs/request hash-bound exactly as creation. Every `range`, including in
`changes`, is an A1 cell or rectangle such as `C3` or `A1:E5`; put table names,
old-to-new descriptions and explanations in `reason`, never in `range`.
`changes` is explicit scope,
including affected formula/chart helpers. Every range operation must be contained
in a declared changed range. Original cells outside scope retain values/formulas
and effective styles. Original sheet order/names and native features persist.
New sheets append only when needed for requested outputs. There is no delete,
rename, structural row/column insert or full-sheet rebuild operation.

Operations (executed in order; arrays are rectangular and range-exact):
- `{op:"add_sheet",sheet}`
- `{op:"values",sheet,range,values:[[...]]}`; null clears contents;
  dates are `{date:"YYYY-MM-DD"}`, identifiers remain text.
- `{op:"formulas",sheet,range,formulas:[["=SUM(B2:B8)"]]}`
- `{op:"copy",sheet,range,source_range}`: copy all; same sheet; copied formulas
  use relative offsets. Override copied values/formulas afterward as needed.
- `{op:"format",sheet,range,format:{...}}`: font/fill/rowHeight/columnWidth/
  numberFormat/wrapText/alignment/borders using ordinary Artifact Tool format.
  Use Artifact Tool property names, not raw OOXML inspection nodes: font size
  is `size` (not `sz`), and an ordinary solid `fill` is a color string (not an
  object with `patternType`/`fgColor`). For example,
  `format:{font:{name:"Arial",size:11,bold:true,color:"#FFFFFF"},fill:"#17324D"}`.
  Choose the actual source font/colors for a matching edit; the example is not
  a required theme. Convert an opaque OOXML ARGB color such as `FF17324D` to
  the RGB color `#17324D` when using these properties.
  On imported workbooks, `copy` has been observed to leave destination styles
  unchanged even with copy-all. When matching a header/input style, follow with
  explicit format properties (font name/size/color/bold as relevant, fill and
  alignment); preserve or explicitly set the destination's intended number
  format. Do not treat a successful copy or preservation report as proof that
  requested styling was applied. Compare saved styles and inspect the preview.
  `columnWidth` is Excel character units (typical 12–32), NOT pixels;
  `columnWidthPx` is pixels. Keep summary columns compact, generally <=60 units.
  Do not widen a whole column to fit a long instruction. Shorten the visible
  note or move detailed instructions into guide.md. Render and review sizing.
  For appended source text, preserve the full text and fit it in the new row:
  use `wrapText:true` with sufficient `rowHeight` for the actual column width.
  An inherited row height may clip a longer new note even when its value is saved.
- `{op:"table",sheet,name,range}`: add or deliberately replace this table's
  extent (same top-left header; no record shifting). Preserve style/filter flags.
  Append cells only to empty reserved rows; if content would be overwritten, stop.
- `{op:"validation",sheet,range,values:[...]}`: list validation.
- `{op:"conditional_format",sheet,range,formula,fill?,color?}`
- `{op:"chart",sheet,type:"pie"|"doughnut"|"bar"|"line"|"area"|"scatter",
  range:"A1:B5",title,from:"E2",to:"L18",number_format?}` adds a native
  cell-bound chart. Pie has one nonnegative measure and a few meaningful groups.
  Reserve a blank destination. Headers must be included in source range.
- `{op:"chart_data",sheet,index,range}` updates an existing chart's source;
  index is zero-based inspection chart order. Preserve placement/title/style.

At least one baseline assertion. At least one meaningful mutation when formulas,
chart source values or pivots are changed; no artificial controls/dashboard for a
narrow edit. Metrics may reference existing/new values or formulas. Assert
independent expected values, counts, totals and representative formulas through
actual results. The adapter restores every tested cell/formula and pivot result.
Test IDs are exact. All required_metrics still apply. Tests never replace a
formula cell. Include changed views and affected dependencies in previews.

For a calendar-date metric, an assertion or mutation expectation may use
`{date:"YYYY-MM-DD"}` (UTC midnight) or the exact canonical UTC ISO string,
such as `"2026-09-19T00:00:00.000Z"`. The renderer records actual Date values
as ISO strings with explicit date-metric metadata. It never parses ordinary
text or numbers into dates for comparison. A typed date expectation cannot
match date-looking source text; wrong dates remain failures. Numeric serial
metrics with numeric expectations remain numeric.
Saved date metrics must be numeric cells with a recognized calendar-date
format, such as `yyyy-mm-dd`. A runtime Date exported with an arbitrary,
unrecognized, conditional, multi-section or time-only format fails the saved-cell check explicitly;
this is not universal Excel number-format recognition. The check validates
saved serials using the workbook's 1900 or 1904 date system. It does not treat
the fictitious 1900-02-29 as a valid calendar date. QA type metadata is emitted
by the renderer, not authored or repaired by the model.
This verifies saved date-system semantics; it does not certify every date-system
case in the import/export engine or in native Excel.
Use this corrected date comparison; never delete date metrics, assertions or
mutation expectations to silence an execution mismatch. Correct a demonstrably
wrong expectation only from selected facts. If the supported engine cannot
establish a result, retain that failure.

`mappings` must explain each source column when appending mismatched headers;
confidence is `high`, `medium` or `unresolved`. Unresolved mappings cannot be used
to append. Preserve unmapped information explicitly or stop for a decision.
No invented business equivalences, unit conversions, keys or deduplication.

Invoke `tools/render`, then `bin/check --mechanical` for worker/manual checks.
Agent separately invokes plain `bin/check` for trusted completion. Only the
caller selects the optional visual stage and external evidence directory under
`VISUAL-CHECK.md` and `VISUAL-CONTEXT.md`; absent selection is mechanical-only.
Follow AGENTS.md's check/repair boundary and the editing skill for cell-specific
formatting repairs, without weakening assertions or expanding edit scope.
These commands route by schema. Edit output is
`output/workbook.xlsx`, `output/qa.json`, `output/change-report.json`, previews
and your guide. Original input is never overwritten. A preservation failure
blocks acceptance even if requested totals are correct.

### Saved-feature evidence and submission

The checker's `saved_native_features` in `output/change-report.json` and
successful mechanical stdout is observed saved-XLSX evidence. Its fields are
`workbook_sha256`, `tables` (sheet, name, range, ordered headers), `charts`
(actual part, references), `counts`, `omitted`, `complete`, `max_compact_bytes`
and `scope`. Compare relevant table identities/extents/headers and chart
reference strings against the request and declared operation once. Counts alone
never establish requested chart bindings; null/empty reference entries are
unavailable evidence, not valid bindings. For a relevant omitted/unavailable
fact, retain the gap and make one supported targeted inspection. Unrelated
omissions with `complete=false` do not require more discovery. Do not infer
omitted facts or guess archive paths/XML prefixes. This evidence does not prove
exact requested chart bindings, appearance, ownership, type or every native
behavior; retain existing targeted checks where needed.

After render/mechanical checking, inspect the actual result, QA and relevant
saved evidence once and verify the request against supplied source facts. Once
complete and sound, the next response is a brief plain-text candidate report,
not another shell action. Follow AGENTS.md's trusted submission/repair boundary;
mechanical acceptance remains provisional.

## Native simple PivotTables (creation and editing)

Optional root `pivots` in either spec:
```
[{name:"SalesByRegion",source_sheet:"Data",source_range:"A1:D20",
 sheet:"Summary",cell:"A5",row_field:"Region",value_field:"Revenue",
 aggregate:"sum",caption:"Revenue total",number_format:"$#,##0.00"}]
```
Source header row unique/nonblank, exact field names, populated bounded data,
one row category, one numeric measure; aggregates sum/count/average/min/max.
No column/filter hierarchies, slicers, OLAP, calculated fields or Data Model.
Destination sheet exists and generated area is blank (except previous own pivot
on an in-memory test refresh). Reserve category count + 2 rows and 2 columns;
do not overlap existing content/charts/another pivot. Native bridge creates
PivotTable, cache definition, cache records and correct package relationships.
Results are cached pivot output, not formulas. Refresh after source edits in
Excel; source extent is fixed until explicitly extended. Mutation tests refresh
the adapter's pivot as part of evaluation. Native Excel UI is separately untested.
Existing local native pivots and their cache parts are preserved byte-for-byte
through package grafting after Artifact export. Editing their layouts is not
supported; adding a new uniquely named simple pivot is supported. If source data
changes, existing pivot results/caches require manual refresh in Excel; disclose
this in the guide. Slicers, external caches and Data Model remain blockers.

## Boundaries

When a material mapping/meaning cannot be inferred, return a clarification
instead of guessing or making a preservation-only workbook. Author
`output/spec.json` as `{schema:"bench.workbook-clarification/v1",
request_sha256,inputs,reason,questions:[{question,source,locator,alternatives:[...]}]}`
and explain the decision in guide.md. `source` is an exact selected input path;
`locator` identifies the relevant row/header/range; alternatives give at least
two concrete interpretations. Invoke ONLY `bin/check --mechanical`, not `tools/render`.
The checker creates result.json with `status:"needs-input"`, no workbook.
This source-bound clarification needs no visual inference or new refusal schema.
The operator resumes with the answer in a fresh bound request. This is a valid
clarification outcome, never a completed edit. Routine choices still proceed.

Macro-free local XLSX, max 20MB compressed/100MB expanded, 8 sheets and 10,000
rows by 100 columns per range. Reject macros, external connections, drawings
other than supported charts, legacy/threaded comments, custom XML, slicers,
Data Model, queries, protected sheets/workbooks and signatures until a preserving
adapter exists. Read-only inspection reports why; do not silently flatten.
Native table formulas, dropdowns, conditional formats, panes and effective
styles are preservation checks. Unsupported source features require a different
capable native adapter, not a destructive conversion. Independent semantic and
visual review remains required; mechanical acceptance is only named evidence.
