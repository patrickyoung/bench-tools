# Matterbridge application boundaries

This is an optional deployment example, not a toolkit-wide runtime or daemon.
All independent Bench tools retain their modules, executable contracts and
state ownership. No implementation imports another executable's code.

| Program | Owns | Public interfaces used |
| --- | --- | --- |
| Matterbridge | Network connections and message relay | Its documented HTTP API, private TOML configuration |
| `bridge` | Identity allowlist, topic routing, conversation records, transport inbox/outbox | Matterbridge HTTP; deployment `service`; Tend provisioning commands |
| `provision` | One admitted package installation and smoke-run workflow | Source packager, package `install`, `service`, Telegram topic creation |
| `queue-worker` | Shell repetition and exit propagation | One `tend work` at a time |
| `service-worker` | Existing deployment worker lifetime | One `service work` at a time |
| `sandbox-builder.py` | Container-only authoring composition and bounded source export | `hire build`, `hire verify`; Hire invokes Agent |

`files.py` holds this example application's file, literal-command and HTTP
helpers. It owns no global state selection, model runtime, policy engine or
job scheduler. No Bench component imports it.

## Streams

`bridge --state ABS_DIR init` reads one configuration JSON on stdin. It creates
new private files and produces no credential-bearing output. `ingest` reads
one Matterbridge message JSON and emits `{"admitted":true|false}`. This is a
trusted local adapter boundary, not an unauthenticated public endpoint.
`status` emits one bounded JSON overview. `render` writes private TOML and
emits only its path. `receive`, `tick` and `flush` each perform a bounded
application step; they emit no model answer on stdout. Diagnostics use stderr.
Success is 0; validation/controller failure is 2. `run` supplies repetition
and process supervision for this optional application.

`provision --state ABS_DIR ID` reads its immutable admission record from
`deployments/ID/request.json`. Tend also retains that exact record as stdin.
Provisioning progress uses stdout/stderr captured by Tend; its final JSON
receipt reports `awaiting-bridge`, not successful network delivery. Zero means
installation, smoke execution and topic provisioning completed. Failure is 2;
an interrupted command is handled by Tend's ordinary uncertainty contract.

Job commands continue using the deployment service's own JSON and status
contracts. All executable composition uses literal argv; user text appears
only in retained data files or stdin. No chat message selects a shell program,
host directory, model credential file, deployment host or recovery decision.

## State and identity

The operator selects one absolute private state directory. The application
represents one collaborating Telegram group, not a multi-tenant server. The
configured Telegram account, exact topic route and numeric sender ID must all
match before a message is admitted. Display names and forwarded text carry no
authority. Each route has its own conversation context. Changing drafts clears
the selected builder context; a later build can explicitly attach that draft's
retained source.

`app.json` records admitted messages, prepared submission digests, last
conversation entries, route bindings and reply delivery state. It does not
own job status: `service get` and Tend remain authoritative. Repeating a lost
submission uses the same key and exact bytes. A conversation with an unknown
or waiting job stays paused. Commands to inspect or cancel remain available.
Prepared bytes live in `submissions/ID.json`; candidate attachments are not
duplicated inside the conversation record. A received API batch is spooled
before individual messages are admitted, and resumes locally after a restart.

Messages are retained before execution; candidates and deployment requests
are saved separately. The latter pins the candidate digest. Source packaging
rejects path escapes, runtime files, links and unsafe executable modes. The
generated check runs only in the job container, never with controller host
authority. A candidate team selects the ordinary root Agent entrypoint; its
specialists and handoffs remain part of its definition.

## Effects and readiness

`/deploy NAME` is the explicit deployment action. It admits one immutable
request to a separate Tend queue, serialized with other provisioning commands.
Install, smoke and topic receipts let an operator resume after inspecting an
interruption. An existing deployment name never silently changes candidate.
Credentials come from operator-selected environment variables or private files,
not model output. Matterbridge receives only its two tokens, while sandbox jobs
receive only the selected model profile. The provisioning controller receives
the separately selected credentials it needs to configure a new service and
create a topic. Token values are absent from generated TOML and argv.

There is deliberately no automatic retry of uncertain topic creation or
message delivery. A marker is durable before a network write. Telegram cannot
deduplicate topic creation, and Matterbridge cannot acknowledge downstream
delivery. Operators reconcile unknown effects explicitly.

A deployment becomes ready in this application only after Tend reports a
successful provisioner, its smoke job succeeded, its route is written, its
service worker is running, and Matterbridge answers its health endpoint after
the new configuration is loaded. This means installed and connected for use;
it does not prove every specialty's semantic quality or every message's
network delivery. Live acceptance remains necessary on the chosen host.

The Matterbridge buffer before local admission is not durable. This known
boundary is not hidden by the durable worker queue. A deployment topic also
shares group visibility; topic separation does not imply confidentiality.
