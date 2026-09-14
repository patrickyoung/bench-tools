# Evidence as a Unix stream

The Context envelope is a portable data contract. An executable connector,
an HTTP client, or an MCP adapter can produce it. Context is its reference
normalizer and checker; using the format does not require an MCP server.

The implementation remains four steps: find the named connector, give it the
query, validate and stamp its records, print them.

## Version 1

Each nonblank line is one UTF-8 JSON object. The required fields and connector
operations are defined in [CONNECTORS.md](CONNECTORS.md). Unknown fields and
non-null JSON content survive normalization, including tables and graph data.

There are two boundaries:

| Boundary | `ref` | `retrieval` |
| --- | --- | --- |
| Connector output | Optional; must match if present | Forbidden |
| Normalized evidence | Required | Optional; required for Context query output |

Context computes `ref` as `ctx:` + source + `:` + the lowercase hexadecimal
encoding of the first 16 bytes of SHA-256(source + NUL + id), using UTF-8.
This identifies an item within a connector's namespace. It is neither a
content digest nor proof of authority. Use identities scoped to the source
instance so two workspaces cannot accidentally identify different items alike.

`context query` records the exact input query and the selected executable's
SHA-256. Provider-generated SQL, parameters, document revisions, and upstream
request identifiers are different facts; adapters retain them in additional
fields rather than replacing the core-owned `retrieval` stamp.

`context merge` adds missing refs, preserves order, removes identical records,
and rejects differing records with the same ref. `context check` requires refs,
checks the same envelope and conflicts, and prints nothing on success. It
checks consistency, not source permissions, truth, freshness, or sufficiency.

Queries are bounded at 1 MiB, records at 8 MiB, and streams at 32 MiB. The core
validates a whole requested result before printing. It refuses an exceeded
bound instead of truncating. Exit 0 means results or a clean check, 1 means
no connector/result/input, and 2 means usage, invalid data, or failure.

## Compose capabilities as programs

| Responsibility | Owner |
| --- | --- |
| Name sources and decide which are required | Calling script or procedure |
| Authenticate, retrieve, preserve provider semantics | Connector executable |
| Validate and stamp evidence | `context` |
| Require task-specific evidence properties | A separate checker executable |
| Call or serve MCP | `mcp`, `mcp-legacy`, or `mcpserve` at the edge |
| Write an answer | Consumer, such as `ask` |
| Check citation identities | `cite` |
| Retain model inputs and verifier history | Ask and Ply |
| Export telemetry | A separately selected adapter |

A task checker can be an identity filter: accept the whole stream and print
the exact input bytes, or print nothing and explain rejection on stderr.
The caller must check every exit status. `context check` itself is a silent
validator, so it is not a pass-through stage.

The [runnable example](examples/unix-evidence/README.md) composes two named
connectors, a local MCP service, Context, a task checker, and Cite without a
model or network account. All providers and business facts in it are fixtures.

## An MCP binding

For a tool under your control, place normalized records in
`structuredContent.records`. Include the serialized structured object in a
text content block for consumers that only read `content`. Mark this binding
with `_meta["io.github.patrickyoung/context"] = {"version": 1}`. This is a
project-specific convention, not an official MCP extension. The marker is a
format declaration, not a trust or authorization signal.

The [example `as-mcp` filter](examples/unix-evidence/as-mcp) validates Context
JSONL and writes one MCP tool result. `mcpserve` can carry that result; the
filter itself does no protocol framing or connection management. Its text
and structured representations must describe the same records. Account for
the size of both representations when setting MCP output bounds.

When adapting an existing vendor tool, preserve its declared output schema.
A named connector maps that provider's result to Context records. Do not
silently replace an upstream `structuredContent` object whose schema is
already part of the tool contract. Handle tool errors, unfinished results,
and pagination explicitly before accepting evidence. A client must actually
forward the evidence and relevant limitations to its consumer; metadata alone
does not guarantee that a model sees them.

This binding uses the existing MCP
[structured-result fields](https://modelcontextprotocol.io/specification/2026-07-28/server/tools)
and [metadata namespace](https://modelcontextprotocol.io/specification/2026-07-28/basic/index#meta).
MCP protocol versions and Context record versions are independent.

## Additional evidence claims

Version 1 permits provider fields without giving them shared semantics. An
adapter may retain source revision, original locator, executed query, result
counts, and continuation data. A checker only interprets extensions whose
meaning it knows. The example's `origin`, `coverage`, and `execution` fields
are illustrative, not new mandatory Context fields.

An authority claim needs an issuer, subject, and basis. A completeness claim
needs a scope: all rows for this executed query is different from all relevant
knowledge for an answer. Represent unknown information explicitly. A gateway
can enforce a supplied policy against supported claims; a required field by
itself does not substantiate a claim. Provider data remains untrusted even
when the field is present.

Telemetry can map known facts to OTel attributes such as
`gen_ai.data_source.id` and `gen_ai.retrieval.query.text`. A connector name is
not automatically a data-source id, and the caller's natural-language question
is not automatically the query executed upstream. Exporters should document
their mapping and selected convention version. Trace identifiers can link to
evidence held by its owner; a trace is not the evidence archive.

## Changes that need a new contract

An empty retrieval currently emits no records and exits 1. Recording an
attempt with zero results needs a separately selected receipt output or an
explicitly versioned result format. Do not insert a status record into the
version 1 evidence stream or turn it into a successful evidence result.

Likewise, repeated observations of one item can conflict because query,
retrieval time, or content differs. Keep those observations separate today;
do not silently choose the newest record. A future observation format should
distinguish source-item identity, source revision, and retrieval occurrence,
with corresponding changes to citation consumers and replay checks.

Neither addition requires a daemon, registry, router, or shared runtime. Prove
the need in calling pipelines before changing the core record contract.
