# Changes

## 0.1.0 — initial release

Weave reads a finite JSONL task graph and a current observation snapshot,
validates both, and prints tasks whose dependencies are accepted. The filter
does not execute work or authenticate evidence.

- Preserve record bytes, extension fields, input order and exact task hashes.
- Reject ambiguous JSON, invalid Unicode, cycles, stale observations and
  inconsistent task states before producing output.
- Bound streams, records, dependencies and nesting; handle CRLF and final
  unterminated records without losing task identity.
- Provide a standalone install, man page, quickstart and reproducible release
  archives for macOS and Linux on arm64 and amd64.
- Include experimental Tend/Ask/Ply business recipes with verified evidence,
  bounded concurrency, interruption handling and no automatic retry of unknown
  outcomes. A bounded research recipe compares parameter search with one agent
  and a small team using a fixed synthetic evaluator and a separate final test.

The synthetic studies validate mechanics and expose limitations. They do not
establish real business gains or an advantage for a large swarm. Only the
filter's documented CLI and stream contract are supported in this initial
release; Python example APIs and study formats may change.
