# Decision: maintain independent Bench tools together

2026-09-09. **Proceed with a source monorepo for the independent tools.**
The review covered 24 Git repositories and two unversioned project directories.
The important architecture already exists at the program boundary. Maintaining
those sources together makes coordinated protocol changes and real executable
integration checks reviewable in one commit. No shared runtime is needed to
obtain that benefit.

Moving every project indiscriminately would endorse incompatible approval
paths, erase important application boundaries, and break suite provenance.
The initial import therefore includes all 17 reviewed tools whose boundaries
can survive unchanged. Nine projects remain outside this repository for the
specific reasons below. This is a substantial first source migration, not a
claim that every Bench project should ultimately share one release.

## All projects reviewed

| Project | Decision | Boundary / reason |
| --- | --- | --- |
| action | Imported | One controlled effect via named connector; policy, exact May decisions, and Ask receipts remain executable seams. |
| agent | Imported | POSIX worker-home composition of public programs; no embedded loop, provider, scheduler, or transcript writer. |
| ask | Imported | Model requests and authoritative session/replay records; provider dependencies remain in this module alone. |
| brief | Imported | Procedure discovery/validation; no model or execution authority. |
| cage | Imported | Native child confinement; caller-selected write roots, backend refusal, and stream/status forwarding remain intact. |
| cite | Imported | Stateless Context reference-to-locator checks; neither retrieval nor truth judgment. |
| context | Imported | Explicit executable source, validated complete evidence records, invocation provenance; no source routing. |
| draft | Imported | Design/build/prove shell composition, private skill assets and generated tool reference retained. |
| hone | Imported | Verified recovery to a reviewable lesson through Ask/Brief programs. |
| may | Imported | Exact, single-use human action decision; no suite-wide approval mechanism. |
| mcp | Imported | Four separate protocol-edge executables within one existing module, including explicit legacy command. |
| oauth | Imported | Narrow credential lifecycle and explicit descriptor handoff, outside Ask and other consumers. |
| ply | Imported | Tool loop, check, steering, and cancellation; public Ask protocol, no imported model client. |
| rules | Imported | Read instruction files under the actual Git root; repository scope change is explicit below. |
| tend | Imported | Durable arbitrary-process execution; no model, agent judgment, or tool policy. |
| trail | Imported | Read-only archive inspection; Ask performs replay verification. |
| weave | Imported | Read-only readiness calculation from supplied graph/observations; no execution or retry engine. |
| bench | Worth importing later as application | Its packager verifies separate Git HEADs/pins; requires explicit monorepo subdirectory/tree support before migration. Preserve conversation UI boundary. |
| hire | Worth importing later as application | Its pinned tool installation must stay coherent. Existing macOS fixture has lexical/physical temporary-path sensitivity; test evidence is in the review. |
| manage | Defer experimental application | Offline suite relies on ignored prebuilt var/tools binaries; fresh-checkout setup and relationship to Hire need explicit treatment. |
| studio | Keep separate pending architecture decision | Unversioned app directly imports Bench packages with `replace => ../bench`; runtime data is mixed into the project directory. |
| clerk | Keep separate | Private argv-based `bin/may` conflicts with standalone May; global link recipe and orphan retry policy need deliberate redesign before common packaging. |
| web | Defer edge tool | Its gate calls the older Clerk May interface; fake-peer tests do not establish compatibility with current May. |
| vouch | Defer edge tool | Distinct useful credential mechanism, but publishing/license/install inventory and relationship to OAuth must stay explicit. Do not merge their credential stores or contracts. |
| guide | Defer learning collection | Valuable executable lessons; known Ask auth/setup drift and independent Cage installer need updating before using it as a common guide. |
| pack | Keep separate experiment | Unversioned source mixed with generated packets/state; model routing and packet IDs deliberately differ from Context/Cite. |

Detailed implementation, dependency, test, and history findings:
[filters](review/filters.md), [runtime edges](review/runtime.md),
[orchestration/apps](review/orchestration.md), and
[Bench/Context/Guide/Pack](review/root-review.md).
Exact inventory: [inventory.json](review/inventory.json).

## Invariants of this migration

- Each `tools/NAME` is the exact tracked source tree at the commit in
  `components.json`. Files, executable modes, licenses, manuals, examples,
  local guidance, module paths, dependencies, and commands are preserved.
- Each tool's full ancestry leading to that commit is retained via an
  unsquashed Git subtree merge. Existing tags are preserved under
  `upstream/NAME/TAG`; `docs/review/source-tags.json` records their object IDs.
  Old history has its original paths; use the recorded source commit to inspect
  it. A path-limited log alone will not display all pre-import history.
- The original repositories remain unchanged and available. This new local
  repository has no remote, published release, global installation, or migrated
  user data. There is no claim of having cut over published source ownership.
- Fifteen Go modules remain independent, plus Agent and Draft as shell tools.
  There is no root `go.mod`, `go.work`, sibling `replace`, shared runtime,
  umbrella command, daemon, synchronized version, or normalized exit contract.
- Companion tools are optional or required exactly as their own contracts say.
  A source checkout next door does not confer runtime authority or implicitly
  choose credentials, state paths, policy, approval, tools, or writable roots.
- Do not turn a common source root into a Cage write grant. Do not import user
  skills, sessions, worker homes, OAuth profiles, May grants, admitted MCP
  capability directories, build caches, or evaluation campaign output.

Root checks inspect actual imports/module metadata and filesystem boundaries,
exercise each tool without sibling source, and run explicit CLI integrations.
These checks do not replace human review of semantic boundaries: a source
scanner cannot prove that a patch has not duplicated policy or weakened an
exit contract. Every component's own design and tests remain authoritative.

## Rules and instruction scope

Rules still uses the nearest real `.git` directory/file. Within this repository
that is the monorepo root, so root `AGENTS.md` is newly visible before the
unchanged tool guidance. Root guidance contains only shared boundary policy;
there are no cross-tool instruction symlinks. We do not fake nested Git roots
or change Rules' discovery semantics. Other tools' AGENTS files are not
ancestors and are not implicitly loaded.

When a task requires a tool's former narrow repository scope, export it to a
separate directory with `scripts/export-tool NAME DEST`, run `git init` there,
and work in that repository. Source scope and runtime permissions are separate
choices in either layout.

## Publishing and next migration steps

The manifest is source provenance and a test/build inventory, **not** a Bench
suite compatibility lock. Current Bench/Hire suite pins differ from the
imported HEADs. Their existing per-repository fetch/release routes remain the
supported routes; do not point their old workspace packagers at `tools/` or
use `-allow-dirty` to bypass the revision mismatch.

Use `tools/NAME` for development in this migration. Retained original
checkouts are source baselines and current release inputs. No automatic
synchronization is configured; carry changes across a publication boundary
explicitly using the recorded source commits and reviewed diffs.

Before a release cutover, teach the packagers to verify a source repository,
commit, subdirectory, and tree digest; retain clean-tree checks and exact
component versions, and validate the extracted relocatable suite with real
binaries. Keep application and filter versions independent. No public module
path is changed by this local migration. A future publication may use subtree
exports/mirrors to preserve existing Go install paths, but those destinations
must be explicitly chosen and verified before any release redirects.

Then consider Bench and Hire under an application tier with their own modules
and process contracts; bring Guide after correcting its setup drift. Review
Web's May adapter and Clerk's private gate separately. Vouch must retain its
own credential contract if later included. Pack, Studio, and Manage need their
recorded design/setup questions answered, not a mass move or a shared runtime.
