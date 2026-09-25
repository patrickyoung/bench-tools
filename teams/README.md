# Reusable teams

For finding, building and running definitions, follow the
[worker/team walkthrough](../docs/BUILD-WITH-AN-LLM.md); harnesses start at
[START-HERE.md](../START-HERE.md).

A worker defines one role. A team selects workers and supplies their existing
execution wiring. Both are versioned in this monorepo; a run supplies a new
brief, workspace and selected inputs.

Use `make check-workers` for worker and team evaluation suites. The
[evaluation runbook](../docs/WORKER-EVALUATIONS.md) maps coverage, explains
prerequisites and provides focused and optional native/browser commands.

| Team | Purpose | Authoritative roster |
| --- | --- | --- |
| [Vector Style Studio](vector-style-studio/expert/README.md) | Create editable vectors through a controlled drawing plan and native Inkscape, then independent bitmap styles with protected factual text. | [team.json](vector-style-studio/team.json) |
| [Vendor comparison team](vendor-comparison-team/expert/README.md) | Lead technical evaluations through Product Manager framing/synthesis, Polars analysis, comparison and independent review. | [team.json](vendor-comparison-team/team.json) |
| [Vendor decision studio](vendor-decision-studio/expert/README.md) | Add dedicated editorial, information design, executive writing, presentation and publication review expertise to the comparison process. | [team.json](vendor-decision-studio/team.json) |
| [Page team](page-team/expert/README.md) | Plan, create, integrate and review a single-file site using the useful available roles. | [team.json](page-team/team.json) |

```sh
python3 scripts/workers list --teams --all
git rev-parse HEAD
python3 scripts/workers export-team page-team /absolute/path/to/team \
  --ref FULL_COMMIT --allow-experimental
```

The exporter copies each selected worker into `expert/agents/ROLE` and its
optional adapter into `expert/bin/workers/ROLE`. One full commit pins the roster,
wiring and all member definitions. `team.lock.json` records every source, file
hash and mode. The assembled copy is runnable; the source template alone is
incomplete. Dependencies are installed only in the exported copy.

To change membership, edit `team.json` in a branch and review the change. To add
a different reusable team, add its template, roster and checks alongside this
one, referencing existing workers. Use Hire when expertise or wiring needs
adaptation; ordinary source assembly does not need a model call. Agent and the
existing Bench commands retain execution, contexts, queues and checks.

These brief-independent usage recipes describe which available roles to use:

- [Simple site](simple-site.md): frontend and review, with creative roles only
  where useful.
- [Creative site](creative-site.md): native artwork and image contributions,
  integration and review.
- [Artistic site](artistic-site.md): p5.js/D3 artistic visualization, integration
  and review.

A recipe is guidance, not an executable team. It grants no tools, credentials
or scheduling authority. The [library guide](../docs/WORKER-LIBRARY.md) explains
source-only exports, adaptation, lifecycle and evaluation.
