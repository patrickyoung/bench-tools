# Reference bitmap stylizer

An adaptation of Image Concept: Bench Agent authors a checked editing request
inside its default no-network Cage. A **caller-owned host** adapter subsequently
runs one explicitly selected existing image generator, then a deterministic Go
PNG normalizer/compositor and independent final check. No runtime, provider SDK,
model client, retry, recursive agent, listener or scheduler is added.

With protection, the result is a **hybrid bitmap**: generated artwork plus original
raster text/data in protected rectangles. With zero regions it is a wholly
generative bitmap, explicitly recorded as having no protected regions. It is not vector; an editable SVG remains a
separate source master. An Inkscape worker can render that master before this
worker starts.

## Install and select capabilities

Requires Go >=1.22 for building (standard library only), POSIX sh, Agent, Brief,
Ply, Ask, Cage, Record and ordinary `cmp`, `mktemp`, `shasum` utilities. Keep Bench
commands on a trusted PATH. Install the helper outside both definition and run
workspaces. From the directory containing `expert/`:

    mkdir -p "$HOME/.local/libexec"
    (cd expert/bitmap && go build -o "$HOME/.local/libexec/bench-bitmap" .)
    export BITMAP_HELPER="$HOME/.local/libexec/bench-bitmap"

No build occurs inside any checker. Missing helper configuration is check status
2 (broken setup); invalid/missing artifacts return 1. Keep the binary paired
with this reviewed source; no executable is fetched automatically.

Before full execution select both capabilities explicitly:

    export BITMAP_AUTHOR_MODEL='YOUR_EXPLICIT_ASK_MODEL'
    export BITMAP_GENERATOR='/absolute/path/to/existing/image-generator'

The backend must be a regular executable, outside writable work. Its existing
contract is stdin JSON `{prompt,output,references}`, with absolute output and
reference paths; it writes a PNG at output, emits one nonempty JSON provenance object on
stdout, and diagnostics on stderr. It must report exact nonzero failures. The
caller owns generator installation, auth and network access. Record bounds the
generator call to eight minutes by default; set `BITMAP_GENERATOR_TIMEOUT` to
an explicit duration when the selected capability requires another deadline.
The existing Codex image-tool executable is compatible; this definition neither
copies that implementation nor chooses a provider or credentials. It does not
inspect credentials. Avoid secrets in prompts/provenance/diagnostic streams,
which are retained without redaction.

## Inputs

Use an existing private WORK with `inputs/source.png` and `brief.json`:

    {
      "version": 1,
      "source": "inputs/source.png",
      "style": "Expressive anime contours, cel planes and hand-painted lighting.",
      "preserve_regions": [
        {"x": 20, "y": 30, "width": 120, "height": 40, "label": "Factual caption"}
      ]
    }

This rectangle example needs a source at least 140x70 with that actual caption
position; do not blindly apply it to another image. Rectangles use integer
original pixel coordinates, top-left origin, half-open extents. All five
rectangle fields are required. Overlap and omitted/empty regions are allowed;
the union cannot cover the whole frame. With zero regions no exact text/data
preservation is promised. No unknown JSON fields or duplicate keys are accepted.

Limits: encoded generator input <=40000 bytes (including JSON escapes and paths);
brief 64 KiB, style 1..8000 UTF-8 bytes, <=256 regions, nonblank labels
<=256 bytes, source path <=1024 bytes; complete PNG <=32 MiB, dimensions each
<=8192 and total <=8,388,608 pixels. No symlinks in any selected path or ancestor,
no path escapes. Paths to WORK/CONTROL must be clean, absolute physical paths
(use `pwd -P`; on systems where /tmp is a symlink use its physical location).
WORK and CONTROL must be disjoint. CONTROL must not exist; its parent must
already exist. Do not concurrently edit work or control during execution.

## Run with Bench

From the directory containing this definition, with the environment above:

    expert/bin/stylize /absolute/physical/WORK /absolute/physical/NEW_CONTROL \
      > /absolute/physical/artifact.json

Run this from the operator's host boundary, **not inside an Agent action**.
It snapshots selected inputs, runs exactly one bounded Agent invocation
(10 turns, one cycle, 10-minute timeout; no network flags), verifies unchanged
inputs, and calls the generator once via public Record. There are no fallbacks
or automatic retries. Agent state is under WORK/state; evidence and frozen
inputs are in CONTROL, outside Cage's writable workspace. Agent stdout/stderr/
exit are retained there. The generator works in a fresh CONTROL/generation,
with its raw PNG, stdout provenance, diagnostics, exact exit and Record archive.
Nonzero Agent/generator status propagates, including unknown/parked statuses;
recorder failures remain 125. Inspect evidence before choosing a new run.

For request-only Unix or bounded subagent use the **same** definition:

    agent run -C /absolute/physical/WORK \
      -state /absolute/physical/WORK/state \
      -evidence /absolute/physical/REQUEST_CONTROL \
      -m "$BITMAP_AUTHOR_MODEL" -turns 10 -cycles 1 -timeout 10m \
      /absolute/path/to/expert -- \
      'Read brief.json and its source; author the bitmap editing request only.'

This does not generate an image. `bin/check`, run in WORK, validates just the
source/brief, bound request shape and composition notes and reports
**"request ready; image not generated"**. `hire verify expert` is structural
only. No MCP/API interface is introduced.

Agent writes `output/request.json` with exactly:
`prompt`, `references: [brief.source]`, `brief_sha256`, `source_sha256`;
plus `output/image-notes.md`. Hashes bind exact selected file bytes. The adapter,
not the model, chooses generator paths: the frozen source is its only reference,
and CONTROL/generation/raw.png is its only admitted output. Worker-generated
code is never executed by the adapter/checker.

## Outputs and checks

On success stdout is a single artifact JSON pointing to retained:
- `snapshot/brief.json`, `snapshot/source.png`, `snapshot.json`;
- `request.json`, `image-notes.md`, `generator-input.json`;
- `generation/raw.png`, `generation/provenance.json`, `generation/record.jsonl`,
  `generation/stderr.log`, `generation/exit-status`;
- `final.png`, `receipt.json` (hash bindings, source/raw dimensions,
  normalization method, labeled regions and editable changed-pixel count).

Final PNG uses deterministic 16-bit color samples (alpha when needed), with
source sample values preserved
in rectangles (8-bit source samples expand to 16-bit by multiplication by 257).
Source PNG bytes and metadata remain in the snapshot; final encoding need not
retain ancillary PNG metadata or source compression.

A generated aspect-ratio change >1% relative to the source is rejected, never
silently stretched. Within that tolerance, dimension differences use separable
Lanczos-3 resampling with antialiased downsampling, premultiplied-alpha filtering
and clamped edges, then restore source rectangles exactly. Unchanged dimensions use an
identity normalization record. The final must differ on at least one editable
pixel; this is a nontrivial-output check, **not evidence of good style**.

Independent final verification (no model/backend invoked):

    expert/bin/check-output /absolute/physical/WORK /absolute/physical/CONTROL

It decodes complete bounded PNGs (including chunk CRCs/IEND), revalidates current
brief/source against snapshots, checks current request and notes, recomputes all
expected normalized/composited pixels from raw and original, compares every final
pixel and dimensions, and recomputes receipt hashes. It also verifies the public
Record archive, successful recorded exit, generator stdin and stdout provenance.
Calling the Go helper's `check` alone omits that public Record verification and
is not full acceptance. The helper does not implement a recorder.

The caller owns trusted CONTROL, installed tools and definition. Hashes and
Record seals detect drift, not a malicious host able to rewrite all archives.
The no-symlink checks are not host-read isolation or protection against hostile
concurrent rename races. No portable archive relocation is promised: original
absolute workspace/snapshot bindings remain authoritative.

## Review limits and validation examples

The checker does not grade aesthetics, interpret facts, verify prompt semantics,
prove the backend actually followed instructions, or establish studio endorsement.
Review source, raw and final side by side: identity/layout, actual factual text,
palette, silhouette, expressive treatment, margins and seams. Plain rectangle
restoration may create hard seams if the surrounding generated background changed.
Unprotected typography may be wrong. No universally invisible blending is claimed.

Positive offline example: colored artwork with a raster label; a fake backend
changes both; compositor keeps changed artwork and exactly restores labeled
pixels. A second fresh brief changes style and region selection and binds
independently. Negative examples: unchanged generator output, full-frame or
out-of-bounds protection, missing/truncated PNG, changed request/source/receipt,
or success without a PNG must fail.

In the source library, the sibling `tests/` contains offline Go contract tests
and a Go fake backend/author; tests are excluded from worker exports. With Go,
Ask and Record installed, run `sh workers/bitmap-stylizer/tests/run.sh` from the
repository root, or the catalog's `--entry bitmap-stylizer` suite. The tests
never invoke a real model or generator. Full real Agent authoring and image
quality need a separately selected fresh-workspace trial; fixtures do not prove them.
