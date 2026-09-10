---
name: operating-bench-agents
description: Operates an existing Bench Agent home interactively, on demand, by checkpoint, tick, or reviewed host schedule; also diagnoses, changes, and retires it. Use after a home exists.
license: MIT
metadata:
  author: patrickyoung
  version: "0.2.0"
  bench-suite: "0.13.0"
  agent: "0.2.1"
---

# Operating Bench agents

Operate an existing directory-shaped worker from its admitted definition,
public commands, executable verdict, and controller evidence. Keep business
ownership visible. Do not replace recorded state with a chat summary.

Claude Code or Cowork may guide the operator. The worker itself always remains
the Bench suite's Agent system, reached by `bench home`, the interactive
`bench -home` view, or the exact suite `agent` executable. Never add a wrapper
that becomes another loop, agent definition, scheduler, or state format.

## Route the request

Use this skill after an Agent home exists. A request to define a new business
job or create the first home is a build task. A request to install the suite,
provision identities/connectors, approve a production verifier, or configure
organization runtime policy is a platform-steward task.

## Standing rules

- Inspect before running. Never infer readiness from a folder name or prior
  conversation.
- Preserve literal exit statuses and unknown outcomes. Status 125 is never
  permission to retry automatically.
- Do not print secrets or environment values. Report resource, identity, scope,
  and capability presence only.
- Keep the worker away from May, Action policy, connectors, OAuth credentials,
  schedules, and controller evidence except through their public controller
  seams.
- Do not widen network, shell, filesystem, connector, or effect authority from a
  routine run request.
- A correction to one result is not automatically a reusable lesson. A skill
  change requires verified failed-then-passed evidence and exact review.
- Pause new work before incident response, authority changes, or retirement.

## 1. Establish operating state

Read [runtime-and-surfaces.md](references/runtime-and-surfaces.md). Identify the
exact Agent home, accountable owner, skill/runtime versions, execution location,
and intended operation. Run only read-only public inspection:

```text
/absolute/prefix/bin/bench version
/absolute/prefix/bin/bench home check HOME
/absolute/prefix/bin/bench home show HOME
/absolute/prefix/bin/bench home history HOME check
```

Classify:

- **OPERABLE** — compatible runtime, structurally valid home, inspected
  composition, known owner, explicit operation, and required evidence roots.
- **PAUSED** — the home is valid but a human decision, budget, connection,
  schedule, or incident blocks new work.
- **BROKEN** — check, history, definition, confinement, or runtime is invalid.
- **STEWARD-REQUIRED** — runtime, identity, credential, verifier, connector, or
  production control is missing or incompatible.

Do not run in the last three states.

## 2. Select one operation

Read [runbook.md](references/runbook.md). Distinguish:

- **run** — pursue the durable goal now as a distinct invocation;
- **interactive** — open Bench's Agent-home operator view in a real terminal;
- **resume** — repeat a named checkpoint only for the same intended Ask
  conversation;
- **tick** — run the cheap wake probe and call a model only when work exists;
- **schedule** — an outer system decides when a fresh invocation begins;
- **supervise** — Tend persists process transitions and explicit uncertainty;
- **inspect** — Trail/Agent history reads retained evidence;
- **act** — controller policy and conditional May review execute one proposal;
- **learn/amend** — propose and exactly review a durable definition change;
- **retire** — stop work, reconcile, revoke, retain, and close ownership.

Ask for the smallest missing decision. Do not combine a routine run with an
authority change or a schedule with an effect approval.

## 3. Admit the run envelope

Before a model-backed operation, show:

- Agent home and compiled definition digest when available;
- goal, input fixture or unit of work, and checkpoint semantics;
- model and reasoning policy;
- toolbox, shell, Cage, network, and writable roots;
- bounds the selected public Agent command actually exposes, plus outer time,
  output, data, cost, proposal, and effect controls owned by the caller;
- check command and controller authority;
- behavior for denial, timeout, interruption, broken verifier, and unknown
  effect.

Obtain approval for this run envelope. Approval does not carry into later runs
or widen the standing definition.

## 4. Run and preserve evidence

Use the literal public command appropriate to the operation. Prefer
`bench home` so the selected suite supplies the exact Agent companions; use the
absolute physical suite `bench` with its transparent `home` boundary in
generated OS scheduler definitions for the same reason. Record stdout, stderr,
exit status, terminal reason, evidence path, and next human decision in the
template from `assets/OPERATING-RECORD.md`.

For an accepted run, still review external business measures on the admitted
cadence. For negative, broken, denied, parked, interrupted, or unknown results,
preserve the exact state and stop. Do not paraphrase it into a successful answer.

## 5. Inspect and diagnose

Read [incidents-and-effects.md](references/incidents-and-effects.md). Use Agent
history and Trail rather than searching private storage formats. Classify the
failure layer:

1. outcome/contract;
2. procedure/skill;
3. evidence/source;
4. tool or connection;
5. authority/policy;
6. mutable state/checkpoint;
7. executable check or external oracle;
8. runtime/confinement/supervision.

Write facts, hypotheses, and unknowns separately. Preserve effect uncertainty
until an external observation reconciles it.

## 6. Change only the owning layer

Read [change-and-retirement.md](references/change-and-retirement.md).

- Correct changing facts in their authoritative source, not memory.
- Correct one unit of work in mutable work/state when no durable rule changed.
- Change a procedure only from replay-verified failed-then-passed evidence.
- Change the Agent definition through an exact reviewed patch and May decision.
- Change a verifier, connection, policy, identity, or runtime only through the
  platform steward.

Rerun structural checks, mutation tests, regression cases, withheld cases, and
authority review after the relevant change. Never use the improved run to erase
the original failed evidence.

## 7. Schedule or supervise deliberately

Read [scheduling.md](references/scheduling.md). A schedule is not a heartbeat;
a heartbeat is not crash recovery; a supervisor is not the business cadence.

For Cowork schedules, confirm that every required source, skill, connector,
artifact, and state path is available to a fresh cloud session. A local folder
or host binary is not durable cloud state merely because the original session
could reach it.

For supported local hosts, inspect
`scripts/prepare-local-operations.sh`. Run its read-only `plan` with the exact
home, verified Bench path, fresh output directory, cadence, and optional
non-secret model identifier for planning. The plan reports
**MODEL-IDENTIFIER-REQUIRED** when
it is absent, and render requires it because a fresh checkpoint-free scheduler
process cannot rely on an inherited model selection. The plan reports Cage
backend discovery but does not call it enforcement proof. After wake fixtures
and an equivalent bounded model probe in the fresh scheduler identity are
proved and the render is approved, prefer `render --approve --run-cage-check`;
it performs the separately disclosed transient 13/13 target-host check before
writing anything. If a nested Claude/Cowork sandbox cannot perform that
kernel-level check, return **CAGE-HOST-CHECK-REQUIRED** with the exact render
command for the user to run in a terminal on that same host. An externally
captured steward `cage-check` transcript may be used only when the operator
explicitly attests it came from this host and exact suite; the helper checks the
full pass markers, pinned suite-manifest digest, platform, host, and timestamp.
The transcript is not
cryptographically authenticated, so that same-host/freshness attestation remains
an operator decision. The helper writes a concrete runbook and disabled launchd
or systemd-user definition that directly calls
the physical suite Bench executable's transparent `home ... tick` entry. The
default is a distinct invocation with no checkpoint; add a named checkpoint
only for explicitly approved same-conversation continuity. Inspect the files;
activation is a separate approval and uses the host scheduler's public
commands. The helper never enables a timer, stores a secret, adds network,
disables Cage, or starts another runner.

Treat the rendered definition as a local pilot scaffold, not a complete
production scheduler. It does not itself enforce maximum runtime or model cost,
rotate logs, deliver failure notifications, or supply the missing
controller-owned provider credential seam. Require those controls in the
surrounding platform before unattended production admission.

Use Tend only when that exact Agent process needs crash-durable supervision.
Tend does not own business cadence, auto-retry unknown effects, or replace the
Agent checkpoint and check.

## 8. Retire completely

Retirement is an explicit operation:

1. disable schedules, triggers, and new Action proposals;
2. stop or drain supervised work;
3. reconcile pending reviews and every sent/unknown effect;
4. revoke OAuth profiles, connector grants, service identities, browser
   sessions, and shared capabilities used only by this worker;
5. record final definition/version, owner, reason, last accepted verdict, and
   unresolved obligations;
6. retain required controller evidence and archive or delete sensitive
   work/state under policy;
7. obtain final accountable sign-off.

Deleting the folder or editing `HEARTBEAT.md` alone is not retirement.

## User-visible handoff

End with:

- operating classification and exact operation performed or refused;
- evidence and terminal status;
- current authority and outstanding decisions;
- unresolved effects or incidents;
- next safe action and accountable owner.

## Bundled resources

- [prepare-local-operations.sh](scripts/prepare-local-operations.sh) — verifies
  a home and renders a concrete runbook plus a disabled platform schedule.
- [LOCAL-RUNBOOK.md](assets/LOCAL-RUNBOOK.md) — literal inspect, interactive,
  run, resume, tick, history, pause, supervision, and retirement record.
- [SCHEDULE-PLAN.md](assets/SCHEDULE-PLAN.md) — cadence, wake proof,
  fresh-process access, activation, pause, and retirement decision.
- [OPERATING-RECORD.md](assets/OPERATING-RECORD.md) — one literal operation and
  its status/evidence.
- [INCIDENT.md](assets/INCIDENT.md) — facts, uncertainty, owning layer, and
  recovery closeout.
- [RETIREMENT.md](assets/RETIREMENT.md) — complete shutdown and sign-off.
