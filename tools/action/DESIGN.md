# Design

## Sentence

`action` runs one explicitly named connector with one exact JSON request after
an explicit deterministic policy and, when required, one exact May decision.

## Boundary

Context is the read-side seam:

```text
named connector + query -> validated evidence records
```

Action is the effect-side seam:

```text
named connector + exact request -> policy -> optional May -> result
```

Connectors are ordinary programs on `ACTION_PATH`. They implement:

```text
CONNECTOR describe    stdout: one descriptor JSON object
CONNECTOR run         stdin: one canonical JSON object
                      stdout: one result JSON object
```

The descriptor and connector executable digest are observations. Admission
and policy remain operator decisions. MCP can generate connectors, but Action
does not know MCP exists.

Agents and other untrusted preparers use only the strict proposal file
`{"version":1,"connector":"NAME","input":{...}}`. It cannot select an
executable path, policy, May, Ask, credential, directory, or approval mode.
The stable controller job is included in the canonical action envelope, so
policy and May bind the surrounding invocation as well as connector and input.

## Catalog fit

The Bench catalogue stays split by invariant rather than by SaaS vendor:

| app surface | filter boundary |
| --- | --- |
| search, fetch, list, inspect | Context connector producing cited evidence |
| create, update, delete, send, buy, publish | Action connector with exact input |
| dynamic MCP discovery and schema pinning | MCP and `mcpbox`; generated Action connector for effects |
| resource-bound login and refresh | OAuth outside the proposal |
| deterministic allow/deny/escalate rules | operator policy program |
| human-in-the-loop | May over the exact Action envelope |
| model and effect replay | Ask sealed events, browsed through Trail |
| durable foreground retry/recovery | Tend outside Action; never retry exit 125 |
| worker write/network confinement | Cage around model actions, not around controller approval |

That covers app catalogues with mixed read/write tools without teaching Agent
or Action about Jira, Slack, GitHub, Canva, Cloudflare, finance, calendars, or
any other provider. A connector may wrap a native CLI, HTTP client, or admitted
MCP tool; its provider schema is data at the edge, while the control contract
is stable.

## Policy

The policy program receives the exact canonical action envelope on stdin and
prints one canonical JSON line:

```json
{"version":1,"action_sha256":"sha256:...","decision":"allow","reason":"..."}
```

Exit 0 must say `allow`, exit 3 must say `deny`, and exit 75 must say `review`.
Review is not approval: Action then calls `may request JOB` with the same exact
action envelope. Only May exit 0 plus a matching `spent` result proceeds.

Without `-policy`, the deterministic decision is `review`, so there is no
implicit auto-approval path.

## Replay

With `-record SESSION`, Action composes `ask note -seal` and writes:

```text
action.proposal/v1  canonical action, connector and input digests
action.decision/v1  exact policy result and optional May result
action.attempt/v1   connector started behind an unreleased stdin pipe
action.sent/v1      complete canonical request released to the connector
action.result/v1    terminal status and exact captured streams
```

Every body binds the prior body's SHA-256. Ask owns the append-only event log
and prefix seals; Action owns only the factual receipts it submits. Replay
therefore distinguishes:

- proposal or denial: no connector process;
- decision without attempt: authorized but never started;
- attempt without a terminal result: conservatively unknown;
- sent without a terminal result: effect may exist and must not be retried;
- result: a trustworthy terminal connector outcome.

The attempt is sealed after process start but before request release. A
connector must not cause effects before reading its complete stdin object.
If Action dies after starting the blocked child but before sealing the attempt,
the controller lacks trustworthy evidence and must resolve the external state
before retrying.

## Exit status

```text
0    trustworthy successful terminal result
1    trustworthy negative terminal result
2    invalid/broken before release, or a receipted connector reports no effect
3    deterministic or human decline; request not released
75   parked for May; request not released
125  request may have taken effect without a complete trustworthy receipt
130  interrupted before request release
```

Connector exit 75 may describe a valid unfinished remote action. Its terminal
result receipt has `sent: true`, unlike approval parking.

## Agent

Agent must invoke Action from a controller command outside Cage. A running
model may prepare an action proposal beneath `work/actions/`, but it cannot
invoke May, choose policy, or execute the connector from inside the model
action boundary. The controller validates the proposal, derives a stable May
job, selects the operator policy and connector path, and points `-record` at
the proposal's replayable Ask session.

This is the same shape as Agent definition amendments: writable proposal,
read-only inspection, exact snapshotted controller action, May, and receipts
outside worker-writable roots.
