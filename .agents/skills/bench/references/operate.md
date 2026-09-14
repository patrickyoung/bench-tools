# Run workers and teams, locally or through a protocol

Locate the selected source and export through [library](library.md), or recover
the pinned definition from the external runbook. Use the selected individual
expert and a fresh workspace with explicitly chosen current inputs:

```sh
agent run -C NEW_WORKSPACE -evidence NEW_RECORDS -turns 8 EXPERT -- 'THE JOB'
```

Replace the uppercase placeholders with real paths and the actual goal in the
solution's runbook. Use absolute paths for host tools and unattended calls.
Read files for deliverables and stdout for the final report; keep stderr as
diagnostics. A passing pre-check can exit without a report or model request.

For intentional continuation of the same task, choose a named
`-checkpoint JOB_ID` from its first run and retain that workspace and evidence.
The pointer lives in the selected evidence root's `checkpoints/` directory.
Read the current Agent/Ply manuals for limits and checkpoint handling.

Agent/Ply outcomes include 0 accepted (when a check exists), 2 unfinished,
3 declined, 75 waiting, 125 uncertain boundary/effect, and 130 interrupted.
Do not loop over all nonzero statuses. Diagnose broken checks and dependencies;
resolve unknown effects before considering a repeat.

## Run an assembled team

Use the team's documented command, not a new host-managed loop. After completing
the exported page team's README setup, for example:

```sh
export PAGE_TEAM_RUN=/absolute/project/runs/page-001
/absolute/project/page-team/expert/bin/page-team < /absolute/project/current-brief.md > /absolute/project/page-001.html
```

The page team emits accepted HTML on exit 0 and diagnostics on stderr. Its
manager proposes bounded work; existing Bench Manage, Tend and Weave admit
and execute it. Each assignment uses a separate Agent context and workspace.
Do not replace that controller with Claude/Codex subagent conversations simply
because the harness also offers delegation. Supply current inputs through the
team's actual handoff contract.

A fresh job gets a fresh run directory. For intentional continuation, retain
the same immutable assembly and admitted brief; the page team resumes with
empty stdin. Inspect its documented status and retained evidence. A team roster
alone supplies neither scheduling nor remote endpoint resolution.

## Optionally expose the same command through A2A

Read `tools/a2a/README.md` in the source checkout for exact serving/calling
commands, TLS/authentication and status mapping. `a2aserve` can directly invoke
an `agent run` command or a team's entry command through the existing Tend.
Standard stdin/stdout use needs an operator-owned card and server configuration;
file deliverables need explicit `-artifact` declarations. Input materialization
or a different application format may need a narrow dispatcher. It never
replaces the worker's loop. Page team already includes an A2A card/dispatcher.
Individual library workers are not all preconfigured as remote services.

Each served task receives selected work, state and control roots. Agent takes
stdin as a goal only when no explicit or standing goal exists; otherwise it
is evidence. Respect that contract when exposing a definition. `a2a` consumes
remote agents as a Unix client. A remote member in a local team needs an
explicit command binding compatible with the team's handoffs; `team.json`
does not currently accept endpoints.

The listener is an optional OS-managed service with explicit hosting and access
configuration. Do not start one merely because a local worker could use A2A.
Use MCP instead when the host needs an exposed tool capability.

## Make this job available to the harness

For a local reusable procedure, create a small host Agent Skill containing the
job's description, input contract, exact Agent invocation, output locations,
and meaningful status handling. Put its discovery files where that harness
expects them. Keep the reusable expert definition and references reachable
from a new session. The host skill invokes Agent; it does not duplicate it.

For an application tool, use the [MCP procedure](mcp.md). Test discovery and an
actual request through the same transport the host will use before reporting
the capability as connected. For independently operated remote agents use the
selected checkout's `tools/a2a/README.md` and its TLS/authentication options.

## Retain attempts or schedule work when requested

Tend can save the exact Agent command and input, then execute one transition:

```sh
tend submit -id CASE_ID -C WORKSPACE -- /absolute/bin/agent run \
  -C . -evidence RECORDS -checkpoint CASE_ID EXPERT -- 'THE JOB' < /dev/null
tend work
tend show CASE_ID
```

Use absolute record/definition paths in a real invocation. Pass only required
model environment names through `TEND_PASS`; secret values are not frozen at
submission. Read the attempt artifacts named in the job record. A successful
`tend work` means a transition occurred, not necessarily that the child succeeded.
The complete example is `tools/tend/examples/agent-checkpoint/README.md`.

Use the host's scheduler only for requested recurring work. A schedule must
name its executable paths, working directory, inputs, credentials source,
limits, and output pickup. Do not add a scheduler to Agent or a background
service merely to support on-demand work. Preserve evidence when retiring a job.
