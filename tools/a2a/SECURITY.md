# Authority and security limits

A2A authenticates a network caller and authorizes access to protocol tasks.
It does not make an arbitrary local executable safe. Run the listener under a
service account with only the authority its admitted worker needs. For Agent,
use its normal Cage boundary and explicitly select model/action credentials.
The A2A workspace separation alone does not confine filesystem reads, process
creation, networking, or access to other files owned by the service account.

Use the direct TLS listener with mTLS or configured OAuth introspection.
Forwarded identity headers are never trusted. There is no insecure TLS switch.
The development HTTP exception accepts only explicitly selected literal
loopback addresses. The public resource metadata endpoint contains no secrets;
all A2A operations, including cards, require the configured authentication.

Owner checks apply to task reads, listing, continuation, subscriptions,
references, and cancellation, including SDK operations that otherwise find an
active execution before consulting the task store. Context IDs are owned data,
not authorization tokens. A new identity cannot claim an existing context.
Private records and Tend evidence are readable by the service account. Do not
put them under an untrusted worker's writable sandbox roots.

The caller selects one explicit endpoint. Client and introspection transports
verify certificates, disable redirects and implicit proxy use, and never
retransmit an application request. JSON-RPC response IDs must match. Bounded
JSON parsing rejects duplicate keys, trailing values, excessive nesting, and
truncated event frames. Credentials use descriptors or protected files;
protocol diagnostics omit private HTTP bodies and supervisor stderr.

The listener's worker is operator-selected literal argv. The remote request
cannot select a command or root directory. Text is stdin, never shell source.
`-pass-env` is operator authority to give a variable to that worker, including
any selected provider secrets. A worker still runs with the listener account's
OS authority unless the selected command establishes confinement itself.

Input, exported output, record size, execution slots, and job duration are
bounded. The state archive and Tend's raw stdout/stderr can accumulate without
a disk quota. Use OS resource limits, storage quotas, and an external retention
policy appropriate to the service. Multiple subscriptions and authenticated
request traffic also need deployment-level connection/rate limits. There is
no multitenant billing, distributed lock, replicated store, or global quota.

A task marked complete reflects an observed worker result. A transport failure
may follow an external effect. Exit 125 and `bench/outcome: unknown` preserve
that uncertainty. A client disconnect is not a cancel request. A hard listener
kill does not necessarily kill Tend; its configured deadline remains in force.
Restart does not rerun unrecorded work. Cancellation does not roll effects back.
Do not promise exactly-once work, safe retry by message ID, or distributed
transaction semantics on top of these interfaces.

A2A 1.0 JSON-RPC and REST are supported. gRPC, push delivery, execution migration,
authentication sessions held open inside a local worker, and earlier A2A wire
versions are outside this listener's contract. This is not a public deployment
claim: validation uses local peers, test certificates, and an offline model
fixture. Production identity-provider interoperability and deployment policies
must be checked with the selected service configuration.
