# Build a small Go tool from development evidence

This caller-owned Improve example prepares explicit editable Go source slots,
asks public Hire for one bounded proposal, and builds/tests/runs that proposal
on fresh frozen cases. It does not change Improve, add a standing team, install
a tool, or promote source. New executable code is Go using the standard library.

Build from the Improve module:

```
go build -o /tmp/tool-building ./examples/tool-building
```

`prepare -source DIR -out NEW -integration AGENTS.md [-slot src/record-summary]`
copies the source without changing the original. The integration file must
already exist. The new tool directory contains dormant `main.go`, `main_test.go`
and a fixed `go.mod`. The manifest printed to stdout identifies the prepared
source and three available mutable paths. Improve still edits only existing
files. The preparation is explicit; this is not permission to create arbitrary
paths during an experiment.

Go source lives under `src/`, outside Agent's reserved executable `tools/`
directory. Direct entries in an Agent definition's `tools/` must be executable
files, so do not select a nested Go source directory there. Building or promoting
an executable into a worker remains a separate caller-controlled operation.

`spec -source PREPARED -development CASE -holdout CASE -go GO -cage CAGE
-record RECORD -offline` emits a study using the deterministic fixture proposer.
Replace `-offline` with `-model MODEL -hire HIRE -ask ASK` for a real proposal.
Generating a spec runs no model. Real authoring requires a complete Hire
definition: the prepared source must pass
`hire verify SOURCE`, including README.md and a valid Agent definition. Before
any authoring call, the adapter runs that read-only public check and retains
`hire-preflight.json`; failure stops without a model call or source edits.
The explicit offline fixture can use a minimal integration file plus Go slot.
The real proposer inherits the caller's
`HIRE_AGENT` and `AGENT_*` tool selections and credentials. The real spec pins
the selected public Agent, Ask, Ply, Record, Brief and Cage executable paths,
using the environment selections or their public command defaults. Add any
wrapper scripts, interpreters and further dependencies to the study's
`dependencies`; the selected Go installation must remain fixed too. A binary
hash alone does not pin its compiler, GOROOT or external runtime installation.

Each case is caller-owned JSON:

```json
{"stdin":"[{\"status\":\"done\"},{\"status\":\"blocked\"},{\"status\":\"done\"}]","expected_stdout":"{\"blocked\":1,\"done\":2}\n","expected_exit":0}
```

Use an independently chosen holdout, for example an empty array expecting
`{}\n`, or different status values/counts. Case policy is exact stdout bytes
and exit status; the adapter imposes no application protocol on a real Go tool.
The fixture alone implements status counts. Then execute the public pipeline:

```
/tmp/tool-building spec -source /absolute/prepared \
  -development /absolute/development.json -holdout /absolute/holdout.json \
  -go /absolute/go -cage /absolute/cage -record /absolute/record -offline \
  > /absolute/spec.json
improve -n < /absolute/spec.json
improve -o /absolute/new-study < /absolute/spec.json
improve verify /absolute/new-study
```

`propose`, `trial`, and `judge` consume Improve v1 JSON on stdin. The settings
are `slot`, `integration`, `tools` (go/cage plus hire/ask for real authoring),
`proposer_model`, `effort`, and explicit `fixture`. A researcher can narrow
`mutable` to a nonempty subset of the three prepared paths; the fixture requires
main.go and integration. Proposal validation preserves the complete file
inventory and modes and every unselected byte. Hire receives only development
cases, observations and supplied research. It writes one hypothesis and exact
replacement texts. Failed authoring produces no changes; unknown Ask replay
cost remains null. No provider events are manufactured.

Compilation uses a fresh private copy/cache with `GOTOOLCHAIN=local`,
`GOWORK=off`, `GOPROXY=off`, `GOSUMDB=off`, `GOENV=off`, empty `GOFLAGS`, and
`CGO_ENABLED=0`. The module is fixed; nonstandard dependencies are refused.
Proposing runs `go build` and `go test -c`, which compile but do not execute
authored tests. A trial executes the compiled tests and tool through the
selected public Cage, with networking denied, only scratch writable, source
and compiled executables protected. Cage is not a read-secrecy boundary. Do
not run this example with sensitive host-readable material available to code
you do not trust. OS boundary refusal and unknown process termination are
adapter errors, never a measured capability gap.

Each trial retains build argv, statuses, bounded stdout/stderr, durations,
source hashes/modes, and the exact executable/hash. Fresh Go caches are removed.
Build, test and execution durations are separate; no warmed-cache or model
savings claim is made. Deterministic trials report known model cost zero.
The example judge requires every candidate case correct, every baseline and
candidate build/test successful, and a baseline behavioral failure. This is
a narrow capability demonstration. It does not prove that a worker adopted the
integration instructions, that the tool saves money, or that an arbitrary
deployment is safe. Callers may select a different frozen judge before starting.

Offline unit checks: `go test ./examples/tool-building`. The opt-in monorepo
integration test also requires Python 3.9+, the experiment-researcher source,
and independently built Improve, Record, Ask, Cage, Trail, Hire, Agent and Brief
executables:

```
IMPROVE_TOOL_TEST_BIN=/absolute/bin go test ./examples/tool-building -run TestPublicImprove -v
```

It verifies prepared source through public Hire/Agent, checks report-v2 assembly
through the actual public readers, then compiled
trials, acceptance, held-out rejection, exact export, replay and tamper refusal.
The proposer is a deterministic fixture; no test selects a paid model by default.
