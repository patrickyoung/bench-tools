# Workers and teams managed with Git

[Home](../README.md) · [Workers](../workers/README.md) · [Teams](../teams/README.md)

Keep reusable expertise in the monorepo, review changes through GitHub, and
export a clean folder at a reviewed commit. Hire builds or adapts definitions;
Agent runs them. Existing Unix programs arrange execution.

For first use, follow [the worker/team walkthrough](BUILD-WITH-AN-LLM.md).
Harnesses start at [START-HERE.md](../START-HERE.md). This reference owns source
layout, assembly, clean exports and lifecycle; the catalogs own discovery.

## Three separate things

```text
bench-tools/
  tools/                     existing independent programs
  workers/ID/
    worker.json              identity, lifecycle, requirements, approved files
    expert/                  one independently usable definition
    tests/                   synthetic cases, never exported
  teams/ID/
    team.json                lifecycle, approved wiring, member roster
    expert/                  team wiring, without bundled worker copies
    tests/                   assembly checks, never exported
  teams/*.md                 brief-independent usage recipes
  scripts/workers            repository source export utility

outside the checkout/
  selected-team/
    team.lock.json           source identities and hashes
    expert/
      agents/ROLE/           clean copies of selected workers
      bin/workers/ROLE       selected executable adapters
  current-run/               current inputs, outputs and private evidence
```

Workers are reusable roles. Teams select and connect them. Runs contain work.
A worker belongs to no single team: another roster can reuse the same source.
The page team is an actual runnable assembly after export; its source template
alone is incomplete. Human recipe documents describe useful ways to use it.

The [page team roster](../teams/page-team/team.json) now selects the independent
p5.js/D3 Visual artist. The original smaller Canvas artist remains available
separately. The older large Architect, Writer and Present applications are outside
this library. The portable Enterprise Architect is an independent thin worker.
Historical
showcases and their evaluation material stay under `examples/`.

## Select and export

```sh
python3 scripts/workers list --all
python3 scripts/workers list --teams --all
python3 scripts/workers check
git rev-parse HEAD
python3 scripts/workers export visual-artist /absolute/path/to/artist \
  --ref FULL_COMMIT --allow-experimental
python3 scripts/workers export-team page-team /absolute/path/to/team \
  --ref FULL_COMMIT --allow-experimental
```

Replace `FULL_COMMIT` with the full reviewed commit. A new destination outside
the checkout is required, even when an existing folder is empty. Uncommitted
changes are never exported. Individual exports contain `expert/` and
`worker.lock.json`; teams contain `expert/` and `team.lock.json`.

One monorepo commit pins the team roster, wiring and every member. Each member
maps a local role to a worker ID, with an optional approved executable adapter:

```json
"frontend": {"worker": "frontend", "adapter": "bin/worker-adapter"}
```

The exporter copies that worker's approved source into `agents/frontend` and
copies the exact adapter bytes into `bin/workers/frontend`. The page adapter
uses its filename as the role and invokes the existing `run-worker` command.
Its manager member needs no such binding. Removing a roster entry removes both
its definition and binding from the next export; source cannot contain hidden
copies under these reserved directories. Role names alone do not establish
compatible input/output contracts: review and test each changed assembly.

Locks record the repository, full commit, source paths, declared requirements,
file hashes and modes. Team locks also identify each member and its adapter.
Agent does not read locks as instructions. Requirements identify tested tools;
provider settings, credentials and native installations remain operator choices.
A source pin reproduces code, not identical model output or external services.
Nothing fetches `main`, resolves versions or upgrades workers during a run.

Optional `--target HOST --execution native|bench` wraps an intact export under
`references/bench/` in a portable host skill. See [worker portability](WORKER-PORTABILITY.md)
for installation, native execution limits and cross-harness evaluation. Default
exports and Bench execution retain their existing contracts.

Only active entries appear in default listings. `--all` shows every status.
`--allow-experimental` permits evaluation of experimental teams and members;
it does not allow deprecated or retired definitions. All selected members must
qualify. An old pin retains its historical status: lifecycle metadata does not
revoke already exported source.

## Locate source and locate a command

Use `workers/README.md` and `scripts/workers list --all` for local definitions;
use `teams/README.md` and `list --teams --all` for assemblies. Read the selected
metadata and README to establish its actual contract. Record the checkout's
absolute path in an external `BENCH-SETUP.md`: an installed skill or binary
prefix does not contain a separate copy of this catalog. `scripts/workers`
is a repository utility; it is not an installed runner or remote registry.

A worker is invoked through `agent run` plus its exported definition and work
folder. A team uses its documented entry command. Both can be exposed by the
existing [A2A server](../tools/a2a/README.md#expose-an-expert). Text streams need
configuration; declared file artifacts can be returned too. Add a dispatcher
only for application-specific translation. A2A card discovery describes an
explicit remote endpoint, while the source catalog describes reusable local
code. The roster does not resolve remote endpoints automatically.

## Approved source only

Exports use Git archives, verify bytes against committed blobs and copy only
explicitly listed regular text files. Missing or undeclared files, escapes,
symlinks, oversized files, known output formats and embedded binary payloads
are rejected. Git attributes cannot silently omit or substitute source. Current
checks also inspect ignored and untracked library files.

Instructions, skills, deterministic tools, checks, interfaces, package locks
and licenses are source. Small synthetic tests stay beside it and are not
exported. Generated pages, artwork, recordings, job briefs, customer data,
sessions, runtime memories, caches, installed packages, credentials and
development records are excluded. No `.git` history is copied.

A worker may own a small Go helper for deterministic file or format work.
Inventory its source and private module under `expert/`; keep synthetic Go
tests and their module under `tests/`. The architecture gate admits these only
for registered workers, requires the standard library exclusively, and rejects
cross-tool or cross-worker imports, module dependencies and replacements.
Tools cannot import a worker's module either. Build the helper outside reusable
source and keep execution through public commands; this grants no shared
runtime, root module, workspace or additional authority.

Review the content of allowed Markdown and programs too: filenames cannot
prove that paragraphs contain no prior job data. Promote generalized learning
as a reviewed instruction or skill change. Never automatically promote run
memory or output. Any seed memory must be curated and explicitly inventoried.

## Build, use and change an assembly

Use a suitable unchanged export directly. Install its dependencies in that
copy according to its README, never in the source library. Supply only the
current brief and explicitly selected inputs, with separate work and control
folders. The page team's filter is `expert/bin/page-team`: accepted HTML goes
to stdout on exit 0, diagnostics to stderr. Each assigned worker receives its
own Agent context and workspace through the existing adapters.

For different membership, edit a team roster in a branch. For a new team, add
its focused wiring, roster, metadata and synthetic checks under `teams/ID`;
reference existing `workers/ID` definitions. Templates are explicit source,
without inheritance or a dependency resolver. Adding a role requires a
compatible contract, not a new runtime or scheduler.

Use Hire when the requested expertise or wiring needs adaptation:

```sh
hire build -C /absolute/path/to/team \
  -evidence /absolute/path/to/authoring-evidence -- \
  'Adapt the existing expert using the supplied recipe and current requirements.
   Reuse its useful roles, tools, handoffs and checks. Keep job inputs and
   outputs outside the definition.' < teams/simple-site.md
hire verify /absolute/path/to/team/expert
```

Inspect and evaluate the changed behavior. Preserve the original lock as the
starting record; its hashes do not describe later edits or dependencies.
Record adaptations and revised digests locally. Promote reusable changes back
to the appropriate worker or team through a pull request, then export the new
commit. The utility does not certify or re-lock arbitrary authoring work.

Clean source packaging prevents accidental carryover. Agent's current Cage
limits writes/network and permits host reads. If old runs must be unreadable,
select an existing Unix account/container boundary exposing only the intended
definitions, tools and current inputs.

## Maintain through GitHub

| Operation | Mechanism |
| --- | --- |
| Add | Curate source, metadata and positive/negative cases outside live work; submit a PR as experimental. |
| Adopt | Review clean exports and checks, evaluate a representative fresh job, then set active. |
| Improve | Change the owning worker or team in a branch; inspect the diff and run affected checks. |
| Upgrade | Select a new full commit for new jobs after exercising assembly acceptance; retain the prior pin for rollback. |
| Deprecate | Set deprecated, a reason and optional successor; stop new exports at that revision. |
| Retire | Set retired and a reason; preserve history and never reuse the ID for an unrelated role. |

Workers and teams have independent lifecycle records. Retiring a member blocks
new exports of teams that still select it at that revision, making migration
explicit. It does not stop running jobs, revoke credentials or erase data.
Full commits suffice; release tags can be added without synchronized tool releases.

CI checks inventories, pinned assembly, source exclusions, role independence,
documentation and executable public-tool integration with local model fixtures.
Run the page contracts and browser cases against an exported copy with its
pinned browser dependency installed:

```sh
python3 teams/page-team/tests/contracts.py /absolute/path/to/team/expert
node teams/page-team/tests/browser.mjs /absolute/path/to/team/expert
```

The standalone Visual artist has its own synthetic suite. An explicitly chosen
live smoke can use `teams/page-team/tests/smoke-brief.py`; its supplied input
and resulting evidence stay outside source. Fixture checks establish packaging
and protocol behavior; evaluate actual creative quality against a fresh brief.
