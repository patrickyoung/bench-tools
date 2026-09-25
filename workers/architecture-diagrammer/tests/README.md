# Architecture Diagrammer verification

These synthetic tests are not exported with the worker. Keep dependencies and
all generated evidence outside the source checkout. The core suite needs only
Python 3.9+ and does not call a model, browser or network:

```sh
python3 -I workers/architecture-diagrammer/tests/test_offline.py
python3 scripts/check-worker-evaluations.py --profile core --entry architecture-diagrammer \
  --out /absolute/fresh/evaluation-records
```

The 72 test methods include valid flowchart/sequence/needs-input packages,
model/source mismatch after hash rebinding, reversed/missing arrows, stale inputs,
closed schemas, ID preservation and alias collisions, qualifiers, cross-view
coverage, unsafe paths/links/files, SVG/CSS rejection, PNG format checks, read-only
checking, and separate definition failure (exit 2) versus bad job data (exit 1).
A deliberately fabricated render receipt documents that format/hash acceptance
cannot prove actual renderer execution or visual quality.

For actual local rendering, install the expert's pinned package.json/package-lock
into a separately selected runtime with `npm ci --ignore-scripts`, and provision a
compatible chrome-headless-shell. Select absolute RUNTIME, NODE and BROWSER paths.
Create a fresh external RESULTS directory, then run from the repository root:

```sh
mkdir -p "$RESULTS"
cage -w "$RESULTS" -- python3 -I workers/architecture-diagrammer/tests/test_render.py \
  --runtime "$RUNTIME" --node "$NODE" --browser "$BROWSER" --out "$RESULTS/ordinary"
cage -w "$RESULTS" -- python3 -I workers/architecture-diagrammer/tests/test_render.py --qualifiers \
  --runtime "$RUNTIME" --node "$NODE" --browser "$BROWSER" --out "$RESULTS/qualifiers"
cage -w "$RESULTS" -- python3 -I workers/architecture-diagrammer/tests/test_render.py --long-participants \
  --runtime "$RUNTIME" --node "$NODE" --browser "$BROWSER" --out "$RESULTS/long"
```

Do not nest Cage inside an already confined Agent action on macOS. The test
invokes the renderer's explicit outer-Cage mode; that flag is an attestation,
not confinement. The host commands above establish the boundary. No network
permission or automatic browser fallback is used.

Set TMPDIR and TEMP to a writable directory under RESULTS before testing,
including offline tests. Inside an existing outer Cage omit the cage prefix.

The seven render probes exercise Dagre, ELK and sequence with ordinary and qualified
models, punctuation, stable IDs, visible literal text, accessible metadata,
and a three-participant long-header/attribute regression. Exact full attributes
must appear in attributed frame text, not participant labels. Header text must
match only name/kind/state/evidence marks and fit actual browser actor bounds.
The compiler preserves full model bytes and detailed alternatives. Probes cover
image extents and structural checks. They retain each exact model, Mermaid,
SVG/PNG, receipt and log. Open the actual images and apply expert/QUALITY.md;
these assertions cannot prove all overlaps, aesthetics, source entailment or
receipt authenticity. Fresh Agent jobs are separately selected live evaluations,
not part of either deterministic suite. No hidden holdout or universal rendering
benchmark is claimed.
