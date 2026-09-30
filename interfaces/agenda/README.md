# Calendar and Kanban for Agenda

Calendar answers what is due and when. Kanban shows planned, ready, active,
attention and completed work. Both consume the same public Agenda projection;
neither keeps another task database or decides whether a worker succeeded.
Human-reported completion stays visibly separate from external acceptance.

This independent Go application uses only the standard library. Agenda is a
separately selected executable. MCP framing belongs to the existing MCPserve
command; there is no model client, scheduler or approval mechanism here.

## Build and render offline

Requires Go 1.26 or later. From this directory:

```sh
make build
bin/agenda-ui version
bin/agenda-ui render --view calendar < /private/projection.json > /private/calendar.html
bin/agenda-ui render --view kanban < /private/projection.json > /private/kanban.html
bin/agenda-ui render --view ics < /private/projection.json > /private/calendar.ics
```

Input is one `agenda.projection/v1` JSON object from the public `agenda project`
command. No Agenda installation or live data directory is needed to render a
saved projection. Output is standalone HTML, with responsive layouts, filtering
and a calendar month selector. Source text is escaped. Saved pages are labelled
as snapshots; opening them never reads local files or calls another program.

Calendar places work on its due date in the obligation's timezone. Kanban
columns are a presentation of projected facts. Cancelled/skipped work stays in
history; moving a card is not an execution command or an approval channel.

## Live, read-only views

```sh
bin/agenda-ui serve --agenda /absolute/bin/agenda --root /private/work \
  --observations /private/current-observations.jsonl --port 8791
```

Open either independent view:

- Calendar: `http://127.0.0.1:8791/calendar`
- Kanban: `http://127.0.0.1:8791/kanban`
- Calendar snapshot download: `http://127.0.0.1:8791/calendar.ics`

The server listens only on loopback and accepts read-only same-origin requests.
It calls `agenda export ROOT`, then `agenda project SNAPSHOT --as-of TIME`, using
a private temporary snapshot and the optional selected observation file on
stdin. It never opens Agenda's private files. Omit `--observations` to supply
an empty observation stream; external work then remains unconfirmed.

Both views refresh every five seconds (`--poll-seconds 1..60`). The browser
retains its filter and selected month. A failed refresh leaves the last view
visible and labels the connection unavailable. A current connection does not
establish current external evidence, a healthy monitor, completed work or a
sent reminder. Input observation validity remains Agenda's projection policy.
No scheduler, remote hosting, notification channel or auth service is installed.

Milestones and human activities appear when an adapter includes them as Agenda
items with their own dates and identity. This interface does not inspect Page
Team/Manage assignments, May waits, or old prototype records. Those require
explicit adapters into the shared projection. A board therefore shows the work
present in Agenda, not an inferred complete inventory of every team controller.
ICS is a portable timestamped snapshot, not an external calendar-account sync.

## AI control through MCP

The controller selects one executable, one Agenda root and optional observation
and evidence roots. Tool arguments cannot replace these selections. CLI and MCP
invoke the same public Agenda operations; the adapter does not interpret private
work history or implement another state machine.

Create a read-only manifest and configure MCPserve:

```sh
bin/agenda-ui mcp-manifest > /private/agenda-read.json
mcpserve /private/agenda-read.json -- /absolute/bin/agenda-ui mcp-dispatch \
  --agenda /absolute/bin/agenda --root /private/work \
  --observations /private/current-observations.jsonl
```

For a host that requires the earlier handshake, explicitly select
`mcpserve -allow-legacy`. The dispatcher receives `tools/call` as the final
argument. There is no MCP SDK dependency in the interface itself.

| Tool | Input | Public command |
| --- | --- | --- |
| `agenda_export` | `{}` | `agenda export ROOT` |
| `agenda_project` | `{"as_of":"2026-09-29T15:00:00Z"}` | `agenda project SNAPSHOT --as-of TIME` |
| `agenda_expand` | `{"as_of":"...","schedule":{...}}` | `agenda expand --as-of TIME`, schedule on stdin |
| `agenda_apply` | `{"change":{...}}` | `agenda apply ROOT`, change on stdin; disabled by default |

Expand accepts an inline Agenda schedule, never a filename. Project uses only
the controller-selected observations. Structured JSON and text results contain
the same result; no tool executes work, sends messages or approves May requests.
The standard Agenda export includes retained evidence and is bounded by Agenda's
snapshot limit; select CLI export when a large archival snapshot is inappropriate
for a model context.

To enable typed changes, select `--allow-write` **both** when generating the
manifest and when starting the dispatcher. Agenda still enforces request IDs,
expected revisions, typed transitions and evidence requirements. An MCP
annotation does not authorize a change; the host/controller does.

Proof-bearing human reports may read only regular files beneath a separately
selected `--files-root`. Tool evidence paths are relative; absolute paths,
`..`, backslashes and symlink components are refused. The adapter verifies each
SHA-256 and retains an immutable content-addressed staging copy beneath a
separate private `--evidence-root`, then passes those selected bytes to Agenda.
Neither tool arguments nor file contents can expand the allowed roots.

```sh
mkdir -m 700 /private/agenda-proof
bin/agenda-ui mcp-manifest --allow-write > /private/agenda-write.json
mcpserve /private/agenda-write.json -- /absolute/bin/agenda-ui mcp-dispatch \
  --agenda /absolute/bin/agenda --root /private/work --allow-write \
  --files-root /private/selected-inputs --evidence-root /private/agenda-proof
```

Use the normal `agenda.change/v1` envelope. An evidence entry looks like
`{"path":"reviews/result.txt","sha256":"64 lowercase hex digits"}`. Evidence
is limited to 16 files and 64 MiB total per change. Keep the same roots, input
bytes, request ID and expected revision when retrying an interrupted request;
stable proof paths preserve Agenda's idempotency. Conflicting content is refused.
The original selected input must still be available for adapter validation.
Staging files are retained for recovery; there is no automatic garbage
collection or total-storage quota. Operators own this private directory.

## Stream and failure contract

HTML, manifests and MCP results go to stdout. Listener URLs and diagnostics go
to stderr. The child-command boundary retains Agenda's process status, including
unknown (125); HTTP projection failures produce an unavailable (503) response. The MCP dispatcher follows MCPserve's result contract: completed
tool failures produce `isError:true` with the exact `agenda_exit` in structured
content, while the dispatcher exits zero to deliver that result. The MCP client
then reports its application-negative exit 1. Unknown outcomes remain explicitly
marked 125 inside the result and must never be retried automatically.
Pre-dispatch validation fails with exit 2. Inputs,
proof files, process streams and process duration are bounded. HTTP failures
return an unavailable view without claiming success.

## Verification

```sh
make check
AGENDA_TEST_BIN=/absolute/bin/agenda \
MCP_TEST_BIN=/absolute/bin/mcp MCPSERVE_TEST_BIN=/absolute/bin/mcpserve \
  make check
```

The first command covers independent rendering, escaping, literal command
composition, controller scoping, HTTP read-only/origin boundaries, proof escapes
and public exit preservation using deterministic fixtures. The second also
exercises real Agenda apply/export/project, proof-bearing idempotent completion,
both views and a real MCPserve/client round trip. No paid models or personal
credentials are used. See [design boundaries](DESIGN.md).
