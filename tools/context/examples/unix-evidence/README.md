# Document and table evidence through Unix filters

This offline example answers a fictional customer export question using a
document and a typed table. It requires Python 3 and installed `context`,
`mcp`, `mcpserve`, and `cite` executables. The MCP tools must support their
documented stateless request and filter-server interface.

There are no accounts, credentials, model calls, or external network requests.
The document and table services are fixtures, not Glean or Databricks adapters.
The table's SQL is illustrative and is labeled as not executed by a database.

From this directory, run:

```sh
work=$(mktemp -d)
./run "$work/result"
cat "$work/result/answer.md"
```

The output directory must not already exist. Results stay in the caller's
directory. The run preserves source evidence, the exact query, merged evidence,
a checked answer, and an MCP tool-result representation.

## Each program has one responsibility

- `connectors/policy` adapts a fictional document into a Context record.
- `connectors/entitlements` invokes `mcp` against a local fixture service and
  adapts its table, query parameters, revision, and truncation information.
- `server/dispatch` implements that fixture as one ordinary JSON filter;
  `mcpserve` owns protocol framing and process lifetime.
- `context query` validates and adds refs and retrieval stamps. The stamp
  hashes the selected connector, not the MCP server or its dependencies.
- `check-evidence` requires both sources, the configured demo revision, and
  explicit untruncated scopes. On success it prints the exact input bytes;
  rejection is exit 1 with empty stdout, malformed input or failure is exit 2.
- `answer` is a deterministic consumer of the fixture's machine-readable
  policy and typed table. It demonstrates composition without an LLM.
- `cite` passes the answer unchanged only when its citation identities match.
- `as-mcp` packages checked evidence under `structuredContent.records`, with
  an equivalent text representation and the project-specific profile marker.
  It validates the envelope, not the example's task policy. Empty input exits
  1, malformed input exits 2, and neither emits a tool result.

The example's extra `origin`, `coverage`, and `execution` fields are illustrative
adapter data, not mandatory fields in the version 1 envelope. The task checker
interprets their specific fixture meanings. Passing it does not authenticate a
provider's claims or establish completeness beyond the declared scopes.

## The explicit composition

These are the substantive steps in `run`. After both retrievals succeed,
`context merge` can combine their streams. `context check` is silent on
success, so the separate task checker supplies the pass-through behavior.

```sh
export CONTEXT_PATH="$PWD/connectors"
printf '%s\n' 'Can demo-customer export its data?' > "$work/query.txt"
context query policy < "$work/query.txt" > "$work/policy.jsonl" &&
context query entitlements < "$work/query.txt" > "$work/entitlements.jsonl" &&
cat "$work/policy.jsonl" "$work/entitlements.jsonl" > "$work/sources.jsonl" &&
context merge < "$work/sources.jsonl" > "$work/merged.jsonl" &&
./check-evidence < "$work/merged.jsonl" > "$work/evidence.jsonl" &&
./answer < "$work/evidence.jsonl" > "$work/candidate.md" &&
cite "$work/evidence.jsonl" < "$work/candidate.md" > "$work/answer.md" &&
./as-mcp < "$work/evidence.jsonl" > "$work/tool-result.json"
```

The files make every exit status explicit. A shell pipeline should use the
shell's `pipefail` option when upstream failure must stop the job. Do not
consume an output after its producing command fails.

`run` also verifies that a truncated or unknown-completeness table passes
envelope validation but fails the task checker without output, a missing
source fails that checker, an empty lookup exits 1, invented citations fail,
and malformed trailing evidence produces no MCP result.

## Replacing the fixtures

A real document connector would map a reviewed provider search/read result;
a table connector would preserve column types, rows, executed SQL, parameters,
source revision when available, and any continuation or truncation information.
Preserve the provider's original identity and the caller's permission scope.
Missing facts remain unknown. Change the task checker to the actual policy.

Use a provider SDK, HTTP client, or an explicitly admitted `mcpbox` executable
inside that connector. Select the appropriate `mcp` or `mcp-legacy` client
explicitly; no Context code needs to know the transport. Credentials stay in
the provider's existing authentication mechanism. The example's fixed
customer recognizer is only a fixture, not a general query parser.

To use a model, replace `answer` with an explicit Ask invocation that supplies
both the question and the evidence, followed by Cite. Ask owns the evidence
history; use Ply when the check result must also be recorded. See the
[guide](../../GUIDE.md#brief-ask-ply-agent-and-trail).

The [envelope specification](../../ENVELOPE.md) describes the MCP binding and
the boundaries for future receipt and observation formats.
