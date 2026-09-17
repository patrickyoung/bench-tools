# Inkscape Decision Diagrammer
Independent worker **inkscape-decision-diagrammer**. One Agent, per-job stdlib
geometry, focused native export/check helpers; no subagents, services, model
clients, universal template app or runtime sibling dependency.

## Run
Prerequisites: Bench Hire/Agent/Brief with their normal action Cage; Python ≥3.9;
local Inkscape ≥1.2 and a usable font with generic fallback. No remote assets.
Operator supplies `INKSCAPE=/absolute/executable` only to override PATH.
Read installed `agent help`, `hire help`, and Inkscape `--version`/`--help`.
Model, permissions, turn budget and image viewing are operator-managed.

Use an existing separate workspace containing request.json, optionally the three
selector files. From your shell (substitute your actual absolute paths):

    EXPERT=/absolute/path/to/inkscape-decision-diagrammer/expert
    WORK=/absolute/path/to/job
    EVIDENCE=/absolute/path/to/job-evidence
    agent run -C "$WORK" -evidence "$EVIDENCE" -turns 40 "$EXPERT" \
      -- "Read request.json and any selector handoff; produce the bound decision package."

Do not add `-no-cage` or network access. Preserve Agent stdout, stderr, exit
status and evidence; `-record-input request.json` and
`-record-output output/result.json` may be added using the public Agent interface.
A separate operator Agent invocation can use the same definition as a bounded
worker, but this expert never recursively calls Agent or another model.
No MCP/API/listener is exposed or needed.

The reusable definition/check remains outside mutable workspace. Helper commands,
from the workspace, AFTER job geometry is authored/executed inside Cage:

    "$EXPERT/tools/finish" --previews
    "$EXPERT/bin/check"

Omit --previews to omit support conversions. `finish --needs-input` binds only a
diagnosed conflict package. `bin/check` does not write the workspace or execute
layout.py; temporary native profiles/exports are removed after independent checks.
Do not invoke unreviewed job geometry from a trusted controller.

## Practical input
Positive example (only brief required):

    {"brief":"Workshop SWOT: decide where to investigate next. Existing support is an internal strength, slow onboarding an internal weakness. No evidence for external opportunities or threats; leave those regions empty. Audience: operations leads. Owner and date unspecified."}

Optional width/height default 1600/1000, bounded 640–4096. Slide/dark brief
triggers same-geometry dark output unless dark:false. No invented owner/date.
The selector consumer preserves `bench.diagram-brief/v1`; see CONTRACT.md and
references/selector-contract.md for exact identity/binding/placement rules.

Negative semantic example:

    {"brief":"Make a decision tree for the launch choice. Supplied chance outcomes: win probability 0.7, lose probability 0.6. Do not change either input."}

Expected: exact 1.3 probability-sum conflict, questions and bound needs-input
report, no diagrams. Do not silently normalize. A negative technical example is
a valid package whose layout.py, selector raw response or master changes after
render: check exit1, not a rewritten successful receipt.

## Outputs and status
CONTRACT.md defines every file/schema and arithmetic relationship. Rendered output
includes layered editable master, genuinely outlined plain SVG, exact native PNG,
deterministic layout.py and retained inputs, inspectable plan, notes, log and
bound render/result receipts. Dark companions only on trigger. Optional full
grayscale and thumbnail previews are deterministic and independently checked.

Check exit0 accepts a **package**:
- `visual-review-pending`: native rendered correspondence checked; human review
  still required, not an accepted design.
- `needs-input`: complete truthful semantic report only, not an accepted diagram.

Check exit1 rejects/unfinishes; other exit indicates broken checker. Missing or
failing Inkscape is unfinished and invalidates previous render/result receipts;
never needs-input. Helper and Agent exit meanings differ; inspect result.json as
well as Agent's exit. No approval or business action follows from any status.

## What the checks prove
Regular bounded files; conservative SVG/no active or external/raster content;
live layered master/outlined final; PNG CRC/decompression/nonblank/dimensions;
strict request and selector binding; current source-plan-script-notes binding
embedded in masters in addition to receipt hashes; source item/relationship
coverage and unknown tray; ceilings and declared framework arithmetic.
Real `--query-all` rows are parsed and every queried object's bounds rejected
outside the page. Master AND outlined exports are independently re-queried.
Live text extents check label pairs, declared card containment and disjoint
relationships (not naive parent/child or bubble collisions). Declared flat
surface contrast uses actual paints/bounds; typography limits are checked.
Footer presence and content require human review. Dark masters must have identical geometry/text/typography.
Native independent master export and final render pixels must each equal the
supplied PNG. Preview correspondence is checked when present.

## What they cannot prove
Semantic truth/evidence sufficiency, a genuinely supported conclusion, good
framework selection, optimal routing/space, all craft judgments, hidden causal
assumptions, honest overlap exemption prose, human legibility or design quality.
Soft Cynefin and Kano semantics, strength judgments, grayscale/deuteranopia
perception, nonrectangular occlusion and loaded framing require image review.
The contrast guard supports opaque rectangular actual surfaces; use flat
backgrounds and external bubble labels, not unverifiable complex compositing.
No claim is made that a hash verifies code behavior or that bounds mean “viewed”.
Reproduction of a job's arbitrary script must be tested separately in disposable
Cage, not by bin/check. Native pixel equality requires the same usable Inkscape/
font environment; differing versions/fonts can legitimately require re-export.

## Evaluation and extension
Tests live NEXT TO expert/ (not in the reusable definition):
    python3 tests/run.py expert
    python3 tests/run.py expert --native --evidence validation

Offline mode requires stdlib only. Native mode exercises one small synthetic
SWOT light/dark fixture through the actual finish/check commands plus adversarial
failures; no model calls. Native outputs/evidence are outside expert/.
See tests/README.md for the reusable recipe and limits.
All 17 grammars are documented; fixture success does not behaviorally test all
17 or validate a live model's design quality. Fresh model-backed cases and human
image review are deliberately left to the operator.

To extend grammar, add an original evidence/axes/ceiling entry to the skill and
a focused arithmetic/placement check if needed, with positive/negative tests.
Do not change Agent or add a renderer/template application. This job needs no
specialist contexts; if future work genuinely needs another specialist, that is
a separately approved definition/controller change, not silent runtime fan-out.

Original MIT attribution is retained in LICENSE and PROVENANCE.md.
