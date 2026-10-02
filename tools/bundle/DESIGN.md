# Bundle

Make an existing Bench application easy to distribute and invoke without
moving its responsibilities into another execution framework.

## Public boundary

The builder accepts a clean application directory, an installed runtime
prefix, selected component package names, and a new executable destination:

```text
bundle build -o APP -runtime PREFIX [-tool NAME ...] APP_DIR
```

The default `goal` interface gives the generated program a small foreground interface:

```text
APP [-w WORKSPACE] [--] GOAL
APP [-w WORKSPACE] < GOAL_FILE
APP -c WORKSPACE [--] GOAL
APP -resume WORKSPACE
APP -info | -help | -version
```

One quoted positional argument is one goal. With an argument, stdin is
evidence; otherwise nonterminal stdin supplies the goal. No terminal conversation
loop or prompt is part of this boundary. An omitted workspace path creates a
retained `./NAME-RANDOM` workspace and reports its path on stderr.

An existing workspace never implies continuation. `-c` admits a new message
to an explicitly selected prior run; `-resume` selects its last immutable goal.
Package identity and physical workspace path bind that selection to private run
state. Neither directory reuse nor a changed binary silently upgrades a run.
There is no latest-conversation pointer, global session registry, or `-run`
option. Concurrent access to a selected run must not produce mixed admissions.

The child owns stdout, stderr, and status. The launcher adds diagnostics only
to stderr, forwards interruption, and retains evidence. It does not reinterpret
model prose as completion or automatically retry any nonzero outcome.

A per-run lock serializes ordinary invocations. Before execution, an
`active.json` marker records the controller PID, invocation mode, and admitted
goal digest. It is removed after the launcher observes the adapter's end.
A killed launcher can release its OS lock while its adapter survives, so a
retained marker independently blocks further invocations with status 125.
Recovery requires inspecting processes, records, and effects, then explicitly
removing the exact marker named in the diagnostic before selecting continuation.
The marker does not establish completion or authorize repetition of an effect.

An application can instead declare `interface: "argv"`. This uses the same
verified archive extraction, command lookup, dependency checks and foreground
process supervision. Its entry receives the caller's literal arguments, stdin
and working directory. It creates no output workspace, goal file, run binding,
mutable state or active marker. The application owns its flags, run admission,
concurrency and recovery through its existing public command contracts.

Only sole `--bundle-info` and `--bundle-version` arguments are reserved in this
interface; they report package metadata without extraction. Other invocations
pass through unchanged, including application help and those strings appearing
alongside other arguments. This supports ordinary command applications without
moving their parser or business operations into Bundle.

## Application source

`app.json` schema 1 declares `name`, `description`, a required relative
executable `entry`, an explicit regular-file `files` inventory, required
executable names in `requires`, an optional `interface` (`goal` or `argv`), and
`followup`/`resume` capability booleans. The latter must be false or absent for
`argv`, whose entry owns any continuation interface.
It is packaging metadata, not a workflow language or authority policy.

An exported team, its lock, its selected member definitions, and its wiring
remain ordinary files. The builder does not assemble a roster, discover
skills, author expertise, admit private knowledge, or repair a team. Those
operations belong to the library exporter and Hire. The allowlist bounds what
can enter an executable; its author must also inspect file contents to exclude
private goals, credentials, results, and development evidence.

The goal entry is an application-owned executable adapter. It receives one literal
mode argument, `run`, `follow`, or `resume`, selected controller paths through
`BUNDLE_*`, and input material on stdin. It maps these to the team's existing
public entry command. Continuation capabilities are explicit because teams
have different admission and handoff contracts. A raw Agent definition or a
team roster cannot establish that a changed brief is safe to admit.

The adapter's acceptance check must account for the current request. Agent's
zero-model pre-check can otherwise accept an old artifact before a follow-up
is processed. `-B` can start work for a new message, but acceptance still needs
to bind current inputs to outputs. Checkpoints remain owned by Agent/Ply and
Ask; Bundle never reads or writes model transcript formats.

## Single-file distribution, independent processes

The builder carries selected installed component packages intact, including
assets, executable modes, licenses, and receipts. A host Go build embeds these
bytes and the application in a generated native launcher. The recipient needs
no compiler or checkout. There is no implicit download or cross-platform build.

At runtime the launcher extracts the payload and selects its runtime `bin`
before the absolute entries of the caller's `PATH`. Empty and relative PATH
entries are removed before both dependency checks and execution; a change into
the worker's working directory must not reinterpret command lookup. Relative
package links retain relocation. Required command names are checked through
that final search path before team work.
External interpreters, native applications, model credentials, and configured
provider access remain external requirements. Explicit caller tool selectors
retain their existing meaning; embedding does not grant new authority.

The transport file is not a shared runtime. Components still execute as
separate programs with their original arguments, streams, statuses, versions,
and package identities. No sibling Go imports, module replacements, shared
provider client, multicall dispatcher, or umbrella command is introduced.

## Files and authority

Private package data lives under:

```text
${XDG_STATE_HOME:-$HOME/.local/state}/bundle/PACKAGE_ID/
  extracted application and independent runtime packages
  runs/WORKSPACE_PATH_HASH/
    mutable application state
    controller admission and process records
```

The separately chosen workspace contains `input/`, `working/`, and `output/`.
`-w` selects its root; `-o` is a compatibility alias for the same layout. A fresh
run admits a new directory, an empty root, or one containing only `input/`.
Existing `working/` or `output/`, even empty, prevent fresh admission across
package identities. Validate and create the layout under the private run lock.
Continuation requires all three real directories and never repairs them.
The adapter runs in `working/` and publishes accepted artifacts to `output/`.
Input selection, task tracking, snapshots, checks and publication remain the
adapter's responsibility. Bundle neither consumes input files nor duplicates
stdin into a file. Input preservation is a convention, not confinement. Definitions/runtime,
work, mutable state, and controller evidence are distinct domains. Definitions
and evidence must not be placed beneath a worker-writable root. Paths are
resolved and unsafe symlink/special-file cases rejected before execution.
Extraction must not expose partial payloads as ready packages or trust altered
cached bytes as the embedded package.

`BUNDLE_ROOT`, `BUNDLE_WORKSPACE`, `BUNDLE_INPUT`, `BUNDLE_WORK`, `BUNDLE_OUTPUT`,
`BUNDLE_STATE`, `BUNDLE_CONTROL`, and `BUNDLE_GOAL_FILE` expose the selected
locations to the adapter. `BUNDLE_WORK` is `working/`, and the workspace root
remains the run identity. Schema 2 run bindings require the three-folder layout.
Rebuilt goal adapters must publish deliverables through `BUNDLE_OUTPUT`. Prior
binaries retain their original behavior; changed packages do not migrate runs. The goal file
is controller input, not an extra model session. Current inputs and outputs
never update the embedded definition. A follow-up does not teach the reusable
team or silently build a new executable.

The `argv` interface sets only `BUNDLE_ROOT` and `BUNDLE_ID`, and removes inherited
goal-interface workspace/input/work/output/state/control/goal selectors. It retains the immutable
package cache but does not establish writable or controller domains on the
application's behalf. Selecting and separating those domains belongs to its
entry and the public tools it composes.

Bundle does not implement confinement. The adapter must preserve the existing
team and Agent boundaries. Caller-selected source, credentials, writable state,
and evidence remain independent decisions. A single file is a distribution
convenience, not a confidentiality or sandbox claim.

## Non-goals

- No provider client, agent loop, scheduler, daemon, listener, or terminal UI.
- No generic planner that replaces an existing team's controller.
- No automatic retries, effect resolution, or exactly-once execution claim.
- No transcript parser, memory service, or automatic learning.
- No automatic dependency installation or credentials bundled into releases.
- No claim that arbitrary existing teams support conversational revisions.
- No overwrite of an unrelated workspace or implicit state migration.

## Verification

The component's offline checks use public-command fixtures. Exercise clean
native builds, package receipts and inventories, relocation, paths with spaces,
literal goal text, piped evidence, clean stdout, exact child statuses,
pre-staged input preservation, separated scratch and accepted output,
continuation selection, missing/replaced layout refusal, concurrent access, cancellation, interrupted extraction,
cache integrity, and refusal of unsafe paths. A follow-up after a previous
accepted result must actually reach the adapter with its newly admitted goal.

Run `go test ./...`, `go test -race ./...`, and `go vet ./...` independently.
Changes affecting another tool's public interface also require that tool's
checks and executable integration checks. Real team quality and paid model
evaluation remain separate from packaging and fake-process evidence.
