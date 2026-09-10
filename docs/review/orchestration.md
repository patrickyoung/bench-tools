# Orchestration and application repository review

Review date: 2026-09-09. Scope: `agent`, `hire`, `manage`, `studio`, `tend`, `weave` under `/Users/patrick/projects/bench`. Source repositories were not edited. Recommendations concern repository placement, not a claim that an application has passed a complete security audit.

## Decision

A monorepo is reasonable for the independent command suite **if repository consolidation never becomes runtime consolidation**. Import `agent`, `tend`, and `weave` as intact, separately buildable components. They express distinct responsibilities and already communicate through commands, files, exit statuses, and JSONL. Joint interface verification is valuable; importing one another's code, sharing state, or replacing public commands with calls would destroy that value.

Keep `hire` and `studio` separate from the first core migration. Defer `manage` until its experimental application/release status and fresh-checkout test setup are explicit. All three are useful applications, but they add orchestration or user-interface policy that must not become an implicit part of the filters.

## Source identity and disposition

| Directory | Exact HEAD | Tracked/untracked Git state | Recommendation |
| --- | --- | --- | --- |
| agent | `f8e566e20f15f718d5104bc776f5ed2b34d5ff7f` | Clean | Import now, independent shell component |
| hire | `631194190f9690091d0eacb837bf5905c073f7cb` | Clean; ignored binaries and `var/` exist | Keep separate application for initial migration |
| manage | `347f38dd550aadcba5c8b8890cda6c73f1a5c8a5` | Clean; ignored `var/` contains tools/run material | Import later as an explicitly experimental application |
| studio | None: directory is not a Git repository | Git dirty state undefined; local `var/studio.db` and design/run artifacts exist | Keep separate; not ready for provenance-preserving import |
| tend | `77e678aec18ccf529fc67892e70dd03fc6a16205` | Clean; ignored binary exists | Import now, independent Go module |
| weave | `57b2f7e28576f98e6e5646c8e508821ef92cb646` | Clean; ignored `var/`, `dist/`, binary exist | Import now; stable filter and explicitly experimental recipes remain distinct |

“Clean” means `git status --short` was empty. It does not mean ignored working state may be copied. Import tracked source, its executable modes, licenses, component instructions, and tests; never copy the whole working directory. Each repository's `AGENTS.md` was read; no parent `AGENTS.md` exists in the checked ancestor paths.

## Agent

**Responsibility:** Compile a standing Agent home into the existing Brief/Ply/Ask/Cage/Hone/Trail/May/Action process boundaries. `AGENTS.md` explicitly forbids a second agent loop, provider adapter, scheduler, skill parser, task database, or transcript.

**Implementation evidence:** `bin/agent` resolves and invokes public executables, uses private files for the goal/task/context, gives Brief skill-selection responsibility, and delegates completion to the home `bin/check` through Ply. `bin/agent-action-shell` remains the explicit Cage action seam. Root definitions, `work/` and `state/`, and controller `.agent/` evidence are separate write domains. Checkpoints are Ask-session pointers, not a snapshot or new transcript. `bin/agent_test.sh` uses fake public binaries and asserts literal argument forwarding, no private task text in argv, input bounds, symlink/hardlink refusals, approval parking, preserved exit status, and separate specialist evidence.

**Migration value:** This is the strongest process-composition example in the review. A coordinated suite can run the same seam tests whenever Ply, Brief, Action, or Cage changes without turning Agent into their owner.

**Risks:** Preserve the relative `bin/agent` / `bin/agent-action-shell` layout and executable modes; symlink installation deliberately resolves the companion wrapper. Keep plugin/skill docs that ship with Agent scoped to Agent. Its installer/documentation pins historical suite releases and upstream source URLs; moving source does not silently authorize republishing or changing those pins. Skill installation scripts can access networks; they are not ordinary offline verification.

**Standalone commands:** `sh -n bin/agent bin/agent-action-shell`; `sh bin/agent_test.sh`; optional `sh eval/run.sh` after checking its public-tool requirements. Agent has no Go module or library dependencies. Runtime dependencies are exact executable seams selected by `AGENT_ASK`, `AGENT_PLY`, `AGENT_BRIEF`, `AGENT_CAGE`, `AGENT_HONE`, `AGENT_TRAIL`, `AGENT_MAY`, and `AGENT_ACTION`.

**Observed checks:** Syntax passed; all 163 shell assertions passed. A `git archive HEAD` extraction in a temporary directory without siblings also passed shell syntax. The fake-binary suite is valuable boundary coverage, not proof of a live model or host Cage capability. Recent history includes builder/skill releases and interface-oriented documentation; preserve the MIT license.

## Tend

**Responsibility:** A crash-durable local supervisor for arbitrary ordinary programs. One `tend work` performs one transition/job and exits; repetition belongs to the caller. No imported Bench code or model dependency exists. `go.mod` is `github.com/patrickyoung/tend`, Go 1.26, with `modernc.org/sqlite` and its pinned indirect dependencies.

**Implementation evidence:** `work.go:88` performs bounded recovery, timer wake, or claim and execution, then returns. Execution argv is persisted and invoked directly; the completion-check shell is a documented operator seam. `store.go` owns local transactional jobs, immutable events/attempt results, leases, signals, and timers. Its partial unique indexes prevent concurrent/unknown jobs from sharing a serialization key. Started attempts with no trusted result become unknown; `work.go` releases the child gate only after the started fact is durable. `main_test.go` builds an independent executable in a temporary directory and exercises it as a process, including crash barriers, concurrent claims, evidence tampering, and resolution races.

**Migration value:** Shared development can verify real process composition with Agent/Weave/Manage while Tend remains useful for shell scripts or unrelated programs.

**Risks:** Never share Tend's SQLite schema/database with an application; no shared repository-level controller state, daemon requirement, retry policy, job DSL, or in-process scheduler API. Keep the SQLite module dependency local to Tend. Preserve its standalone version and release identity. The scrubbed child environment has explicit public-tool variables; adding convenience inheritance is an authority change, not repository plumbing.

**Standalone commands:** `GOWORK=off go build .`; `go test ./...`; `go test -race ./...`; `go vet ./...` inside this component.

**Observed checks:** Source-tree test/race/vet passed (Go reported cached test results). More strongly, tracked HEAD was extracted without siblings, built with `GOWORK=off GOPROXY=off GOSUMDB=off`, and `go test -race -count=1 ./...` passed in 27.174s there. This confirms independent source/build and uncached crash/concurrency verification with cached third-party dependencies. No network fetch or live credentials were used. MIT license is present. Recent history hardens concurrent evidence sealing rather than widening its responsibility.

## Weave

**Responsibility:** The most literal Unix filter here: finite task graph + finite observation snapshot → unchanged ready task records on stdout. It performs no execution, clock reads, model calls, queue/database updates, retry, or evidence authentication. `main.go` imports only Go standard-library packages; its `github.com/patrickyoung/weave` module has no requirements.

**Implementation evidence:** `main.go` fully reads and validates both bounded inputs before output; tasks retain raw bytes and SHA-256 identity. Tests cover exact unknown-field/numeric preservation, input order, stale task hashes, duplicate/ambiguous JSON, Unicode, cycles, incomplete snapshots, output failures, and finite graph bounds. Accepted observations release dependencies; unknown and rejected outcomes cannot automatically reappear. Empty stdout is explicitly not completion. Python recipes call public executables through bounded subprocess I/O; they live in `examples/`, not filter implementation.

**Migration value:** Straightforward independent import with zero code coupling. Coordinated process tests can prove that adapters translate Tend/Ask/Ply receipts into observations without making Weave an executor or verifier.

**Risks:** Preserve the documented distinction between supported filter v0.1.0 and experimental research/business recipes. `scripts/release.py` exports an explicit release-file list and stages an independent source build; do not replace that with an indiscriminate monorepo tarball. Existing pinned tool bootstrap is networked and optional. `tests/check --siblings` is explicit convenience only; `--bin DIR` supports released/independently built tools. Do not make sibling sources mandatory. Preserve module path, MIT license, man page, byte contracts, component version and independent release artifacts.

**Standalone commands:** `GOWORK=off go build .`; `make check` (format, Go test/race/vet, pure Python tests); `tests/check --bin DIR` for the separate offline composition suite after supplying exact Ask/Ply/Tend executables. `make bootstrap` fetches dependencies and was not run.

**Observed checks:** `make check` passed: Go test, race, vet, format, and 36 pure Python tests across business (9), protocol (10), receipts (1), research domain (9), and release (7). Tracked HEAD independently extracted and built without siblings, with Go workspace and fetching disabled. Full optional example integration was not run by this review. Recent HEAD strengthens byte-exact Ply receipt verification in recipes; the prior commit prepares the release candidate.

## Manage

**Responsibility:** Bounded, experimental research/review manager application. It owns admission, immutable plans/decisions, allowances, and root acceptance. Ask owns sessions/check records, Tend owns attempts, Weave chooses readiness, and a supplied independent root check accepts the combined result. The read-only board is a projection. `managelib/models.py` calls Ask and Ply as executables and uses a report-only action-shell adapter; it does not import their internals.

**Positive evidence:** `managelib/controller.py` resolves exact executable paths, fingerprints tools/source, freezes deadline/allowances, reserves before submission, and reuses admitted identities on resume. `managelib/runtime.py` writes controller receipts outside worker directories and attaches records using public `ask note ... -seal`. Tests cover started unknown work, interrupted admissions, root rejection, preserved allowances, tampering, stale plans, graph cycles, and local fake-provider composition.

**Why defer:** This is a higher-level application, not another filter. It has only two commits and explicitly experimental status; no tracked LICENSE/COPYING file exists. `Makefile:4` builds sibling checkout HEADs without a compatibility lock, and `tests/test_controller.py` hardcodes `var/tools/{ask,ply,tend,weave}` even when external tool variables are supplied. A fresh source-only checkout therefore cannot execute the full suite until separately provisioned. The current 22-test success uses existing ignored tool binaries; those identities are not pinned by the test harness. There is no reason to turn this convenient workspace arrangement into mandatory code coupling.

**Standalone usage/build:** Python 3.9+ standard library; preserve `bin/bench-manage`, `managelib/`, and `libexec/` together. `make tools` is a sibling-source developer helper; runtime accepts exact `ASK`, `PLY`, `TEND`, `WEAVE` paths. `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests -v` passed 22 tests in 119.551s. The native model path used a local inference fixture, not credentials or an external provider. Documented live-model CLI examples were intentionally not run.

**Before later import:** Establish a source/release manifest and explicit license provenance, make fresh-checkout offline tests accept a prepared binary directory or build locked dependencies in temporary storage, and retain experimental app placement. Never move manager planning, budget admission, or board state into Weave/Tend/Ask.

## Hire

**Responsibility:** Useful local web controller around Agent homes and Tend jobs, with its own intake, reviews, skills, connected-service grants, scheduling loops, and result UI. Independent Go 1.26 module `github.com/patrickyoung/bench-hire`; no non-standard-library requirements; browser assets are embedded and use no framework.

**Positive evidence:** `exec.go` calls exact Agent argv and preserves its exit status. Its verification path binds the original request and check hash. `runner.go` wraps repeated public Tend work calls; it does not replace Tend execution facts. `connected_calls.go` supplies an explicit deterministic policy to Action, and terminal May retains approval authority. `suite_test.go` tests pinned companion binaries and opt-in real CLI integration with local inference fixtures.

**Why keep separate initially:** Hire is already a large application (130 tracked files), with product policy and application-owned network/UI/broker surfaces. `codex.go` directly reads Codex login files and implements OAuth refresh HTTP; `connected_broker.go` opens a Unix socket for application calls. These may be deliberate app responsibilities, but they must never become shared core libraries or ambient services for the filters. A new core monorepo does not justify merging Hire and Studio or choosing one product implicitly. No tracked LICENSE/COPYING file exists. Tend jobs retain the Hire executable path; moving/rebuilding an installed executable under active work is a lifecycle change, so source import must not relocate existing installations/state.

**Portability finding:** Full baseline `go test ./...` failed after 104.134s at `TestConnectedPermissionsCallsSourcesAndResolution`, `connected_workflows_test.go:70`: the automatic fake-Action call received policy denial exit 3. The same single test failed again on demand. It passed when only `TMPDIR` was changed from its `/var/...` spelling to the resolved `/private/var/...` physical path. The evidence supports a lexical/physical path mismatch: `newTestApp` retains `t.TempDir()` spelling, while the Python Action fixture uses `os.getcwd()` and `runAppPolicy` compares `envelope.Directory == args[1]`. This is a reproducible test portability issue; the targeted diagnostic pass does not make the original full suite green.

**Standalone commands:** `GOWORK=off go build -o hire .`; `go test ./...`; `go vet ./...`; `node --check web/app.js`; `node --check web/connections.js`. Optional real-suite tests require explicit `HIRE_INTEGRATION_BIN_DIR` and should remain a separate composition stage.

**Observed checks:** Tracked-source isolated build passed without siblings, with Go workspace/fetching disabled. Vet and both JS syntax checks passed. Baseline full tests failed as above; targeted physical-TMPDIR rerun passed. Source was not changed to hide the failure. Recent history adds connected-app workflows, skill review, and Codex wrapper compatibility, reinforcing separate application release concerns.

## Studio

**Responsibility:** Local/gateway design and lifecycle UI around Bench Agent, with gated materialization, immutable releases/deployments, retained evidence, and an explicitly dormant execution-broker adapter. It is a substantial unversioned source directory with local state, not presently an importable Git repository.

**Positive evidence:** It has extensive unit/regression tests for CAS writes, file leases, immutable evidence, private object storage, lineage, runtime proof binding, and fail-closed gateway execution. `runtime.go` executes absolute binaries with literal argv. Gateway broker transport remains disconnected, as documented. No claim of production hosted execution follows from these tests.

**Blockers for initial import:** `go.mod` requires `github.com/patrickyoung/bench v0.0.0` with `replace ... => ../bench`; `broker_adapter.go:10` and `broker_projection.go:10` import `bench/brokerprotocol` directly. This is actual compile-time cross-component coupling, not merely a sibling example script. `main.go:265` also defaults the builder skill to the adjacent Agent checkout. The direct protocol import may eventually deserve a deliberately versioned public contract, but a new monorepo must not silently bless internal coupling as a substitute for the mandated process boundary. No Git HEAD/history or root LICENSE exists here. `var/studio.db` and local design/review files must not be imported.

**Authority documentation inconsistency:** `AGENTS.md` says SQLite is a rebuildable index/queue/projection. The README durability section says SQLite is not disposable and cannot currently be reconstructed from files. The code stores lifecycle bindings in SQLite. Resolve that ownership/reconstruction contract before broadening its distribution or claiming all controller state is a projection.

**Standalone commands (current workspace-dependent form):** `go test ./...`; `go test -race ./...`; `go vet ./...`; `node --check web/app.js`. Current source-tree `GOWORK=off GOPROXY=off GOSUMDB=off go test ./...` passed in 25.483s, with the sibling Bench replacement available. This is workspace compatibility evidence, not standalone reproducibility. Follow-up race tests passed in 78.250s, and vet plus JavaScript syntax also passed. These still require the current sibling replacement.

## Required monorepo enforcement

1. One directory and module per command, unchanged module paths, executable names, CLI/stream contracts, exit semantics, versions, and component licenses/instructions. No root `go.work` required for builds; run with `GOWORK=off`.
2. Independently export each component's tracked source into a temporary directory without siblings. Build and run its local tests there; do not mistake root-workspace builds for independence.
3. Reject cross-component Go imports and local `replace` directives for the core. Do not create a shared runtime, state package, transcript type, scheduler, provider catalogue, approval layer, or library merely because code looks duplicated.
4. Keep public process-composition verification separate: build exact binaries in a temporary bin directory, supply explicit tool path variables, use finite local fixtures and fresh state, and assert each component's status/output/evidence boundaries.
5. Keep independent source/binary release manifests and dependency locks. Commit provenance per imported HEAD, preserve licenses and executable bits, and explicitly decide future tag/split publication behavior before any release. Co-location does not authorize publication or replacement of the currently installed suite.
6. Exclude `.git`, ignored binaries, `dist`, `var`, sessions, SQLite files, auth material, generated output, and private evaluation runs from source migration. Use tracked-source extraction rather than recursive directory copying.
7. Applications consume public commands and own application policy. Core CI must remain usable without starting Hire/Studio, bootstrapping credentials, connecting services, or creating a global controller. Gate any later app import independently; green primitive tests cannot justify importing an unreviewed app.

The safe initial set from this review is **Agent + Tend + Weave**, alongside independently approved filters from the other repository reviews. That decision preserves the user's unique Unix tools rather than folding them into an agent framework.
