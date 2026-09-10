# Bench tools

Independent Unix programs, maintained together. Each directory under `tools/`
keeps its own module or script, public commands, documentation, license, tests,
and release identity. There is no root runtime or shared Go module.

The first migration includes **17 tools, 15 independent Go modules, and 20
commands**, selected after reviewing all 26 Bench project directories.
[The review and decision](docs/DECISION.md) explain every inclusion and deferral.

| Responsibility | Programs |
| --- | --- |
| Model, procedure, work loop, learning | [Ask](tools/ask/README.md), [Brief](tools/brief/README.md), [Ply](tools/ply/README.md), [Hone](tools/hone/README.md) |
| Evidence and its identity | [Context](tools/context/README.md), [Cite](tools/cite/README.md), [Trail](tools/trail/README.md) |
| Effects, human decisions, confinement | [Action](tools/action/README.md), [May](tools/may/README.md), [Cage](tools/cage/README.md) |
| External protocols and credentials | [MCP](tools/mcp/README.md), [OAuth](tools/oauth/README.md) |
| Instructions, design, worker composition | [Rules](tools/rules/README.md), [Draft](tools/draft/README.md), [Agent](tools/agent/README.md) |
| Process durability and graph readiness | [Tend](tools/tend/README.md), [Weave](tools/weave/README.md) |

Build an individual tool from its directory with Go 1.26 or newer:

```sh
cd tools/ply
GOWORK=off go build -o /tmp/ply .
GOWORK=off go test ./...
/tmp/ply help
```

MCP and OAuth use their documented `cmd/` packages. Agent and Draft remain
shell programs with adjacent private assets; preserve their complete directory
layout. Runtime companions are separate executables selected through the
existing public command flags/environment, exactly as each tool documents.

To extract one clean committed tool without any siblings:

```sh
./scripts/export-tool ply /tmp/standalone-ply
cd /tmp/standalone-ply
git init
GOWORK=off go build .
```

That also gives Rules a separate real repository root when narrow instruction
scope is needed. No fake nested `.git` markers are used inside this repository.

`components.json` records original commit/tree IDs, module paths, and command
entry points. `./scripts/verify-imports` proves the initial exact import and
source ancestry; it is an import audit, so future intentional tool changes
will rightly differ from that original baseline. The ongoing architecture
check is `python3 scripts/check-boundaries.py`. Add `--baseline` to compare all
current component file bytes and modes against the original source trees.

Run the complete local gate with Go, Python 3.9+, Git, a C compiler, Make, Bash,
and `jq` available:

```sh
./scripts/check --native-cage
```

Each tool is copied to its own temporary source directory without siblings,
then built and checked with `GOWORK=off`. Go tests run uncached, including race
checks and vet; shell/example checks and explicit executable integrations run
separately. Model transport uses local fixtures. Logs and exact tested-source
inventories go to `.checks/`; source files and personal installations stay
unchanged. Dependencies may download through Go if absent from cache; use
`GOPROXY=off GOSUMDB=off` when all dependencies are already available offline.

For a narrower development check use `./scripts/check --component ply`;
`--standalone` runs all independent checks, and `--integration-only` builds
the commands and exercises their process contracts. Native Cage proof is
explicit and fails if the host cannot supply confinement. Linux needs its
Bubblewrap backend. The prospective CI workflow covers Linux and macOS;
local results and their limits are recorded in [verification](docs/VERIFICATION.md).

This local source migration is not a new Bench suite release. Existing
installation instructions in the component READMEs continue to refer to their
original repositories. Bench/Hire retain their exact tested suite pins and
release paths while monorepo-aware packaging is developed. No remote,
published release, installed binary, worker state, or credential was changed.
Each component's original license remains in its own directory.
