# Context

Context retrieves cited external evidence through small, executable source
connectors. It gives every source the same interface without forcing every
source into the same language or SDK.

```sh
$ context ls
{"description":"Company handbook search","kind":"source","name":"handbook","version":1}

$ context query handbook 'How much paid leave do we get?'
{"citation":{"locator":"handbook.md#paid-leave"},"content":{"text":"..."},"id":"paid-leave","kind":"context","ref":"ctx:handbook:...","retrieved_at":"2026-08-28T16:00:00Z","source":"handbook","title":"Paid leave","type":"document","version":1}
```

A connector is one executable on `CONTEXT_PATH`. It implements two operations:
`describe` prints one catalogue entry and `query` reads the query on standard
input and prints context records as JSONL. The connector can be a Python
program using LlamaIndex, a Go program calling Glean, a Java client for an
internal service, or a shell script over local files. Context does not know or
care.

## Install

Context requires Go 1.26 or later and a Unix.

```sh
go install github.com/patrickyoung/context@latest
```

From this source tree:

```sh
go build .
go test ./...
```

Put connectors in `.context/connectors` for one project or
`~/.context/connectors` for your user. `CONTEXT_PATH` replaces those defaults
with a colon-separated search path. The first executable with a given name
wins, exactly as on `PATH`.

## Four verbs

```text
context ls
context query source [query]
context merge
context check
```

`ls` lists the sources an agent may choose from. `query` sends one exact query
to one explicitly named source. If the query argument is omitted, it is read
from standard input. `merge` validates, adds any missing citation references,
keeps first-seen order, removes identical duplicates, and rejects two different
records that claim the same reference. `check` validates already-normalized
records without printing them.

Multiple sources remain ordinary composition:

```sh
context query glean "$q" > glean.jsonl
context query genie "$q" > genie.jsonl
cat glean.jsonl genie.jsonl | context merge > evidence.jsonl
```

The caller chooses whether one source is enough, whether both are required,
and what to do when one has no result. Context does not hide that policy in a
router.

## The record

Every result has a common envelope:

```json
{
  "kind": "context",
  "version": 1,
  "source": "genie",
  "type": "table",
  "id": "conversation/abc/result/1",
  "title": "Revenue by quarter",
  "retrieved_at": "2026-08-28T16:00:00Z",
  "content": {
    "columns": ["quarter", "revenue"],
    "rows": [["Q1", 42]]
  },
  "citation": {
    "locator": "space/7/conversation/abc/result/1",
    "url": "https://example.test/genie/abc"
  },
  "ref": "ctx:genie:7a3b..."
}
```

`content` may be any non-null JSON value. A document can carry text and
headings; a table can carry columns and rows; an entity can carry fields. The
common envelope is for composition and provenance, not a lowest-common-
denominator chunk format. Unknown connector fields are preserved.

The connector owns `source`, `id`, `retrieved_at`, and `citation.locator`
because it is closest to the source. Context verifies them and derives `ref`
from `source` plus `id`. Brief can tell an agent when citations are required;
Ask can cite the refs in its answer; Ply can use `context check` or a domain
check before accepting that answer. If the context records are supplied to
Ask, Ask's event history and Trail retain the exact retrieved snapshot. Replay
therefore reads history; it does not silently refetch a source that may have
changed.

See [CONNECTORS.md](CONNECTORS.md) for the complete connector contract and
[GUIDE.md](GUIDE.md) for use with Brief, Ask, Ply, Agent, and Trail.
[DESIGN.md](DESIGN.md) explains why source selection and answer synthesis stay
outside this program.

## Unix contract

Standard output is records only. Connector progress and diagnostics go to
standard error. Exit status 0 means records or a clean check, 1 means no source,
no result, or no input, and 2 means bad usage, a broken connector, or invalid
data. Query output is buffered and fully validated before it is printed, so a
bad final record cannot leave a plausible partial answer in a pipeline.

## Scope

Context has no provider SDK, credentials, model, source-selection algorithm,
answer synthesis, cache, index, database, daemon, server, workflow engine, or
session format. Provider variability belongs in connectors. Procedure belongs
in Brief, reasoning and history in Ask, iteration and checks in Ply, and durable
goal coordination in Agent.

## License

MIT. See [LICENSE](LICENSE).
