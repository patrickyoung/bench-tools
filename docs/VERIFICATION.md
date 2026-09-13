# Migration verification

This is the historical September 9, 2026 migration gate. For the current source,
see the [public CI runs](https://github.com/patrickyoung/bench-tools/actions/workflows/check.yml)
and [verification commands](DEVELOPING.md); publication is described in
[Source and releases](RELEASES.md).

**Passed:** the complete local gate exited 0 with all 104 recorded verification
steps successful. The [summary](review/gate/summary.json), per-command logs,
and exact tested-source inventories are preserved in `docs/review/gate/`.
All 17 source inventories were independently compared with their original Git
trees and with the current file bytes/modes after the run.

The canonical local command is:

```sh
GOPROXY=off GOSUMDB=off ./scripts/check --native-cage
```

This validates the current source with cached dependencies and no public
network fetch or paid model calls. The host is macOS/arm64 with Go 1.26.1.
Linux/macOS CI is configured in `.github/workflows/check.yml` but has not been
run remotely; this is a local repository with no remote configured.

## Source and architecture evidence

- `components.json` covers 17 independent tools, 15 Go modules, and 20 commands.
  The reviewed inventory covers all 24 top-level Git repos plus Pack/Studio.
- `scripts/verify-imports` confirms the original commit is an ancestor of the
  monorepo HEAD and the imported subtree matches its original Git tree.
- `scripts/check-boundaries.py --baseline` additionally hashes every current
  component file and checks executable modes, including untracked/ignored
  files. All 17 initial source trees remain exact.
- Four original tags are retained as identical Git objects under
  `upstream/TOOL/TAG`; `docs/review/source-tags.json` records them. Git's full
  object/ancestry check (`git fsck --full --no-reflogs`) passes.
- A standalone Rules export was initialized as its own Git repository. Its
  resulting tree exactly matched the original, and exporting over an existing
  destination was refused. Export carries source, assets, and modes without
  sibling directories or Git metadata.
- All 24 original repositories retain their reviewed HEADs and clean tracked/
  untracked status. See `docs/review/originals-unchanged.json`.
- Thirty-one verification-harness tests pass, including mutation tests that try
  cross-tool imports (also under other-platform build tags), local module
  replacements, workspace/shared-module additions, command collisions,
  cross-tool instruction symlinks, ignored build inputs, and source-export
  omission. See `docs/review/harness-tests.log`.

The guard covers all reviewed Go module identities, including applications
that were deferred. It parses actual Go imports rather than matching comments,
and its ordinary mode permits future independent edits. The optional baseline
mode deliberately continues to describe this initial import, not every future
revision. Neither static checks nor this review proves arbitrary runtime
behavior; the unchanged per-tool contracts and actual process tests matter.

## What the local gate exercises

Every Go tool is built and tested from a distinct temporary source copy without
sibling sources, using `GOWORK=off`, uncached tests, race tests, vet, and each
built command's version path. Exact tested source digests are retained. Agent
runs its 163-assertion public-process fixture suite. Draft runs shell syntax,
a scratch-only reference refresh, its 78 assertions, and strict Brief lint.
Ply's separate evaluation/job/edit suites and Weave's Python checks run too.
May's own isolated self-check runs; no ordinary user approval request is made.
Cage is cross-built for Linux/Windows and proves native macOS confinement with
its actual positive/negative host checks. Cross-builds do not prove Linux
kernel behavior.

Explicit integration verifies required Ask/Ply and Hone/Brief tests cannot
silently skip, built Ply's passing pre-check uses real Ask receipts without a
model, Context/Cite preserve structured evidence and exact candidate bytes,
and rejection/error exits remain distinct. A loopback model fixture lets real
Ask create and seal a session. Action records real Ask receipts, Trail verifies
and reads those records without changing the archive, and tampering is refused.
Weave's complete example suite runs against independently built Ask/Ply/Tend,
including readiness, recovery, refusal, receipt integrity, concurrency, and
interruption. The fixture transport is not a model-quality evaluation.

Action's parked May peer is an explicit strict fixture. Real May's OS-user
state lookup cannot be redirected by setting HOME, so a real parked request
would modify the operator's store. Its isolated built-in checks cover May's
own state machine without doing that. This is a deliberate limit of the
integration evidence, not a claim of real human approval.

## Harness correction discovered by isolation

The first aggregate run reached Draft and reported 76 passes / 2 failures.
Darwin's native no-template `mktemp -d` chose its OS user temporary directory
before the runner's private TMPDIR. The test therefore put its supposedly
unsafe verifier outside the write root it asked Draft to refuse. This was a
fixture-layout mismatch, not a reason to weaken Draft's refusal.

The corrected runner probes the actual native temporary root and places
Draft's trusted source-side verifier outside it. The unsafe fixture then falls
inside the real temporary write root. Both the unchanged 78-assertion Draft
suite and a native-mktemp regression pass. No utility is wrapped and no leaf
source or test is edited. Evidence is retained in
`docs/review/draft-harness-failure.log`, `draft-harness-corrected.log`, and
`draft-temporary-layout.json`. The tracked generated reference remains exact;
synchronization affects the scratch copy only.

The second aggregate run reached Ply's optional evaluation fixtures and
revealed an ambient CLI dependency: its public-auth-root rejection test
nominated neither Claude nor Pi before expecting a credential refusal. That
passed on a developer machine with both installed, but failed under the clean
PATH. The runner now supplies explicit private sentinel executables only to
that fixture suite; both return 125 if invoked. Tests of actual execution
supply their own local fixture programs. The 27 evaluation tests, 11 job tests,
and 46 edit assertions then pass without real CLIs, credentials, or leaf edits.
The added runner regression physically verifies the sentinels fail closed.

## Scope

The detailed 26-project review records relevant failures and limits outside
the initial import as well: Hire's temporary-path fixture issue, Guide setup
drift, Web/Clerk's older May contract, and unversioned/coupled experiments.
A green tool gate does not make those deferred projects a tested suite.
Publishing, changing public module destinations, updating Bench/Hire pins,
and adapting their source packagers remain separate migration steps.

## Agent runner / headless Hire builder

`python3 scripts/check agent hire` builds separate native commands from isolated
component exports and runs each component's Go tests/race/vet. Hire's embedded
home-maintenance and builder-check assets also receive shell syntax checks.
Agent contains no builder or shell action helper.

The public-process suite runs the 163 original runtime/maintenance assertions
against the separate Agent and Hire executables. It exercises Agent -> Brief
-> Ply -> Ask, checked artifacts and repair, portable definitions across
workspaces, child context separation, an admitted local MCP server, unfinished
verdicts, preserved nesting depth, and direct/nested cancellation with replay.
It also proves Hire builds a definition through Agent and another Agent runs
the generated expert independently. Builder verification never executes the
generated verifier with controller authority.

Model transport uses loopback fixtures without user credentials. Native Cage
enforcement remains explicit: `--native-cage` adds definition-write refusal
on a supported backend. The ordinary offline suite explicitly uses host actions.

```sh
python3 scripts/build agent hire
python3 scripts/check-integration.py --bin-dir .build/bin --agent-only --portable --native-cage
```

Packaging/install checks exercise 23 commands, relocation, repeated install and
removal in disposable prefixes. They do not update an existing pinned Hire web
installation; its old authoring calls need an explicit release migration.

## A2A executable boundary

`tools/a2a` has standalone Go/race/vet checks against the official SDK. Root
full integration additionally supplies separately built Tend, OAuth, and A2A
commands to its TLS/ownership/continuation tests. `scripts/check-a2a.py` exercises
the actual listener and client, graceful and abrupt interruption, persistent
unknown outcomes, and A2A -> Tend -> Agent -> Ask/Ply with a local model fixture.
It proves separate workspaces/model contexts, exported checked files, and
independent Ask replay verification. No live model evaluation or remote
production deployment is implied.

For focused iteration after building the needed tools:

```sh
python3 scripts/check a2a
python3 scripts/check-a2a.py --bin-dir .build/bin
```

The component's manual gives environment variables for running its companion
executable cases independently; absent companions produce explicit skips.
