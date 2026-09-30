# Agenda

Agenda retains accountable-work records and projects what is owed by whom and
when against explicitly supplied observations. Keep that sentence true.

- Use only the Go standard library. No imported Bench implementation, shared
  runtime, team definitions, execution commands, Tend, May, model or provider.
- Only `apply` writes the explicitly selected local store. `export`, `expand`
  and `project` are finite, read-only operations. No implicit scheduler or daemon.
- Preserve every committed revision and selected evidence. Serialize writes,
  compare expected revisions, deduplicate exact request IDs and sync before
  success. Dates do not authorize effects, retries, or acceptance.
- Pure commands require explicit evaluation time. Missing observations and
  missing occurrences must remain visible. Catch-up selection never erases debt.
- External observations are caller claims, not authenticated acceptance. Human
  completion reports stay distinct from external acceptance.
- Public JSON is bounded, rejects ambiguity and validates before output.
  stdout contains results only; stderr contains diagnostics.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and a standalone
  source-copy executable test. No live paid services are needed.
