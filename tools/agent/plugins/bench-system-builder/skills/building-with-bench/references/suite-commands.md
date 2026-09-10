# Bench suite 0.13.0 command runbook

This is a versioned public-command reference, not permission to run commands.
Read `compatibility.md` first. Prefer the runtime's own `help`, `version`,
`status`, `show`, and `check` output when it matches the pinned suite.

## Contents

1. Runtime doctor
2. Source-free suite installation
3. Open and admit a task contract
4. Design, build, and prove
5. Materialize the standing Agent home
6. Exercise fixtures
7. Evidence connections
8. Effects
9. Handoff records

## 1. Runtime doctor

Inspect `scripts/doctor.sh`, then run it from the skill directory when shell
execution is available. It is offline and read-only: physical suite-root
resolution, internal checksum verification, public version calls from that one
root, `uname`, and `cage status`. Select `--prefix ABS`, `--suite-dir ABS`,
`--bench ABS`, or no option for PATH. It never reads the environment,
credentials, workspace contents, or network.

The doctor reports `runtime=suite-compatible` after exact byte, command, and
Cage-backend discovery checks; failures remain literal. Compatibility is
necessary but not sufficient for **BUILD-READY**; workspace, controller-evidence
roots, and user authorization must also be valid. **MODEL-READY** is the next
separate gate before any command that can call Ask.

## 2. Source-free suite installation

Installation is a platform-steward action. Inspect the steward skill's
`scripts/install-bench-suite.sh`; run `plan`, obtain approval for its exact
archive/hash/prefix/writes, then run `install --approve` with identical paths.
The source-free input is the exact archive selected from:

`https://github.com/patrickyoung/bench/releases/tag/v0.13.0`

The steward selects the archive from observed `uname -s` and `uname -m`. Its
versioned helper embeds the reviewed SHA-256 for each supported archive,
downloads only the selected archive, verifies it against that pin before
extraction, and installs to an explicit prefix through the verified archive's
installer. The helper does not download a checksum sidecar. Never pipe a
network response to a shell and never resolve independent component `@latest`
versions as if they were a tested suite.

After verification, record **SUITE-INSTALLED**. Review `cage-plan`, then run the
approved `cage-check` on the target host; only its literal 13/13 pass records
**SUITE-READY**. If a nested Claude/Cowork sandbox blocks that kernel-level
test, record **CAGE-HOST-CHECK-REQUIRED** and the exact host-terminal command.
Then establish model access before the first command below that can call Ask. A
Claude login is not an Ask
credential. Record **MODEL-READY** only after the non-secret provider/model and
approved controller-owned access pass the bounded probe in
`model-readiness.md` for this lane. If they do not, preserve
**MODEL-ACCESS-REQUIRED**, the earliest blocked command, its owner, and the exact
sanitized command to resume; do not lose the completed installation. Record
**AGENT-FIRST-RUN** only later, after one bounded `bench home run` has a literal
retained result.

## 3. Open and admit a task contract

The business-friendly path is the interactive workbench in a dedicated
workspace:

```text
bench -C WORKSPACE -m provider/model -n -mode review
```

Start with Ask-only or no tools while negotiating. Inside the workbench:

```text
/status
/tools off
/mode review
/contract
/contract accept
```

Review mode compiles intent into visible deliverables, invariants, evidence,
approval boundaries, assumptions, questions, and limits. Work begins only after
the user accepts the exact proposal.

The headless seam is available for an integration:

```text
bench contract draft -m provider/model OUTCOME
bench contract show -f SESSION
bench contract accept -m provider/model -f SESSION -expect SHA256
bench contract run -m provider/model -f SESSION
```

`draft` and `revise` do not invoke Ply. `accept` requires the exact displayed
digest. Do not guess the session or digest, parse undocumented storage, or hide
the proposal from the owner.

## 4. Design, build, and prove

Create a Draft project only after the business package is reviewed:

```text
DRAFT_MODEL=provider/model draft new PROJECT "reviewed description"
draft check PROJECT
```

Open and review `PROJECT/DESIGN.md`. It must describe requirements,
deterministic versus model stages, refusals, and a `## Check` command whose exit
status owns done.

After the verifier and its authority are reviewed:

```text
draft admit PROJECT
draft build -admitted PROJECT -m provider/model
draft prove PROJECT
```

Interpret Draft exits per its public contract: 0 yes, 1 not yet, 2 broken;
admission may preserve May 3 or 75, and confinement may return 125. Capture
stderr evidence and exit status. Mutation proof challenges the executable
check; it does not replace external business holdouts.

## 5. Materialize the standing Agent home

Draft's design project and an Agent home are distinct artifacts. The current
promotion bridge is orchestrated and reviewed, not one magic command.

Create the home through the suite entry point:

```text
bench home new HOME "reviewed standing job"
```

Populate and inspect:

```text
bench home check HOME
bench home show HOME
```

The home contains:

- `AGENTS.md` — governed operating instructions;
- `GOAL.md` — durable obligation;
- optional `SOUL.md`, `HEARTBEAT.md`, and `MEMORY.md`;
- Brief skills and nested specialist homes;
- a deliberately constructed toolbox;
- mutable `work/` and `state/`;
- executable `bin/check` and optional `bin/wake`;
- controller-owned `.agent/runs/` evidence.

`agent check` proves structural validity, not business readiness. A fresh
scaffold has placeholder work that must not be reported as complete.

## 6. Exercise fixtures

First run without network and without external effects:

```text
bench home run -m provider/model HOME -- fixture-id
bench home history HOME check
bench home history HOME ls
```

Use a stable checkpoint only when continuation semantics are intended. A
passing pre-check should cost no model turn. Preserve stdout, stderr, exit
status, compiled composition, check evidence, and the external case score.

For Agent run/tick/specialist, the composed Agent/Ply statuses are: 0 accepted;
1 runtime, provider, or broken-verifier error; 2 not done or a bound reached; 3
declined; 75 parked; 125 uncertain confinement; 130 interrupted. Agent `check`
instead uses 0 valid, 1 invalid, and 2 usage/controller error. Do not apply one
suite-wide meaning to every status.

## 7. Evidence connections

Context reads one source connector and emits normalized JSONL:

```text
context ls
context query SOURCE QUERY
context check
```

Cite verifies exact evidence-link identity:

```text
cite evidence.jsonl
```

For MCP, a steward discovers and digest-admits only reviewed capabilities with
`mcp` and `mcpbox`. Do not make a discovered or annotated tool an approval
grant. For OAuth, use `oauth status` for non-secret inspection and `oauth with`
for an exact child; never include tokens in the Agent home.

## 8. Effects

The worker writes a small proposal under its mutable work area. The controller
inspects it and invokes Agent's action seam outside Cage:

```text
agent actions HOME
agent actions HOME PROPOSAL
agent act HOME PROPOSAL SESSION
```

The current Agent action commands may be absent from an older runtime even when
other Agent commands exist; that is a version mismatch, not a reason to bypass
the boundary. Action policy, May review, connector execution, and receipts
remain controller-owned. Never retry status 125 automatically.

## 9. Handoff records

For every command record:

- exact public argv with secrets removed;
- component version;
- working directory and declared writable roots;
- start/end time, exit status, and terminal reason;
- stdout result path and stderr evidence path;
- admitted contract/check/definition digest when the command exposes it;
- next human decision or safe retry condition.

Do not copy internal databases or private event formats into a new source of
truth. Use Agent history, Trail, and Ask replay through their public commands.
