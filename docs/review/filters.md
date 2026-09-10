# Filter repository review — 2026-09-09

Reviewed from the checked-out source, tracked files, local commit history, module manifests, contributor guidance, implementation, tests, and existing CI. No source repository was edited. All eight repositories had empty `git status --porcelain=v1 --untracked-files=all` at initial and subsequent inspection. Ignored local binaries, Web's `.venv`, and local state are not source to migrate. No credentialed provider call, browser login, live evaluation, or public network test was run.

## Recommendation

Import **Ask, Brief, Ply, Hone, Cite, and Draft** now, as separate directories with their existing process interfaces and language/build boundaries. A repository boundary is not the reason these tools stay small; explicit implementation ownership, separate executables, file/stream contracts, and tests are. Sharing a checkout can improve synchronized protocol review without sharing runtime code. Recent commits demonstrate the practical reason: Ask's changed session flags required Brief and Hone updates, and Ply's versioned verifier receipts required Hone updates.

Import **Vouch later** after writing down its ownership and distribution policy alongside the separate OAuth tool. Keep **Web separate for now**: its legacy Clerk/May gate must be reviewed explicitly before presenting it as part of the current exact-action family. Neither should be merged into another tool.

The safe initial filter set is the six named above. Preserve `tools/{ask,brief,ply,hone,cite,draft}` as siblings: that retains Ply's optional real-Ask test discovery and Draft's local installation recipe. Root automation should coordinate commands, not introduce a shared runtime, common session library, dispatcher binary, omnibus config, implicit learning, or universal exit-code vocabulary.

## Recorded sources

All remotes are `https://github.com/patrickyoung/NAME.git`; these are local HEAD observations, not assertions about remote branch freshness.

| Repository | Exact HEAD | Worktree | Decision |
| --- | --- | --- | --- |
| ask | `57e1d609fbeed1f31906f238ea4004d3f053779f` | Clean | Import now |
| brief | `7116c0695331a835509ba65d05032bb8200f2045` | Clean | Import now |
| ply | `1ea0593f7373c9df238cf3ce068f3b58db9bf341` | Clean | Import now |
| hone | `dc51765d0367c1f25a3b18f5731733b4618d4e00` | Clean | Import now |
| draft | `2fc900908cd1f90bd75537bec3063cfc7e695db5` | Clean | Import now; generated-reference check needs an explicit tool environment |
| vouch | `bf98d394083bf25a85476109958df573903248ba` | Clean | Import later |
| web | `22b632da5d72209fb1b5db25b365ceb4c3f57b90` | Clean | Keep separate for now |
| cite | `7322285d66199166423a650ae24a2c3c812d62be` | Clean | Import now |

## Per-repository findings

### Ask

Owns model transport and the append-only replayable conversation. `internal/provider/provider.go` specifies streamed-text/log equality and replayable reasoning; `internal/provider/contract_test.go` tests provider fixtures and native schema placement. `internal/event/log.go` locks the session before reading and checks the existing prefix before append. `internal/event/fold.go` rejects missing request proof and missing immediate seals; notes stay outside conversation folding. Context input becomes message data plus a manifest over those exact bytes (`context.go`, `internal/event/evidence_test.go`). These are unusually concrete boundaries worth retaining.

Independent Go module `github.com/patrickyoung/ask`, Go 1.26, with provider SDKs and JSON Schema validation as dependencies. All `github.com/patrickyoung/...` Go imports are its own `internal` packages. Other tools must continue invoking Ask/replay/note/append rather than importing those packages. Credentials through `-header-fd` remain inherited transport input; do not move credential acquisition into Ask.

Standalone: `GOWORK=off go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -o /tmp/ask .`. Existing GitHub CI tests Linux and macOS, vet, formatting, race, build, and `go install .`. `ASK_LIVE` gates paid/live provider checks and must be empty in ordinary CI. The CI comment still mentions PID-style lock mechanics while source uses `flock`; preserve source semantics and treat comments as documentation to review, not architecture to resurrect.

### Brief

Owns skill discovery, progressive disclosure, body/resource reads, and lint. It does not own execution or model access. `find.go:askPick` writes a private catalogue containing only names/descriptions, puts task text on stdin, and calls an external Ask with a fresh `-f` session. `find_test.go:TestFindAskSendsLevelOneAndNothingElse` inspects bytes crossing the executable seam, including refusing unoffered names and preserving empty output for no match. `skill.go` and `store.go` validate names and confine resource paths before opening them.

Independent Go module `github.com/patrickyoung/brief`, Go 1.26; its sole direct dependency is YAML v4. Keep YAML parsing and skill specification limits here. Do not share the catalogue implementation with Ply/Hone or add a use counter, registry, execution metadata, or automatic installer during migration. `ASK` selects the executable and ordinary discovery needs no Ask at all.

Standalone: `GOWORK=off go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -o /tmp/brief .`. Existing Linux/macOS CI additionally round-trips `brief new` through strict lint. Current HEAD is specifically the 2026-09-09 correction to current Ask session flags, useful evidence for cross-executable integration CI.

### Ply

Owns the action/check loop. `ask.go` invokes external Ask/Brief. `exec.go` owns process groups, timeouts, input/output handling, and the interpreter seam. `receipt.go` and receipt tests bind the candidate, exact execution, binary output, and observed result. `may.go`, `cage.go`, and boundary reliability tests keep approval before action and confinement around actions; controller, model call, and verifier remain separate processes. `AGENTS.md` explicitly rejects provider code, built-in tools, a second log, a daemon, a workspace format, and interpreting toolbox PATH as confinement.

Independent Go module `github.com/patrickyoung/ply`, Go 1.26, **no module dependencies**. Runtime dependencies remain executable names/environment overrides (`ASK`, `BRIEF`, `MAY`, `CAGE`), not Go imports. The standalone `contrib/job` supervisor is an ordinary optional program, not a reason to embed supervision in Ply.

Standalone: `GOWORK=off go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -o /tmp/ply .`. Extra maintained suites: `python3 -m unittest discover -s eval -p 'test_*.py'`, `python3 contrib/job_test.py`, `sh contrib/edit_test.sh`. Go-only root CI would miss these. Live evaluation drivers are explicitly optional and must not be run in a default check.

**Migration-sensitive test:** `ask_integration_test.go:buildContractAsk` defaults to `../ask` and silently skips when that implicit sibling is absent. `PLY_TEST_ASK_SOURCE=/absolute/path/to/ask` turns absence into failure. The root integration gate should always set it, while standalone module CI should remain valid without sibling sources. The current executable contract verifies resume, exact observed effects, compaction ancestry, retained original goal/input, low verbosity, and replay refusal after damage with a loopback provider fixture.

### Hone

Owns the decision that a recorded recovery earned a lesson and the explicit append/scaffold delta to a skill. It does not own retrieval, sessions, YAML parsing, or automatic learning. `session.go` intentionally decodes only the needed fields of another program's file format, rather than importing Ask. `receipt_versions_test.go` protects v1/v2 verifier receipt compatibility and rejects inconsistent interrupted outcomes. `ask.go` runs Ask to word evidence and Brief to lint what is written; `proposal.go`/`recover.go` preserve exact reviewed deltas and provenance. A successful model reply without a program verdict does not qualify as learning.

Independent Go module `github.com/patrickyoung/hone`, Go 1.26, **no module dependencies**. Keep `BRIEF_PATH` as the environment contract and lesson format validation with Brief. A shared Go `event` or `skill` library would remove the independent reader/writer contract and broaden authority.

Standalone: `GOWORK=off go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -o /tmp/hone .`. The real consumer test `TestScaffoldWritesFrontmatterBriefCanRead` skips when Brief is unavailable; integration CI must build and explicitly set `BRIEF` to the imported executable. This review ran that exact test against a newly built Brief and it passed without skipping.

### Cite

An especially strong initial import: the identity filter is small, zero-dependency, and its proof has a precise limit. `main.go` takes a regular evidence file plus candidate stdin, buffers completely, and only writes the original bytes after validation. `evidence.go` checks Context v1 records and rejects a conflicting ref-to-URL mapping. `check.go` requires at least one exact `[ctx:...](citation.url)` occurrence and rejects other `ctx:` references. `main_test.go` verifies byte-for-byte pass-through, empty rejection output, invalid evidence, and bounds. No Context/Ask/Ply library is imported or executable required.

Independent Go module `github.com/patrickyoung/cite`, Go 1.26, **no module dependencies**. Preserve literal identity only; no semantic claim checking, retrieval, repair, or rendering belongs here. Cite's 0/1/2 status has different semantics from Ask/Ply despite using the same numeric values.

Standalone: `GOWORK=off go test ./...`, `go test -race ./...`, `go vet ./...`, `go build -o /tmp/cite .`. Unlike Ask/Brief/Ply/Hone it has no existing `.github` workflow; root CI should provide equivalent independent checks without moving its source or widening scope.

### Draft

A shell composition worth retaining, with `DESIGN.md` as the reviewable agreement and the extracted shell check handed to Ply. The current HEAD refreshed the tool reference for current Ask/Ply contracts. `bin/draft` resolves its own symlinks to find `../skills/draft`, so copying just the launcher would break it. Keep `bin/`, skill, template, and generated reference together. The sibling `just install` recipe builds `../ask`, `../brief`, `../ply`, `../hone`; a sibling `tools/` layout preserves that assumption. Do not replace this composition with a framework or move its skill execution policy into Brief.

Tests are an external shell script with temporary projects, fake May/Cage/Ply, and mutation-verifier checks. The script inspects all stdout/stderr/status channels and tests symlinked launchers. No model is called by these checks. `draft prove` intentionally mutates the requested target while measuring its checker, so default migration checks should run its test suite against temporary fixtures, not invoke prove on arbitrary source repositories.

Standalone prerequisites are the four executables and Python 3/Unix tools. Documented sequence is `draft sync`, `sh -n bin/draft`, `sh bin/draft_test.sh`, `brief lint -strict skills/draft`. **Generated-reference caveat:** `draft check` regenerates all installed tools' help/version output, including optional Cage/May/Vouch/Web, and compares it with tracked `skills/draft/references/tools.md`. Different installed tools therefore legitimately change the expected bytes. The first archived-source test run in this review found 49 passes/29 failures cascading from reference drift. Root CI should select exact executables, copy the tracked Draft tree to scratch, sync that scratch reference, and run its behavioral suite there; separately inspect any committed reference update. Do not silently rewrite the imported source as a side effect of checking it.

### Vouch

A distinct credential filter, not obsolete OAuth duplication. `main.go` invokes donor programs and exposes credentials either as direct output or only to a credential-aware child; `device.go` owns device flow, scope accounting, refresh/rotation; `store.go` owns private grant files. Import, device, static, and saved-browser-session grants have explicitly different scope guarantees. OAuth's auth-code/PKCE and inherited header descriptor should remain another executable.

Independent Go module `github.com/patrickyoung/vouch`, Go 1.26, **no module dependencies**. `bin/check` is a strong external 208-assertion contract suite, including malicious/noisy donors, process deadlines, scope refusal before provider contact, loopback device-flow/rotation, child environment, connector credential boundaries, and unchanged checkout. Tests live outside the binary; `go test ./...` alone would provide no behavioral coverage.

Standalone: `GOWORK=off GOPROXY=off GOTOOLCHAIN=local sh bin/check`, plus `go vet ./...`, `go build -o /tmp/vouch .`. Suite requires Python 3, curl, Unix shell and uses only a temporary loopback fixture server. Preserve executable fixture bits. Service config is source; grants and audit records are not.

Recommend later, not rejection: no `AGENTS.md`, license file, or CI workflow is tracked, and credentials deserve an explicit maintained boundary comparable to OAuth's before a common release/policy is implied. The existing source does demonstrate enough discipline to be a later separate module. Do not infer that one repository means credential stores, headers, and child environments should be unified.

### Web

A Python 3 executable plus fixtures, with lazy Playwright import, checkout-relative private `.venv`, and checkout-relative default operation state. `bin/web` owns HTML reduction and browser operations; there is no model. It maintains useful get/text/html/links/shot interfaces and explicit plan transcripts. The offline suite covers reduction, content-as-data, attach refusal/deadline and fake-browser gates. It passes here, but no real browser behavior was exercised.

**Current-family incompatibility:** `in_job()` requires `CLERK_WORKER`, `JOB_ID`, `CLERK_WROOT`; `gate()` invokes `may DESCRIPTION`; `grant_digest()` hashes `worker/job/description`. Current standalone May reads action bytes from stdin with job argv, and Ply uses May's strict `request` action envelope. A fake May passing Web's suite does not demonstrate compatibility with current May, and no such compatibility should be claimed. This deserves a separately designed adapter/contract, not an opportunistic monorepo rewrite.

No packaging lock/requirements file, license, `AGENTS.md`, or CI is tracked. `web setup` installs/upgrades Playwright and browser assets from the network, and state stays under the checkout unless `WEB_STATE` is set. Importing `.venv`, profiles, screenshots, or state would be wrong. `bin/web check` is the standalone offline check; actual render behavior additionally requires Playwright/Chromium. Keep separate until the gate seam and reproducible packaging have an explicit decision. Its browser identity, retained tabs, and local terminal authorization must remain its own concerns even if imported later.

## Verified checks

Host: macOS arm64, `go1.26.1`, system Python 3 and mandoc available. Go commands used `ASK_LIVE=`, `GOWORK=off`, `GOPROXY=off`, `GOTOOLCHAIN=local`; cached dependencies sufficed. Fresh race runs used `-count=1`. Ordinary Go tests passed for Ask/Brief/Ply/Hone/Cite. Fresh race tests, vet, and builds passed for all five (Ply race suite took 80.4 seconds). Vouch vet/build and its 208-case offline suite passed. Web's full offline check passed. Ply's extra suites passed: 27 evaluation unit tests, 11 job tests, and 46 edit checks. Hone's real Brief compatibility test passed with an explicitly rebuilt Brief.

Temporary review artifacts and binaries are under `/tmp/bench-filter-review.snDtR8`; no installed binary was replaced. Draft's unsynced scratch run failed on reference drift as described above. After syncing only the scratch copy against the freshly built Ask/Brief/Ply/Hone executables, its suite passed **78 checks**, strict skill lint reported **zero errors/warnings**, and shell syntax passed. Explicitly setting `PLY_TEST_ASK_SOURCE` and rerunning `TestAskPlyContract` fresh passed all eight cases, including streamed incomplete-response handling and refusal to advance a checkpoint to a corrupt compaction result. These checks used no model service.

## Boundary enforcement to carry into the monorepo

1. One tool directory, one executable and own existing module/manifests. Preserve module paths for the local migration; retain per-tool licenses, security docs, AGENTS, history and executable modes. Do not introduce a root Go module or automatic Go workspace. A future published install-path change requires its own deliberate release decision.
2. Reject cross-tool Go imports, local `replace` directives, path dependencies, source inclusion, and symlinks escaping the component; provider SDKs stay within Ask. Check builds/tests with `GOWORK=off`, including a copy of each module isolated from siblings. A single root `go test ./...` is insufficient for nested modules.
3. Keep output, exit status, environment variables, inherited descriptors, and file schemas as public seams. Maintain tool-specific exit meanings. Coordinated changes should exercise real executables and deliberately version receipts; do not share internal parsers to make both sides agree automatically.
4. Root integration should require real Ask/Ply replay and real Hone/Brief round-trip, and include Context/Cite filtering when Context is selected. Set executable/source paths explicitly so ambient installed tools and optional skips cannot make CI falsely green.
5. Run Linux/macOS independent CI where existing projects claim both, plus shell/Python auxiliary checks. Keep live provider/evaluation/browser setup opt-in. Root tests must avoid credentials and source-local runtime state.
6. Root conveniences are explicit build/test/install orchestration. Preserve every command's own help, manual, installability, and ability to function in an isolated checkout. Build sibling executables into a temporary bin directory; do not silently replace user installations or global skill links.
