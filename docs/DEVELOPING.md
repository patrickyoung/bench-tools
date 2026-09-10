# Develop and verify

[Installation](INSTALL.md) · [Home](../README.md)

Read the root [AGENTS.md](../AGENTS.md) and the affected component's guidance.
Each tool must remain independently buildable, testable, usable, and versioned.
Root scripts coordinate builds and checks; they supply no runtime authority.

## Choose the right check

Run from the checkout root:

```sh
make test                              # everyday checks for every component
make test TOOLS="ask ply"               # everyday checks for selected components
make check                             # harness tests + full component/integration gate
python3 scripts/check --native-cage     # full tool gate plus native confinement proof
make check-docs                        # entry-guide links and heading anchors
make check-examples                    # runnable starters against local fixtures
```

`make test` uses independent source exports, uncached ordinary Go tests and
built-command checks, plus Agent/Draft shell checks. It skips race, the separate
vet pass, supplementary example suites, process integration, and native Cage
proof. `make check` includes race, vet, supplementary suites, and integration
with local model fixtures. Neither makes paid model calls. `make check-examples`
builds the five starter components and runs copies of the examples in temporary
directories, checking success and failure behavior without provider credentials.

Before publishing, also exercise the complete installation lifecycle:

```sh
make build
python3 scripts/check-install.py
python3 scripts/check-examples.py --bin-dir .build/bin
```

CI runs isolated component checks, process integration, native Cage proof,
installation checks, and starters on both Linux and macOS. Its results are
attached to the exact source commit in the [public workflow](https://github.com/patrickyoung/bench-tools/actions/workflows/check.yml).

Builds need Go 1.26+, Python 3.9+, Git, and a Unix shell. Draft shell tests need
Perl; race checks need a C compiler; Weave release tests need Make and `install`.
Inspect the exact prerequisites for your selection before a long run:

```sh
python3 scripts/check --requirements
python3 scripts/check --quick --component ask --requirements
```

Make is optional for root coordination: `python3 scripts/build`,
`python3 scripts/check --quick`, and `python3 scripts/install` are the direct
interfaces. For the equivalent of `make check`, run the harness tests before
the full checker:

```sh
python3 -m unittest discover -s scripts/tests -v
python3 scripts/check
```

## Understand the evidence

Builds and checks export each component into a temporary source directory
without siblings, including current working-tree changes. Go workspaces are
disabled. Checks retain logs and exact tested-source inventories in `.checks/`;
check failures print their final output and log location. Builds print their
output directly and keep source/file receipts in `.build/tools/NAME/package.json`.

Local model fixtures test transport, streams, and composition. They cannot
establish a real model's ability to solve a task. Native Cage proof is also
separate: a portable test pass does not demonstrate confinement on this host.

Go may download pinned modules that are absent from cache. Set
`GOPROXY=off GOSUMDB=off` when the cache is complete and downloads should be
prohibited. The [runner reference](../scripts/README.md) describes isolation,
check modes, dependencies, and platform limits.

## Work on one independent tool

From the repository root:

```sh
cd tools/ply
GOWORK=off go build -o /tmp/ply .
GOWORK=off go test ./...
/tmp/ply help
```

MCP and OAuth use their documented `cmd/` packages. Agent and Draft are shell
programs with adjacent private assets; keep their directory layouts intact.

To extract one clean **committed** tool without siblings, return to the root:

```sh
python3 scripts/export-tool ply /tmp/standalone-ply
cd /tmp/standalone-ply
git init
GOWORK=off go build .
```

Choose a destination that does not already exist. This committed export differs
from the build/check runner's export of working-tree changes. A real separate
repository also gives Rules a narrow repository boundary; do not create fake
nested `.git` markers in this repository.

## Keep history and architecture distinct

`components.json` records original source commit/tree IDs, module paths, and
entry points. `scripts/verify-imports` audits the initial exact import and
source ancestry. Intentional later changes can differ from that baseline.
`python3 scripts/check-boundaries.py` checks the current architecture; add
`--baseline` to compare component bytes and modes with the original source trees.

The [decision](DECISION.md) and [verification](VERIFICATION.md) are historical
migration records. Current check output is the evidence for current changes.
Public-contract changes need the affected components' own checks and explicit
executable integration checks; an aggregate build alone does not prove them.
