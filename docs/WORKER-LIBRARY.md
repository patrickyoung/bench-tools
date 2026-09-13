# A worker library managed with Git

[Home](../README.md) · [Catalog](../workers/README.md) · [Team recipes](../teams/README.md)

**Keep reusable expertise in the monorepo, review changes through GitHub, and
assemble a clean, pinned folder for each team.** Hire builds or adapts that
folder; Agent runs it. Git owns history and versions. The shell and existing
Bench commands arrange execution.

## What is installed

The [page team](../workers/page-team/expert/README.md) is the first library
entry. Its manager, specialists, checks and adapters form one bundle. Its
[metadata](../workers/page-team/worker.json) is the authoritative record of ID,
owner, description, lifecycle status, requirements and approved source files.
The catalog links to that record rather than duplicating its status.

Architect, Writer and Present remain outside the library. The page team's
historical showcase, briefs, recordings and evaluation stay under `examples/`.
They are not exported. Its runtime definition is now independent of that
showcase; domain-specific acceptance is supplied explicitly for a current job.

```text
bench-tools/
  tools/                     existing independent programs
  workers/
    README.md                small catalog
    page-team/
      worker.json            identity, lifecycle, requirements, approved files
      expert/                runnable definition, including child experts
      tests/                 small synthetic tests; never exported
  teams/                     reusable assembly recipes, without job content
  scripts/workers            repository source utility; never a job runner
```

## Select and export a version

From the checkout root:

```sh
python3 scripts/workers list --all
python3 scripts/workers check
git rev-parse HEAD
python3 scripts/workers export page-team /absolute/path/to/team \
  --ref FULL_COMMIT --allow-experimental
```

Replace `FULL_COMMIT` with the complete reviewed commit. The export requires a
new destination; it refuses overwriting even an empty folder. It creates only
`expert/` and `team.lock.json`. Uncommitted changes are never exported. Install
the worker's dependencies in this copy using its setup instructions; keep the
source library free of installed dependencies and runtime files.

`list` defaults to active workers. `--all` also shows experimental, deprecated
and retired records. Export requires active status, except that the explicit
`--allow-experimental` option enables an evaluation of an experimental entry.
It never enables deprecated or retired entries at that selected source commit.
An old commit retains its historical status: this is a maintenance policy,
not a remote revocation service.

The lock records the source repository, full commit, definition path, declared
runtime requirements and each file's digest and executable mode. Agent need
not parse it. The requirements include the tested Bench tools and Bench Manage
source pins; provider/model and optional native-tool versions belong in the
operator's private configuration. A source pin reproduces code, not identical
model output or unchanged external services.

## Approved source only

The utility uses Git's archive facility, verifies its bytes against committed
blobs, and materializes only explicitly listed regular files. Missing files,
undeclared committed files, path escapes, symlinks, missing licenses/checks,
oversized files, known output formats and embedded binary payloads are rejected.
The current-source check also inspects untracked and ignored library files.
Git attributes cannot silently omit or substitute exported source.

Approved source includes instructions, skills, deterministic tools, acceptance
checks, interfaces, dependency manifests/locks and license notices. This first
library supports text source only; binary source assets would require a
separately reviewed packaging change. Small synthetic tests are kept beside
the source and excluded from runtime exports.

Generated sites, slides, documents, images, recordings, original job briefs,
customer inputs, transcripts, sessions, checkpoints, run reports, runtime
memory, caches, credentials and development files do not belong in the export.
Never copy a live worker home or a whole repository into an assembly. An export
contains no `.git` history. `.gitignore` alone is insufficient because it does
not remove previously tracked content.

Review the content of allowed Markdown/programs as well as their filenames.
Mechanical checks cannot prove an arbitrary paragraph is free of prior job
data. Improve a worker by reviewing a generalized instruction or skill change;
never automatically promote a successful run's memory or output. Any useful
seed `MEMORY.md` must be curated, reviewed and explicitly listed as source.

## Assemble with Hire; run with Agent

Choose a [recipe](../teams/README.md) and a reviewed source snapshot. Export into
a fresh authoring directory. If the existing team fits, use it directly.
Otherwise give Hire the recipe and requested adaptation:

```sh
hire build -C /absolute/path/to/team \
  -evidence /absolute/path/to/authoring-evidence -- \
  'Adapt the existing expert using the supplied recipe and current requirements.
   Reuse its useful roles, tools, handoffs and checks. Keep job inputs and
   outputs outside the definition.' < teams/simple-site.md
hire verify /absolute/path/to/team/expert
```

Inspect the diff and evaluate the changed behavior. The original export lock
still describes the starting source: preserve it, record adaptations and the
revised source digests before relying on a changed assembly. The source export
utility does not certify or re-lock arbitrary authoring work. Promote a useful
adaptation through a pull request, then export its new commit for repeat use.

Each assignment gets a fresh workspace and explicitly selected current inputs.
Use the worker's existing entry point; the page team's is `expert/bin/page-team`.
It writes accepted HTML to stdout only on exit 0. Workspaces and controller
records remain outside the definition. Nothing fetches `main`, resolves a
registry, or upgrades workers while a job runs.

The page team's children remain together until each role has been checked
without hidden dependencies on that bundle. Role names do not establish
compatible handoffs. Recipes describe inputs, outputs, dependencies and
acceptance; they introduce no additional runtime or scheduler.

Clean exports prevent accidental carryover, not host-file access. Agent's
current Cage limits writes/network and permits host reads. If old runs must
be unreadable, select an existing Unix account/container boundary exposing
only intended definitions, tools and current inputs. Do not call packaging
alone a confidentiality boundary.

## Maintain the library through GitHub

| Operation | Mechanism |
| --- | --- |
| Add | Build outside source; curate an expert, metadata and synthetic positive/negative cases; submit a pull request as `experimental`. |
| Adopt | Review source/export contents and checks; evaluate a representative fresh job; change status to `active`. |
| Improve | Modify a branch or clean Hire authoring copy; review the source diff and affected checks. |
| Upgrade | Export a new commit, exercise the team's acceptance cases, then deliberately select it for new jobs. Keep the earlier pin for rollback. |
| Deprecate | Set `deprecated`, a reason and optional successor; keep it discoverable for migration, outside new-team selection. |
| Retire | Set `retired` and a reason; exclude new exports at this revision and preserve history for tracing older work. |

Lifecycle changes do not stop admitted runs, revoke credentials or delete
private data. Those are separate operator actions. Never reuse a retired ID
for an unrelated worker. Full commits suffice initially; optional namespaced
release tags can be added later without synchronized tool releases.

Existing CI validates the catalog and export rules, documentation links and
synthetic packaging cases. Its starter integration also exercises the relocated
team through real public Bench commands and local model fixtures. Run the
browser suite on an exported copy with its pinned dependency installed:

```sh
python3 workers/page-team/tests/contracts.py /absolute/path/to/team/expert
node workers/page-team/tests/browser.mjs /absolute/path/to/team/expert
```

A separately selected live smoke uses `workers/page-team/tests/smoke-brief.py`;
its synthetic input is explicitly supplied for that evaluation and is never
part of a runtime export. Keep its results and model evidence outside Git.
Structural, fixture and live judgments establish different claims.

GitHub [pull requests](https://docs.github.com/en/pull-requests/reference/pull-requests)
provide review and check results. Git's [archive facility](https://git-scm.com/docs/git-archive)
provides committed source. Hire remains the builder and Agent the runner.
