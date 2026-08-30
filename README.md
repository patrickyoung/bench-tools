# Action

Action is the effect-side companion to Context. Context gives heterogeneous
evidence sources one read interface; Action gives heterogeneous state-changing
connectors one controlled execution interface.

```sh
export ACTION_PATH=/operator/owned/actions

action ls
action show create-ticket
action inspect work/actions/ticket.json
action run -job support-ticket-42 -record run.jsonl \
  -proposal work/actions/ticket.json
```

The proposal is deliberately small:

```json
{"version":1,"connector":"create-ticket","input":{"title":"Broken login"}}
```

It does not name a command, executable path, approval mode, policy, or
credential. The controller supplies those authorities. Action resolves and
hashes the named connector, asks it for a descriptor, canonicalizes the input,
and constructs the exact action envelope used by every later stage.

## Connector interface

Every executable on `ACTION_PATH` implements two operations:

```text
CONNECTOR describe  -> one JSON descriptor on stdout
CONNECTOR run       <- one canonical JSON object on stdin
                    -> one JSON result on stdout
```

Example descriptor:

```json
{"version":1,"name":"create-ticket","description":"Create one support ticket","input_schema":{"type":"object"}}
```

Connector discovery is not admission. Put only reviewed connectors on the
controller-owned `ACTION_PATH`. The first matching executable wins, as with
`PATH`. Action hashes both its bytes and its canonical descriptor, then checks
both again after authorization and before releasing the request.

MCP fits without a protocol branch in Action. `mcpbox admit BOX actions NAME`
generates a connector with this interface and pins the MCP tool descriptor.
The same shape covers the action surfaces in issue trackers, calendars,
storage, CRM, messaging, music, shopping, finance, design, infrastructure,
and other app catalogues. Provider-specific schemas stay in connectors; the
control plane stays one filter contract.

## Policy and human review

Without `-policy`, every proposal is reviewed by May. A deterministic policy
receives the exact canonical action envelope on stdin and must emit one
canonical JSON line:

```json
{"version":1,"action_sha256":"sha256:...","decision":"allow","reason":"matched operator rule"}
```

The exit status and result must agree:

```text
0   allow
3   deny
75  review through May
```

There is no model decision, MCP-annotation bypass, or auto-approve flag.
Review calls `may request JOB` with the same exact envelope. Only a matching,
single-use `spent` result releases the connector input.

## Replay events

`-record SESSION` appends sealed structured notes through Ask:

```text
action.proposal/v1
action.decision/v1
action.attempt/v1
action.sent/v1
action.result/v1
```

The connector starts behind an unreleased stdin pipe. Action seals `attempt`,
releases the complete canonical input, seals `sent`, waits for the connector,
then seals `result` before publishing connector stdout. Each receipt binds the
prior receipt body by SHA-256. Ask remains the only event-log implementation,
so ordinary `ask replay -check SESSION` and Trail verify and browse the same
history as model and verifier events.

This sequence distinguishes denial, approval parking, a prepared process, a
released effect, and a trustworthy terminal result. If the request may have
escaped but the terminal result cannot be validated or sealed, Action returns
125, suppresses result stdout, and tells the caller not to retry automatically.

## Agent integration

An Agent worker writes proposals beneath `work/actions/`; it does not receive
Action, May, policy, or connector paths in its Ply environment.

```sh
agent actions HOME
agent actions HOME create-ticket.json
AGENT_ACTION_PATH=/operator/owned/actions \
  agent act HOME create-ticket.json SESSION
```

`agent actions` is bounded and read-only. `agent act` runs outside Cage,
selects controller-only connectors, derives a stable May job from the current
definition and proposal hashes, and records into an existing Ask session under
that home. `-policy PROGRAM` can select an operator policy. Action's status is
returned unchanged.

## Exit status

```text
0    trustworthy successful connector result
1    trustworthy negative connector result
2    invalid or broken before release, or a trustworthy connector rejection
3    policy or human denial before release
75   waiting for May, or a receipted unfinished connector result
125  an effect may exist without a complete trustworthy receipt; do not retry
130  interrupted before release
```

Connector exits 0, 1, and 75 require one JSON result object. A connector that
crosses its own remote effect boundary must itself use 125 when it cannot
determine the outcome.

## Install and verify

Action requires Go 1.26 or later and a Unix.

```sh
go install github.com/patrickyoung/action@latest
go test ./...
go test -race ./...
go vet ./...
```

See [DESIGN.md](DESIGN.md) for the event model and [SECURITY.md](SECURITY.md)
for the authority and crash boundaries.
