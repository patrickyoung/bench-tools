# Agent runs; Hire builds

For a complete first run, use the
[support-reply expert](https://github.com/patrickyoung/bench-tools/blob/main/examples/support-reply/README.md).
It turns a question and supplied policy into a checked draft. This document
explains the boundaries that let the same folder serve a second workspace.

Two native executables in the Bench tools monorepo, with independent modules:

- `hire new EXPERT` scaffolds a reusable definition without a model.
- `hire build -C BUILD_WORKSPACE -- JOB` runs Hire's builder expert through
  Agent and produces `BUILD_WORKSPACE/expert`.
- `agent run -C WORKSPACE EXPERT -- GOAL` runs that definition against a
  separately selected workspace. `agent check/show` provide read-only inspection.

The definition folder is their handoff. Agent contains no builder, embedded
home-maintenance script, provider client, MCP client, scheduler, registry or
second transcript. Hire's builder instructions live in its expert folder. Its
only AI execution path is the public Agent command. Existing home-authoring
and controller operations have moved to Hire; the old shell runner is removed.

## Runtime contract

A portable expert requires `AGENTS.md` and executable `bin/check`. GOAL, SOUL,
PLAN, MEMORY, skills, tools and nested experts are optional. `-C` chooses an
existing workspace; `-state` and `-evidence` can select the other mutable and
controller roots. Defaults are documented and printed on stderr, without a
registry. Definitions and controller evidence cannot overlap mutable roots.

An explicit portable goal replaces GOAL.md. `-goal-file` keeps private goals
out of arguments. Stdin is evidence, or the goal when no other goal exists.
Brief loads private instructions and selects procedures. Ply owns the loop,
limits and verifier. Ask owns intelligence and replayable sessions. Admitted
MCP capabilities are ordinary executable tools; Agent has no MCP-specific code.

Stdout is the answer, stderr is progress, files are deliverables, and exit
status is the process outcome. A passing `bin/check` pre-check needs no model
call. A textual success claim cannot override a rejected check. Hire starts a
build work session by default so a new brief can revise a valid definition;
`-B=false` explicitly permits the zero-model pre-check.

Children start with their own definition, memory, skills, task and Ask session.
They receive explicitly passed input, not the parent's conversation. Runtime
model/effort, approval gates and Ply's nesting depth are inherited. Ordinary
recursive execution requires a selected host boundary. A custom inherited
action interpreter is refused rather than rebound to different writable roots.
Default Cage grants neither nested provider access nor broader evidence writes.
Context separation is not filesystem confidentiality. Unix callers still own
process composition, scheduling, shared-workspace choices and exit handling.

## Builder acceptance

Hire's generated expert is an artifact. Its outer check calls `hire verify`,
which delegates structural validation to `agent check` and requires usage and
validation documentation. It never executes generated verifier code with
controller authority. Structural readiness does not prove check soundness,
semantic quality, or suitability for an untested real task.

## Verification

`python3 scripts/check agent hire` independently exports/builds/tests both
components, including Go race/vet checks. The monorepo's
`scripts/agent-hire_test.sh` preserves the 163 original runtime/maintenance
assertions against the two actual executables. `scripts/check-integration.py`
uses real installed companions with a local model wire fixture to verify:

- private definition, memory, Brief skill selection and literal input;
- rejected candidates repaired to checked artifacts, unfinished status,
  zero-model re-entry and replayable Ask evidence;
- ordinary child calls with separate contexts, inherited depth and model;
- admitted local MCP execution through the existing MCP tools;
- direct and nested cancellation, descendant cleanup and temporary-file cleanup;
- one unchanged definition across two workspaces, including default Cage
  refusal of definition writes when `--native-cage` is selected;
- Hire building a definition through Agent, a separate Agent running that
  generated expert, and a later Hire request revising the valid definition.

The independent package/install tests cover relocation, both commands, repeated
install and removal in disposable prefixes. No paid model evaluation or live
installation migration is implied by these offline checks.

## Adoption and release boundary

Hire is the first thin application of Agent. Presenter, Writer and Architect
remain further adoption candidates: their current code calls Ask/Ply directly.
Preserve their useful renderers, checks, source authority rules and versioned
results when replacing duplicated execution/context setup. Do not move those
deterministic responsibilities into Markdown or add a workflow engine.

Existing `agent run HOME`, `tick`, `specialist`, `check` and `show` remain
available. Authoring commands intentionally move to Hire: there are no legacy
builder aliases in Agent. The old Hire web checkout and pinned installations
are retained separately. They need explicit call-site/pin migration before
adopting this new CLI split; this development build does not silently replace
that application or claim compatibility for its old authoring calls.

## Design basis

Pike and Kernighan's [Program Design in the UNIX Environment](https://lists.gnu.org/archive/html/nmh-workers/2012-11/pdfGsAjwogf3t.pdf)
focuses on how programs fit together. Ritchie's [Unix history](https://www.nokia.com/bell-labs/about/dennis-m-ritchie/hist.html)
describes the evolution of processes, files and pipes, including Thompson's
work. These are design influences, not claims that the authors reviewed this
implementation. Here the result is focused processes and ordinary files.
