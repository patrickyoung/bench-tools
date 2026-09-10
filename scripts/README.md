# Verification

From the repository root:

```sh
scripts/check                              # all standalone checks, then public-process integration
scripts/check --component ask              # one independent source export
scripts/check --component agent --component weave
scripts/check --standalone                 # all leaves, no composition stage
scripts/check --integration-only           # independent builds, then composition fixtures
scripts/check --component cage --native-cage
python3 -m unittest discover -s scripts/tests -v
```

Go 1.26, Python 3.9+, Git, a C compiler, Make, sh, Bash, and jq must be available.
The runner reports missing prerequisites before running component checks. Go
may fetch pinned module dependencies; set `GOPROXY=off GOSUMDB=off` when the
local module cache is complete and network fetching should be prohibited.
Fixtures use local processes and loopback services, with no paid model calls.

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
checks also run. Agent uses its fake-public-binary shell suite. Draft builds
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

The GitHub workflow derives its component matrix from `components.json` and
checks every leaf on Linux and macOS. It also runs the architecture mutation
tests, public-process integration, and native Cage proof in explicit jobs.
