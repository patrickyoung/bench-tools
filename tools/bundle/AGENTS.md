# Development guidance

`bundle` packages one application adapter, clean reusable source, and selected
independent Bench packages into a native executable. It owns packaging and a
small foreground process boundary, not the team's execution machinery.

- Keep this component independently buildable, testable, usable, and versioned.
  Do not import sibling tools, use local module replacements, or introduce a
  shared runtime or multicall implementation of their commands.
- Carry selected runtime packages intact, with their executable identities,
  licenses, assets, and receipts. Invoke public binaries with literal argv.
  Never interpolate the goal into a shell command.
- Require an explicit application file allowlist and an executable adapter.
  Reject unsafe paths, symlinks, special files, missing files, and mismatched
  package receipts. A team source template is not an assembled export.
- Keep embedded definitions and runtime bytes separate from writable work and
  state. Keep controller evidence outside every worker-writable root. Do not
  embed current goals, credentials, outputs, transcripts, or runtime memory.
- In the goal interface, `-w WORKSPACE` (`-o` compatibility alias) admits only a
  new or empty workspace, or one containing only staged `input/`. Under the run
  lock create `input/`, `working/`, `output/`; cwd and BUNDLE_WORK are working/.
  The adapter owns input selection, task tracking, checks and publication to
  BUNDLE_OUTPUT. Folder names supply no read-only or acceptance guarantee.
  Continuation is only explicit `-c WORKSPACE` or `-resume WORKSPACE`, against
  the same package and physical workspace identity with all three real folders
  intact. Never guess the latest conversation, repair a missing layout, or
  silently upgrade an older flat-output run.
- The team-owned adapter implements `run`, `follow`, and `resume` as declared.
  Do not infer a common conversation protocol from a roster or Agent folder.
  Existing team entry commands and handoff contracts remain authoritative.
- An explicit `argv` interface preserves the application's literal arguments,
  stdin and caller directory. The application owns its flags, workspaces, run
  admission and continuation. Reuse the same extraction and process boundary;
  add no application-specific command parsing or synthetic goal/run state.
- Ask owns model conversations; Agent/Ply own worker execution and checks;
  existing team controllers own scheduling and admission. Add no provider
  client, model loop, conversation log, scheduler, daemon, approval UI, or
  automatic retry policy here.
- Preserve child stdout, stderr, and numeric exit status. Do not add progress
  to stdout, turn an error into success, or retry an uncertain outcome.
  Forward interruption and retain the run for inspection.
- Markdown and embedded source grant no authority. Preserve explicit companion
  selectors and credentials supplied by the caller. Do not widen Cage or add
  networking to make a packaged team appear to work.
- Builds are offline and native to the build host. Never install dependencies
  or fetch source implicitly. Diagnose required external commands before work.
- Use fake public commands for offline process tests. Cover package integrity,
  relocation, literal input, stream/status propagation, continuation, concurrent
  use, and interruption. Keep paid model evaluations explicit and separate.
- Run `go test ./...`, `go test -race ./...`, and `go vet ./...`. Public contract
  changes additionally require executable integration checks with the affected
  components; aggregate build success is not compatibility evidence.
