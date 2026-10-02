# Trait

Create, edit, teach, and evaluate a reusable worker or team. Supply the outcome;
Trait uses Hire to choose the instructions, checks, programs, and specialists,
then prepares and runs fresh evaluation cases automatically.

```sh
trait create ./request ./worker-v1
trait edit -request 'Add a checked CSV export.' ./worker-v1 ./worker-v2
trait train -request 'Learn the supplied review method.' ./teaching ./worker-v3
trait eval ./worker-v2 ./fresh-evaluation
```

Workers remain ordinary Bench definitions and procedures remain Brief skills.
Trait is the application name, not a new worker format. Teaching changes
instructions and skills; it does not train model weights.

## Install and package

The standalone core builds independently with Go 1.26+. From `tools/trait/`:

```sh
go build -o trait .
```

It uses installed public Bench commands. For a single-file distribution, use
Go 1.26+, Python 3.9+, and an installed, verified runtime prefix. From the
repository root:

```sh
scripts/build trait bundle
scripts/package-trait --runtime /path/to/bench/runtime --output ./trait
```

The packager selects complete Trait and Bundle packages through `.build/bin`;
`--bin-dir /path/to/build/bin` selects another location. A bare executable lacks
the required package receipt and assets. The output executable must be new.

Bundle embeds Trait and the selected independent Bench programs. Hire already
contains the creator expert. The recipient needs no Go compiler or checkout
to launch Trait. Provider access, credentials, a working Cage backend, and
dependencies of the authored capability remain external. An authored native
program may need its own compiler or installed application. Trait reports
missing prerequisites; it does not install host software or change credentials.

## Supply the outcome and optional overrides

```text
trait ACTION [OPTIONS] INPUT_DIR NEW_OUTPUT_DIR
```

Flags follow the action and precede the two directories. `-request` overrides
`REQUEST.md` without editing the input. Choose a configured model with `-m`;
otherwise existing `ASK_MODEL` or provider defaults apply. Defaults are 12
turns per worker and five minutes per subprocess, including authoring, review,
the complete controller entry, and checking. These are not a monetary budget
or a total multi-case deadline. All actions can make model calls.

`-entry-boundary host|cage` selects controller execution; the default is `host`
for compatibility with existing teams and their child Agents. `cage` requests
an additional enclosing write boundary. If that boundary or nested sandboxing
is unsupported, execution fails with 125; there is no fallback to host mode.
The option does not change ordinary worker execution through Agent.

```text
INPUT/
  REQUEST.md             outcome/change; required except eval, unless -request
  expert/                starting definition for edit, train, eval
  sources/               optional selected supporting material
  cases/                 optional caller-selected evaluation cases
  evaluation/run.json    optional explicit entry selection
  evaluation/cases/      alternative location for selected cases
  CAPABILITIES.md        optional existing capability assessment
  train.json             optional explicit training route and scope
  worker.lock.json       optional original export provenance
  team.lock.json         optional original export provenance
```

Cases, executable helpers, team adapters, and capability assessments need not
be written by the caller. Trait prepares missing evaluation material through a
separate Hire invocation. Its copied expert must remain byte-for-byte unchanged,
and supplied evaluation files must be preserved. `CAPABILITIES.md` explains
the selected instructions, checks, programs/apps, specialists, dependencies,
entry, and handoffs, including why a category is unnecessary.

An authored Trait output can be selected directly for edit or train with a new
`-request`, or for fresh eval. Trait reuses its `expert/` and evaluation recipe;
old `cases/ID/work` results are excluded from current case inputs. Supply cases
in `cases/` or `evaluation/cases/`, not both. Original locks describe the
starting source; edited candidates do not inherit unchanged lock claims.
Training from a previous result preserves its existing cases and adds a new
transfer case for the new teaching request before amending the definition.

Input stays untouched. Output must be new and disjoint from input and private
controller storage. Trait has no interactive prompt, stdin request handling,
implicit resume, in-place edit, or automatic library publication. Use
`trait help` and `trait version` for the core interface.

## Author and evaluate

`create` and `edit` use Hire to author the complete requested capability,
including deterministic code or team wiring when needed. Hire verifies
structure without executing the generated verifier. Trait then prepares missing
cases, freezes the evaluation recipe, and asks Ask to review the exact selected
definition, capability assessment, entry, and cases. The review must return an
approval decision and reason before evaluation proceeds. `REVIEW.md` labels
this automated model review; it is not human approval or proof of safety or
quality. A rejected review stops the run and preserves the candidate.

All four actions run fresh cases. Missing cases are normally generated as a
small suite containing transfer and regression cases. Generated expectations
are model-authored judgments, not independent domain truth. Supplied cases
remain caller-selected criteria. `evaluation_origin` identifies their origin.
Only the observed case checks determine `accepted_on_cases`; authoring prose,
capability lists, and structural validity cannot establish that result.

## Teach knowledge or a checked recovery

With no `train.json`, training uses supplied knowledge with private scope for
the selected worker. Use an explicit file to choose a different route or scope:

```json
{"kind":"knowledge","scope":"general"}
```

Explicit settings must include valid `kind` and `scope`. Put supporting material
under `sources/`; describe the lesson in `REQUEST.md` or `-request`. General
scope excludes private organizational facts. Private scope supplies no
filesystem confidentiality or publication authority.

For a recorded failure followed by checked recovery:

```json
{"kind":"recovery","scope":"private","session":"/absolute/run/session.jsonl","skill":"selected-skill"}
```

Retain the original session and its replay evidence. Hone checks eligibility,
prepares the exact proposal, and admits it only after a bounded Ask model
review approves those bytes. No useful lesson remains a no-change outcome;
there is no fallback to a different teaching route. Preserve the selected
provider wrapper across Agent's `AGENT_ASK` and Hone's `ASK` interfaces.

Training freezes at least one transfer and one regression case **before the
amendment**. Missing cases are generated first. Training changes nonexecutable
Markdown only: every other file, every existing executable including executable
Markdown, and its path, mode, and bytes must remain unchanged. New executable
Markdown is refused. Use `edit` for code, data, checker, or runtime changes.

Only accepted fresh cases support `retained_on_cases`. This does not establish
improvement over the original worker or prove that a changed skill caused the
result. Inspect the actual Agent evidence for loaded instructions and skills;
controller-written child records have the limitations described below.

## Evaluation and execution contracts

Each `cases/ID/` contains `goal.md`, optional `input/`, executable `check`,
`case.json`, and optional expected data. Metadata names its purpose and outputs:

```json
{"purpose":"transfer","outputs":["output/report.md"]}
```

A refusal case may select `"expected_exit":2` and empty `outputs`. Omission
expects exit 0 and requires 1–64 regular output files. A separate check must
validate the actual refusal reason and absence of false completion; exit 2 alone
is insufficient. It receives `TRAIT_EXECUTION_EXIT`, recorded JSON at
`TRAIT_EXECUTION_STATUS`, and controller state/evidence paths in
`TRAIT_EXECUTION_STATE` and `TRAIT_EXECUTION_EVIDENCE`. Those retain the stated
controller trust limits. Other expected statuses are rejected; unknown execution,
interruption and infrastructure errors can never be expected test successes.
An unexpected successful exit fails the refusal case.

Purpose is `transfer`, `regression`, or `general`. Outputs are relative artifact
paths in fresh case work. Without `evaluation/run.json`, Trait uses Agent with
`-B`. A team or deterministic application selects its real entry, for example:

```json
{"entry":"bin/run","args":["{goal}","{work}","{state}","{evidence}"],"requires":["python3"]}
```

`entry` is a regular executable relative to `expert/`. Arguments are literal;
only whole tokens `{goal}`, `{work}`, `{state}`, `{evidence}`, and `{expert}`
are replaced with selected paths. `{goal}` is a file path; its bytes also
arrive on stdin. `requires` names external executables. There is no shell
interpolation or permission field. Hire supplies a thin adapter when an existing
team's argv, stdin, and environment contract needs translation.

Entries run through Record. The default `host` entry has ordinary host write,
read, and network authority, like existing Bench team controllers. Separate
directories, snapshots, and hash postchecks do not confine it or protect
evidence against deliberate alteration or change-and-restore races. Automated
source review is advisory; it does not provide a security boundary.

With `-entry-boundary cage`, Cage permits writes only to fresh case work,
state, child evidence, and private temporary storage. Trait source, criteria,
outer records, and result snapshots remain outside those grants. Full network
and unrestricted host reads, including readable credentials, remain available.
Child Agents normally retain their default action Cage; a controller can
bypass that inner default. Only the optional outer Cage enforces the controller
write boundary. Child records are controller-writable in either mode and are
not independently trusted Trait evidence.

The entry receives `TRAIT_AGENT`, `TRAIT_EXPERT`, `TRAIT_WORK`, `TRAIT_STATE`,
`TRAIT_EVIDENCE`, `TRAIT_GOAL`, `TRAIT_TURNS`, and `TRAIT_TIMEOUT`; a selected
model is supplied as `ASK_MODEL`. These select existing programs and limits,
not a new provider client, model loop, or scheduler.

After execution, Trait copies the work into a separate snapshot, outside the
write grants when controller Cage is selected. The independent check runs
with no argv from that snapshot;
`TRAIT_CASE_DIR` identifies the frozen case. Record binds all case files and
declared outputs. Cage denies check networking and workspace writes while
allowing private temporary files. Exit 0 accepts, 1 rejects, and another status
means a broken check. Trait publishes the captured bytes, not a later reread
of mutable execution work. Host-authorized controllers can still tamper with
snapshots and records; their separate locations are not access isolation.

The expert's own verifier retains its existing Agent/Ply contract; in the
ordinary worker path it runs outside Agent's action Cage. The independent case
check is separate. Cage allows host reads, so expectations are visible
regression material, not hidden tests. File existence, arithmetic, source
fidelity, and semantic quality require checks appropriate to those claims.

## Read the result

```text
OUTPUT/
  expert/                valid authored candidate; absent for eval
  REQUEST.md             selected request, when present
  CAPABILITIES.md         selected capability assessment
  REVIEW.md              automated review decision and limitations
  evaluation/            frozen reusable cases and optional run.json
  cases/ID/work/          checked result snapshots
  result.json            controller-written machine result
  EVALUATION.md          observed outcomes, claims, and limitations
```

Stdout contains the same final JSON as `result.json`; progress goes to stderr.
Some early failures produce only diagnostics. Private source, work, state,
streams, proposals, and model records remain under
`${XDG_STATE_HOME:-$HOME/.local/state}/trait/runs/RANDOM`. Retain the referenced
records. `evaluated_sha256` binds the selected definition; Trait rechecks it,
the admitted source, and the frozen evaluation after execution and checking.
Detected drift returns 125. Hashes identify bytes and modes, not quality.

Trait preserves child outcomes including 2 unfinished, 3 declined, 75 waiting,
125 uncertain/boundary failure, and 130 interrupted. Independent rejection and
review rejection return 2; broken checks retain their status. Invalid usage is
2, local errors 1, local subprocess timeout 124, and missing commands 127.
Hone's no-lesson outcome remains distinguishable. Inspect the recorded phase;
a status number alone does not identify its meaning. Trait never retries
automatically or converts an unfinished result into acceptance.

## Verify the application

From `tools/trait/`, run `go test ./...`, `go test -race ./...`, and `go vet ./...`.
Offline fixtures and loopback model integration establish process and packaging
contracts. Real provider access, semantic quality, and useful retained teaching
require separate evidence.
