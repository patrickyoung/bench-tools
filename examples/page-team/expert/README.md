# Single-file page team

A reusable team assembled through Hire and subsequent operator repair. The
initial Hire draft and a focused handoff revision were incomplete; operator
review repaired additional adapters. The complete definition now passes
structural verification, but Hire did not author every final byte.

Agent runs the manager and specialists in separate contexts. Existing Bench
Manage, Tend, and Weave own planning admission, execution, dependencies, and
retained outcomes. The adapters map packets and checked files; they are not a
model client, scheduler, or network server.

## Setup

Install the existing Bench commands through the bench-tools monorepo installer
and use a separately installed Bench Manage application. Required selectors are
listed in `config/example.env`. Configure executable paths, a supported Ask
model, and credentials with the existing provider and credential tools; keep
credentials out of this definition. Run `npm ci` in the definition to install
the locked browser dependencies. Select `PAGE_TEAM_BROWSER` or install
Playwright Chromium using its documented procedure. Runtime work belongs
outside the immutable definition bundle.

Creative runs also require Blender, the GIMP 3 console, and
`PAGE_TEAM_IMAGEGEN`, an existing executable that accepts JSON
`{prompt, output, references?}` on stdin, writes a real PNG at the requested
absolute output, and returns JSON provenance. Image generation is an explicitly
selected external capability in private controller work. Agent-written actions
retain their default Cage.

## Unix use

After exporting the selected configuration:

    export PAGE_TEAM_RUN=/absolute/runs/page-001
    /path/to/expert/bin/page-team < brief.md > index.html

Accepted HTML is written to stdout, progress to stderr, and a nonzero status
means failed or unfinished work. The run retains its backlog, separate worker
and control roots, source masters, provenance, and checks. Inspect it with
`bench-manage status -json RUN`, `tasks RUN`, or `board RUN`. Resume the same
run with the same command and empty stdin. A changed nonempty brief is rejected;
use a fresh run directory and immutable bundle for changed work.

The manager may use up to six Agent turns to read a large spooled snapshot,
verify its digest, and emit checked JSON directly. It is not guaranteed to be a
single-turn or inline exchange. Specialists return manifests binding paths,
sizes, and hashes. Frontend performs integration. The fixed review adapter
always runs browser journeys and screenshot Ask review before invoking the
review Agent. Final HTML must exactly match the reviewed artifact and original
brief hash; every changed page requires a new review.

The browser checker fulfills one entry navigation in memory, blocks all runtime
requests, and bounds diagnostics. It exercises desktop, mobile, narrow, tablet,
reduced-motion, and rendering-fallback behavior plus declared interaction
journeys. The root check validates a supplied `output/index.html`; stronger
integrated acceptance belongs to `bin/root-check`.

## Experts and subprocess use

Each `agents/NAME` definition can run independently with:

    agent run -C WORK -evidence CONTROL DEFINITION -- GOAL

Put explicit accepted inputs under `WORK`; children do not inherit another
expert's conversation. The team is also an ordinary subprocess filter usable
by another worker at an operator-selected external process boundary. Default
Action Cage does not grant recursive model, network, or controller-write
authority; do not disable confinement implicitly.

## MCP and A2A

Use the existing MCPserve with `interfaces/mcp/manifest.json` and its
dispatcher. Set `PAGE_TEAM_MCP_RUN_ROOT` to a private operator-owned run root;
`build_page` accepts `brief` and `run_id`. Select legacy host compatibility only
when required.

Use the existing A2Aserve with `interfaces/a2a/card.json` and its dispatcher.
Pass only necessary nonsecret selectors and credential-wrapper variable names
through documented `-pass-env` options. The server owns REST/JSON-RPC task
lifecycle, artifact export, TLS, authentication, and Origin policy. Its
workspace must provide a fresh controller run.

## Extend and validate

Add a focused `agents/NAME` definition and check plus an executable
`bin/workers/NAME` binding to the existing worker adapter. Bench Manage
discovers executable worker names, so no new registry or runner is needed.
Define explicit handoffs and use distinct work and control roots. A real
copy-editor extension has run without changes to the core adapters.

Read `EVALUATION.md` before relying on creative quality. Monorepo regressions
cover root review binding, handoff rejection, image-capability failures,
malformed MCP arguments, browser boundary behavior, and interaction failures.
Structural verification and fixtures do not prove an arbitrary page acceptable.

The manager input adapter shortens successful command traces while preserving
every task input, state, dependency, candidate, receipt and allowance. The
authoritative digest still binds the complete retained snapshot, not its display
projection. A 112 KB retained snapshot was reduced to 55 KB; a real manager
returned a checked correction plan in one model response.
