# Headless expert authoring

Hire builds filesystem experts. Agent runs them. Keep these separate commands
in independent modules within this monorepo; the directory is their interface.

- Reuse Agent for intelligence, Brief procedures, the goal loop, Cage and Ask
  sessions. Do not recreate Hire's former web router, review team, session
  store, model client, scheduler, or worker registry in this command.
- Model-backed building is an ordinary Agent invocation with Hire's expert
  definition. Keep builder instructions in files, not Go prompt assembly.
- `hire verify` checks generated definition structure through `agent check`.
  It must not execute the generated worker's check or other generated code
  with controller authority. Structural readiness is not task-quality proof.
- Reuse the extracted home authoring/maintenance code. All execution belongs
  to the public Agent command. No fallback to an embedded runner.
- Keep outputs in the caller's build workspace and controller evidence
  outside it. Forward stdin, stdout, stderr, cancellation and exact outcomes.
- Tests use fake public tools or the monorepo's loopback model fixture, never
  user credentials, user homes, live effects, or a web server.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and
  `sh -n builder/home.sh expert/bin/check`. Test the Agent/Hire composition
  through the monorepo's public-process integration suite.
