# Moniker development

Read README.md and DESIGN.md before changing the command. Keep this component
independently buildable with `GOWORK=off`, standard-library-only and versioned.

- Preserve the explicit absolute registry. Never discover a home, hidden state,
  shared service, provider or model.
- Reserve a full name with atomic exclusive creation. Never replace, recycle
  or remove a reservation after an uncertain outcome.
- Keep the registry private, refuse a symlinked registry root, and anchor writes
  under the selected directory. User text cannot supply a filename.
- Preserve stdout/stderr and exit contracts. MCP tool errors are result objects;
  unknown methods are ordinary dispatcher failures.
- Keep the adapter a bounded stdin/stdout program behind the separate mcpserve.
  Do not import another tool or implement a server/client runtime.
- Tests use temporary registries, never personal state, network or model calls.
  Include concurrent processes and strict input cases when changing contracts.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...` and a standalone build.
  Adapter changes also need public mcpserve/mcp integration.
