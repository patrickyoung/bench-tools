# Evaluation recipe
Run from the build parent, with the expert path passed explicitly:
    python3 tests/run.py expert
    python3 tests/run.py expert --native --evidence validation

No models, agents, external assets or GUI calls. Run inside the normal action
boundary. `fixture_layout.py` is a single synthetic SWOT source (not distributed
as worker templates) and is explicitly executed by the test driver; generated
workspace scripts are never executed by the trusted expert checker.

Offline: request edges/duplicate keys/execution fields; selector hashes and
uncertainty; plan preservation/ceilings; tree probabilities/EV, BCG area,
matrix scores, MoSCoW totals and accountability; vector/active/raster/URL safety.
Native: actual finish→check light/dark/thumbnail/grayscale package plus missing/
active/raster/external/symlink/non-outlined/stale-source/plan/layout/selector
failures; independent overflow, native card overflow/label collisions;
wrong master/PNG pixels; dark geometry; honest conflicts vs tool failure.
Receipt rehashing in adversarial tests deliberately isolates the independent
checks instead of getting only an easy stale-hash failure.

Keep observed logs and native artifacts outside expert/. Use a fresh evidence
directory for each native run. Copied receipts name their original temporary
workspace; to check at a new location rerun the public finisher, never patch
receipts outside adversarial tests. No live model or image-view review is run.

Operator reproduction test for fresh jobs: in a disposable Cage, copy only
layout.py, its documented JSON inputs and design-notes.md into a new directory;
record master/plan hashes, rerun Python from a DIFFERENT cwd and compare bytes.
Native finish is a separate step. The checker intentionally does not do this.


Provenance/medium regression (bounded correction):
- Brief-only Owner/Document date quotes, structured precedence, mixed sources,
  legacy/no metadata and genuine absence; malformed/oversize/source/value/quote
  claims reject. Single footer, wrong/prefix values and duplicate/API footer
  reject. Exact quotes do not prove semantic attribution; that remains review.
- Ordinary slides/dark decks, negation, independent positive cue, light-only,
  no dark companion, canvas light, and boolean overrides in both directions.
- Native fixture regenerated with brief-only provenance + negated slide/light
  request passes finish/check without dark outputs; unsupported claims reject
  even after receipt rehash. Structured legacy provenance also passes natively.
Run all without models or touching pinned exports:
    mkdir -p validation/correction-tmp
    TMPDIR="$PWD/validation/correction-tmp" python3 tests/run.py expert --native --evidence validation/correction-native
    brief lint -strict expert/skills
    hire verify expert
