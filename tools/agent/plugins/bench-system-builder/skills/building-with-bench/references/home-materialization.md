# Materializing the Bench Agent home

The Draft project proves a design and its check. The Agent home is the durable
worker that will reopen that obligation. They are deliberately different
artifacts; promotion is a reviewed mapping, not a file copy or a new runtime.

Claude Code or Cowork can guide the mapping. The executable worker is always the
Bench suite's shipped `agent`, reached directly or through `bench home`.

## 1. Fix the roots

Record absolute paths for:

- the verified suite entry point;
- the parent directory that will contain Agent homes;
- this Agent home;
- withheld cases and their oracle, outside every worker-readable root;
- controller schedules, identities, credentials, receipts, and long-term
  evidence, outside `work/` and `state/`.

Do not place a new home over an existing nonempty directory. Do not make the
whole business project worker-writable for convenience.

## 2. Scaffold with Bench Agent

Use the suite entry point, not a Claude-native agent generator or a custom
configuration format:

```text
/absolute/prefix/bin/bench home new /absolute/agents/NAME "reviewed standing job"
```

The fresh scaffold is intentionally unfinished. Its placeholder check is not
business proof and its quiet wake file is not a recurring-work design.

## 3. Map reviewed decisions to owning files

| Reviewed input | Agent-home destination | Meaning |
| --- | --- | --- |
| Job, boundary, escalation, source precedence | `AGENTS.md` | Operating procedure and constraints; not changing facts or secrets. |
| Durable finished result and stop condition | `GOAL.md` | The obligation reopened by every distinct run. |
| Voice or stable behavioral posture, if needed | `SOUL.md` | Style that cannot weaken the job, check, or authority. |
| Reusable verified procedures | `skills/` | Brief-discoverable skills with narrow descriptions and progressive disclosure. |
| Deterministic worker tools | `tools/` | Capability selected by the caller; never hidden authority. |
| Candidate work and explicit continuity | `work/`, `state/` | The only ordinary persistent worker-write roots. |
| Independent completion logic | `bin/check` | Controller-reviewed executable verdict; the worker cannot rewrite it. |
| Cheap recurring-work probe, when needed | `HEARTBEAT.md`, `bin/wake` | Whether a later `tick` should call a model; not a scheduler. |
| Runs, checkpoints, proposals, amendments | `.agent/` through public commands | Controller evidence and state; do not edit its private representation. |

Translate the meaning, not the headings. If an approved rule has no owner in
this table, stop and assign one. If a destination contains a credential,
withheld answer, schedule, approval policy, or effect connector, the boundary
is wrong.

## 4. Prove the home before model work

Run the public structural views:

```text
/absolute/prefix/bin/bench home check /absolute/agents/NAME
/absolute/prefix/bin/bench home show /absolute/agents/NAME
```

Then test these independent facts:

1. An already-complete fixture makes the check pass and a `run` creates no Ask
   session.
2. An incomplete or wrong fixture is rejected by the check.
3. A mutation that violates each important invariant is detected.
4. With recurring work, `bin/wake` returns 0 for quiet, 1 with bounded evidence
   for work, and a distinct non-0/1 status for a broken probe. It never calls a
   model.
5. `history HOME check` validates the retained run archive after an exercise.
6. The worker cannot write the check, expected labels, scheduler, credentials,
   or controller receipts.

Structural validity, check proof, model access, and business quality are four
separate claims. Record them separately.

## 5. First bounded run

Require the previously recorded **MODEL-READY** milestone: a non-secret
provider/model identifier and controller-owned model access that passed the
bounded probe in `model-readiness.md` without printing credential values. Start
with a fixture, network off, no
external effects, and no `-no-cage` override:

```text
/absolute/prefix/bin/bench home run -m provider/model /absolute/agents/NAME -- fixture-id
```

Use no checkpoint for a distinct run. Add `-checkpoint CASE-NAME` only when a
later invocation should continue that exact Ask conversation; it is not work
rollback or effect safety.

Record the literal status and evidence in `FIRST-RUN-REPORT.md`. Agent/Ply run
statuses are operation-specific: 0 accepted; 1 runtime, provider, or broken
verifier error; 2 not done or a bound reached; 3 declined; 75 parked; 125
uncertain confinement; 130 interrupted. Inspect current command help and
evidence before acting on any nonzero result.

A literal completed invocation earns **AGENT-FIRST-RUN**; it does not by itself
earn pilot readiness. If provider access is unavailable or fails, retain
**SUITE-READY**, record **MODEL-ACCESS-REQUIRED**, its owner, and this exact
sanitized command as the resume point. Do not redo the Agent home or owner
interview when access is later approved.

## 6. Leave the user ready to operate

Create a concrete `LOCAL-RUNBOOK.md` containing the absolute paths and commands
for inspect, interactive operation, distinct on-demand runs, named-checkpoint
continuation, tick, history, pause, incident response, and retirement. Use the
operator skill's preparation helper to render disabled host-scheduler files
only after wake behavior is proved. The local schedule uses the physical suite
`bench home ... tick` entry and is checkpoint-free by default; a checkpoint is
only explicit same-conversation continuity. Activation remains a separate
decision.
