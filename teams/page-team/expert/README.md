# Single-file page team

Build a self-contained page from a new brief, using only the needed roles.

Agent runs the manager and specialists in separate contexts. Existing Bench
Manage, Tend, and Weave own planning admission, execution, dependencies, and
retained outcomes. The adapters map packets and checked files; they are not a
model client, scheduler, or network server.

## Setup

This source folder contains team wiring. First use the repository's
`scripts/workers export-team page-team DEST --ref FULL_COMMIT --allow-experimental`
to assemble its roster at a reviewed commit. The exported `expert/` contains
independent member definitions under `agents/` and their selected bindings.
Do not run or copy the source template alone.

Install the existing Bench commands through the bench-tools monorepo installer
and use a separately installed Bench Manage application. Required selectors are
listed in `config/example.env`. Configure executable paths, a supported Ask
model, and credentials with the existing provider and credential tools; keep
credentials out of this definition. Run `npm ci --ignore-scripts` in the exported
root expert to install locked browser dependencies, and in
`agents/visual-artist` to install its pinned p5.js, D3 and bundler dependencies. Select `PAGE_TEAM_BROWSER` or install
Playwright Chromium using its documented procedure. Runtime work belongs
outside the immutable definition bundle.

For briefs requiring those capabilities, configure Blender, the GIMP 3 console,
and `PAGE_TEAM_IMAGEGEN`, an existing executable that accepts JSON
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

Add or reuse a focused definition in the source library's `workers/ID` and
select it in the team's `team.json` roster. An optional `bin/worker-adapter`
entry creates its executable `bin/workers/ROLE` binding on export. Membership
is owned by the roster; do not duplicate child definitions in team source.
Bench Manage discovers the resulting executable names. Define compatible
handoffs and evaluate changes using distinct work and control roots. A real
copy-editor extension has run without changes to the core adapters.

Run the library's synthetic contract and browser tests against an exported
copy. Structural verification and fixtures do not prove an arbitrary page
acceptable. Evaluate each new product against its actual requirements.

The default browser review uses the page's declared interaction journeys.
An operator may select an external acceptance module through
`PAGE_TEAM_BROWSER_CHECK`: an absolute path to a trusted JavaScript module
whose default export is an async function taking a Playwright Page. This code
runs in the controller, outside Agent's action boundary. Keep its source fixed
for an admitted run and record its digest with that run's configuration. It is
never discovered from a prompt or fetched by the worker.
