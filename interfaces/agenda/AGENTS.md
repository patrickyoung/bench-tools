# Agenda interfaces

Calendar and Kanban are views of the same public Agenda projection. Keep them
independently usable; neither owns a work database, workflow engine or scheduler.

- Invoke the selected Agenda executable with literal argv and bounded streams.
- Never read Agenda's private storage or import another component's code.
- MCP uses the existing MCPserve dispatcher contract. Root, executable and
  observation paths are fixed by the controller, never tool arguments.
- Read-only is the default. Explicit `--allow-write` enables typed Agenda
  changes over MCP; it never enables arbitrary file reads or command execution.
- Human reports, external acceptance, observation coverage and UI connection
  freshness must remain distinguishable.
- Calendar/Kanban HTTP routes only inspect and render. They cannot approve,
  submit, cancel or retry an execution.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, a standalone build
  and the optional public Agenda executable integration before delivery.
