# A2A: agents across a network boundary

Your report expert runs on a Linux server; its caller lives on a laptop or in
another service. They can exchange a job and a result without sharing a model
runtime, filesystem, or programming language.

`a2a` is a Unix filter for calling an agent on another machine. It reads one
A2A request from stdin, writes JSON to stdout, and gives the outcome through
its exit status. `a2aserve` exposes one operator-selected executable over A2A.

Hire builds expert definitions. Agent runs them. Tend supervises the listener's
local attempt. These programs retain their existing jobs; this component adds
network admission and A2A task records, using the official Go SDK v2.5.0 and
A2A 1.0 JSON-RPC or REST. It contains no model client, goal loop, scheduler,
remote agent registry, automatic retry, or shared Bench runtime.

The client is a filter. The listener is a network service, started and stopped
by your operating system. Calling the listener a filter would obscure that
lifecycle. Neither command is required for local Agent use.

## Install

From the Bench tools checkout, with Go 1.26+, Python 3.9+, and Git:

```sh
python3 scripts/install a2a tend
```

This installs `a2a`, `a2aserve`, and the existing `tend`. Calling a remote agent
needs only `a2a`. Serving Agent also requires Agent's documented companions:

```sh
python3 scripts/install agent ask brief ply cage
```

The component is independently buildable, with no sibling source imports:

```sh
cd tools/a2a
go install ./cmd/a2a ./cmd/a2aserve
```

Install Tend separately before using `a2aserve`, or select its absolute path
with `-tend`. There is no installation-time daemon or credential setup.

## Try the boundary locally, without a model

From a fresh directory, save a small agent card:

```sh
cat > card.json <<'JSON'
{"name":"Echo worker","description":"Returns the supplied text.","version":"1.0.0","skills":[{"id":"echo","name":"Echo","description":"Return input text","tags":["demo"]}]}
JSON
a2aserve -dev-loopback -listen 127.0.0.1:8080 -state echo-state \
  card.json -- /bin/cat
```

Leave that terminal running. In a second terminal with `a2a` on PATH:

```sh
printf '%s\n' '{"message":{"messageId":"echo-001","role":"ROLE_USER","parts":[{"text":"Hello from another process"}]}}' |
  a2a request -http-loopback send http://127.0.0.1:8080/rpc
```

Expect a completed task whose text artifact contains `Hello from another
process`. The response is a protocol object, not bare text. The worker's
stderr and execution evidence stay with Tend. Stop the listener with Ctrl+C.
Use another port if 8080 is occupied.

Both loopback flags are explicit: this demonstration uses no authentication
and accepts only a local connection. It proves the request → ordinary process
→ result path without a model. To serve a real expert, replace `/bin/cat`
with the Agent invocation below and configure authenticated TLS.

## First request

Save this as `request.json`. Generate a new `messageId` for each intentional
message; preserve it and the returned handles in your calling program.

```json
{
  "message": {
    "messageId": "report-request-001",
    "role": "ROLE_USER",
    "parts": [{"text": "Write the weekly report from the supplied evidence."}]
  }
}
```

With credentials issued for an admitted worker:

```sh
a2a request -ca ca.pem -cert client.pem -key client.key \
  send https://worker.example/rpc < request.json > result.json
```

Flags precede the method and URL. The JSON is the A2A method's parameters;
`a2a` supplies the wire envelope. Inspect the process status before treating
the output as completed work:

| Exit | Meaning |
| --- | --- |
| 0 | Completed task or successful query/cancel operation |
| 1 | Known negative: failed/rejected/canceled task, or explicit protocol denial |
| 2 | Invalid input/configuration or failure before transmitting the HTTP request |
| 75 | Accepted unfinished task, including input/authentication handoff |
| 125 | Uncertain outcome: interrupted transmission, malformed/truncated response, or retained unknown execution |
| 130 | Interrupted before transmission |

A successful cancellation operation returns 0. Reading a canceled task returns
1. Listing working tasks returns 0 for the query, without declaring those tasks
complete. Completion describes the peer's report; it is not independent proof
that its deliverable satisfies your goal. Agent's check supplies that judgment
when Agent is the configured worker.

`-timeout 5m` bounds the client invocation. A client timeout or disconnected
pipe does **not** cancel a remote task. Never blindly repeat a transmitted
mutation after exit 125: it may already have had effects. Use a known task ID
to inspect it; if the connection failed before delivering a handle, reconcile
through the peer's records. `messageId` alone does not promise deduplication.

## Discover, stream, and continue

```sh
a2a discover -ca ca.pem -cert client.pem -key client.key https://worker.example

a2a listen -ca ca.pem -cert client.pem -key client.key \
  send https://worker.example/rpc < request.json > events.jsonl

printf '%s\n' '{"id":"TASK_ID"}' |
  a2a request -ca ca.pem -cert client.pem -key client.key get https://worker.example/rpc
```

Discovery returns the card without following its endpoints. Select the endpoint
and transport yourself; cards cannot redirect credentials. To use REST, choose
`-transport rest` and the advertised REST base, `/rest` on this listener.
There is no fallback between versions, transports, hosts, or origins.

`request` supports `send`, `get`, `list`, `cancel`, `card` (authenticated extended
card), `push-get`, `push-list`, `push-set`, and `push-delete`. `listen` supports
`send` and `subscribe`. Peer capabilities determine which methods succeed.
This listener advertises streaming and extended cards, but no push delivery.

A stream is one JSON object per line, using A2A's `task`, `message`,
`statusUpdate`, or `artifactUpdate` envelope. It returns on a terminal outcome
or an input/authentication handoff; it never reconnects automatically.

For a prompt return with a handle, add this top-level request field:

```json
{"configuration": {"returnImmediately": true}}
```

For continuation, send a new message with the returned `taskId` and `contextId`
inside `message`, plus a new `messageId`. The task must belong to that identity
and permit continuation. Each new task gets a fresh workspace and context by
default. Naming an owned context explicitly does not copy a model transcript
or merge workspaces. A follow-up to the same task reuses its files; Agent
continues according to its own explicit state/checkpoint options.

## Expose an expert

An operator-owned `card.json` describes the admitted worker:

```json
{
  "name": "Report expert",
  "description": "Produces a checked report from supplied material.",
  "version": "1.0.0",
  "skills": [{"id":"report", "name":"Report", "description":"Write a report", "tags":["reporting"]}]
}
```

For mTLS, save this as `auth.json`:

```json
{"mode":"mtls", "clientCA":"client-ca.pem"}
```

Then start the listener with literal arguments:

```sh
a2aserve -listen 0.0.0.0:8443 -public-url https://worker.example:8443 \
  -state /srv/report-service -auth /srv/auth.json \
  -tls-cert /srv/server.pem -tls-key /srv/server.key \
  -pass-env ASK_MODEL -pass-env ANTHROPIC_API_KEY \
  -artifact report.md /srv/card.json -- \
  agent run -C . -state ../state -evidence ../control /srv/experts/report
```

The expert directory must include Agent's valid definition and `bin/check`.
This example assumes an Anthropic model already configured in the listener's
environment; select the credential/routing variable names for your provider.
Only their names appear in the command. Model configuration must reach the
worker explicitly; the remote caller's authentication is a separate identity.
If it has no fixed goal, Agent uses the incoming text as the goal. A fixed
`GOAL.md` instead makes incoming text evidence for that goal. The configured
command is invoked directly through Tend, without shell evaluation. A remote
request cannot choose another program or filesystem root.

The listener updates the card's endpoints, capabilities, and authentication
schemes to match its actual configuration; prior card signatures are removed.
The authenticated card lives at `/.well-known/agent-card.json`, JSON-RPC at
`/rpc`, and REST at `/rest`. Nonmatching browser Origins are refused.

State must be a private directory, mode 0700 or stricter, on a local filesystem.
Only one listener may own it. TLS private keys and credential files must be
regular files, mode 0600 or stricter. Paths inside `auth.json` resolve relative
to that file; command-line paths resolve relative to the invoking directory.

For local experiments only, `-dev-loopback -listen 127.0.0.1:8080` explicitly
selects unauthenticated HTTP. Clients must explicitly use `-http-loopback`.
Production TLS/authentication flags cannot be mixed with this exception.

## OAuth uses the existing OAuth tool

For OAuth, configure the listener with an authorization server's authenticated
RFC 7662 introspection endpoint:

```json
{
  "mode":"oauth",
  "issuer":"https://login.example",
  "introspectionURL":"https://login.example/introspect",
  "credentialsFile":"introspection.json",
  "audience":"https://worker.example",
  "scopes":["agent:run"]
}
```

The private `introspection.json` contains `client_id` and `client_secret` for
the resource server. `ca` optionally selects a private CA bundle for that
introspection connection. The listener checks active status, audience, required
scopes, issuer when returned, and any expiration/not-before claims. It derives
a stable owner from issuer, subject, and client ID. Invalid or insufficient
credentials cannot access tasks. No caller token is passed to the worker.

RFC 9728 resource metadata is public at
`/.well-known/oauth-protected-resource`; protocol operations and cards require
authentication. Publish the exact configured resource identity as the OAuth
login target, and configure its authorization server's discovery metadata.

```sh
oauth login reports -client-id YOUR_CLIENT_ID https://worker.example
oauth with reports -- a2a request -header-fd 3 \
  send https://worker.example/rpc < request.json > result.json
```

OAuth acquires/refreshes credentials before invoking A2A and passes a header on
descriptor 3. Stdin remains the request; the child's status is preserved. The
operator must bind the profile to the intended resource and explicitly select
the correct child endpoint. A2A does not acquire tokens, follow card-selected
URLs, refresh mid-request, or retry an authentication challenge.

mTLS owners are SHA-256 hashes of the verified client certificate's public
key (`mtls:HEX`). OAuth owners are `oauth:HEX` hashes of the JSON tuple
`[issuer, subject, client_id]`. Either policy may include an `allow` list of
those identities. Without one, all valid identities admitted by the configured
CA or OAuth policy may create their own tasks. A new mTLS key or OAuth client
identity does not silently inherit an earlier identity's task records.

## Worker streams and files

The default `-input text` requires exactly one text part and passes its bytes
unchanged to stdin. `-input json` passes `{taskId, contextId, message, metadata}`,
including text, structured data, inline bytes, and URL parts. It does not fetch
URLs or include HTTP headers, credentials, or another task's history.

The default `-output text` exports stdout as a text artifact. Exit 0 completes,
75 requests more input, other ordinary nonzero exits fail, and Tend's unknown
status or worker 125/130 remains uncertain. Stderr stays in private Tend
evidence. An exit 75 is a handoff, never an automatic local retry.

`-output json` accepts a task-shaped object with `status`, `artifacts`, and
optional `metadata`. Optional `id`/`contextId` must match the current task.
A nonzero process cannot claim completion by writing successful JSON. Worker
metadata cannot overwrite reserved `bench/` facts. A local `AUTH_REQUIRED`
result becomes `INPUT_REQUIRED` with `bench/authRequired: true`: the process has
exited, and the caller must explicitly resume after arranging authentication.
The client can consume native `AUTH_REQUIRED` results from other peers.

Each repeated `-artifact RELATIVE_PATH` exports a required regular file from
the task workspace when the task completes, as an inline binary part with a
filename and media type. Missing files or escaping symlinks fail the result.
No incoming filename selects a writable local path automatically.

`-max-input` defaults to 16 MiB; `-max-output` to 64 MiB. Exported JSON and
combined artifacts must fit the output bound. Required files together may use
at most half that bound before encoding. Tend currently permits at most 64 MiB
of stdin. `-max-active` defaults to 8; `-timeout` gives Tend a 30-minute job
deadline. These are admission/export limits, not disk quotas or a sandbox.

Only a small documented environment reaches Tend: `PATH`, `HOME`, `LANG`,
`LC_ALL`, and existing `ASK`, `PLY`, `MAY`, `CAGE` selectors. `-pass-env NAME`
explicitly adds variables such as a model provider credential; reserve that for
trusted worker configuration. Incoming authentication headers never become
worker variables. Task-specific `TMPDIR`, `A2A_TASK_ID`, `A2A_CONTEXT_ID`,
`A2A_WORKSPACE`, `A2A_STATE`, and `A2A_EVIDENCE` identify selected directories.

## Retained work and interruption

Private files under the state directory are:

- `records/SHA256(taskId).json`: owner and latest SDK task record.
- `workspaces/SHA256(taskId)/{work,state,control,tmp}`: separate task roots.
- `evidence/SHA256(taskId)/SHA256(messageId)/tend`: Tend's private job root,
  containing its one `worker` job and sealed stdout/stderr evidence.

The listener verifies Tend evidence with `tend check` before exporting a result.
For inspection, select that exact evidence root through `TEND_ROOT` and use
Tend's ordinary `show worker`, `events worker`, and `check` operations. Do not
run `tend work` as an inspection operation: it can perform work.

Graceful shutdown cancels local execution through Tend. After an abrupt
listener kill, a surviving Tend process can continue until its existing job
deadline. Restart marks unrecorded active tasks **unknown**, retains their
handles and evidence, and does not reattach or resubmit them. Inspect those
facts before deciding what to do next. Cancellation stops local supervision;
it cannot undo external effects already performed.

## Verification and limits

```sh
go test ./...
go test -race ./...
go vet ./...
A2A_TEST_TEND=/absolute/path/to/tend \
  A2A_TEST_OAUTH=/absolute/path/to/oauth \
  A2A_TEST_CLIENT=/absolute/path/to/a2a \
  go test -race -count=1 -timeout 90s ./internal/serve
```

Without the named companion executables, those integration cases explicitly
skip. Root `scripts/check --integration-only` supplies separately built tools
and also runs real listener/client/Agent process tests. No paid model is called;
Agent uses the existing offline provider fixture for these tests.

Read [SECURITY.md](SECURITY.md) for the authority boundary and
[DESIGN.md](DESIGN.md) for why this focused adapter exists beside the SDK.
