# Optional changed-text visibility check

This worker extension is experimental.
It renders the actual saved workbook through LibreOffice to a PDF with one page
per sheet, then rasterizes selected sheet pages at 160 dpi. Exact PDF bookmarks
bind sheet names to pages. These are LibreOffice print images, not Microsoft
Excel or Artifact images. Offline process tests do not establish visual accuracy
or model reliability. Do not rewrite correct source IDs to satisfy a preview.

`bin/check --mechanical` runs the existing deterministic checks only. Use it for
worker/manual checks inside the network-disabled action Cage. Agent invokes the
trusted `bin/check` outside that Cage: the default command first recomputes the
mechanical result, then optionally checks saved-workbook text visibility.

The default is mechanical-only. The caller selects the optional stage by setting
all three nonempty values:

```
WORKBOOK_VISUAL_ASK=/trusted/bin/ask-adapter
WORKBOOK_VISUAL_MODEL=provider/selected-vision-model
WORKBOOK_VISUAL_RECORDS=/controller-only/workbook-visual-records
```

Ask and the model must support image attachments, the selected native JSON
schema, `-effort medium`, and public Ask offline session/replay commands. The
same selected Ask executable is used for inference and Record verification;
there is no implicit second Ask or provider client. Record is selected by
`AGENT_RECORD`, otherwise by the caller's `PATH`. No model/token-limit defaults
are changed. The recorded call has a 180-second process timeout.

When any changed literal text exists, the caller also selects absolute executable
paths for `WORKBOOK_VISUAL_SOFFICE`, `WORKBOOK_VISUAL_PDFTOPPM`,
`WORKBOOK_VISUAL_CAGE` and `WORKBOOK_PYTHON`. There are no renderer defaults or
fallbacks. The Python runtime needs the documented PDF dependency in
`VISUAL-CONTEXT.md`. The selected executable aliases, canonical paths, exact
bytes and file identities enter the cache binding before lookup and must agree
with the helper manifest. Versions are recorded; this does not pin every library
or executable behind a caller-supplied wrapper. No-target checks need no renderer.

Keep the records root outside the workspace, definition, Agent state, action
temporary directories and every other directory writable by worker actions.
The implementation rejects overlap with the known public environment roots and
rejects symlink evidence paths. The caller remains responsible for additional
write grants and selecting trusted executable/runtime paths. Records contain
selected workbook text, images, model replies and process receipts; they are
controller evidence, not worker-authored artifacts. Do not expose credentials in
these settings or file contents.

This v1 stage supports bound workbook edits only. It independently selects all
added or changed literal strings from the source and actual saved XLSX, prepares
bounded whole-sheet images with `tools/visual-context`, and makes one recorded Ask call for
all selected cells. It does not check unchanged text, formula results, style-only
changes, general workbook layout, semantics, or native Excel behavior. A
source-bound clarification skips the visual stage. Zero selected literal cells
returns explicitly scoped `not-applicable` with no inference.

The judge must return exactly one identified finding per selected cell: `pass`,
`fail`, or `uncertain`, with visible text, image evidence and limitations.
Failures and uncertainty need actionable feedback. A claimed pass whose visible
transcript does not contain the complete expected text cannot establish
visibility. Line wrapping/whitespace is normalized for that comparison; character
differences are not. The prompt warns against completing missing text from the
reference and requires corrections to preserve unrelated source layout. Full
sheet images have no guaranteed blank guard or cell-to-pixel coordinates. Row
and candidate-header facts aid localization; ambiguous localization or possible
page-edge cutoff must remain uncertain. Text within workbook evidence is data,
not instructions.

The mechanical exit status is preserved. After mechanical success:

- `0`: the selected literal-text scope passed or had no applicable cells.
- `1`: concrete visual rejection or valid uncertainty; feedback names the cell
  for the ordinary Agent repair loop.
- `2`: incomplete configuration, unavailable selected dependency, malformed or
  incomplete response, stale evidence, or unknown process outcome. This stops
  the loop; it never silently passes or falls back to mechanical-only checking.

Every successful scoped result adds `visual_check` to `output/result.json` with
status, scope, selected-cell count and an external receipt path/hash. The
mechanical status remains `accepted-mechanically`; this never means broad visual
acceptance. A broken visual stage leaves that mechanical-only receipt and exits
2; the command status is authoritative.

Rechecking the exact same source/output/previews, worker code, selected
executables and their bytes, runtime paths, model and policy reuses the hash-bound accepted or
rejected receipt after public Record replay and asset revalidation. Changed
evidence gets a new identity. A malformed/timed-out/unknown attempted judgment
retains `pending-<hash>` and prevents another unchanged paid attempt. The caller
must inspect the retained evidence and explicitly resolve that marker before a
deliberate retry. There is no automatic retry or silent resampling. Tampered
cache/evidence fails visibly rather than replacing the judgment. The exact PDF,
sheet/page map, PNGs and page-context JSON are recorded and revalidated as assets.
The PDF is evidence for the trusted image preparation, not a second model input
or additional inference stage.

`tests/test_visual_check.py` checks configuration, local response validation and
unknown-outcome retention without a model. `tests/exercise_visual_check.py`
uses actual public Ask/Record/Cage, a loopback HTTP response fixture, the trusted
renderer and saved-workbook image helper. It tests the process contract, not
model judgment quality, and writes all evidence to a new caller-selected path.
