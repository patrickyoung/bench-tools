# Bench, Context, Guide, and Pack review

Reviewed 2026-09-09 from the local working trees. Exact source commits, tree IDs,
and initial status for all 26 project directories are in `inventory.json`.
There are 24 Git repositories, plus unversioned Pack and Studio projects.
Hidden `.agent-interface`, `.container-build`, and `.readme-refresh/worktrees`
checkouts are worktrees of Bench/Ask; `.readme-refresh/baselines/hire` and the
live-eval Draft clone are snapshots of already inventoried projects. Generated
build, session, review, application-state, and evaluation directories are not
additional products and must not be imported as source.

## Bench — worthy application, defer import

`bench` at `983dd31e374f4705e786ad40cb6a67061af9cb6e` is clean. Its
AGENTS.md correctly separates conversation UI from persistent workers and
requires subprocesses for Ask/Ply/Brief. Actual imports are of Bench's own
packages plus TUI dependencies; the filter internals are not imported.
`internal/{askexec,plyexec,briefexec,agentexec,filterexec}` implement the process
edges. `go test ./...` passed all 20 packages on macOS/arm64 (bench-test.log).
Those tests use local executable fixtures and cover literal argv, stream and
exit forwarding, explicit admission, sessions, and the TUI state machine.

Its packaging is a concrete reason to defer moving the application. In
`cmd/benchpack/main.go:637`, `resolve` asks Git for each component directory's
HEAD, requires equality with a manifest pin, and checks the whole worktree for
cleanliness. In a monorepo every component would report the same HEAD. Most
suite 0.13.0 pins also deliberately differ from current sibling HEADs. A move
must not silently treat current source HEADs as a tested suite release or
relax exact revision/dirty checks. The manifest supports a repository and
revision per component but no subdirectory source field. The existing
`-fetch` route and original repos remain valid; preserve them until packaging
can name and verify a monorepo commit plus component tree/subdirectory.

`PACKAGING.md` is already the correct architecture: separate component and
suite versions, independent programs, exact tested composition, relocatable
bundle, explicit caller overrides, no credentials/state in releases. Preserve
it rather than replace it with a shared module, launcher, or root runtime.
Bench also exposes broker packages consumed by Studio; importing that
application pair prematurely would make the wrong dependency look endorsed.

## Context — import as an independent filter

`context` at `8133264852426395dc26aae34231502e9501a983` is clean, MIT,
Go 1.26, standard library only. `main.go` invokes the explicitly named source
executable; `connectors.go` performs PATH-like executable discovery without
provider selection; `record.go` validates complete JSONL, preserves unknown
fields/structured content, and stamps query and executable digest. No model,
session writer, provider SDK, policy router, daemon, or cross-tool Go import.
The retrieval stamp is provenance, not trust; Ask owns conversation history
and Cite owns citation identity validation. This boundary is worth retaining.

`GOWORK=off go test ./...`, `go test -race ./...`, and `go vet ./...` all passed.
The tests cover exact stdin, shadowing, exit 1 as ordinary no result, atomic
rejection of broken output, source identity, record limits, merge conflicts,
help/manpage contracts, and Wikipedia through local HTTP fixtures. Standalone
build: `GOWORK=off go build -o /tmp/context .`. Keep its executable connectors,
protocol documents, manual, examples, and AGENTS.md together without converting
connectors to imported code. No sibling source is needed.

## Guide — keep as a separately maintained learning collection for now

`guide` at `14a62ca913d7e4293823cb35941d4c6628d8c029` is clean. It teaches
pipes well and includes separate shell programs `shape`, `bound`, and `snag`,
fixture-driven chapters, and application compositions. `./bin/check-docs`
passed 9 checks, including links staying within the checkout, shell pitfalls,
and a real test that an open stdin cannot hang the check runner. This is not a
claim that the full guide or current model behavior has been verified.

The README explicitly admits drift in `bin/doctor` and chapter references:
`ask auth`/`ask login` are gone from current Ask; the doctor still prints a
v0.5.0 suite label and recommends removed login commands. The guide also
installs Cage v0.1.0 independently, so its installation recipes must not become
the monorepo's central installer. Preserve as education, update the stale
contracts and deterministic full-guide tool selection before later import
under examples/docs. Do not turn its helper programs into a shared runtime
that every filter must use.

## Pack — retain as experiment; do not mix source with local run state

`pack` is not a Git repository and has no checked-in license record. It has
source programs/cards/schema plus `.ask`, answers, packets, derived packs,
and a fixture database in the same directory. No safe history-based whole-tree
import exists. A source-only staged copy of bin/skills/schema/fixtures/suite
passed all 38 offline assertions in `bin/check` (pack-test.log). Tests run with
canned Ask, isolated PACK_HOME, and read-only fixture sources. No live calls.

The useful idea is measurable retrieval with per-source reduction and replay
from packets. Its protocol is nevertheless different from Context/Cite:
`id/src/ref/text`, content-addressed text IDs, model-driven `route` and `draw`,
and Pack-local citation checks. Context explicitly refuses source selection
and synthesis. Folding Pack into Context or changing either envelope merely
to consolidate repos would violate those tools' purposes. Preserve the
experiment separately until its intended composition, source inventory,
licensing, and version history are explicit. A future import belongs as its
own program/example, not as a Context subsystem.
