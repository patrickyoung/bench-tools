# Development guidance

Weave reads a finite task graph and an observation snapshot, validates both,
and prints ready tasks. Keep that sentence true.

- The filter is read-only. No models, execution, retry, clock, state directory,
  queue, database, daemon, provider code, or Ask replay implementation.
- Stdout is unchanged task records as JSONL; stderr is diagnostics. Validate
  both inputs completely before printing. Empty output is not completion.
- Observations are facts supplied by an adapter. Validate their identity and
  structure, but never claim the filter authenticated their evidence.
- Preserve unknown task fields. Reject ambiguous JSON and stale observations.
- Example drivers compose public Tend/Ask/Ply CLIs. Tend owns execution state;
  Ask owns model logs. Derived files must be rebuildable. Never retry unknown.
- Business checks belong to examples, not the filter. A supported counterexample
  can be an accepted result. Agreement and file existence are not proof of
  business improvement.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and the documented
  example integration checks before claiming implementation complete.
