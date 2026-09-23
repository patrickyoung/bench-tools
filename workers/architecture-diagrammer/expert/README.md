# Architecture Diagrammer — reusable Bench worker

Independent Mermaid-first diagram specialist, not an EA/SA replacement.
Read AGENTS.md, PROFILE.md, CONTRACT.md and QUALITY.md. No sibling worker, case
data, research folder or source-library dependency is needed.

## Inputs and manual handoff
Controller creates a separate workspace with request.md and inputs/. Request
specifies audience, architecture question/scope, intended presentation/document
size and authoritative source selection. Copy only explicitly selected EA
architecture.json/report/catalog/roadmap or SA solution.json/design/standards/
controls into inputs/, preserving provenance and locators; include their referenced
artifacts if material. Controller resolves missing authority through those roles.
Do not import sibling code, scan sibling runs, or interpret a staged manifest as
access permission. Empty inputs/ is valid when request.md contains the evidence.

## Exact invocation sequence
Choose your own absolute paths and model through the existing controller. The
variables below are operator selections, not baked-in locations. Definition and
review evidence must be outside worker write access; WORK is a separate workspace.

    export DEF="/absolute/selected/architecture-diagrammer"
    export WORK="/absolute/selected/job"
    export EVIDENCE="/absolute/controller-owned/evidence"
    export MODEL="explicit-controller-selected-model"
    agent run -C "$WORK" -evidence "$EVIDENCE" -m "$MODEL" "$DEF" -- \
      "Read request.md and staged inputs. Author and compile the diagram model; stop at structural preparation."

Agent/Brief are only needed for the worker interaction. Tools are standalone:
Python >=3.9 stdlib, Cage for host rendering, Unix env supporting `-S` (or `python3 -I` explicitly).

    cd "$WORK"
    mkdir -p inputs output
    # Author output/model.json per CONTRACT, never runtime code.
    python3 -I "$DEF/tools/compile"
    python3 -I "$DEF/bin/check"

Compile creates deterministic .mmd, alternatives, coverage, report and manifest.
It invalidates earlier renders; preserve prior useful artifacts outside output
first. Do not edit generated Mermaid and rehash; edit model.json and regenerate.

Host/controller phase only, with explicitly admitted installed dependencies:

    export RUNTIME="/absolute/operator-selected/npm-runtime"
    export NODE="/absolute/operator-selected/node"
    export BROWSER="/absolute/operator-selected/chromium-executable"
    cd "$WORK"
    cage -w "$WORK" -- python3 -I "$DEF/tools/render" --outer-cage --runtime "$RUNTIME" --node "$NODE" --browser "$BROWSER"
    python3 -I "$DEF/bin/check" --rendered

Runtime must have package.json, package-lock.json, CLI 11.17.0, resolved Mermaid
11.17.2 and Puppeteer 25.11.0. Node >=22 and a compatible local Chromium executable
are required. Under restrictive macOS host confinement, explicitly select the
installed chrome-headless-shell executable; full Chrome may fail creating its
ProcessSingleton socket. There is no automatic browser fallback. package.json pins direct dependencies and package-lock.json pins the evaluated dependency graph: installation,
browser provisioning, full lock generation, dependency admission and fonts are
operator work OUTSIDE this definition and outside jobs. Do not copy node_modules
into the definition. No automatic npm/npx/download fallback, global changes or
uploads. Renderer uses the CLI's public renderMermaid API (including bundled ELK),
literal argv, local assets, request interception, disabled DNS/background network
and bounded timeouts. DevTools uses a pipe, not a listening port. Chromium's
inner sandbox is disabled for nested host confinement; the host Cage boundary
is REQUIRED, not optional. The required --outer-cage flag is an explicit
controller attestation, not sandbox detection or permission. Invoke the shown
Cage command from the host; when already running under a controller-selected
outer Cage, retain that invocation evidence and do not attempt nested Cage.
A flag alone does not establish confinement. Controller must deny OS network access: browser
flags/request interception are defense-in-depth, not a replacement for sandboxing.
Runtime packages and browser are trusted code admitted by the operator, not inputs.

## Independent review and production gate
Default check proves bounded structure, complete byte inventories and exact
model/source agreement. Render check additionally checks stage, version strings,
source/config/renderer/artifact hashes, safe SVG subset and PNG structure/dimensions.
Renderer compares expected literal graph labels with graph DOM text and full
attributed attributes with visible frame-note text (sequence participant and
boundary attributes live in frame notes; flowchart node attributes stay in labels).
Sequence headers are wrapped name/kind/state/evidence marks and must fit actor
boxes. The renderer rejects numeric escape artifacts and text outside graph/frame extents, and strips
only two exact unused bundled keyframe rules. Active animation, other at-rules,
external URLs and unsafe SVG remain rejected. Sequence's inert symbol definitions
are allowed; links/use/external resources remain forbidden.
Neither proves receipt authenticity, source entailment, raster correctness,
legibility, aesthetics or organizational approval. Check never launches a browser
or reads arbitrary runtime files from a receipt.

Controller retains its actual render invocation/result and trusted dependency/
browser identities outside WORK. Freeze the final package, independently hash
request/profile/definition, staged inputs and all outputs, and record the rubric
hash. For example, ordinary host tools can inventory immutable bytes:

    cd "$WORK"
    shasum -a 256 request.md output/*
    shasum -a 256 "$DEF/PROFILE.md" "$DEF/QUALITY.md"

Manifest contains sorted recursive input and authoring hashes; render.json binds
image hashes. Controller must compare those with its own snapshot, not trust
worker assertions. Reviewer opens actual final view-NN.png and SVG, at requested
size, using an image-capable viewer/controller (current Codex can inspect pixels).
Record concrete per-criterion findings under QUALITY.md with exact hashes,
reviewer identity, intended size and evidence in EVIDENCE, never output/.
No independent review file is accepted by bin/check: controller owns this gate.
Review record structure is described in QUALITY.md, not a worker approval schema.
Any model/source/layout/config/runtime/render change needs fresh review.
“Prepared” and “rendered-unreviewed” never mean approved or publication-ready.

## Positive and negative validation recipes
Positive: use CONTRACT's complete minimal model with a request actually supplying
the capability and owner; compile then check returns 0. A complete needs-input
example there also compiles/checks with no diagrams, but --rendered rejects.
Positive sequence: select existing logical participants and supplied ordered
request/response/async rows, LR/dagre, honest scenario conditions and arrow key.

Negative: change a compiled arrow from A-to-B to B-to-A, or delete an edge and
update manifest hashes: check still returns 1 because it recompiles the model.
A source input edit without recompilation, selected edge without both endpoints,
current/target mixture outside a transition view, output symlink or arbitrary
renderer option is also rejected. A self-consistent but false source model may
pass structurally and MUST fail independent source review.

Authoring repository tests stay beside, never inside, the reusable definition:

    export RESULTS="/absolute/fresh/results-outside-source"
    mkdir -p "$RESULTS/tmp"
    export TMPDIR="$RESULTS/tmp" TEMP="$RESULTS/tmp"
    python3 -I tests/test_offline.py
    cage -w "$RESULTS" -- python3 -I tests/test_render.py --runtime "$RUNTIME" --node "$NODE" --browser "$BROWSER" --out "$RESULTS/ordinary"
    cage -w "$RESULTS" -- python3 -I tests/test_render.py --qualifiers --runtime "$RUNTIME" --node "$NODE" --browser "$BROWSER" --out "$RESULTS/qualifiers"
    cage -w "$RESULTS" -- python3 -I tests/test_render.py --long-participants --runtime "$RUNTIME" --node "$NODE" --browser "$BROWSER" --out "$RESULTS/long"

When already inside the admitted outer Hire Cage, omit the `cage ... --` prefix;
do not nest Cage. Keep all test outputs and temporary files in the selected
external results directory (set TMPDIR and TEMP there before testing).

Offline fixtures test discrimination, not pixels. Integration tests render actual
SVG+PNG with Dagre, ELK and sequence. Neither is fresh production-case evaluation.
No paid judge or Weigh path. See tests' retained results in the authoring workspace,
not in this export.

## Known limits
Closed v1 supports flowcharts and focused ordered sequences, not formal modeling
conformance, arbitrary Mermaid, full branching/concurrency, automatic layout
optimization or design decisions. Sequence boxes have one containment level.
Small diagrams are intentional; source scope completeness is semantic, not a
graph-count proof. Inputs and outputs have finite bounds (CONTRACT). Render fonts
and browser versions affect pixels; receipts record identities but reproducibility
requires the operator's lock graph, browser and fonts. Do not claim visual quality
without current actual image review. Failed renders remain unfinished. Flow edge/boundary quotes, hashes and
entity-looking ampersands are unrepresentable in the selected runtime and fail
closed; node and sequence punctuation uses tested Mermaid escaping. Exact literal
browser checks may reject further unsupported cases rather than change labels.
Extent checks do not prove absence of overlaps, arrow occlusion or all glyph/font
issues: independent pixels remain mandatory.

Checker exit contract: 0 structural acceptance; 1 missing/malformed job or rejected
deliverable; 2 missing/unreadable/empty/non-UTF-8 immutable definition dependency or
broken imported helper/checker. The definition is read-only and controller-frozen.
No subprocess, browser or model is invoked by the verifier.

## Install the pinned rendering dependencies

Copy this export's package.json and package-lock.json into an external RUNTIME directory, then run `npm ci --prefix "$RUNTIME" --ignore-scripts --no-fund --no-audit`. Provision and explicitly select a compatible chrome-headless-shell separately. Dependency downloads happen during operator setup, never during a diagram job. Keep the exported source clean.
