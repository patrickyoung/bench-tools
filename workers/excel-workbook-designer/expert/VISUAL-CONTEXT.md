# Saved-workbook changed-text evidence

A trusted caller runs this from the work directory:

```
/path/to/expert/tools/visual-context --out /absolute/new/controller/evidence
```

The destination must be new, with an existing private parent outside work,
definition, and exposed `AGENT_WORK`, `AGENT_HOME`, `AGENT_STATE`,
`AGENT_ACTION_TMP`, `PLY_DIR`, and `TMPDIR` roots. Repeat
`--exclude-root /absolute/path` for any additional caller-selected state or action
write grants. These path checks complement the caller's authority boundary.

When changed literals exist, explicitly select absolute executable paths in
`WORKBOOK_VISUAL_SOFFICE`, `WORKBOOK_VISUAL_PDFTOPPM`, `WORKBOOK_VISUAL_CAGE`, and
`WORKBOOK_PYTHON` (the bundled Python with public `pypdf`). There are no implicit
renderer defaults, installations or fallbacks. Trusted executable aliases are
resolved and the selected alias, canonical file identity, bytes and reported
version are bound and rechecked. A wrapper's hash identifies the wrapper; it is
not a transitive pin of every library, font or executable the wrapper loads.
Zero in-scope targets needs none of these renderer dependencies.

The tool requires a current edit request and `accepted-mechanically`
`output/result.json`. It verifies exact request and selected input bytes, the
complete edit artifact set, spec/QA bindings, mechanical preview hashes and
preservation report. It rejects symlink components and nonregular/oversized
files, then rechecks bytes and identities before publishing. The receipt is
existing caller-selected evidence, not independently authenticated provenance.
Original source and saved XLSX are never rewritten or reformatted.

The bound original and saved output are independently inventoried. Every added
or changed literal string is selected by exact decoded text and semantic type.
Shared, inline and ordinary string storage are equivalent. Case, whitespace,
number-to-text and formula-to-literal changes remain distinct; new sheets and
empty strings are included. Unchanged strings, removed strings, format-only
changes and formula results are outside this v1 criterion. Zero targets does not
mean the workbook passed a general visual review.

Rendering uses public LibreOffice CLI `calc_pdf_Export` with `SinglePageSheets`
and bookmarks explicitly enabled. Each saved sheet must have exactly one PDF
page and one unique exact-name bookmark; every PDF page must be mapped once.
Missing, duplicate, nested or mismatched bookmarks and unsupported page geometry
reject preparation. Page order alone is never a sheet mapping. The documented
filter includes hidden sheets and ignores print ranges; this tool still rejects
hidden selected sheets and hidden/zero-sized rows or columns on affected sheets.
Native pivots and the existing unsupported editing features remain excluded.
Selected literal text inside a non-anchor merged cell is also unsupported.

One **complete sheet page** is rasterized per affected sheet with public
`pdftoppm` at fixed 160dpi. All affected page dimensions are checked before the
first rasterization. There are no tight cell crops, borrowed Artifact Tool
coordinates, inferred glyph bounds, tiling or automatic downscaling. Limits are
512 targets, 16 images, 100 columns/10000 rows in the inventory, 4096×2048 pixels
per image, 16000000 total image pixels, 20MB PDF and 8MiB manifest metadata.
Oversized or unsupported coverage fails visibly and publishes no manifest.

The entire renderer pipeline runs through the selected public Cage command,
with one private render write grant, private `TMPDIR` inside that grant, and
networking denied. The immutable saved XLSX snapshot and trusted renderer script
are outside the write grant. Host reads remain unrestricted by Cage. The process
has a bounded timeout and no unconfined fallback. Conversion, rasterization,
version commands, their status and timing are recorded in the manifest.

Success prints one `bench.workbook-visual-context/v1` object and writes identical
`manifest.json` bytes after publishing and rechecking all assets. Existing
request/source/workbook/spec/QA/result and mechanical-evidence hashes remain.
The top-level `pdf` binds the PDF bytes and complete sheet/page map. `cells`
contains exact text, prior value/type/formula and one image ID per target.
`crops` retains its compatibility name but means a full sheet page, with PNG and
layout-metadata hashes. The layout JSON is `bench.workbook-page-context/v1`:
page identity/dimensions, cell addresses, neighboring saved rows and the first
three nonempty rows as header candidates. These candidate rows are orientation
facts, not a claim of inferred headers. Inventory ranges are not pixel geometry.
Input paths are relative to work; PDF, image and metadata paths are relative to
`output_directory`.

The helper makes no model call and gives no visual verdict. A reviewer must
locate every target using the actual image and supplied exact cell/text and
row/header context. Ambiguous localization must remain uncertain. A full-sheet
PDF can end at its used-content boundary: **no blank guard is guaranteed**, and
possible continuation beyond a page edge is uncertain, never proof of clipping.
Concrete clipping at an occupied neighboring cell remains visible evidence.

These pixels are LibreOffice print-rendering evidence, not Artifact Tool or
native Microsoft Excel behavior. Selecting this backend does not erase a
recorded renderer disagreement or prove that stored XLSX text was lost. The
separate selected Ask reviewer owns the narrow acceptance policy. Independent
full-workbook semantic/visual review remains necessary.

Exit 0 means complete preparation; exit 1 means preparation failed.
Official filter contract:
https://help.libreoffice.org/latest/en-US/text/shared/guide/pdf_params.html
