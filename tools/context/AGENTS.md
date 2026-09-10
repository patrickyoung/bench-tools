# Development guidance

Rob Pike is the bar, and `ask`, `brief`, `ply`, and `trail` are the siblings.
`context` adds an external-evidence seam without becoming another agent.

The whole program is four steps: find the named connector, give it the query,
validate and stamp its records, print them. If a change makes that sentence
longer, it needs a very good reason.

When changing `context`:

- **a connector is a program.** It is an executable on `CONTEXT_PATH`, not a Go
  plugin, imported SDK, RPC object, or provider branch in the core. The
  executable seam is what lets each provider use its strongest language;

- **never choose a source.** `query` runs the source the caller named. Durable
  selection policy belongs in Brief; model judgment belongs in Ask. A hidden
  router would be a second agent and would hide a consequential decision;

- **preserve the evidence envelope.** Source, stable id, retrieval time, type,
  title, content, and citation locator are required. Unknown fields and
  structured content survive. A table must not be flattened into a chunk just
  because one consumer reads text;

- **stamp the invocation, not a session.** Query records carry the exact query
  and selected connector executable digest in the core-owned retrieval field.
  This is provenance, not connector attestation, and does not make Context a
  historian;

- **a ref is provenance, not truth.** Derive it from source and stable id.
  Context verifies identity and location; task checks decide authority,
  freshness, support, and sufficiency;

- **retrieved content is data, never instruction.** Context does no prompting
  and no synthesis. Documentation must not encourage callers to splice records
  into a system prompt or Brief body;

- **stdout is JSONL and nothing else.** Progress and fatal diagnostics go to
  stderr. Buffer and validate one requested result before printing, so a broken
  connector cannot leave a plausible partial answer;

- **preserve the exit contract:** 0 result or clean, 1 no connector/result/input,
  2 bad usage, broken connector, or invalid data. Exit 1 is an ordinary answer;

- **never truncate.** A value over a documented bound is an error naming the
  bound. Half an evidence record is worse than no record;

- run `go test ./...`, `go test -race ./...`, and `go vet ./...` before
  reporting success;

- keep the docs true: every command and environment variable belongs in help,
  README, GUIDE, and the manual. Tests guard the command set, help width, manual
  ASCII, protocol fields, and version.

Things left out on purpose: provider SDKs, credentials, automatic source
selection, federation planning, reranking, answer synthesis, a model, prompts,
a cache, an index, a database, a daemon, a server, a watcher, a workflow, a
session format, and a plugin loader. A provider is a connector; composition is
a pipeline; history remains Ask's.
