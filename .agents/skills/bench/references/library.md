# Find, assemble and maintain reusable source

Resolve all repository paths below against the selected `BENCH_SOURCE` from
[setup](setup.md). If a fresh session has no environment variable, read the
external `BENCH-SETUP.md` to recover that absolute path and inspect it. A copied
host skill or installed Agent binary does not contain the worker catalog.

## Locate expertise before authoring

Read `workers/README.md` and `teams/README.md` in the checkout, then:

```sh
cd "$BENCH_SOURCE"
python3 scripts/workers list --all
python3 scripts/workers list --teams --all
```

These commands emit one JSON record per entry. Default listings show only
active entries; `--all` is essential when inspecting an experimental library.
Match the user's outcome against descriptions. Read the selected `worker.json`
or `team.json`, `expert/README.md`, instructions and checks before deciding it
fits. Search source descriptions if the catalog is large; do not infer a
capability from an ID alone. Report status and missing prerequisites accurately.

A team record has a role-to-worker roster and optional executable adapters.
Inspect its members and wiring. Reusing Frontend directly and selecting Frontend
in a team use the same source. Image Concept only creates a generator request;
Page Reviewer needs observations; Page Planner needs an admitted snapshot.

## Export a reviewed snapshot

```sh
git rev-parse HEAD
python3 scripts/workers export frontend /absolute/project/frontend \
  --ref FULL_COMMIT --allow-experimental
python3 scripts/workers export-team page-team /absolute/project/page-team \
  --ref FULL_COMMIT --allow-experimental
```

Use the full selected commit and fresh destinations outside the source checkout.
The experimental option permits deliberate evaluation, never deprecated or
retired entries. Omit it for entirely active selections. The committed export
ignores working-tree edits; do not describe an uncommitted change as exported.
One monorepo commit pins a team's roster, wiring and every selected definition.

Individual exports contain `expert/` and `worker.lock.json`. Team exports
contain `expert/` with `agents/ROLE` copies and `bin/workers/ROLE` bindings,
plus `team.lock.json` identifying the members and hashes. The source team
`expert/` template alone is incomplete. Run an assembled export.

Read the exported README; install its dependencies there. For the page team,
install its declared Bench commands and separately installed Bench Manage,
configure selectors, and install the root browser dependencies and Visual
Artist's own pinned packages. Configure optional native tools only for jobs
that require them. Follow [operate](operate.md) for actual execution.

## Assemble or adapt

If an existing team fits, export it directly; assembly makes no model call.
Its manager may use only useful available roles for a particular brief. If a
fixed different membership is needed, change the roster, preserving the team's
required manager/integration/review contracts and evaluating the result.

For a new reusable team, add `teams/ID/team.json`, its focused `expert/` wiring
and synthetic `tests/` beside them. A team owns its roster and approved wiring;
it references `workers/ID` instead of storing child source in its template.
The exporter supports explicit source and member selection, not template
inheritance, remote endpoint discovery or arbitrary workflow interpretation.

Use Hire through [build](build.md) when expertise or wiring needs adaptation.
Give it the clean existing definitions, desired contracts and current outcome.
A proposed manager is itself a worker; the existing controller admits its
plans and runs assignments. Do not create another scheduler, provider client
or loop. A team can also use a fixed composition of public commands.

## Promote the reusable part through GitHub

Read `docs/WORKER-LIBRARY.md` for the full metadata and lifecycle contract.
Build/evaluate outside source. When maintaining the library is part of the
request, curate only reusable files into a source branch:

- Worker: `workers/ID/worker.json`, its `expert/` and separate synthetic `tests/`.
- Team: `teams/ID/team.json`, wiring template and separate assembly tests.
- Recipe: brief-independent guidance in `teams/*.md`; it is not executable.

Metadata owns identity, owner, description, status, requirements and an explicit
approved file list. Start new entries as experimental. A valid Agent definition
needs instructions and an executable check; library packaging additionally
requires README, license and the declared source inventory. Keep imports and
copies independently usable; do not rely on a sibling checkout at runtime.

Run `python3 scripts/workers check`, relevant existing synthetic cases and an
export of the committed revision. Inspect every exported path. A team change
also needs its executable handoff/acceptance checks. Use the same GitHub source
review process as other changes; provide model evaluation evidence separately
when behavior changed. Keep libraries free of credentials, generated sites,
art, prior briefs, installed packages, run histories and runtime memory.

Promote to active after representative evaluation. Deprecate or retire with
a reason and optional successor. Check teams that select the changed worker;
retiring one blocks new exports of those rosters at that revision. Old commits
and exports remain traceable and usable according to their historical state.
Do not automatically update existing jobs. Preserve pins for deliberate
upgrades and rollback, and never reuse an unrelated retired ID.
