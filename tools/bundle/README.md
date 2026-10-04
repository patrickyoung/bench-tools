# Bundle

Turn a clean team application and independently built Bench tools into one
native executable. Its user supplies a goal, optionally chooses a work
directory, and runs the team's existing command through a small adapter.
Applications with their own command syntax can explicitly select the `argv`
interface to retain it.

```sh
bundle build -o ./researcher -runtime /path/to/bench/runtime ./research-app
./researcher -w ./job 'Compare the supplied migration options'
./researcher -c ./job 'Include ongoing operating costs'
```

Bundle packages and launches. Agent runs workers, Ask owns conversations, and
the team's existing wiring owns coordination. The recipient needs no Go
compiler or Bench checkout. Provider access, credentials, and declared system
dependencies still come from the recipient's environment.

## Build an application

Build Bundle independently with `go build -o bundle .` using Go 1.26 or later.
Building a packaged application also requires Go on the build host and an
installed Bench runtime prefix containing `lib/bench-tools/NAME/package.json`
and its packages.

```sh
bundle build -o ./researcher -runtime /path/to/runtime ./research-app
```

By default, all installed packages in that prefix are included. Repeat
`-tool NAME` to select component packages explicitly:

```sh
bundle build -o ./researcher -runtime /path/to/runtime \
  -tool agent -tool ask -tool brief -tool ply -tool cage -tool record \
  ./research-app
```

Each package retains its own programs, assets, licenses, and receipts. These
programs are embedded as files, extracted, and invoked through their public
command interfaces. They are not linked together into a new Bench runtime.
The build targets the host's operating system and architecture and downloads
nothing. Build separately on each recipient platform.

Package identity includes the application, companion receipts and files, and
the launcher's source digest. Rebuilding unchanged inputs with the same Go
toolchain produces the same executable. Changed launcher source also selects
new private run state, even while its version still ends in `-dev`.
Runtime packages must match their receipts, including generated assets;
rebuild or reinstall a pristine package before embedding locally modified assets.

An application directory contains `app.json` and an explicit inventory of
reusable files. For a team, first assemble a clean pinned export with the
library exporter. Keep its original lock and member definitions; a source
team template alone is incomplete. Add only the adapter and its application
metadata. Current briefs, results, dependencies installed for development,
credentials, and runtime memory do not belong in this source.

```json
{
  "schema": 1,
  "name": "researcher",
  "description": "Run the research expert against a supplied goal.",
  "entry": "bin/run",
  "files": [
    "bin/run",
    "expert/AGENTS.md",
    "expert/README.md",
    "expert/LICENSE",
    "expert/bin/check"
  ],
  "requires": ["agent", "ask", "brief", "ply", "cage", "record"],
  "followup": true,
  "resume": true
}
```

`entry` is a required relative path to an executable regular file in `files`.
`files` enumerates every payload file; there is no recursive directory glob.
Add all selected expert/team files, lock files, skills, and helpers to the
inventory. Symlinks and special files are not application payloads.
`requires` names commands that must be discoverable through the final runtime
`PATH`. Include external commands such as `python3` or `node` when the adapter
needs them. Declaring a command does not install it or configure model access.

The booleans declare adapter capabilities. Do not enable them simply because
an embedded folder contains workers. Existing teams may require a structured
input packet or prohibit changes to an admitted brief.

## Run the generated command

```sh
./researcher 'Explain the options'
./researcher -w ./job 'Prepare the comparison'
./researcher -w ./job < brief.md
printf '%s\n' 'Selected source material' | ./researcher 'Summarize this'
```

Flags precede one quoted goal argument. `--` permits a goal beginning with a
hyphen. With an argument goal, nonterminal stdin is evidence for the adapter.
Without an argument, stdin supplies the goal. The launcher does not read goals
from a terminal or enter an interactive chat.

`-w` selects the workspace for a new run. With no `-w`, the command creates
and retains `./NAME-RANDOM` and reports its location on stderr. Bundle creates
three folders automatically:

```text
job/
  input/       supplied source material
  working/     drafts, task files, and intermediate work
  output/      deliverables published by the adapter after its checks
```

You can start with a nonexistent directory, an empty directory, or a directory
containing only `input/`. For example, stage source material before launching:

```sh
mkdir -p ./job/input
cp brief.md ./job/input/
./researcher -w ./job 'Prepare the comparison using the supplied brief'
```

Existing `working/` or `output/` directories, even empty ones, prevent a fresh
run. They may belong to prior or incomplete work. Select an admitted run with
`-c` or `-resume`; otherwise choose a new workspace. Unrelated top-level files
and symlinks in place of the workspace or its three folders are refused.
Bundle does not automatically copy stdin into `input/` or ingest its files;
the team adapter selects the supplied material through `BUNDLE_INPUT`.

The adapter runs in `working/`. All three folders remain available after
success, failure, or interruption. Stdout is exactly the adapter's result
stream; diagnostics and paths go to stderr. Deliverable filenames, task
tracking files, input admission, and acceptance checks belong to the team's
own contract. Bundle creates the layout; it does not certify or automatically
promote files into `output/`.

`-o` remains a compatibility spelling for `-w`; both now select the workspace
root with this three-folder layout. Do not supply both. Existing binaries keep
their original layout. When rebuilding an older goal adapter, update it to
publish accepted files through `BUNDLE_OUTPUT`; its scratch files now live in
`working/`. Old run state is not migrated into a changed package.

```sh
./researcher -c ./job 'Use the revised cost assumptions'
./researcher -resume ./job
./researcher -info
```

`-c WORKSPACE` explicitly selects a prior run and supplies a new message.
`-resume WORKSPACE` resumes its last immutable goal without admitting a new
one. Each requires the corresponding declared adapter capability. Neither is
inferred from an existing directory. The physical workspace path and package
identity select private controller state; moving the directory or rebuilding
a different package is not an implicit migration. There is no separate `-run`
option or implicit latest-session selector. A continuation requires all three
layout directories to remain real directories; it never recreates a missing one.

`-info` prints the embedded package manifest as JSON without running the team.
`-help` and `-version` are also available. Runtime commands are found first in
the extracted package's `bin`, then in the absolute entries of the caller's
`PATH`. Empty and relative entries are dropped before both dependency checks
and execution, so changing into the workspace cannot reinterpret them.
Existing explicit tool and provider selectors still belong to the caller;
use absolute paths for executable selectors.

## Application command interface

The optional `interface` field in `app.json` selects `goal` (the default) or
`argv`. The goal interface uses the commands and adapter contract described
below. Choose `argv` for an application that already owns its command syntax:

```json
{
  "schema": 1,
  "name": "publisher",
  "description": "Build and inspect a publication.",
  "interface": "argv",
  "entry": "bin/publisher",
  "files": ["bin/publisher"],
  "requires": ["agent", "hire"]
}
```

After the same verified extraction and dependency checks, Bundle passes every
argument literally to the entry, with the caller's working directory and stdin
unchanged. There is no synthetic mode argument, goal file, output directory,
mutable state or run admission. The application owns `--help`, directory
selection, concurrency, retained evidence, continuation and crash recovery.
`followup` and `resume` must be absent or false with this interface.

The entry receives `BUNDLE_ROOT`, the extracted application directory, and
`BUNDLE_ID`, the exact package identity. Inherited `BUNDLE_WORKSPACE`,
`BUNDLE_INPUT`, `BUNDLE_WORK`, `BUNDLE_OUTPUT`, `BUNDLE_STATE`, `BUNDLE_CONTROL`
and `BUNDLE_GOAL_FILE` are removed. Runtime PATH, companion
selection, separate streams, exit statuses and signal forwarding use the same
process boundary as the goal interface.

Only the sole arguments `--bundle-info` and `--bundle-version` are reserved for
package metadata without extraction or entry execution. Those strings alongside
other arguments pass through literally, including after `--`. All other flags
belong to the application. Packaging supplies no application-specific parser or
additional authority.

## Goal adapter contract

The executable entry receives exactly one argument: `run`, `follow`, or
`resume`. Its working directory is the workspace’s `working/` folder. Bundle supplies:

| Variable | Meaning |
| --- | --- |
| `BUNDLE_ROOT` | Extracted application definition directory. |
| `BUNDLE_WORKSPACE` | Selected workspace root; also the continuation selector. |
| `BUNDLE_INPUT` | Workspace `input/`, for supplied source material. |
| `BUNDLE_WORK` | Workspace `working/`, for drafts and intermediate work; also cwd. |
| `BUNDLE_OUTPUT` | Workspace `output/`, for accepted deliverables. |
| `BUNDLE_STATE` | Separate writable application state directory. |
| `BUNDLE_CONTROL` | Controller records outside writable work and state. |
| `BUNDLE_GOAL_FILE` | Private file containing this invocation's goal. |

Arguments never carry the goal to the adapter. Evidence stdin and the adapter's
stdout/stderr remain ordinary process streams. The adapter translates this
small contract into the team's documented entry command, not a replacement
planner or worker loop.

For a single Agent expert with a request-aware check, an adapter can be:

```sh
#!/bin/sh
set -eu
case "$1" in
  run|resume) set -- ;;
  follow) set -- -B ;;
  *) exit 2 ;;
esac
cp "$BUNDLE_GOAL_FILE" "$BUNDLE_WORK/request.md"
agent run -C "$BUNDLE_WORK" -state "$BUNDLE_STATE" \
  -evidence "$BUNDLE_CONTROL/agent" -checkpoint conversation \
  -goal-file "$BUNDLE_GOAL_FILE" -record-input request.md "$@" \
  "$BUNDLE_ROOT/expert" >&2
# This expert’s check must accept result.md against the current request.
cp "$BUNDLE_WORK/result.md" "$BUNDLE_OUTPUT/result.md"
cat "$BUNDLE_OUTPUT/result.md"
```

The example expert produces `result.md` and must require its accepted result
to match the current `request.md`. An adapter using staged files must also
select and record the current inputs and bind its checks to those inputs. Keep
worker write grants scoped to `working/` and appropriate state; the adapter can
publish the accepted result to `output/` after the worker succeeds. Folder names
alone do not enforce read-only input, sandboxing, or assurance.
`-B` starts work on a follow-up even if the previous check already passed;
it does not make an inadequate check correct. Ask/Ply retain the conversation
through the named checkpoint. This example does not make an arbitrary existing
team conversational: such a team needs its own adapter and checked handoffs.

Private files live beneath
`${XDG_STATE_HOME:-$HOME/.local/state}/bundle/PACKAGE_ID/`.
Extracted source/runtime and per-workspace run directories are separate. Run
identity uses the physical workspace path; work, state, and control must remain
disjoint. Retain this private state together with the workspace when
continuation matters. Bundle is not a sandbox; the existing Agent/Cage and
team controller boundaries retain that responsibility.

## Outcomes and verification

The adapter's numeric exit status is returned unchanged. Common Agent/Ply
statuses are 0 accepted, 1 broken, 2 unfinished, 3 declined, 75 waiting,
125 uncertain or boundary failure, and 130 interrupted. Other team commands
retain their documented meanings. Bundle never retries. Diagnose an uncertain
effect before deliberately resuming it; a retained conversation does not prove
that repeating an action is safe.

In the goal interface, before starting an adapter, Bundle writes `active.json` beside that run's
`control` directory. It removes this marker after observing the adapter's
termination. If the launcher crashes or is killed before confirming the end,
the marker remains and subsequent `-c`/`-resume` invocations stop with status
125. The diagnostic names the exact marker. The adapter or its children may
still be running even though the launcher has stopped.

For recovery, inspect the retained records, relevant live processes, and any
external effects. Establish that the prior invocation has stopped and decide
whether repeating work is safe. Only then remove the exact marker named by
the diagnostic and explicitly choose a follow-up or resume. Marker removal
does not establish task completion or resolve an uncertain external effect.

```sh
go test ./...
go test -race ./...
go vet ./...
```

Offline package and process fixtures establish packaging and launcher behavior.
They do not establish the team's task quality, model access, or completion.
Evaluate those separately against the application's actual output contract.
