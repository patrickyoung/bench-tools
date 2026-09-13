# Build, install, and verification

These scripts maintain the monorepo. They build independent programs, check
their public interfaces together, and install selected packages. Digital
workers run through Agent; the build harness is not their runtime.

From the repository root:

```sh
scripts/build                              # all 23 commands under .build/bin
scripts/build ask ply                      # only selected components
scripts/install                           # build and install into ~/.local
scripts/install --from-build .build --prefix /tmp/bench-preview
scripts/uninstall --prefix /tmp/bench-preview
scripts/check --quick                      # everyday checks, no integration/race/supplemental suites
scripts/check                              # all standalone checks, then public-process integration
scripts/check --component ask              # one independent source export
scripts/check --component agent --component weave
scripts/check --standalone                 # all leaves, no composition stage
scripts/check --integration-only           # independent builds, then composition fixtures
scripts/check --component cage --native-cage
python3 -m unittest discover -s scripts/tests -v
python3 scripts/check-docs.py              # local guide links and anchors
python3 scripts/check-examples.py --bin-dir .build/bin  # copied starters, local fixtures
python3 scripts/check-harnesses.py --bin-dir .build/bin # portable skill + real MCP calls
python3 scripts/check-harnesses.py --host-clis          # optional installed Codex/Claude/Pi discovery
```

Builds require Go 1.26+, Python 3.9+, Git, and sh. The test runner selects its
prerequisites for the requested plan: Perl for Draft shell tests, a C compiler
for race checks, and Make plus `install` for Weave release checks. jq is not required. Standard Unix
utilities must be available. `scripts/check --requirements` lists prerequisites;
combine it with component selection or `--quick` to inspect that plan. The
runner reports missing commands before running component checks. Go
may fetch pinned module dependencies; set `GOPROXY=off GOSUMDB=off` when the
local module cache is complete and network fetching should be prohibited.
Fixtures use local processes and loopback services, with no paid model calls.

`--quick` retains isolated source exports, before/after boundary guards,
ordinary uncached Go tests (including Go's built-in vet checks), built command
versions, and Draft's shell checks. It skips race, the separate
`go vet` pass, May self-check, Cage cross-build/native proof, Ply supplemental
suites, Weave Python/smoke tests, and public-process integration. It cannot be
combined with `--native-cage` or `--integration-only`. If a C compiler is absent,
ordinary checks explicitly use `CGO_ENABLED=0` and record that choice. No current
component requires CGo. `--verbose` includes full command lines; default progress
prints one result per check. Detailed command output always goes to `.checks/`.

Build output uses `.build/tools/NAME` for each tool's own payload and receipt,
with relative command links under `.build/bin`. The source and file checksums in
`package.json` identify the actual working-tree bytes and resulting artifacts;
they are local integrity records, not release signatures or a suite version.
Builds compile with `-mod=readonly -trimpath -buildvcs=false` and do not rewrite
module files. A failed build leaves previous published packages available.
Rebuilding replaces generated output. Use `--output DIR` for an external build
directory; inside the checkout, only `.build` is accepted.

The installer verifies each package before copying it into
`PREFIX/lib/bench-tools/NAME`, exposing only public commands through relative
links in `PREFIX/bin`. It refuses unrelated existing paths and changed owned
files, uses a lock and staged replacement with rollback on ordinary errors,
and retains an ownership receipt for repeat installation and removal. Do not
run installs while those programs are active. `--from-build DIR` needs only
Python and completed packages; normal installation first runs the builder.
Default installation is user-owned under `~/.local`; no privileged setup or
shell startup edits occur automatically. Runtime dependencies remain separate
programs on PATH, and provider configuration remains each tool's responsibility.

Draft's `skills/draft/references/tools.md` is a declared generated file. Its
contents may change through `draft sync`; updates preserve it along with any
more restrictive permissions from your umask, and uninstall accepts those
changes. Other assets and executable modes remain
verified. After updating companions, run `draft sync` and put the packaged skill
on `BRIEF_PATH` as printed by the installer. Agent is a native runner; Hire
owns expert authoring. The existing manuals and top-level documentation remain
inside each package (for example, `man ~/.local/lib/bench-tools/ask/ask.1`).

`python3 scripts/check-install.py` tests all built commands and their help/version
interfaces and moves an installation to a path with spaces. It exercises Hire
scaffolding, Agent inspection, and Draft's reference/template/Brief lookup
without a model, then repeats installation and uninstalls while preserving an
unrelated file. It uses
a disposable prefix and never installs into your actual home.

Each component is copied into a separate temporary directory without any
sibling checkout. The export uses the current contents of tracked files and
nonignored new files, including working-tree edits and deletions. Ignored
runtime data and binaries are omitted. Ignored source files cause a failure;
components with `go:embed` also refuse ignored files that could otherwise alter
their embedded build inputs. This prevents a source-only export from silently
testing a different program than the current leaf build.

Every Go command is built from that export with `GOWORK=off`, followed by
uncached ordinary and race tests, vet, and a built version check. Ply's offline
evaluation, job, and edit suites and Weave's pure Python and built-binary smoke
checks also run. Agent and Hire have independent Go tests; their preserved
fake-public-binary shell suite runs in process integration. Draft builds
its named public dependencies independently, refreshes its generated reference
only in the scratch copy, then runs its shell suite and Brief lint. Its tracked
reference is not rewritten or represented as freshly synchronized.

Ply's native-adapter rejection tests receive explicit Claude/Pi executable
sentinels that exit 125 if invoked. This satisfies executable nomination before
the tests reject unsafe fixture auth paths; it cannot discover a real installed
model CLI. Execution tests supply their own local fixture programs, and no
personal authentication directory is passed to either path.

The process integration stage receives the exact built binaries through
`scripts/check-integration.py --bin-dir DIR`. Its fixtures verify the command,
stream, receipt, and status boundaries separately from standalone compilation.
They do not turn an optional live-provider test into a mandatory model call.

Each export gets a private home, temporary directory, and environment that
omits personal credentials, tool overrides, configuration paths, and Go
workspace settings. The runner exposes ordinary system tools and explicitly
selected Bench binaries where composition is required. This is test setup,
not a kernel sandbox or a guarantee that arbitrary future tests cannot access
the host. May's safe built-in self-check runs; the runner never creates an
ordinary real approval request against the user's approval state.

Darwin Draft tests use the actual native `mktemp -d` root, because that utility
prefers the OS user temporary directory over `TMPDIR`. Draft's trusted source
fixture is exported beneath `/private/tmp`, outside that write root. The runner
probes and verifies that separation and records it in the log directory. This
preserves the fixture's real safe-versus-unsafe verifier distinction without
wrapping mktemp or altering Draft's admission check.

Cage cross-compilation is part of its independent check. Native confinement
is an additional explicit requirement selected by `--native-cage`; it fails
when the host cannot prove the boundary. CI keeps native proof in a separate
Linux/macOS job and installs Bubblewrap for Linux. A passing portable suite
does not claim native confinement when that proof has not run.

Full command output, exit status, timing, source digests, and the overall
`running` / `complete` / `failed` status are retained under `.checks/<run>/`.
Temporary source/home directories are removed when the runner exits. The
architecture guard runs before and after verification. `scripts/verify-imports`
separately checks original migration provenance; an ordinary future source
change need not match the historical import byte for byte.

The GitHub workflow also runs the build/install smoke on Linux and macOS. It derives its component matrix from `components.json` and
checks every leaf on Linux and macOS. It also runs the architecture mutation
tests, public-process integration, and native Cage proof in explicit jobs.
The entry-guide link check runs before the matrix. After building packages,
the runnable starters are copied into temporary directories and exercised
against actual commands and local model fixtures, including failure paths.

`check-examples.py` covers four starters: meeting brief, evidence answer,
signup audit, and the support-reply expert folder. It needs Ask, Brief, Context,
Cite, Tend, Agent, Hire, Ply, and Cage. The support case checks two independent
workspaces, unchanged expert bytes, rejected/corrected citations, replay, and
zero-model re-entry. `--native-cage` retains Agent's default boundary instead
of the host boundary explicitly selected for the portable fixture.

`check-docs.py` follows local destinations and heading anchors across root
guides, all tool READMEs, field guides, and selected integration references.
It does not execute examples or verify remote pages; those need separate
evidence. Keep setup, expected output, and the scope of each check beside the
commands a reader will run.

`check-harnesses.py` uses Brief to lint the canonical Bench skill and a relocated
plugin ZIP, then performs actual MCP discovery and tool calls over stdio. Both
the modern lifecycle and explicit `mcpserve -allow-legacy` compatibility are
checked; the default must still refuse legacy initialization. This check runs
inside public-process integration on Linux and macOS. `make check-harnesses`
builds its two required components and runs it on its own.

The optional `--host-clis` path also requires installed Codex, Claude Code, and
Pi. It validates and discovers the extracted Claude plugin, connects Claude to MCP, asks a
fresh Codex app-server to discover the installed skill and MCP tool, and asks
a fresh Pi RPC session to discover its installed package. All configuration
is in temporary homes; no model turn or personal profile change is requested.
These host checks are separate from CI because those CLIs are optional external
dependencies. See the [harness verification record](../docs/HARNESS-VERIFICATION.md)
for observed versions and the limits of that evidence.

## Worker source library

`python3 scripts/workers list --all` reads catalog metadata; `check` inspects
source and approved export inventories. `export ID DEST --ref FULL_COMMIT`
uses Git to write only committed, explicitly approved source into a new folder
with `team.lock.json`. Experimental entries need `--allow-experimental`;
deprecated and retired entries cannot be exported at that selected revision.
This is a repository utility, not an installed Bench command or runtime. See
[the library guide](../docs/WORKER-LIBRARY.md) for content exclusions, source
pins, authoring, checks and lifecycle policy.
