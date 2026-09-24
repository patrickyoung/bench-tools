# Vector Style Studio

Experimental fixed Bench team: one editable vector illustration, its unchanged
native bitmap render, and 1–8 independent bitmap treatments of that same render.
Styles and subject matter are entirely case inputs. MIT; see LICENSE.

## Assemble and select capabilities

The source template has no child copies. Library export uses the team.json roster:
`vector -> inkscape-illustrator`, `bitmap -> bitmap-stylizer`. Reviewed member
baseline: `e823f19f07a65db939183b11b64d84b505a3fd11`. Runtime export must contain
unchanged definitions at `expert/agents/vector` and `expert/agents/bitmap`.
They remain independently runnable; read both complete member READMEs.
Missing assembled members cause an explicit runtime setup error.

After curation at a reviewed full commit, use the catalog's public export:

    python3 scripts/workers export-team vector-style-studio /absolute/new/export \
      --ref FULL_REVIEWED_COMMIT --allow-experimental

The new team must first be committed; the historical member pin does not yet
contain this team. Export pins roster, wiring and members together. Build the
existing bitmap helper outside definitions and runs exactly as its README says;
the team neither installs it nor contains a Go module.

Requirements: POSIX sh, jq 1.6+, xmllint, cmp, shasum, standard Unix file tools;
public Agent, Ask, Brief, Ply, Cage, Record; Python 3.9+ for unchanged vector
helpers; Inkscape 1.2+ (tested coordinate query on 1.4.4); the existing bitmap
helper built using Go 1.22+ and an explicitly admitted compatible generator.
Keep trusted commands/definitions outside run directories. No credentials are
read by the wiring, no model client/SDK, Weigh, service or new permissions layer
is introduced. Preserve inherited AGENT_ASK/PRESENT_ASK and HOME.

Select capabilities explicitly, with no embedded provider/model defaults:

    export VECTOR_STYLE_MODEL='YOUR_SELECTED_VECTOR_ASK_MODEL'
    export BITMAP_AUTHOR_MODEL='YOUR_SELECTED_BITMAP_AUTHOR_ASK_MODEL'
    export BITMAP_HELPER='/absolute/physical/installed/bench-bitmap'
    export BITMAP_GENERATOR='/absolute/physical/installed/image-generator'
    export INKSCAPE='/absolute/installed/inkscape'

INKSCAPE may be omitted to use installed `inkscape` on trusted PATH.
Generator stdin/stdout, protection, normalization and Record contracts are the
unchanged bitmap member's. Its default generator timeout is 8m, overridable
with BITMAP_GENERATOR_TIMEOUT. Each author invocation is 10 turns, one cycle,
10m. Native vector check operations and team bounds query have 2m deadlines.
There is no retry, fallback, parallel fan-out or scheduler.

## Real host entry and independent check

From an **authorized caller-owned host boundary**, not from an Agent action:

    /absolute/export/expert/bin/studio \
      /absolute/physical/current-job.json /absolute/physical/new-run \
      > /absolute/physical/artifact-manifest.json

The run must not exist, even empty. Its parent must already exist. Use physical
paths (`pwd -P`); paths containing spaces are supported. Job/reference/run paths
must be absolute, <=1024 characters, without symlink ancestors, backslashes,
control characters, empty components, `.` or `..`. Runs belong outside the
definition. Source files are regular; the optional SVG is <=16 MiB, well-formed,
SVG-rooted and without DTD/entities. No implicit previous run is selected.

Only a successful manifest JSON reaches stdout. Diagnostics go to stderr;
member streams and exact statuses remain in the run. Nonzero upstream Agent
and stylizer statuses propagate unchanged, including 75/42. Invalid inputs are
status 1; missing configuration/members is 2. Partial runs are never overwritten.
Check without any model or generator call:

    /absolute/export/expert/bin/check /absolute/physical/new-run

A no-argument check uses the physical current directory for Agent prechecks.
Do not run the team's full flow via `agent run`; its host entry owns composition.
`hire verify expert` checks structure only, even without assembled members.

## Version-1 job

Exactly one UTF-8 JSON object, <=1 MiB; duplicate keys and unknown fields rejected:

- `version`: 1.
- `vector_request`: the unchanged Inkscape request schema: `description` string
  (1..12000 characters), optional `style` null or string (1..2000), optional
  integer `width`/`height` (256..4096; defaults 1200/900). Combined bitmap limit
  adds <=8,388,608 pixels. No NUL strings. Encoded request must fit the member's
  64 KiB limit.
- Optional `source_svg`: explicit absolute editable SVG reference path. Copy goes
  to vector/work/inputs/source.svg with an external snapshot/hash. The caller's
  description should identify it as selected reference. Agent must still author,
  finish and pass its checker; reference is never promoted directly to output.
- `preserve_regions`: required array, possibly empty; <=256 exact objects
  `{x,y,width,height,label}`. Integers in original PNG pixel coordinates,
  top-left origin, half-open extents, positive sizes, wholly within frame.
  Nonblank labels <=256 UTF-8 bytes. Overlaps allowed; union must not fill frame.
- `required_text`: required array, possibly empty; <=256 exact `{id,text}` pairs.
  Unique IDs match `[A-Za-z][A-Za-z0-9_-]{0,63}`. Nonblank exact text <=8000 UTF-8
  bytes. Require one actual SVG text node with that ID in the editable master,
  exact concatenated descendant text content, and one positive finite queried
  final-SVG bounding box wholly contained in a single protected rectangle.
  Use stable IDs retained by Inkscape outlining. Bounds come from an independent
  actual Inkscape query, not caller/worker-authored coordinates. Missing, duplicate,
  zero-size, invalid or uncovered bounds reject before bitmap work.
- `styles`: 1..8 exact `{id,style}` objects. Unique IDs match
  `[a-z][a-z0-9-]{0,31}`, excluding reserved `original`. Nonblank style string
  <=8000 UTF-8 bytes; preservation/composition directions may be included here.
  Each string is preserved verbatim as a JSON value, without team suffix/prefix.
  Exact preserve_regions also passes through unchanged. Encoded per-style brief
  must fit the member's 64 KiB bound.

For art-only work explicitly supply empty required_text and preserve_regions.
For lettering, the vector goal supplies required text and caller-selected
rectangles alongside the unchanged vector_request. If these cannot be satisfied
together, the run stops; the caller must revise composition or rectangles.

## Files, provenance and acceptance

    control/                 frozen job/reference, admission hashes, vector goal/status/bindings
    vector/work/             request, optional reference, state, checked vector outputs/handoff
    vector/evidence/         public Agent evidence, outside writable vector work
    styles/ID/work/          brief, canonical input PNG, member request/state
    styles/ID/control/       member snapshots, raw PNG, receipts, final PNG, Record evidence
    styles/ID/stylize.*      captured stdout, stderr and exact exit
    result/original.inkscape.svg
    result/original.svg
    result/original.png
    result/ID.png
    manifest.json

The editable master and portable outlined SVG are vector artifacts. **Every PNG
is wholly raster**, including protected lettering. Original PNG is exactly the
checked worker's native preview.png bytes; it receives no model/filter,
enhancement or normalization. Each backend receives the same SVG-rendered PNG
pixels, not vector paths. Each style is independent, never a chain.

The checker has these hard criteria:
- INPUT: current selected job/reference, external snapshots, hashes and vector
  input bytes agree; missing/stale/unselected references fail.
- VECTOR: original member check independently establishes master/final/preview
  correspondence; initial checked output bindings remain unchanged.
- TEXT: exact master lettering and independently queried bounds meet protection.
- BASELINE: published original files byte-match the checked vector outputs.
- VARIANT: exact expected brief and same canonical source for every style;
  original bitmap check-output verifies pixels, receipts and Record evidence.
- MANIFEST: recomputed fixed paths/hashes/data flow/review labels match exactly.
  No run-supplied script is ever executed.

Trusted definition/checker/tool code and control folders must remain outside
worker write permission. Cage allows host reads; use an operator-selected
account/container if prior cases must be unreadable. Do not concurrently modify
the job, reference, runs or tools. Snapshots/hash bindings detect drift, not an
adversarial host that can rewrite every receipt. Original absolute bindings
remain authoritative; retain selected upstream inputs and do not relocate runs.
The check uses disposable native renders/queries outside the run; installed
Inkscape may attempt native cache writes/warnings. It does not alter case files.

## Review and examples

Acceptance is limited to byte bindings, geometry coverage and component
deterministic contracts, **not aesthetics, style resemblance, actual visible
lettering, unprotected factual fidelity, endorsement, or seam quality**.
No visual model is silently selected. Semantic/visual review is always pending:
caller inspects baseline and all variants full-size and thumbnail, with actual
rendered-image perception if desired, and retains review evidence outside source.

Positive offline example: a synthetic caption lies inside its rectangle, two
different literal styles each receive the same original bytes and pass all
bindings. Negative examples: duplicate/unsafe style ID, mismatched or uncovered
caption, changed baseline, chained style source, refreshed wrong manifest, or
member failure must reject. In the source repository, run
`sh teams/vector-style-studio/tests/run.sh` from the repository root.
These repository-only tests and build notes are not included in clean exports.
The tests assemble explicitly fake capabilities and run the real entry/check,
starting from either an unassembled template or an assembled export. Fixtures
use marker bytes, not genuine PNGs: they prove wiring only. No model or paid
image generation runs. Live component integration and visual review remain
caller responsibilities.
