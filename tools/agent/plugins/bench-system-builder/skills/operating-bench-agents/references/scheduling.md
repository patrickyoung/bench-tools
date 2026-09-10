# Scheduling and supervision

## Distinct responsibilities

- `bin/wake` decides cheaply whether useful work exists.
- `bench home ... tick` transparently reaches the suite Agent, runs that probe,
  and invokes the worker only when it wakes.
- An external scheduler decides when to call tick or run.
- Tend persists local process transitions, signals, timers, retries after
  observed failure, and explicit unknown-effect resolution.
- The business owner defines cadence, freshness, quiet behavior, and escalation.

Do not call one of these a substitute for the others.

## Local setup-to-schedule path

Inspect `../scripts/prepare-local-operations.sh`. It is a renderer, not a
scheduler or Agent wrapper. First run its read-only plan with literal absolute
paths:

```text
prepare-local-operations.sh plan \
  --bench /absolute/prefix/bin/bench \
  --home /absolute/agents/NAME \
  --out /absolute/controller/NAME/local-operations \
  --name NAME --model provider/model --minutes 60
```

`--model` is optional only while exploring the read-only plan. The renderer
requires an explicit non-secret provider/model identifier so the fresh,
checkpoint-free scheduler invocation never falls back to an absent `ASK_MODEL`
or nonexistent continued session.

The plan discovers the Cage backend but does not prove kernel enforcement.
Its preferred approved continuation is the same command changed to
`render --approve --run-cage-check`; that target-host check uses only temporary
fixtures and loopback and must report 13/13 before any schedule files are
written. If the current Claude/Cowork environment is nested and blocks the
check, emit **CAGE-HOST-CHECK-REQUIRED** and give that exact render command for
the user to paste into a terminal on the target host. Do not downgrade a nested
denial into a claim that the installed Cage is broken, and do not award
**SUITE-READY** from `cage status` alone.

Before rendering, require:

- `agent check` and `agent show` review;
- quiet, wake, and broken-wake fixtures for `bin/wake`;
- an approved cadence, timezone/DST expectation, missed-run behavior, maximum
  staleness, log retention, overlap policy, and notification owner;
- proof that a fresh scheduler process can reach its model provider without a
  secret in the home, argv, plist/unit, generated directory, or logs;
- no direct external effects in the ordinary unattended pilot, unless their
  proposal, policy, review, receipt, and uncertainty boundary is separately
  admitted.

After the render plan is approved, rerun with identical arguments plus
`render --approve --run-cage-check`. It creates a concrete
`LOCAL-RUNBOOK.md`, retained `CAGE-CHECK.txt`, disabled host definition, log
directory, and `ACTIVATE.txt`. It never copies into a scheduler directory or
enables the schedule. Inspect all rendered bytes; activation is a distinct
platform action. When a reviewed Cage transcript was necessarily captured by
the steward helper in a host terminal, `--cage-proved --cage-evidence ABS` is an
explicit operator attestation that it came from this same target host; never
use an invented, copied-from-another-host, stale, or partial record. The helper
binds the transcript to the exact suite manifest, platform, host name, and UTC
creation time, snapshots at most 64 KiB, validates that snapshot, and archives
those same bytes. This local record is not cryptographically authenticated; the
operator remains accountable for its same-host origin and freshness.

The renderer is intentionally a pilot scaffold. It does not enforce a maximum
runtime or model-cost budget, rotate/retain logs, or send failure notifications.
Do not call the resulting timer production-ready until the surrounding platform
implements and proves those named controls.

### macOS launchd

The rendered user LaunchAgent contains literal `ProgramArguments` for the
suite's absolute physical `bench home -C PARENT tick -q -m provider/model NAME`
(plus an optional reviewed checkpoint),
`StartInterval`, `KeepAlive=false`, and explicit stdout/stderr paths. Calling
the physical Bench executable ensures that the exact pinned Agent companions
come from the same suite even in launchd's sparse environment. It contains no
shell, credential, `-net`, or `-no-cage` argument.

After separate approval, install under `~/Library/LaunchAgents`, then use the
commands printed in `ACTIVATE.txt` to bootstrap, run once, inspect, and boot out
the exact label. Resolve the literal numeric user ID before use. Do not enable
automatic restart. launchd suppresses a new start while the same job is still
running. The default tick has no checkpoint and begins a distinct invocation.
Use a checkpoint only when the owner explicitly wants the same Ask conversation
to continue; it is never overlap policy or effect safety.

### Linux systemd user timer

The rendered `Type=oneshot` service invokes the suite's absolute physical Bench
executable with `home ... tick` and uses `Restart=no`. Its timer uses a fixed
elapsed interval, `Persistent=false`, and no random delay. Those defaults mean
no catch-up after downtime and no DST calendar semantics; choose a reviewed
`OnCalendar` design manually when the business obligation requires wall-clock
cadence. The default is checkpoint-free; explicit same-conversation continuity
is a separate business decision.

After separate approval, install both files under `~/.config/systemd/user` and
use the printed daemon-reload, enable, run-once, inspect, disable, and stop
commands. Do not silently enable login lingering; that changes host policy.

The user scheduler owns recurrence. Agent owns the home, any explicitly chosen
continuity checkpoint, wake gate, model loop, and check. Bench's `home` boundary
only supplies the exact suite companions. These files introduce neither a
second Agent runtime nor a Bench scheduler.

## Cowork scheduled tasks

Cowork scheduled work starts a fresh cloud session. Before scheduling, prove
that the skill/plugin, sources, connectors, managed Bench runtime, Agent home,
controller evidence, identities, and output destination are available to that
fresh session without a host-only path or the original conversation.

Define:

- exact prompt and job scope;
- timezone and cadence;
- no-work behavior;
- maximum duration/cost/effects;
- permission and approval behavior;
- artifact and evidence destination;
- failure and unknown-effect notification;
- pause, update, and retirement owner.

Do not schedule a design-only Cowork skill and describe it as a persistent Bench
Agent runtime.

## Tend

Use Tend when a local process needs crash-durable supervision. Submit one exact
command with explicit state root and bounds. Inspect immutable events. Retry only
an observed failure. Resolve an effect-unknown attempt only after external
reconciliation. Tend supervision does not authorize the child or certify its
business output.

When needed, submit the exact absolute suite Bench `home` command to Tend with a
Tend root outside worker writes, one portable job ID reused as the Agent
checkpoint, and an admitted resolution check. Bench supplies the pinned Agent
companions even under Tend's reduced environment. The host scheduler may call `tend work` as a
one-step worker; Tend still does not create recurrence. Retry only an observed
failure. After execution started, interruption or lost outcome is unknown until
an operator inspects Agent history, Tend events, the work product, and any
external system, then explicitly resolves it. Cancellation is not rollback.

The literal composition is:

```text
TEND_ROOT=/absolute/controller/NAME/tend tend submit \
  -id JOB -key NAME -C /absolute/agents/NAME/work \
  -check ../bin/check -- \
  /absolute/prefix/lib/bench-suite/0.13.0/bin/bench home \
  -C /absolute/agents run -q -m provider/model -checkpoint JOB NAME

TEND_ROOT=/absolute/controller/NAME/tend tend work
TEND_ROOT=/absolute/controller/NAME/tend tend show JOB
TEND_ROOT=/absolute/controller/NAME/tend tend events JOB
TEND_ROOT=/absolute/controller/NAME/tend tend check
```

`tend work` status 1 is idle, not controller failure. For an observed failed
attempt, a person may choose `tend retry JOB`, then call `tend work` again. If
execution may have started but its outcome is unknown, inspect the Agent and
external state first; only a person may choose `tend resolve JOB retry|done|fail`.
`done` is valid only when the submitted resolution check independently proves
completion. Never put a secret in Tend argv or durable signal data. Tend's
environment allowlist does not make a provider credential invisible to the
model-authored Agent action; apply the same provider-readiness gate as direct
scheduling.
