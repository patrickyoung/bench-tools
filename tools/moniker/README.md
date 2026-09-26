# Moniker

Reserve a friendly team name such as **Cosmic Otter Crew**. Moniker is a small
Go command with no model, network, configuration discovery or background process.
The caller explicitly selects a private directory that remembers names.

```sh
go build -o moniker .
./moniker -dir /absolute/private/team-names
./moniker -dir /absolute/private/team-names -theme space -json
./moniker version
```

Plain stdout is one name followed by a newline. `-json` writes one object:

```json
{"id":"ad1b364034fde9585902547186017453","name":"Cosmic Otter Crew","slug":"cosmic-otter-crew","theme":"space"}
```

Themes are `playful` (default), `space`, and `nature`. Names combine a friendly
adjective, animal and group. Each result has a random 128-bit lowercase hex ID.
The slug contains only lowercase ASCII letters, digits and single hyphens.

`-dir` must be absolute. Moniker creates a missing registry with permissions
0700 and refuses an existing non-private directory or final symlink. All
reservations are private 0600 JSON files. It reserves the full name atomically
with exclusive file creation; concurrent processes and later invocations cannot
reserve that spelling again in the same registry. After 32 word-combination
collisions, it tries up to 16 names with a random eight-hex-digit suffix.
The uniqueness scope is that registry, not other machines or directories.

Reservations are permanent until the operator changes the registry. Do not
delete or rename records while relying on uniqueness. A failed write, lost
response or interrupted caller can leave a name occupied. Moniker never removes
an uncertain reservation or claims that retrying will return the same result.
The local filesystem must support exclusive creation and file/directory sync.

Exit status is 0 for a reservation, 1 for a runtime failure, and 2 for an invalid
invocation. Diagnostics use stderr; no progress text is mixed with stdout.

## MCP through the existing server

The static [manifest](mcp/manifest.json) and ordinary dispatcher compose with
Bench's separate `mcpserve`. Moniker contains no MCP server or protocol runtime.

```sh
mcpserve /absolute/moniker/mcp/manifest.json -- \
  /absolute/bin/moniker mcp -dir /absolute/private/team-names
```

For a direct dispatcher call:

```sh
printf '%s\n' '{"name":"generate_team_name","arguments":{"theme":"nature"}}' |
  ./moniker mcp -dir /absolute/private/team-names tools/call
```

The one tool, `generate_team_name`, accepts an optional `theme`. Its result
contains a text name and the same four-field object in `structuredContent`.
Omitted `arguments` means an empty object; explicit null is invalid. Input is
bounded to 4096 UTF-8 bytes. Unknown or duplicate fields, invalid types, trailing
data and arbitrary path arguments are rejected before making a reservation.
The operator's `-dir` cannot be changed by tool arguments.
An optional protocol `_meta` object is accepted within that bound and ignored;
it cannot select paths or change generation options.

Tool input and reservation errors return MCP `isError: true` with process exit
0. Unsupported dispatcher methods return a JSON error with code -32601 and
exit 1. Dispatcher invocation errors remain exit 2. The manifest marks this as
a local, non-idempotent write, not a read-only query. Server annotations grant
no permissions; the caller still owns access to the registry and server.
The manifest omits an output schema so both successful structured results and
ordinary tool-error results work with the Bench MCP server.

## Build and check independently

The component builds directly from this directory or an exported copy. Its
module path is `github.com/patrickyoung/moniker`; that identifier does not imply
a separately published repository. Only Go 1.26 and the standard library are
needed.

```sh
GOWORK=off go test ./...
GOWORK=off go test -race ./...
GOWORK=off go vet ./...
GOWORK=off go build .
./install.sh -prefix /absolute/install/prefix
```

The optional installer runs tests, then installs the command, manual and MCP
manifest under the selected prefix. It does not start a server or reserve a
name. No test needs credentials, a provider or the user's registry.
