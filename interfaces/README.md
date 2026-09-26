# Interfaces

Human interfaces to Bench programs live here. Each application has its own
module, executable, version, assets, documentation and checks. The programs in
`tools/` remain independently usable without a browser or interface process.

| Application | Purpose | Build and run |
| --- | --- | --- |
| [Hire UI](hire/README.md) | Browse reusable workers and teams, export pinned source, and author expert folders | `cd interfaces/hire && make build` |

An interface composes public commands and documented files. It does not import
another tool's implementation or own its model connection, execution loop,
approval policy or confinement. Its selected runtime directory holds its own
workspaces and records outside reusable source.

The shared standard is a small, accessible, responsive application: semantic
HTML, keyboard operation, clear states, server-rendered content and only the
JavaScript an interaction needs. Prefer Go's standard HTTP server, templates
and embedded assets when they meet the application's needs. Each application
documents its own decisions; this directory is not a shared framework.

From the repository root, `make build-interfaces` builds applications and
`make check-interfaces` runs their offline checks. These are separate from
tool installation and release packaging. See each application's README for
its runtime dependencies and command contracts.

[manifest.json](manifest.json) declares interface source roots, Go modules and
executables independently of the headless tool manifest. An interface named
`hire` lives at `interfaces/hire`; its boundary owner is `interfaces/hire`,
distinct from the `hire` tool. Command names and module paths remain unique
across both manifests. The root architecture gate applies the same import,
requirement, replacement, symlink and main-package checks in both directions.
Original tool-import provenance checks remain limited to the tool manifest.
