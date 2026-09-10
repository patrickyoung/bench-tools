# Local Bench Agent runbook

Fill this with literal absolute paths. It documents how to reach the shipped
Bench Agent; it is not another agent definition or controller.

## Identity and paths

- Business job and accountable owner:
- Bench suite / Agent version:
- Absolute Bench entry point:
- Absolute Agent entry point:
- Absolute Agent home:
- Parent directory / home basename:
- Controller evidence, schedule, log, and Tend roots:
- Non-secret provider/model identifier:
- Provider access owner and revocation path:

## Inspect before work

- `bench home -C PARENT check NAME`
- `bench home -C PARENT show NAME`
- `bench home -C PARENT history NAME check`
- Current classification: OPERABLE / PAUSED / BROKEN / STEWARD-REQUIRED

## Run modes

- Interactive terminal: `bench -C PARENT -m provider/model -home NAME`
- Distinct on-demand run: `bench home -C PARENT run -m provider/model NAME`
- Named continuation: `bench home -C PARENT run -m provider/model -checkpoint CASE NAME`
- Cheap wake, distinct by default: `bench home -C PARENT tick -m provider/model NAME`
- Optional same-conversation wake, only when explicitly intended:
  `bench home -C PARENT tick -m provider/model -checkpoint CASE NAME`
- History inspection:

## Schedule

- Outer scheduler and owning identity:
- Disabled definition path:
- Cadence / timezone / DST behavior:
- Missed-run behavior:
- Quiet, wake, and broken-wake fixture evidence:
- Fresh-process provider test:
- Overlap behavior and checkpoint:
- Enable command and approval:
- Inspect command:
- Disable and stop commands:
- Logs and retention:

No secret may appear in this file, a plist/unit, argv, logs, or the Agent home.

## Crash-durable supervision, if admitted

- Why Tend is needed:
- Exact submitted Agent argv:
- Tend root outside worker writes:
- Job ID / serialization key / checkpoint:
- Inspect, cancel, and observed-failure retry commands:
- Unknown-state reconciliation owner:

## Pause, incident, and retirement

- Disable new triggers:
- Stop or drain current work:
- Inspect Agent/Tend history and May/Action receipts:
- Reconcile sent or unknown effects:
- Revoke provider, OAuth, connector, and workload access:
- Retain/archive/delete evidence under policy:
- Final owner sign-off:
