# Agenda Calendar and Kanban

Two independent views consume one `agenda.projection/v1` contract. Saved
projections render offline from stdin. The optional loopback server obtains a
fresh snapshot with `agenda export ROOT`, writes it to a private temporary file,
and invokes `agenda project SNAPSHOT --as-of TIME` with explicitly selected
observation bytes on stdin. The snapshot and observations stay outside source.
No private Agenda, Tend, May or team files are inspected.

The Calendar presents due dates and the Kanban presents work state. A human
report is labelled separately from external acceptance. Both show missing work,
unknown outcomes and incomplete observation coverage. A successful refresh means
the display connected and projected current inputs; it is not a periodic monitor
checkpoint or proof that external observations are fresh.

The server is optional, read-only and loopback-only. It validates Host and Origin,
bounds inputs and child output, escapes text through `html/template`, and refreshes
the selected view every five seconds. No notification, remote authentication or
publishing service is installed.

MCPserve owns protocol framing. `agenda-ui mcp-dispatch` accepts a tools/call
request and calls the same selected Agenda executable used by the UI. Its root,
executable and optional observation file are controller-selected startup flags.
The manifest defaults to read-only tools. `--allow-write` explicitly enables
`agenda_apply`; changes require Agenda's request identity/revision semantics.
Filesystem/command selectors are never accepted in tool arguments. Optional
proof paths are relative to a fixed controller-selected files root; the adapter
refuses symlinks/escapes and verifies the digest before staging bytes beneath a
separate controller-selected evidence root. Stable content-addressed proof paths
preserve exact request identity on retries. Staging is private retained recovery
data, not a work database. An MCP annotation is a hint, never approval authority.

The application uses only the Go standard library. It has its own module,
version and checks; no sibling imports or shared runtime are required.
