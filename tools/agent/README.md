# Agent

**Run a filesystem expert against a workspace and a goal.**

Build an expert in a folder, choose a workspace, give it a goal, and collect
its answer and files. The native Go command composes installed Bench tools.
It adds no model client, action loop, transcript format, registry, or daemon.
Existing recurring Agent run/check/show interfaces remain supported.
[Hire](../hire/README.md) is the separate headless builder.

A support expert can answer today's customer and tomorrow's customer using
the same policy and procedure. Each case gets its own workspace and records.
The [support-reply starter](https://github.com/patrickyoung/bench-tools/tree/main/examples/support-reply)
includes the whole expert, a question, source records, and a citation check.
Copy the folder and run it; there is no specialty-specific runtime to build.

Agent connects [Brief](https://github.com/patrickyoung/bench-tools/tree/main/tools/brief),
[Ply](https://github.com/patrickyoung/bench-tools/tree/main/tools/ply), [Ask](https://github.com/patrickyoung/bench-tools/tree/main/tools/ask),
and [Cage](https://github.com/patrickyoung/bench-tools/tree/main/tools/cage). The goal lives in Markdown;
`bin/check` decides whether the work is done.

[Install](#install) · [First worker](#build-your-first-worker) ·
[Recurring work](#run-again-or-wake-only-when-needed) ·
[Builder/runner guide](RUNNER.md)

## Install

From the [Bench tools monorepo](https://github.com/patrickyoung/bench-tools),
run `python3 scripts/install agent hire ask brief ply cage record` at the repository root. See
[installation and updates](https://github.com/patrickyoung/bench-tools/blob/main/docs/INSTALL.md)
for prerequisites and PATH setup, or
[getting started](https://github.com/patrickyoung/bench-tools/blob/main/docs/GETTING-STARTED.md)
for a guided first result.

The [Bench application suite](https://github.com/patrickyoung/bench#install)
also includes Agent, with its own pinned companion versions.

**Independent source build.** Agent remains its own Go module. From an
exported `tools/agent` source directory, run `go build -o /your/bin/agent .`
with Go 1.26+. Keep the installed companion commands on PATH. No sibling
source tree or shared Go workspace is needed. The runtime contains no builder code or separate action helper. Hire is
installed independently for authoring.

Hone, Trail, May, and Action are optional additions for learning, archive
inspection, human decisions, and external operations. Install them when using
those sections below. Building from one monorepo commit keeps companion
versions coordinated without coupling their source modules.

**After either source installation**, run `cage check`. Linux Cage requires
Bubblewrap and usable kernel namespaces; macOS uses the system Seatbelt backend.

Configure [Ask](https://github.com/patrickyoung/bench-tools/tree/main/tools/ask#install) with a supported
model and credential before running model work. For example, replacing both
placeholders:

```sh
export ASK_MODEL='anthropic/YOUR_MODEL_ID'
export ANTHROPIC_API_KEY='YOUR_API_KEY'
ask 'Reply with hello.'
```

## Reuse an expert across workspaces

The [support-reply walkthrough](https://github.com/patrickyoung/bench-tools/tree/main/examples/support-reply)
is the practical first run. This smaller sorting exercise makes the file and
check mechanics easy to inspect; sorting alone does not need a model.

```sh
hire new experts/names 'Normalize a supplied list of names'
mkdir names-work
printf '%s\n' pear apple pear banana > names-work/names.txt

cat > experts/names/bin/check <<'SH'
#!/bin/sh
set -eu
test -f sorted.txt || exit 1
LC_ALL=C sort -u names.txt | cmp -s - sorted.txt
SH
chmod +x experts/names/bin/check

agent show -C names-work -evidence names-evidence experts/names
agent run -C names-work -evidence names-evidence experts/names -- \
  'Read names.txt and write sorted.txt with sorted, unique names.' > answer.txt
cat names-work/sorted.txt
```

`hire new` uses the existing builder to create only reusable
files. Edit its instructions, memory, and procedures as needed. A portable
definition requires `AGENTS.md` and executable `bin/check`; the other Markdown
files and `skills/`, `tools/`, and `agents/` are optional. A second workspace
can use the same definition unchanged.

With `-C`, explicit goal text replaces the optional standing `GOAL.md`.
`-goal-file FILE` keeps a goal out of the invocation arguments. Piped stdin
is evidence; if no explicit or standing goal exists, stdin supplies the goal.
Private definition and input bodies never appear in companion-program argv.

The existing workspace holds deliverables. Mutable state defaults to
`WORKSPACE/state`; `-state DIR` selects another location. Controller evidence
defaults to `~/.agent/KEY`, or `$AGENT_DIR/KEY`, where KEY derives from the
physical definition and workspace paths. `-evidence DIR` selects it directly.
These are ordinary directories, with no registry. State may live within work;
definition and controller evidence must remain outside mutable roots. The
selected paths are printed on stderr. `trail check names-evidence/runs`
verifies the Ask sessions through the existing archive tool.

The command reads stdin, writes its answer to stdout, sends progress to stderr,
and returns Ply's outcome unchanged: 0 accepted, 1 broken, 2 unfinished,
3 declined, 75 parked, 125 boundary failure, 130 interrupted. Limits such as
`-turns`, `-cycles`, `-timeout`, and `-compact` pass to Ply. A passing `bin/check`
pre-check needs no model call. An empty answer on that re-entry is expected;
the deliverable is already in the workspace.

That pre-check only knows the condition you wrote. Use a fresh workspace for
a new case, or make the check bind its result to the current input. An old
nonempty report must not stand in for work on a different question.

## Build your first worker

This small worker turns a list of names into a sorted, deduplicated file.
There is an exact expected answer, so success is easy to inspect.

```sh
hire new -home names-worker 'Maintain a clean list of names'
printf '%s\n' pear apple pear banana > names-worker/work/names.txt

cat > names-worker/GOAL.md <<'GOAL'
# Outcome
Read names.txt and write sorted.txt with one name per line, sorted and deduplicated.

## Acceptance evidence
The exact output is apple, banana, pear, each on its own line.

## Constraints
Keep names.txt unchanged. Work only in the mutable work and state directories.
GOAL

cat > names-worker/bin/check <<'SH'
#!/bin/sh
set -eu
test -f sorted.txt || exit 1
printf 'apple\nbanana\npear\n' | diff -u - sorted.txt
SH
chmod +x names-worker/bin/check

agent check names-worker
agent show names-worker
agent run names-worker
cat names-worker/work/sorted.txt
```

`agent check` validates the home's structure; `agent show` prints the compiled
instructions and authority. Neither is a model verdict. `agent run` asks Ply
to pursue the goal until `bin/check` accepts, or a limit/error stops it.
The check runs from `work/` before the first model turn and after a candidate.

Expected result:

```text
apple
banana
pear
```

Run `agent run names-worker` again. A passing pre-check means there is no work
to do and no model call is needed. For a larger job, write a check that covers
its real requirements; a nonempty file alone rarely proves a useful result.

![Animated diagram: A standing job has a home. Cage denies action networking by default. Host reads remain unrestricted.](docs/readme/flow.gif)

[Static version of the diagram](docs/readme/flow.png). This illustrates the workflow; it is not a recorded run.

## Understand the home

```text
names-worker/
  AGENTS.md       operating instructions
  GOAL.md         standing outcome and constraints
  SOUL.md         optional tone/persona
  PLAN.md         optional strategy
  MEMORY.md       small, curated facts
  HEARTBEAT.md    optional recurring instructions
  skills/         Brief-readable procedures
  tools/          ordinary programs
  agents/         specialist homes
  bin/check       executable definition of done
  bin/wake        cheap probe: quiet, wake, or broken
  work/           mutable inputs and deliverables
    proposals/    definition patches awaiting review
    actions/      external-effect proposals awaiting controller execution
  state/          mutable durable state
  .agent/         controller-owned runs, checkpoints, learning, and receipts
```

Definition, mutable work, and controller evidence are separate write domains.
Agent does not inject every state file into every model call. The worker reads
state when it needs it; memory and skills change through explicit operations.

By default, Cage permits action writes to `work/`, `state/`, and a private
per-action temporary directory, with network denied. **Host reads remain
unrestricted.** `-net` deliberately enables action networking; `-no-cage`
uses the ordinary host boundary. Markdown cannot choose either permission.
The verifier remains outside the action boundary.

## Run again, or wake only when needed

```sh
agent run -checkpoint daily names-worker
hire history names-worker
hire history names-worker check
```

A checkpoint keeps the current Ply/Ask conversation across interruption and
compaction. It is not a snapshot or a reason to repeat an uncertain effect.
History uses Trail and Ask's replay verification without modifying sessions.

For recurring work, fill in `HEARTBEAT.md` and supply `bin/wake`:

| Wake status | Meaning |
| --- | --- |
| 0 | Quiet; no model call or new Ask session |
| 1 | Work is needed; run the heartbeat with the probe's output as evidence |
| Anything else | Broken probe; stop |

`agent tick HOME` runs that process once. Scheduling belongs outside Agent.
[Tend](https://github.com/patrickyoung/bench-tools/tree/main/tools/tend) can durably submit an exact Agent
invocation, retain output, and hold uncertain attempts for review.
The separately pinned legacy Hire web application supplies task review and
routines. The headless Hire command in this monorepo builds definitions.

## Give it skills and specialists

Put a skill under `skills/NAME/SKILL.md`. Brief selects from the home's own
catalogue; ambient personal skills are not silently inherited. Programs in
`tools/` are prepended to PATH. An empty skills or tools directory is fine.

Create a separate specialist and edit its goal and check before running it:

```sh
hire new -home names-worker/agents/reviewer 'Review one bounded result'
agent check names-worker
agent specialist names-worker reviewer -- 'Review the supplied evidence.'
```

A specialist gets its own definition, check, state, and history. It does not
inherit the parent's conversation. Supply actual evidence explicitly; the
example task text is not permission to inspect arbitrary private context.
Under an explicitly selected ordinary host boundary (`-no-cage`), a parent
can invoke the same Agent command as a foreground child, pipe explicit input,
collect stdout, and inspect its exit status. Use `agent run -C WORKSPACE CHILD
-- GOAL` for portable children or `agent specialist PARENT NAME` for existing
homes. The child inherits Ply's model, effort, approval gate, and depth count.
A custom inherited action wrapper cannot be rebound and is refused. Inside
default Cage, specialist execution belongs to the external controller. A
failed child never establishes parent completion: the parent's check still
must accept. Scheduling and parallel composition stay with Unix callers.

## Build and maintain experts separately

Use [Hire](../hire/README.md) for `new`, model-backed `build`, and existing
home maintenance. Authoring commands are no longer part of Agent. Hire itself
runs an expert through this same runner; the generated folder is its handoff.
Hone, Trail, Action and May retain learning, archive, external-effect and
approval responsibilities. See [Hire's security boundary](../hire/SECURITY.md).

## Reference and learning resources

`agent help` lists `check`, `show`, `run`, `tick`, `specialist` and `version`.
`AGENT_ASK`, `AGENT_PLY`, `AGENT_BRIEF`, `AGENT_CAGE` and `AGENT_RECORD` select exact runtime
companions. `HIRE_AGENT` selects Agent for the separate builder.

- [Build a worker with an LLM](https://github.com/patrickyoung/bench-tools/blob/main/docs/BUILD-WITH-AN-LLM.md)
- [System-builder skills](plugins/bench-system-builder/README.md) for guided design and operation
- [MCP integration](MCP.md), [design](DESIGN.md), and [security boundary](SECURITY.md)
- [Evaluation corpus](eval/README.md) for offline behavior checks

Contributors: read [AGENTS.md](AGENTS.md), then run `go test ./...`,
`go test -race ./...` and `go vet ./...`. The monorepo's
`scripts/agent-hire_test.sh` preserves the 163 existing runtime/maintenance
checks across the two public executables. See [runner extraction](RUNNER.md)
for executable integration and release-boundary details.
[MIT license](LICENSE).

## Automatic execution recording

Agent requires Record and a Ply with `process_recording: ply.recording/v1`.
Every run enables full action and verifier recording through Ply, outside
the selected action interpreter and Cage. No model instruction is needed.
Ask still owns all session files and seals; every component remains an
independently installed executable. Missing Record or incomplete recording
stops with 125. Older Ply versions are refused before execution.

The selected evidence root contains `recordings/run.*/index.jsonl`, an Ask
session linking attempted processes, completed recordings, conversation
sessions, and nested invocations. `inputs.jsonl` snapshots the private compiled
context, invocation goal and checker before work. Each action/verifier has
its own `session.jsonl` with full separate streams, before Ply's presentation
cap. `outputs.jsonl` snapshots selected output files and the conversation
sessions used during this invocation, preserving their bytes across resume.
Compaction summaries are included in the conversation snapshots. Nested
agents retain their own evidence roots; keep those roots with the parent
when collecting a complete run.

Select task files explicitly; paths are relative to the workspace:

```sh
agent run -C WORKSPACE -evidence RECORDS \
  -record-input source.json -record-output result.json EXPERT -- 'The job'
record replay -f RECORDS/recordings/run.ID/action.ID/session.jsonl -stream stdout
ask replay -check RECORDS/recordings/run.ID/index.jsonl
```

Replace IDs with the paths reported in the retained index. `record check`
verifies a complete process receipt; `ask replay -check` checks the index's
seals. A sealed index prefix alone is not a completed invocation: inspect its
terminal record and all referenced receipts. Replay never reruns actions.
Selected missing outputs are incomplete evidence, even on an unfinished run.
This is stream and selected-file capture, not a filesystem or network snapshot.
A quiet heartbeat that stops at `bin/wake` still creates no Ply run.
