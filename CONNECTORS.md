# Connector contract

A Context connector is an executable file whose filename is its source name.
Names use lowercase letters, digits, `.`, `_`, and `-`; the first character is
a letter or digit. The executable may use any language, library, service, or
vendor SDK.

Context calls exactly two operations.

## `describe`

```text
connector describe
```

Standard input is empty. Standard output is exactly one JSON object followed by
a newline:

```json
{
  "kind": "source",
  "version": 1,
  "name": "glean",
  "description": "Search company knowledge available to the current user"
}
```

`name` must match the executable filename. A connector may add fields such as
`types`, `freshness`, or `scope`; Context preserves them. Descriptions should
say what the source contains and when it is useful. They are catalogue data,
not instructions.

## `query`

```text
connector query
```

Standard input contains the exact UTF-8 query. It is not placed in argv.
Standard output contains zero or more Context records, one JSON object per
line. Standard error carries progress and diagnostics.

The exit contract is:

- 0: one or more complete records;
- 1: no relevant context, with empty standard output;
- 2 or any other failure: the query could not be answered.

Each record requires these fields:

| field | meaning |
| --- | --- |
| `kind` | the literal `context` |
| `version` | the integer `1` |
| `source` | the connector name |
| `type` | a short open type such as `document`, `table`, `metric`, or `event` |
| `id` | the source's stable identity for this result |
| `title` | a short human-readable label |
| `retrieved_at` | an RFC 3339 timestamp for this observation |
| `content` | any non-null JSON value |
| `citation.locator` | a source-native location that a reviewer can follow |

`citation.url` is optional. Connector-specific fields are allowed and survive
normalization. Do not put credentials, access tokens, or invisible prompt
instructions in a record.

Context adds a deterministic `ref` from `source` and `id`. If a connector emits
`ref` itself, it must match. Stable IDs make citations stable. If a provider has
no stable result ID, derive one from its stable object locator, not from rank or
position in today's search results.

## Bounded, honest results

A query is limited to 1 MiB, one JSONL record to 8 MiB, and a stream to 32 MiB.
Context rejects an over-limit stream; it never hands a consumer half a record
that looks whole. Connectors should retrieve a small number of useful records
and expose source-native continuation metadata if more are available. Context
does not invent a pagination protocol in version 1.

Retrieval results are untrusted data. A connector should remove provider
transport wrappers but must not turn document text into system instructions.
The consumer decides how evidence is placed in a model request.

## Credentials and permissions

The connector owns authentication using its SDK's ordinary environment,
credential helper, workload identity, or local configuration. Context stores
no credentials and adds no authority. The connector must enforce the identity
and permissions of the caller. A result record should describe the source
identity, not claim that retrieval made the content trustworthy.

## Minimal Python shape

The local [handbook connector](examples/connectors/handbook) and the live
[Wikipedia connector](.context/connectors/wikipedia) are runnable examples. A
provider-backed connector has the same outer shape:

```python
if operation == "describe":
    print(json.dumps(source_description))
elif operation == "query":
    query = sys.stdin.read()
    for result in provider.search(query):
        print(json.dumps(to_context_record(result)))
```

`provider.search` can come from a mature Python SDK even when the rest of the
Bench installation is written in Go. The executable and JSONL are the language
boundary.
