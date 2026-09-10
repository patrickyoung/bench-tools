# Runtime and protocol tools review — 2026-09-09

## Judgment

Import `action`, `cage`, `may`, `mcp`, `oauth`, `rules`, and `trail` as seven unchanged, separately buildable tool directories. Keep Clerk separate. A monorepo is appropriate for these seven because their composition already crosses executable/file/descriptor boundaries, and their contributor contracts make those boundaries explicit. Repository ownership can change without turning their implementations into a framework.

This judgment is conditional on preserving those contracts mechanically: no root Go module/workspace, no sibling Go imports or replacement directives, no combined command dispatcher, no unified mutable state, and no implicit suite-wide permission or credential mechanism. The first migration should change source ownership/layout only. It should not modify live installations, credential profiles, approval grants, admitted capability folders, worker homes, or existing source repositories.

All reviewed source repositories were clean before and after review, including untracked-file checks. All are complete (non-shallow) local histories. This review read the applicable leaf `AGENTS.md`, primary docs, implementation seams, tests, module/build/install files, and history. No applicable ancestor `AGENTS.md` was present above these repositories. No source files were modified.

## Source inventory

| Tool | Exact HEAD | Commits reachable from HEAD | Tracked files | Recommendation |
| --- | --- | ---: | ---: | --- |
| action | `511cd7e7380e03b047a81634127954d8a05322bb` | 3 | 10 | Import now |
| cage | `771973ea1cf5ee0a73a4e4f09f7e495ff2e37cc7` | 3 | 20 | Import now |
| may | `6d04f14e8220b0d32abfc5df2d46564747500f66` | 3 | 15 | Import now |
| mcp | `df8aac0e95756eb08cdb9b12750030f7db5f6dbd` | 10 | 35 | Import now |
| oauth | `134526d885f123c97917b3846ae779303ad04270` | 3 | 16 | Import now |
| rules | `da945ba984e7a552cbefa9043bd24e793d959e89` | 2 | 12 | Import now, with repository-root semantics documented |
| trail | `2ca0dad51b54bfc42843338d805ab69f395c1d81` | 2 | 13 | Import now |
| clerk | `a91fe8df0d54c3acc45d83eb6c622fa25640af64` | 4 | 23 | Keep separate |

Each origin is `https://github.com/patrickyoung/NAME.git`. Cage has a `v0.1.0` local tag; the other seven repositories have no local tags. All heads were committed on 2026-09-08. These short histories make full-history preservation affordable; retain source commit provenance and namespace imported tag/ref names. Seven Go tools retain their own MIT `LICENSE`; Clerk has no tracked license file, another reason not to fold it silently under a suite license.

## Per-repository findings

### Action

The invariant is concrete in `main.go:214` (`runAction`): resolve one named connector, canonicalize its JSON input, fingerprint executable/descriptor, obtain a policy result or exact May result, revalidate the connector, and release stdin only after the prepared-attempt receipt. Post-release missing/invalid results are conservatively exit 125. `main.go:528` records through the external `ask note ... -seal` process; it does not append Ask's file format itself. `authorize` checks strict canonical result JSON against exit code, action digest, and exact request. No provider or MCP package is imported.

Runtime dependencies are explicit: connector `describe`/`run` processes on `ACTION_PATH`; optional deterministic policy; standalone `may request JOB` when policy requires review; and Ask only with `-record`. `ACTION_POLICY`, `ACTION_MAY`, `ACTION_ASK`, and command flags are controller configuration. There are no Go dependencies outside the standard library.

Tests cover allowed/parked decisions, a policy allow that never resolves May, exact proposal shape, duplicate/unknown fields, receipt failure before and after request release, post-send uncertainty, and bounded deadline behavior. They use executable helper processes, not live connectors.

Import unchanged. Do not collapse its canonicalization/receipt logic into MCP, Agent, May, or a shared suite package. Absolute executable paths and hashes participate in exact requests; moving/rebuilding live connectors can intentionally invalidate pending authority, so source migration must not rewrite or migrate approval state. The initial tests use controlled peers; a separate integration gate should also exercise real Action→May parking and Action→Ask receipt replay using synthetic data.

### Cage

`backend.go` requires readiness from inside the backend and refuses an unavailable backend before creating a requested child. Native backends are in separate platform files: fixed-path macOS Seatbelt and trusted-system-path Linux Bubblewrap. Standard streams are passed directly; child status/signals are preserved. Unsupported platforms have an explicit refusal backend. No dependency on any other Bench tool or any Go package outside the standard library exists.

Tests cover default/explicit write roots, bad roots, stream/status propagation, readiness timeout, setup failure, unavailable-backend refusal, documentation, and platform-specific command/profile construction. The native acceptance command additionally tests actual positive and negative kernel effects, not just generated argv.

Import unchanged and retain the platform checks. Its dependency-free leaf is one of the strongest initial candidates. A source monorepo must never become an implicit `cage -w MONOREPO` runtime grant. Write roots should stay selected by the caller for the actual workspace; controller binaries, policies, and records stay outside worker write roots. Filesystem reads remain unrestricted by design; neither source proximity nor a shared installer strengthens that boundary.

### May

`state.go` owns one exact job/action request and its state transition. It validates private regular state entries, binds the digest to both job and exact action bytes, atomically renames a grant into a unique spent directory before reporting success, and appends audit evidence. `request JOB` exposes the same transition as strict machine-readable data; it does not introduce another approval path. Human decisions come from `/dev/tty`. Production deliberately has no environment-variable decision controls; `docs_test.go` tests that absence.

It is a standard-library-only tool with no Bench runtime peer required. Tests cover missing terminal, exact-byte/job binding, single-use/concurrent grant spending, machine result failures, bad state ownership/modes/types, tampering, unknown/trailing input, and the built-in acceptance check.

Import unchanged. May remains the only standalone exact human authority mechanism in the selected tool set. Do not put it in model toolboxes, inject controller state into worker roots, add a suite approval flag, or route it through a common daemon. Clerk's different `bin/may` must not enter a shared installation manifest.

### MCP

One local Go module owns four deliberately separate executables: `mcp`, `mcp-legacy`, `mcpbox`, `mcpserve`. Shared `internal` packages are internal to this one protocol-edge tool, not cross-tool coupling. `internal/mcpclient/record.go` tracks wire-response/send facts in memory; it is not a durable second event log. The transport owns Unix process-group lifetime. Modern requests never select legacy sessions by implicit fallback.

`internal/box/box.go` stages a capability folder, pins endpoint and reviewed descriptor digest, and renders executable adapters. `renderActions` generates an ordinary Action connector with `describe`/`run`, while permission stays with the separately invoked Action tool. Generated runtime programs resolve the chosen MCP executable and pin endpoint metadata; source movement is safe, but moving already admitted runtime paths is a separate operator action. OAuth arrives through an explicit header descriptor/file interface; no credential lifecycle is absorbed here.

The module independently pins the official MCP Go SDK (`v1.7.0`), jsonschema-go (`v0.4.3`), uritemplate (`v3.0.2`), and its indirect dependencies in `go.mod`/`go.sum`. No sibling Bench module is imported. Keep those dependencies local to MCP; a suite dependency upgrade must not drag standard-library-only tools into this graph.

Tests cover wire result preservation, post-send uncertainty, output bounds, stateless HTTP, modern/legacy negotiation, pinning, progress/listen without reconnect, task outcomes, process-group timeout, atomic capability generation/admission, generated Action adapters, and reverse dispatcher outcomes. Peers are subprocesses/loopback fixtures. `install.sh` and `justfile` build relative to this module and remain leaf-local.

Import now as one module with four named commands. Do not merge those commands into an umbrella executable or teach Ask/Ply/Agent MCP internals. No external protocol-version validation was needed for the source-layout judgment; this review establishes the checked-in implementation and tests, not future SDK compatibility.

### OAuth

`internal/run/command.go` invokes literal argv without a shell, gives the child one authorization header on descriptor 3, preserves its streams, forwards signals to its process group, and does not retry. `internal/state/state.go` keeps independently permissioned definition/secret records tied by a binding value and serializes refresh under a profile lock. `internal/protocol` validates resource/issuer identities and rejects unsafe endpoints/redirects. OAuth is a separate standard-library-only module, not an imported Ask/MCP credential helper.

Tests use local fake authorization/token endpoints and synthetic credentials. They cover PKCE callback/state, discovery identity refusal, resource binding, redirect refusal, error redaction, rotating refresh concurrency, symlink/hard-link state refusal, and exact descriptor transfer with child status. No real login or credential profile was read or modified for this review.

Import unchanged. Its install script builds from its own directory. Retain `OAUTH_HOME`/XDG state ownership; a monorepo must not add a common credential directory or ambient token injection. The deliberate `oauth header` command emits secret bytes explicitly; ordinary composition should retain `oauth with ... -- CHILD` and descriptor 3.

### Rules

This is a standard-library-only deterministic read filter. `resolve.go:12` finds the nearest regular `.git` file or directory. `loadInstructions` loads only root-to-leaf `AGENTS.md` then `CLAUDE.md`, validates the whole bounded set before output, refuses symlinks leaving the root, deduplicates canonical aliases but not equal-content distinct files, and emits provenance separately. It neither executes instructions nor grants permission.

Tests assert real nested Git-marker behavior, worktree `.git` files, ignored `.git` symlinks, instruction path escape refusal, ordering, canonical aliases, individual/aggregate byte bounds, no partial output, and no repository writes. The tool itself has no sibling build/runtime dependency.

Import now, but explicitly account for the one layout-dependent semantic effect: once leaf source directories no longer contain their own `.git`, the enclosing monorepo is the true repository root. Root instructions become applicable, and an instruction symlink into another leaf would now be inside the repository. Preserve Rules' honest nearest-Git-root contract. Do not add fake nested `.git` markers or teach Rules about monorepo package names. Keep root `AGENTS.md` to shared boundary/contributor policy, retain every leaf `AGENTS.md`, and enforce absence of cross-tool instruction symlinks. For work requiring the former narrow repository scope, export a tool into its own initialized Git repository and run there. Adding a monorepo root is not permission to broaden runtime write roots.

### Trail

`archive.go` defines a small public JSON wire view and retains raw event bytes, including unknown event types. File discovery/reading, semantic selection, and JSONL printing remain local. `main.go:287` delegates each replay verdict to a separate `ask replay -check FILE`; Trail does not reproduce Ask's replay fold or write to its archive. It is standard-library-only.

Tests cover damaged/torn files without hiding later sessions, unknown event pass-through, recorded lineage only, duplicated identity paths, explicit semantic-text exclusions, maximum line refusal, output failures, Ask command delegation, and byte-for-byte unchanged archives. It already has the right abstraction for a colocated Ask: the file format stays public, implementation stays unshared.

Import unchanged. Preserve `ASK` and `ASK_DIR` process/file configuration, never replace its wire view with an internal Ask package, and do not add an index/writer/cache in the migration. Cross-tool integration should make a synthetic Ask session, invoke real Trail commands, verify replay via Ask, and compare the archive before/after.

### Clerk

Clerk is coherent as a shell spool application, but it is an older operational composition with different authority/recovery behavior. `bin/clerk-work` drains file queues, recovers every orphan in `working/` by requeueing, sources operator shell configuration, builds a PATH toolbox, and runs job-type scripts through Ply. Defaults place worker state beneath the source checkout. `workers/crier` and the launchd example embed personal operational assumptions (Pitch/HN logs, Shop/Heed container routing, `~/projects/clerk`), rather than defining a reusable primitive only.

The decisive collision is `bin/may`: it receives the action in argv, uses Clerk worker/job environment and its own grant directory, and accepts a human decision through `clerk grant`. It is not compatible with standalone May's stdin/TTY/machine-result contract. The README explicitly says not to install this binary over standalone May, but `justfile`'s `link` recipe still links `bin/may` globally into `~/.local/bin/may`. A monorepo-wide naive installer would create an authority-path collision despite both isolated suites passing.

Its recovery model also differs from the newer exact-effect tools: `bin/clerk-work` assumes a recovered job may run again, while Action reserves unknown outcomes for manual resolution. Toolbox scope is documented as not a sandbox. The crier bulletin job deliberately owns notification delivery, including a fallback through a Docker container; that is application policy, not a core filter mechanism.

Keep separate. Its offline suite passes and its useful shell patterns are worth retaining as reference. A future import would need an explicitly separate application tier, a private/renamed gate installation, external runtime data defaults or very clear separation, adapted operational examples, and a considered relationship to Agent/Tend. Those are real design decisions and should not be smuggled into a source move. No license file is tracked here; preserve provenance instead of implicitly relicensing it.

## Verification performed

Host: `go version go1.26.1 darwin/arm64`. For each of the seven Go modules, the following completed successfully with `GOWORK=off`, `GOPROXY=off`, and `GOTOOLCHAIN=local`:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Some unchanged package results used Go's valid test cache; the test source and repository tree were inspected. MCP's race transport/client suite ran in approximately 30 seconds. No model/network-account integration tests were selected. HTTP fixtures and Cage's network probe used local loopback only.

All ten executables built to a temporary directory outside source and printed the expected version:

| Tool directory | Standalone build from that directory | Observed version |
| --- | --- | --- |
| action | `go build -o /tmp/action .` | `action 0.1.0` |
| cage | `go build -o /tmp/cage .` | `cage 0.1.0` |
| may | `go build -o /tmp/may .` | `may 0.1.0` |
| rules | `go build -o /tmp/rules .` | `rules 0.2.0` |
| trail | `go build -o /tmp/trail .` | `trail 0.1.0` |
| oauth | `go build -o /tmp/oauth ./cmd/oauth` | `oauth 0.1.1` |
| mcp | `go build -o /tmp/mcp ./cmd/mcp` and the equivalent `cmd/mcp-legacy`, `cmd/mcpbox`, `cmd/mcpserve` builds | all `0.3.0` |

The actual commands used a unique temporary output directory; `/tmp/NAME` in this table is a compact build recipe. Additional checks completed:

- Built May `check`: passed, with temporary synthetic approval state only.
- Built Cage `check`: all 13/13 native Seatbelt probes passed, including allowed/denied writes, read-only workspace, explicit write replacement, streams, exits/signals, network denial/explicit grant, and unavailable-backend refusal.
- Cage `GOOS=linux GOARCH=amd64 go build` and `GOOS=windows GOARCH=amd64 go build`: passed. Cross-compilation is not a Linux runtime boundary proof; Linux native `cage check` remains a separate host gate.
- Built Trail `ls` and `check` on an explicit empty temporary archive: passed without Ask or archive mutation.
- `mandoc -T ascii` rendered `action.1`, `cage.1`, `may.1`, `rules.1`, and `trail.1` successfully. `nroff` was not installed; the available native `mandoc` was used.
- Clerk `sh bin/check`: parser/executable/dependency/plist checks and every offline scenario passed, including concurrency, grants, queue recovery, environment filtering, and the stubbed crier job.

## Boundary enforcement recommended for the new repository

1. Preserve every leaf `go.mod`, `go.sum` where present, command layout, README, security document, license, man page, and contributor contract. Leave module paths unchanged during the first import. Publish/install-path migration is a later explicit release decision; current README upstream installation commands can continue to identify the intact existing origins during transition.
2. Build/test a selected leaf with `GOWORK=off`. Also export each selected leaf without siblings or a parent module and verify it builds/tests independently. Shared orchestration may enumerate tools but must not import runtime implementations.
3. Reject root `go.mod`/`go.work`, sibling module `require`/`replace` directives, production imports from another Bench tool, source symlinks crossing tool boundaries, and shared runtime packages outside a leaf. Do not let a guard rely only on human declarations; inspect actual module metadata and import edges.
4. Have a manifest explicitly list executable names and reject collisions. MCP's four commands are intentional within one module. Clerk's private May is not a second suite May.
5. Keep integration checks outside individual tools. Exercise real binaries over public stdin/stdout/JSONL/file-descriptor/exit-code interfaces with synthetic fixtures: Action→May parking, Action→Ask recording, Trail→Ask verification, and MCP-generated Action connectors. Never make ordinary leaf unit tests require sibling source or live credentials.
6. Preserve meaningful refusal behavior: no Cage fallback, no May automatic authority, no Action/MCP retries after uncertain send, no OAuth ambient credentials, no Trail writes, no Rules instruction-as-authorization. A shared command should not normalize their distinct exit codes into one suite outcome.
7. Keep root instructions minimal and leaf instructions authoritative for their implementation. Document Rules' real monorepo root scope and preserve an isolated leaf export path when independent repository scope is needed.
8. Keep source migration reversible and evidenced: record original HEAD/tree, preserve complete history/provenance, compare imported tracked contents/modes, and keep original repositories untouched. Build artifacts, worker data, approval state, credentials, generated admissions, and ignored local files are not import payloads.

This is a repository suitability and migration-boundary review supported by implementation inspection and offline checks. Passing these checks supports moving the checked-in source as independent leaves; it does not assert a fresh security certification of every tool or operational deployment.
