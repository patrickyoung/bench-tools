# Remote agents through ordinary programs

The user-facing objective is to expose and consume agents on other machines
without replacing Bench's Unix composition. Hire builds definitions. Agent runs
them through its existing companions. This component owns A2A communication,
authenticated request admission, and the protocol's task records only.

## Implementation and reuse

Use `github.com/a2aproject/a2a-go/v2` v2.5.0. The upstream CLI was exercised before
building this adapter: piped JSON, multiline results, task retrieval, independent
context IDs, and cancellation worked. Its process exit remained zero for both a
failed task and accepted unfinished work. Its executable mode uses `sh -c`, and
the CLI lacks Bench's protected credential descriptor and server authentication
boundary. These are the reasons for this focused adapter, rather than a new A2A
implementation or changes to Agent.

## Required public contracts

`a2a discover [options] URL` returns an Agent Card. `a2a request [options] METHOD
URL` reads one bounded JSON request and writes one JSON result. `a2a listen
[options] METHOD URL` writes one JSON event per line. Methods cover send, get,
list, cancel, subscribe, extended cards, and push configuration where supported
by the peer. Endpoint and transport selection are explicit; there is no silent
protocol downgrade, redirect, reconnection, or resubmission.

REST and JSON-RPC use the official SDK. TLS certificate verification is always
enabled. Plain HTTP requires an explicit loopback-only development option.
Credentials arrive on a caller-selected descriptor, compatible with `oauth
with`. Private CA roots and client certificates support mTLS. An Agent Card
cannot redirect credentials or execution to an unapproved origin.

Stdout contains protocol data. Stderr contains diagnostics without credentials
or private request bodies. Exit 0 means a complete positive outcome, 1 a known
negative, 2 pre-transmission validation failure, 75 accepted unfinished work,
125 an uncertain outcome after transmission, and 130 an interruption before
transmission. Task IDs and context IDs remain data. Polling a working task does
not mean its goal completed. Canceling a task is an explicit operation.

`a2aserve` takes an operator-owned card, state directory, authentication/TLS
configuration, and literal worker argv. It creates a separate workspace for
each new task. Remote input cannot choose a program, workspace root, identity,
credential, or inherited conversation. Follow-ups explicitly name an owned task
or context. Worker input/output support full structured requests/results as well
as the text interface needed by existing `agent run`. File deliverables are
exported only through explicitly selected, bounded paths.

The listener authenticates every request and enforces ownership on reads,
listing, continuation, subscription, and cancellation. Support real mTLS and
OAuth bearer validation through a configured authorization server; obtaining
tokens remains the existing OAuth tool's job. Production serving requires TLS
and authentication. Development exceptions are explicit and loopback-only.

Tasks survive listener restarts as protocol records. Interrupted active work is
recorded as uncertain, never automatically rerun. Tend owns execution and descendant cleanup on cancellation/graceful shutdown.
After a hard listener kill, a surviving Tend process can continue until its
already selected job deadline. Restart records uncertainty instead of claiming
it stopped. The adapter verifies Tend's retained evidence before exporting
results and bounds input, exported output, concurrency, and job duration.
Raw supervisor output is not a disk quota. SDK protocol task handling is reused;
no provider calls, goal retries, new work queue, or scheduler are added.

The listener requires the existing Tend executable. Each admitted message has
one private Tend root, one literal submission, and one `tend work` transition.
It never repeats that transition or resolves unknown effects automatically.
A2A leaves lease timing to Tend's default rather than imposing a subsecond
startup lease. The configured job deadline and explicit cancellation remain
independent bounds; a missing terminal receipt still reports uncertainty.
A local worker that exits requesting authentication is an input-required
continuation with explicit metadata; the SDK's open auth-required execution
would retain a slot after the process has exited. Other peers' native auth
handoffs remain supported by the client.

## Completion evidence

Implementation is complete only after the following are demonstrated:

1. Native Go request and serving commands, pinned SDK, standalone build/check
   and monorepo package/install/relocation support.
2. Real executable stdin/stdout integration in both directions with an upstream
   SDK peer, including text, structured data, file artifacts, and task handles.
3. Correct complete/failed/unfinished/uncertain outcomes, bounded streams,
   explicit continuation, cancellation, and no mutation retries.
4. Verified TLS, mTLS, OAuth credential handoff and server validation; rejected
   invalid credentials, origin changes, and access to another caller's work.
5. Per-task context/workspace separation, persistent records, cancellation and
   restart behavior, and actual composition with Agent and existing tools.
6. Manuals, examples, security limits, and meaningful Go/race/vet and executable
   checks. Offline evidence must not be presented as live remote deployment or
   a paid model evaluation.
