# Run again and give the harness a useful capability

Use the selected expert and a fresh workspace with the new input files:

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
