# Inkscape-controlled illustrator — experimental

One filesystem expert for polished vector illustration. Input and final handoff
match the baseline `inkscape-illustrator`; the handoff specialist identifier stays
`inkscape-illustrator` for side-by-side evaluation. No team is needed: design is
one coherent responsibility, while constrained authoring/export are deterministic.

## Run

Provide `request.json` in a separate workspace: nonempty `description` (max
12000 characters), optional `style` (max 2000, or null), optional integer `width`
and `height` (256–4096; defaults 1200×900). Unknown fields are rejected.

    agent run -C WORKSPACE -evidence EVIDENCE_ROOT /absolute/path/to/expert -- Create the requested illustration from request.json

Same invocation serves a bounded subagent assignment; pass actual request bytes
or an accessible input file. Keep the definition outside WORKSPACE. Preserve exit
status and Agent's evidence recordings. Do not use `-net` or `-no-cage`.
No listener, provider client, scheduler or second model is supplied.

Prerequisites: Bench Agent/Ask/Brief/Ply/Cage/Record, Python >=3.9 standard library,
and installed Inkscape >=1.2. `INKSCAPE` may select an absolute executable, else
PATH is used. No installs or downloads occur. Operator supplies these capabilities
and a no-network Cage boundary. Helpers alone are not sandboxes.

## Authoring contract

The model writes `output/inkscape-plan.json`, never raw SVG. See the bounded schema
in `references/plan-schema.md`. Run `tools/author_in_inkscape`, then `tools/finish`,
then `bin/check`, from the workspace. All commands take no arguments.

A fixed Inkscape-native namespace DOM adapter constructs layers and constrained
vector objects, invokes installed Inkscape to open/serialize the document, and
normalizes only generated editor metadata for the conservative baseline contract.
It uses stdlib DOM facilities, not the optional inkex package. The model controls
the Inkscape document through this adapter; it neither drives a GUI nor hands off
a raw SVG as the accepted artifact. Never hand-edit the generated master.

Each authoring/regeneration run has private temporary profile/XDG/temp directories,
120-second subprocess timeouts and process-group termination on timeout. HOME is
preserved; Cage may deny native platform cache writes. No network-bearing plan
operations exist. Receipt includes request, plan, master, adapter source bundle,
executable and version digests, literal argv, exit outcomes and diagnostics.
Definition files are trusted read-only inputs; workspace receipts are untrusted.

Outputs: `output/illustration.inkscape.svg`, `output/illustration.svg`,
`output/preview.png`, `output/design-notes.md`, `output/render.json`,
`output/inkscape.log`, `handoff.json`, plus the plan and authoring receipt.
Native export outlines text and preserves vector-only art. Notes cover intent,
composition, palette, style, simplifications and honest review limits.

## Checks and limits

The check independently regenerates the layered master from the current plan and
requires byte equality. It verifies all authoring bindings and command shapes,
then retains the baseline conservative SVG rules, request-bound full manifest,
native independent exports and exact decoded pixel comparisons. `finish` also
requires valid authoring first. Missing, malformed, stale, unsafe and divergent
artifacts fail. Changing reported hashes cannot authorize a different master.

Receipts cannot cryptographically prove historical execution; identical valid
artifacts with a fabricated identical history are indistinguishable without
trusted external evidence. Regeneration establishes reproducibility, not intent.
Structural checks cannot judge semantic accuracy, style fidelity or polish.
Review the preview at full and thumbnail sizes. Fonts remain host-dependent.
Unsupported plan effects/characters require simplification or an unfinished report.

## Deterministic evaluation

From the Bench source checkout (tests are deliberately outside the reusable
definition):

    python3 scripts/workers check
    PYTHONDONTWRITEBYTECODE=1 python3 workers/inkscape-illustrator/tests/contracts.py

The suite's synthetic positive example is a cream field and blue Bézier silhouette
on two named layers, at 256×256. Negative examples inject markup, unknown operations,
out-of-range geometry, stale receipts and master changes with forged hashes.
It also fault-injects divergent regeneration. All art lives in temporary workspaces.
No real brief, model transcript, credential, dependency or generated artwork belongs
in this definition. Keep model-backed evaluation evidence outside the source
library.

Keep the native contract when extending the schema: add bounded declarative
validation and fixed DOM construction, document it, then add both positive and
negative tests. Do not add another runner or turn code into plan input.
