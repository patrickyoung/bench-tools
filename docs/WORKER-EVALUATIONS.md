# Worker and team evaluation runbook

`make check-workers` runs the library's offline regression suites. All 24 workers
and three teams have mapped tests and separate quality cases in
[`scripts/worker-evaluations.json`](../scripts/worker-evaluations.json).
The runner uses **current working-tree bytes**, including uncommitted edits.
It makes no paid model calls and installs no dependencies.

The library already had substantial regression coverage, but much of it was
not in CI or a common command. The current catalog brings it together, tests
each publication worker's own shipped checker, expands planner/request and
vector cases, and exercises the comparison/studio entry commands with local
process fixtures. All entries remain experimental; a green regression run is
not a claim of representative model or artistic quality.

## Start here

From the repository root, inspect the plan without executing anything:

```sh
python3 scripts/check-worker-evaluations.py --list --profile all
```

For the default **core + analysis** run, select Python 3.12+ with the analyst's
pinned numerical dependencies, Node 22+, and Poppler's `pdftotext` on PATH.
Create a dedicated environment outside the checkout once:

```sh
python3.12 -m venv "$HOME/.local/share/bench/evaluation-runtime"
"$HOME/.local/share/bench/evaluation-runtime/bin/python3" -m pip install \
  -r workers/polars-analyst/expert/requirements.txt
make check-workers EVAL_PYTHON="$HOME/.local/share/bench/evaluation-runtime/bin/python3"
```

Dependency installation may download packages; the suites themselves use local
fixtures (one HTTP test uses loopback). Install Node and Poppler through your
normal platform setup. For the smaller standard-library/Node group:

```sh
make check-workers EVAL_ARGS='--profile core'
```

Focused runs select every matching suite in the selected profiles. Shared suites
may also test other roles. Unknown IDs and empty selections fail explicitly.

```sh
make check-workers EVAL_ARGS='--entry product-owner'
python3 scripts/check-worker-evaluations.py --profile core --entry page-team
python3 scripts/check-worker-evaluations.py --profile analysis \
  --entry polars-analyst --python /absolute/evaluation-runtime/bin/python3
```

The command prints a new external evidence directory (by default under the OS
temporary directory). Use `--out /absolute/new/directory` for durable evidence;
an existing directory is refused. The exit code is 0 only when **every selected
suite** passes. Missing prerequisites, execution errors, failures and timeouts
are nonzero and are never counted as a pass. Unselected suites appear separately
in `summary.json`. Default timeout is ten minutes per command; use `--timeout`
only when the selected environment needs a different bound.

## What runs

| Profile | Coverage | Prerequisites |
| --- | --- | --- |
| `core` | Product roles, independent page/art handoffs, planner/request, vector XML/plan checks, all five publication workers, workbook contracts/metrics, team handoffs and command composition | Python 3.9+, Node 22+, sh |
| `analysis` | Polars statistics with independent numerical references; comparison scoring, gates, evidence extraction and analyst/manager bindings | Selected Python with pinned NumPy/Polars/SciPy; `pdftotext` |
| `native` | Both Inkscape workers through actual native author/export/check and deliberate corruptions | Inkscape on PATH or `INKSCAPE` |
| `browser` | Actual page-team Chromium acceptance, interactions, mobile view, reduced motion and network rejection | Node 22+, pinned Playwright, Chromium |
| `workbook-visual` | Actual LibreOffice/PDF/image context and Cage write/network denial | Selected workbook Python with pypdf, LibreOffice, Poppler, Cage |
| `workbook-render` | Artifact Tool saved-date round trip and wrong-day rejection | Selected workbook Node and Artifact Tool modules |
| `workbook-process` | Ten explicit loopback Ask/Record/Cage visual-gate cases | Workbook rendering/visual dependencies; explicit Bench binaries and Ask |

Profiles are repeatable. `--profile all` requires all optional dependencies and
fails if any are unavailable. No profile implicitly enables a live provider.
`make check` still verifies the independent toolkit; `make check-workers` is the
separate library evaluation gate. CI runs core, analysis, native and browser
checks explicitly. Proprietary/bundled workbook runtimes are operator-selected.

## Coverage by library entry

| Entry | Principal assertions | Further quality evidence required |
| --- | --- | --- |
| Django expert | Current request/input/output hashes, four environments, honest status/evidence, minimal build output and malformed/path counterexamples | Real PostgreSQL/auth/admin tests, browser/user review, dependency compatibility and live IdP/operational acceptance |
| Experiment researcher | Current request/event bindings, deterministic plan assembly, fixed experiment protocol, missing-input/no-result handling and public reader composition | Actual Trail investigation, faithful hypotheses, fresh-case discipline and useful experiments |
| Product Owner | Four deliverable modes, needs-input, malformed data, evidence/output bindings | Problem framing, useful priorities and faithful assumptions |
| Product Manager | Strategy/evaluation/synthesis, scope, weights and no premature selection | Outcome-linked features, useful slices, platform judgment and leadership decisions; see the worker quality rubric |
| Enterprise Architect | Ten deliverable types, required roles, profile/input bindings, unsafe/malformed files | Architecture tradeoffs and feasible transitions |
| Polars analyst | Independent t-distribution quadrature, paired IDs, Holm family, refusals and missingness | Whether the supplied study assumptions support the actual question |
| Vendor comparison | Independent totals, anchored weights, failed/unknown gates, quotes and uncertainty | Source entailment and defensible judgments |
| Editorial Director | Own shipped checker, bound story, valid references and labels | Strength and fidelity of the audience-specific argument |
| Information Designer | Own spec, fact references, stale bindings, artifact requirements | Actual saved graphics/workbooks, correct encodings and readable pixels |
| Executive Writer | Own spec, prose length, shared-message coverage and artifact requirements | Sustained prose, faithful qualifiers and rendered document quality |
| Presentation Designer | Own spec, layouts, copy density, shared messages and artifact requirements | Actual editable objects, exact chart values, slide pacing and pixels |
| Publication Reviewer | Own checker, per-criterion gate, material defects and candid revise | Independent actual-media judgment and cross-format fidelity |
| Frontend | Standalone HTML handoffs and actual browser checks | Brief fidelity, design and accessibility beyond fixture scenarios |
| Visual artist | Input/library/output identity, syntax, sensor fallback and SVG rejection | Actual art, interactions, truthful data and visual craft |
| Canvas artist | Standalone source handoffs and tampering | Actual rendered behavior, performance, accessibility and fallback |
| Blender artist | Standalone file-signature/handoff checks | Open real master/GLB and inspect geometry, materials and pixels |
| Inkscape illustrator | XML rejection plus native export/pixel and editable-source mutations | Brief fidelity, composition and drawing craft |
| Inkscape-controlled illustrator | Bounded plan, native author/export, occluded targets and independent pixels | Composition and fidelity on new subjects |
| Image editor | Standalone bound originals and file signatures | Open real XCF, inspect editability, originals and actual edited pixels |
| Image concept | Request limits, missing/malformed fields, explicit not-generated result | Prompt fidelity; generation is a separate capability |
| Page planner | Snapshot identity, valid plans/finish, admitted workers/criteria and bounds | Useful dependency graph and honest completion decisions |
| Page reviewer | Bound review output, verdict structure and stale-output rejection | Correct interpretation of current observations and candid limitations |
| Page team | Assembled file transfers, accepted review bound to original goal/final bytes, image/MCP failures and Chromium | End-to-end planning, actual creative contribution and visual review |
| Vendor comparison team | Lead scope/confidence, statistical/evidence handoffs, fixed stage order and failure propagation | Fresh real Agent runs and independent source-based final review |
| Vendor decision studio | Publication/image gates, exact analysis command copy, entry failure propagation | Complete multi-format rendering, cross-format facts and independent visual/semantic review |

Publication spec tests intentionally use repetitive synthetic prose. Blender and
GIMP signature fixtures are deliberately not valid native artwork. Neither is
creative-quality evidence. Team command fixtures substitute explicit local
Agent/IO programs to verify command composition; the real IO contracts have
separate suites. No fixture certifies a model's judgment or every possible input.

## Native, browser and workbook commands

Inkscape:

```sh
INKSCAPE=/absolute/path/to/inkscape \
  python3 scripts/check-worker-evaluations.py --profile native
```

For browser dependencies, copy the team's two package files to a new external
folder, then install the exact lock. Set `BENCH_SOURCE` to this checkout first:

```sh
BROWSER_RUNTIME=$(mktemp -d)
cp "$BENCH_SOURCE/teams/page-team/expert/package.json" \
   "$BENCH_SOURCE/teams/page-team/expert/package-lock.json" "$BROWSER_RUNTIME/"
(cd "$BROWSER_RUNTIME" && npm ci --ignore-scripts --workspaces=false)
PLAYWRIGHT_BROWSERS_PATH="$BROWSER_RUNTIME/browsers" \
  "$BROWSER_RUNTIME/node_modules/.bin/playwright" install chromium
PLAYWRIGHT_BROWSERS_PATH="$BROWSER_RUNTIME/browsers" \
  python3 scripts/check-worker-evaluations.py --profile browser \
  --node-modules "$BROWSER_RUNTIME/node_modules"
```

Linux may need Playwright's documented system dependencies (`install --with-deps
chromium` on a disposable CI host). The runner links only the explicitly chosen
external dependency directory into its test staging copy. It does not modify
worker source or install packages in the library.

For workbook visual preparation, select existing executables:

```sh
WORKBOOK_PYTHON=/absolute/python-with-pypdf \
WORKBOOK_VISUAL_SOFFICE=/absolute/soffice \
WORKBOOK_VISUAL_PDFTOPPM=/absolute/pdftoppm \
WORKBOOK_VISUAL_CAGE=/absolute/cage \
  python3 scripts/check-worker-evaluations.py --profile workbook-visual
```

For Artifact Tool rendering, also set `WORKBOOK_NODE` and
`WORKBOOK_NODE_MODULES` to the selected Node and its module directory, then use
`--profile workbook-render`. With both runtime groups configured:

```sh
python3 scripts/check-worker-evaluations.py --profile workbook-process \
  --bin-dir /absolute/bench/bin --ask /absolute/bench/bin/ask
```

These profiles run the existing workbook fixtures unchanged. More detail is in
[workbook test documentation](../workers/excel-workbook-designer/tests/README.md)
and [visual runtime setup](../workers/excel-workbook-designer/tests/VISUAL-TESTS.md).
Their observed render/record bytes still do not establish live vision accuracy.

## Separate model-quality evaluation

Prepare an external review plan for all entries, or select one:

```sh
python3 scripts/check-worker-evaluations.py \
  --quality-plan /absolute/new/quality-review --entry vendor-comparison-team
```

This writes `EVALUATION.md` and `cases.json`, each explicitly **NOT RUN**. Every
entry has an ordinary case, missing/conflicting input, convincing near miss,
transfer case and role-specific rubric. Treat these as visible regression cases;
a worker that can read the checkout can see expected outcomes. Genuine held-out
work requires inaccessible expected answers or an input-only interface.

Before authorizing a paid batch, select a model, per-run limits and evaluation
scope explicitly. Freeze the independent expected facts/criteria and prepare
inputs using each definition's contract. Export a clean full Git pin with
`scripts/workers export` or `export-team`; the source team's template alone is
incomplete. Uncommitted evaluation-runner edits are tested by this offline
runner, but committed exports deliberately ignore working-tree changes.

For an individual worker, use its existing public entry after populating a new
workspace according to its README. A typical invocation is:

```sh
agent run -C /absolute/new/work -evidence /absolute/new/records \
  -m "$ASK_MODEL" -turns 12 -timeout 12m /absolute/export/expert -- \
  'Read the selected current inputs and complete the contracted output.'
```

Add the role's required environment selectors and explicit `-record-input` /
`-record-output` files from its README. Judge final outputs independently against
the frozen rubric; the worker's own checker is not the gold label. Art, Office
and browser outputs require actual render/native inspection.

For teams, retain their existing commands and all prerequisites in their READMEs:

```sh
/absolute/comparison-export/expert/bin/compare-team \
  /absolute/cases/01-mixed-materials/job.json /absolute/new/analysis-run --offline
/absolute/studio-export/expert/bin/compare-studio \
  /absolute/cases/01-mixed-materials/job.json /absolute/new/studio-run --offline
PAGE_TEAM_RUN=/absolute/new/page-run \
  /absolute/page-export/expert/bin/page-team < /absolute/brief.md > /absolute/result.html
```

Here `--offline` controls comparison source fetching; **these team commands
still invoke models**. It is not the offline suite runner. Check the
[page-team README](../teams/page-team/expert/README.md),
[comparison README](../teams/vendor-comparison-team/expert/README.md), and
[studio README](../teams/vendor-decision-studio/expert/README.md) before invocation.
The page team's separately printed
[smoke brief](../teams/page-team/tests/smoke-brief.py) is a paid composition probe,
not a creative benchmark. Generate the three fictional comparison jobs with:

```sh
python3 teams/vendor-comparison-team/tests/make_cases.py /absolute/new/cases
```

Keep inputs, source pin/adaptations, command/model/limits, output artifacts,
checker feedback, independent verdicts, elapsed time and measured usage/cost for
**every attempt**. Use `unknown` for unavailable measurements. Passing requires
all ordinary/transfer criteria and appropriate handling of adverse cases, with
no material error. Classify failures as rejected output, unfinished work, broken
check or unavailable dependency; don't turn missing evidence into a pass.
For checker calibration, record false passes over independently bad cases and
false rejections over independently good cases, with counts and denominators.
Changing a worker/check requires new matched work and unchanged independent
review; do not infer improved quality or lower cost from fixture passes.

## Retained evidence and maintenance

Each offline run retains `source/`, `source-manifest.json`, `library-check.log`,
per-command `logs/`, and `summary.json` outside the checkout. The manifest records
current source hashes and modes before test assembly. Team member copies come
from the same staged source roster, and the original metadata remains intact.
`summary.json` records selected/unselected suites, argv, working directory, exit,
time and log paths. Keep the whole directory to reproduce the exact tested bytes.
The recorded Git revision is a baseline, not a claim that uncommitted files match it.

Each run gets a private home/temp and an environment omitting provider keys,
model selection, reviewer selection and admission hashes. Only documented native
runtime selectors are passed through. This is process-fixture hygiene, not a
filesystem sandbox. Tests use their existing contracts and may read selected
installed runtimes. Never place real customer inputs in these synthetic suites.

When adding a worker/team or a test, update the catalog and quality cases.
`python3 -m unittest discover -s scripts/tests -p test_worker_evaluations.py -v`
checks catalog completeness and runner failure/isolation behavior. The guard
rejects unmapped library test files and library entries without suites. Register
shared tests only when they actually execute the affected shipped code. Keep
source changes, offline evidence and live-quality evidence separately identified.

## WorldWeaver-Omega

Focused core/browser suites use retained external scratch and exact source copies. See [its test guide](../workers/worldweaver-omega/tests/README.md) for native browser sandbox prerequisites and optional offline dependency-profile checks. Fresh generated worlds, selected-model records and independent visual reviews remain external evidence; fixtures do not certify aesthetics or physical mobile 90 FPS.
