# Workbook contract and optional visual tests

All tests use synthetic inputs or explicitly supplied loopback replies. Tests,
fixtures and their documentation are separate from expert/ and never exported.
Run them from a disposable source/test staging copy with its adjacent expert/
definition. Some visual suites create temporary directories beside themselves;
keep that staging copy outside the owning source library and clean exports.
All persistent engine/model evidence must use new external directories.

The dependency-free group has 63 Python 3.9+ standard-library tests: the 46 prior
creation/preparation/preservation/Date cases, 11 visual-gate unit cases and six
saved-native-feature evidence cases.
With TESTS pointing to this directory in the disposable copy, run:

```sh
(
unset WORKBOOK_VISUAL_ASK WORKBOOK_VISUAL_MODEL WORKBOOK_VISUAL_RECORDS
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$TESTS" -p test_spec.py -v
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$TESTS" -p test_edit_context.py -v
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$TESTS" -p test_renderer_repairs.py -v
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$TESTS" -p test_validator_diagnostics.py -v
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$TESTS" -p test_date_metrics.py -v
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$TESTS" -p test_visual_check.py -v
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s "$TESTS" -p test_saved_native_features.py -v
)
```

The subshell clears optional reviewer selection only for these tests; it does
not change the caller's persistent configuration. Legacy public-check fixtures
inherit the environment, so do not omit this isolation when a reviewer is set.

The separate JavaScript file has nine assertions and needs Node, without Artifact
Tool, a model or networking:

```sh
"$WORKBOOK_NODE" "$TESTS/test_metric_values.mjs"
```

The retained test files cover:

- test_spec.py: 10 creation-contract binding and rejection cases.
- test_edit_context.py: 7 lossless compact-context, stale/unsafe input and skeleton cases.
- test_renderer_repairs.py: 8 narrow conditional-fill, preservation and invalid-range cases.
- test_validator_diagnostics.py: 8 precise matrix/scope/metric validation cases.
- test_date_metrics.py: 13 strict typed/ISO Date, saved 1900/1904, wrong-day, format,
  marker and invalid-date/blank-cell cases. Numeric/boolean/text distinctions remain.
- test_visual_check.py: 11 optional selection, exact finding coverage, pass/fail/
  uncertain handling, renderer identity and retained unknown-attempt cases.
- test_saved_native_features.py: 6 synthetic saved-XLSX table/header/chart-reference,
  hash-binding, empty-feature, explicit-omission and stale/failed-check cases.
  Observed references do not prove exact requested chart bindings or owner/type.
- test_visual_context.py: 14 environment-dependent synthetic saved-workbook,
  exact-bookmark, PDF/image-binding and real Cage write/network-denial cases.

The last suite needs explicitly selected WORKBOOK_VISUAL_SOFFICE,
WORKBOOK_VISUAL_PDFTOPPM, WORKBOOK_VISUAL_CAGE and WORKBOOK_PYTHON, with public
pypdf available in that selected Python. See VISUAL-TESTS.md. It makes no model
call. Full unittest discovery includes this suite and therefore requires those
dependencies; do not call full discovery a dependency-free check.

The explicit render_date_fixture.py uses the caller's WORKBOOK_NODE and
WORKBOOK_NODE_MODULES Artifact Tool runtime, generates a synthetic source, then
uses public render/check. It retains the Date assertion/mutation, accepts valid
typed/ISO expectations and rejects the wrong day. Select a new external path:

```sh
(
unset WORKBOOK_VISUAL_ASK WORKBOOK_VISUAL_MODEL WORKBOOK_VISUAL_RECORDS
PYTHONDONTWRITEBYTECODE=1 python3 "$TESTS/render_date_fixture.py" /absolute/new/date-evidence
)
```

The explicit exercise_visual_check.py additionally needs the selected visual
renderers, public Bench binaries, an Ask executable/bridge supporting the public
offline contract, and Artifact Tool for synthetic source authoring. It always
selects a loopback OpenRouter-compatible fixture, never a paid endpoint:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 "$TESTS/exercise_visual_check.py" \
  --out /absolute/new/visual-process-evidence \
  --bin /absolute/bench/bin --ask /absolute/selected/ask-or-bridge
```

Its ten local judgment cases cover pass/fail/uncertainty, incomplete/forged/
malformed replies and PDF/image/workbook tampering. It checks recorded assets,
selected Ask replay, unchanged-candidate cache reuse, retained broken attempts,
mechanical checks inside Cage, missing-renderer failure for actual targets and
renderer-free zero-scope/clarification outcomes. Fixture answers do not establish
vision accuracy. No paid/model-quality evaluation is implicit in these scripts.

The four existing tests/fixture files remain available for external creation
render exercises. Numeric/early-1900/1904 synthetic saved-file tests do not prove
every engine round trip. None of these checks establishes business correctness,
representative model reliability, lower cost, native Excel behavior or a general
visual pass. Do not promote historical workbooks, gold answers, model sessions
or generated evidence into this package.
