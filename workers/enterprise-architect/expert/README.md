# Portable Enterprise Architect base worker

A reusable filesystem expert for platform strategy, reusable capabilities,
application and platform portfolio stewardship, product/SaaS choices, standards,
service contracts, transition gates, and evidence-based investment guidance.
It advises accountable people while delivery teams implement.

Catalogs can begin as Markdown and structured files in GitHub. A "portal" means
an optional internal directory for discovering shared services, owners and usage
guidance. Feed one if the company already has it; this worker does not require
or build a portal application.

The definition includes instructions, a short profile, ten Brief skills, an
artifact contract, and a read-only Python standard-library checker. It contains
no web app, model/provider SDK, runner, scheduler, credentials, memory,
recordings, generated job content, or proprietary framework text.

## Prerequisites

- Bench `agent` with an operator-selected model and permissions
- Bench `brief` available to Agent
- Python 3.9 or newer for `bin/check`
- A fresh writable job workspace
- The `expert/` definition kept separate and preferably read-only
- Any optional MCP/A2A programs selected and configured by the operator

## Standalone use

Write the complete request to `request.md`. Optionally place current evidence
beneath `inputs/`; inputs may be binary but cannot be symlinks.

    mkdir -p /absolute/job/inputs /absolute/control
    agent run -C /absolute/job -evidence /absolute/control /absolute/expert -- \
      'Read request.md and selected inputs and produce the contracted artifacts.'

The worker always returns `output/report.md` and
`output/architecture.json`. It selects one deliverable family and emits only
the additional artifacts needed by that family. Consume files only after Agent
exits successfully and the definition check accepts them. `ready` means
reviewable, not authorized or implemented.

## Team handoff and optional interfaces

A manager or other worker invokes this unchanged definition in a fresh
workspace with an explicit assignment and copied evidence. It must explicitly
select and transfer each artifact path needed downstream; variable deliverables
are not magically transmitted. Preserve stdout, stderr, output files, checker
status, and evidence separately. Re-run the receiving team's integration check
after combining accepted contributions.

For MCP, use the existing MCPserve manifest and dispatcher contract to stage
`request.md`/`inputs/`, invoke the same fixed Agent command, and explicitly
export the selected `output/...` paths. For API/A2A, use A2Aserve's existing
task workspace, authentication, and artifact-export support with the same
command. Consult the installed commands' current manuals for lifecycle and
flags. Listener policy, authentication, credentials, staging, workspace/state,
network access, and exports remain operator-owned. This definition adds no
server, adapter, protocol implementation, or second model loop; local Unix use
does not need a listener.

## Validation scope and limits

Run from a completed job workspace:

    /absolute/expert/bin/check

The checker reads only the current request, input tree, output tree, and the
definition's own `PROFILE.md`. The test suite accepts `ENTERPRISE_ARCHITECT_EXPERT`
to select an isolated export; the runtime check has no profile override.
It enforces closed JSON shape,
enums and types; bounded regular files/directories; UTF-8 generated artifacts;
complete sorted path manifests; exact request/input/artifact/profile SHA-256
bindings; required roles; question limits; and minimum ready handoff rows. It
does not execute or import policy, test, CI, or other artifact code and makes no
network calls.

A passing check proves only a structurally complete, current byte-bound package.
It cannot prove that recommendations are competent, source claims are true,
diagrams are useful, catalogs are semantically complete, policies are safe,
controls enforce requirements, or an architecture is approved. Hashes establish
byte identity, not trust. Limits and exact fields are in `CONTRACT.md`.

## Reproducible positive and negative checks

The synthetic suite lives beside, never inside, the exportable expert:

    python3 -m unittest -v tests/test_check.py
    brief lint -strict expert/skills
    hire verify expert

It copies the definition for each test, exercises all ten ready deliverables
and `needs-input`, and checks rejected hashes, malformed JSON, missing roles,
symlinks, tree limits, invalid UTF-8, and non-execution of policy code. A simple
negative manual case is to alter `request.md` after generating an otherwise
accepted package; the checker must exit 1 for a stale request hash. A positive
case is any suite-built package whose exact profile, request, inputs, output
files, required roles, and closed manifest agree.

These are offline contract tests, not live model-quality evidence. Run real
requests in fresh external workspaces. No nested live Agent evaluation is part
of this definition's local verification.

## Specializing

Read `SPECIALIZE.md`. Make a reviewed, commit-pinned clean export with
the existing `scripts/workers` utility; preserve its lock as provenance; use
Hire to amend the profile and domain skills; retain the base contract and
evaluation; and publish an independently owned/versioned worker. There is no
implicit inheritance or sibling-worker loading.
