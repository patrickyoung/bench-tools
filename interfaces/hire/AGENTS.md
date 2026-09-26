# Hire UI

This application is a local browser adapter. Read DESIGN.md and README.md
before changing its behavior. It must build from this directory alone with
`GOWORK=off`; its module is independent of every Bench tool.

- Keep Hire headless. Call the selected public `hire` executable and selected
  source checkout's `scripts/workers`; never import their implementation.
- Keep the catalog, worker/team details, source export, expert authoring and
  structural verification understandable as separate operations.
- A catalog entry describes source. An export selects a full commit. A build
  writes a new expert folder. Verification inspects structure and does not
  establish job quality or run generated checks.
- Use the standard library HTTP server, html/template and embedded assets.
  Preserve useful HTML navigation and forms without JavaScript.
- Accept only loopback listeners. Validate Host and protect mutating requests
  against cross-origin submissions. Escape source and process text.
- Require explicit source and data roots; data must remain outside source.
  Keep job records and controller evidence separate from authoring workspaces.
- Permit one active public command. Real model builds require `-allow-build`;
  retain default Cage behavior and bounded turns/time. Do not add a provider
  client, execution scheduler or automatic retries. Explicit worker runs require
  -allow-run and compose the public Agent command; Agent owns check execution.
  Structural verification must never execute generated checks.
- Retain stdout, stderr and exact exit status. Explicit cancellation must
  reach Hire's signal forwarding. Browser navigation must not cancel a build.
  A process restart must not turn an unknown outcome into success or a retry.
- Test through isolated temporary directories and fake public executables.
  Exercise actual public-process composition separately without user homes,
  credentials or paid model calls. Never load personal configuration in tests.
- Run `make check`; verify the standalone binary and responsive browser flows
  when changing the public interface. Document the scope of evidence.
