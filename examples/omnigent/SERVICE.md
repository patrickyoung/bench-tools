# Clients, sessions and durable jobs

The optional service uses ordinary JSON commands for requests, **Tend for the
durable queue, and Docker for each execution**. SSH authentication and MCP are
optional adapters around those commands. Durable submission is the recommended
path for both local and remote jobs. It adds no HTTP server, provider client or
queue implementation. The original `./run` and `./chat` remain available as
synchronous local utilities.

One deployment serves one pinned worker or team. Many clients can submit many
jobs to that deployment, locally or over SSH. More deployments can offer other
workers/teams without a shared runtime or mandatory central service.

```text
Omnigent or another MCP client
  → SSH key / forced client identity
  → service-mcp → service submit (one JSON request)
  → Tend's durable queue on the execution machine
  → independently supervised service-worker
  → sandbox-job → Docker → existing Agent or team entry command
```

The [program boundary design](DESIGN.md) describes the executable and file
contracts. Tend launches the sandbox runner; callers normally use `service`.

## Prepare the execution machine

Package and install as described in [README.md](README.md). New installations
build both MCP and Tend on the host. Then, in the installed deployment:

```sh
BENCH_STATE=/srv/bench-state/product-owner
./service --state "$BENCH_STATE" init
./service --state "$BENCH_STATE" add-client acme < /private/location/acme-provider.json
./service-worker --state "$BENCH_STATE"
```

`acme-provider.json` is an operator-provided JSON object, for example:

```json
{"ASK_MODEL":"openai/YOUR_MODEL","OPENAI_API_KEY":"YOUR_CLIENT_SPECIFIC_KEY"}
```

Use the client's actual provider settings, delivered through your credential
manager or a protected file. Values are stored mode 0600 under
`$BENCH_STATE/clients/acme.json`, outside all container mounts. `add-client`
also rotates that client's credentials; queued work uses the current values
at execution time. Only names already admitted by `installed.json` may be
selected. Process/SSH/Docker configuration cannot be supplied as client
credentials. Ambient provider credentials are excluded from service attempts.

The client key is a **service identity**; the provider key pays for model use.
They are separate. A client's own worker can see its selected provider values.
Other clients' values and the host home are never mounted into that job.

The explicitly selected state directory must remain mode 0700, owned by the service account, on a local
filesystem. Create its parent directory first; `init` creates a new state directory. The
state records the deployment path and package manifest digest, so a different
package cannot silently inherit its jobs. Keep both paths fixed after admission.
There is no implicit state location or automatic migration of earlier template
queues: retain their original bundle/state until their work is reconciled. Do not put
Tend state on NFS or share it across hosts. Back up the whole service state
with workers stopped; do not copy only the SQLite database while it is active.

## Connect locally

The command interface works without MCP or Omnigent. Save the submission
arguments shown below as `request.json`, then use ordinary files and streams:

```sh
./service --state "$BENCH_STATE" submit acme < request.json > submitted.json
printf '%s\n' '{"job":"JOB_ID"}' |
  ./service --state "$BENCH_STATE" get acme
printf '%s\n' '{"session":"planning-september"}' |
  ./service --state "$BENCH_STATE" list acme
```

Substitute the returned `id` from `submitted.json` for `JOB_ID`.

Each request operation reads one JSON object and writes one JSON result:

| Command after `--state DIR` | JSON input | Result |
| --- | --- | --- |
| `submit CLIENT` | `session`, `key`, `request`, optional `files` | Durable job handle and current status |
| `get CLIENT` | `job` | Status and bounded output |
| `list CLIENT` | Optional `session`, `offset`, `limit` | This client's jobs |
| `cancel CLIENT` | `job` | Cancellation request/current status |
| `read-file CLIENT` | `job`, `path`, optional `offset` | Bounded base64 file bytes |

Exit **0** means the operation succeeded. For `get`, that can include a job
whose status is `failed` or `unknown`; inspect the returned status separately.
Exit **1** rejects an invalid or unauthorized request. Exit **2** reports a
configuration/controller failure. Errors leave stdout empty and put diagnostics
on stderr. `work` and `check` retain Tend's own stream/exit contracts.

Client identity and the state directory are trusted command arguments. JSON
cannot override either. As with direct Tend use, local command access under
the service account grants operator authority; it is not remote authentication.

`service-mcp` only maps MCP tools to those executable commands and wraps their
JSON results. It contains no queue, Docker or recovery policy. Its declarative
tool manifest is packaged source, outside writable service state.

As the trusted operator, open a client-specific Omnigent chat:

```sh
./service-chat --state "$BENCH_STATE" acme
```

This creates separate Omnigent specifications and state for that client under
`$BENCH_STATE/chat/` and `$BENCH_STATE/omnigent/`. Omnigent's own model credentials still
need configuring. The service's provider file configures the Bench job, not
Omnigent's chat model.

Other local MCP callers can launch `./service-mcp --state "$BENCH_STATE" acme` as their stdio
server. Local shell access to the service account is **operator access**, not
client isolation. Give remote clients only the restricted SSH connection below.
Do not expose an endpoint that lets untrusted callers select the client argument.

## Connect remote clients with SSH

Use a dedicated, operator-managed account with access to the local Docker
daemon. An administrator installs **one distinct SSH public key per client**,
with a fixed forced command in that account's `authorized_keys`:

```text
restrict,command="/srv/product-owner/service-mcp --state /srv/bench-state/product-owner acme" ssh-ed25519 CLIENT_PUBLIC_KEY acme
```

Use the real absolute deployment path and public key. Never interpolate
`SSH_ORIGINAL_COMMAND` into the forced command. The client cannot change the
fixed `acme` argument by asking SSH to run another command. `restrict` disables
PTY allocation, forwarding and user startup scripts. For this account, disable
password/interactive authentication and user-controlled environment files;
do not permit client environment overrides of PATH, HOME, Python, Docker or
Tend settings. Use a reviewed absolute path without shell metacharacters in
the forced-command entry. Manage the account and its keys outside the sandbox.

On each caller machine, make an SSH host alias (for example `bench-acme`) that
selects its private key, known host key and service account. Verify the host key
normally; do not disable checking. In that caller's Omnigent worker directory,
use this MCP definition as `tools/mcp/bench.yaml`:

```yaml
name: bench
transport: stdio
command: ssh
args: ["-T", "-o", "BatchMode=yes", "bench-acme"]
timeout: 30
retry:
  max_retries: 0
  timeout_per_request_s: 30
```

SSH carries MCP over encrypted stdio. There is no public application port and
no bearer credential in job input or command arguments. Omnigent's prompt
should use `submit_job`, preserve the returned job ID, check `get_job`, and
require operator attention for `unknown`. A caller can disconnect immediately
after submission; worker supervision has a separate lifetime.

## Sessions and request identity

| Field | Meaning |
| --- | --- |
| Client | Fixed by the local operator or the authenticated SSH forced command |
| Session | Client-chosen conversation/group identifier; scoped to that client |
| Key | Client-wide idempotency key for one intentional request |
| Job | Server-derived durable handle for that client and key |
| Attempt | A particular Tend invocation; changes only on explicit continuation/retry |

The tools are:

- `submit_job(session, key, request, files={})`: persist and return a handle.
- `get_job(job)`: status, recent event names and bounded stdout/stderr tails.
- `list_jobs(session?, offset=0, limit=50)`: recover this client's handles.
- `cancel_job(job)`: request cancellation; inspect its eventual outcome.
- `read_file(job, path, offset=0)`: up to 1 MiB as base64, under `work/` or `team/`.

For example, the arguments to a submission can be:

```json
{
  "session": "planning-september",
  "key": "request-001",
  "request": "Review the attached requirements and produce the worker's deliverable.",
  "files": {"requirements.txt": "U2VsZWN0ZWQgcmVxdWlyZW1lbnRzCg=="}
}
```

After a lost response, repeat **the same key with identical inputs**, or use
`list_jobs` to recover its handle. Tend deduplicates the same submission even
after restart. Changing its session, request or files under that key is an
error. A new key always means new work; deduplication is not exactly-once
external effects. Identifiers are 1–64 letters, digits, underscores or hyphens.

Different sessions can execute concurrently. Active attempts with the same
client/session serialize through Tend's key. An unknown attempt blocks that
session until resolved. A manual waiting job is not an active attempt and
does not block later jobs; resuming it joins the queue again. Retry does not
guarantee it precedes other already-ready jobs.

Sessions provide ownership, grouping and execution ordering. They do **not**
automatically merge model transcripts, workspaces or all earlier outputs.
Omnigent owns its conversation; each job gets only its request and selected
files. To use a prior result in a later request, read that result and explicitly
attach the needed bytes. Multiple requests can therefore continue a client
conversation without granting every job all of that client's data.

Uploads contain at most 64 regular files and 8 MiB of decoded data; the goal
is at most 1 MiB. Paths must be relative and nonoverlapping. `request.txt` is
reserved. The synchronous deployment's shared `inputs/` is never consulted.
Result reads refuse symlinks (including parent directories), hard links and
special files. Files from a running job can change between paged reads.

## Supervise the workers

`./service --state "$BENCH_STATE" work` performs at most one Tend transition and exits. Its statuses
are Tend's: 0 performed a transition, 1 idle/no free slot, 2 controller error,
130 interrupted. The submitted command's success is in the **job record**;
worker-loop exit 0 does not mean that job succeeded.

`service-worker` is the small shell loop around that operation. Run it under
systemd, launchd, your existing process supervisor, or a persistent terminal.
For Linux, a template unit can look like this (replace the account and paths):

```ini
[Unit]
Description=Bench deployment worker %i
After=docker.service

[Service]
User=bench-service
WorkingDirectory=/srv/product-owner
ExecStart=/srv/product-owner/service-worker --state /srv/bench-state/product-owner
Restart=on-failure
RestartSec=3
KillMode=control-group
TimeoutStopSec=20
Environment=PATH=/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=multi-user.target
```

Save as `bench-worker@.service` in your administrator-managed unit directory
and start two instances if you want two concurrent attempts. On macOS, launchd
can supervise two invocations of the same absolute `service-worker` path and explicit `--state` argument, with
the PATH and local Docker context configured for that service account. On
either platform, the operator chooses worker lifetime; no service is silently
installed or enabled by this package.

`$BENCH_STATE/config.json` defaults to two worker slots, 32 unfinished jobs
per client, 1000 retained jobs per client, and a two-hour attempt timeout.
Change these operator-owned settings between worker starts. Admission is
serialized so concurrent requests cannot exceed job-count limits. Slot locks
bound concurrent Tend workers even if too many loops are started. Docker also
enforces the deployment's per-job memory, CPU and process limits.

These are admission/execution limits, **not a filesystem quota**. Set OS/volume
quotas and retention for a larger installation. Client requests cannot delete
retained evidence. An orphan container can outlive a killed worker slot; stop
and reconcile unknown work before relying on the normal concurrency limit.
This starter does not provide cross-host routing, high availability or automatic
orphan reconciliation. The existing A2A tool remains available for its own
network contract; it is not required for this SSH/MCP deployment.

## Inspect and recover

Tend alone owns transactional execution state under `$BENCH_STATE/tend/`.
Its input file contains the immutable client, session, key, request and selected
file bytes. There is no second queue database. Sandbox files live separately
under `$BENCH_STATE/runs/JOB/`; the container never receives Tend's directory,
client profiles, SSH files, or other jobs. `launch.json` records the selected
sandbox and container, not a competing execution status.

```sh
./service --state "$BENCH_STATE" check
TEND_ROOT="$BENCH_STATE/tend" ./prefix/bin/tend show JOB_ID
TEND_ROOT="$BENCH_STATE/tend" ./prefix/bin/tend events JOB_ID
```

Completed work is `done`. Known nonzero outcomes are `failed`. Exit 75 is a
durable **manual wait**, not an automatic retry. Interruption, exit 125 or a
missing trusted outcome is `unknown`. A canceled queued job is `cancelled`;
canceling started work can be `unknown`, since effects may already exist.

After inspecting the retained outputs and any external effects, the operator
can explicitly choose one of these:

```sh
./service --state "$BENCH_STATE" recover acme JOB_ID retry
./service --state "$BENCH_STATE" recover acme JOB_ID fail
```

Both first invoke `sandbox-job stop` to inspect the deterministic Docker container name and verify ownership
labels if it exists, then forcibly remove it. Container identity includes the
selected state directory: separate queues using the same deployment and job
key cannot stop each other's containers. If Docker is unavailable or the
container cannot be confirmed gone, recovery fails and preserves the fence.
Tend additionally refuses unknown resolution while its recorded launcher is
still alive. Do not bypass the adapter with direct `tend resolve`/`retry` while
a daemon-owned container might remain.

`retry` explicitly accepts the possibility of duplicate external effects. For
a worker, it keeps that job's workspace and Agent/Ply checkpoint. For a team,
it starts its existing entry command in a **fresh attempt workspace**, retaining
the old one; this is a team restart, not a claim of generic team continuation.
`retry` on a manual wait explicitly wakes it. `fail` closes an unknown job or
cancels a manual wait; a known failed job remains failed. There is deliberately
no remote recovery tool or unchecked “mark successful” operation.

## Verify this adapter

From the Bench source checkout, with separately built Tend, MCP and Agent
executables in `.build/bin`:

```sh
python3 -m unittest discover -s scripts/tests -p 'test_omnigent*.py' -v
```

Set `BENCH_OMNIGENT_SSHD` to the absolute `sshd` executable to include the
loopback authentication test. It creates temporary host/client keys, verifies
the host key, tests forced client identity and rejects an untrusted key. It
does not change system SSH configuration and removes its test server and keys.

The service tests use real Tend, real MCP clients/server and an actual Agent
precheck/Record path. Docker is a test double for queue, isolation, credentials,
concurrency and recovery cases; those tests do not prove kernel confinement or
your production SSH server policy. Installation's Docker probe and an operator-managed SSH
connection verify those separate boundaries. No paid model calls are made.
