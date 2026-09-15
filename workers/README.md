# Reusable workers

For finding, building and running definitions, follow the
[worker/team walkthrough](../docs/BUILD-WITH-AN-LLM.md); harnesses start at
[START-HERE.md](../START-HERE.md).

Each entry owns one focused definition that can be exported on its own or
selected by a team. Status, owner, requirements and approved files live in its
`worker.json`; source changes go through ordinary GitHub pull requests.

| Worker | Purpose |
| --- | --- |
| [Excel workbook designer](excel-workbook-designer/expert/README.md) | Organize selected CSV, JSON and notes into traceable interactive Excel workbooks with formulas, controls, charts and a tested update workflow. |
| [Product Owner](product-owner/expert/README.md) | Clarify customer problems, test outcome assumptions, recommend backlog priorities and delivery trade-offs, and review AI product decisions. |
| [Enterprise Architect](enterprise-architect/expert/README.md) | Guide enterprise platforms, portfolio investments and sunsets, capability reuse, emerging-product evaluation, standards and transition gates. Delivery teams implement. |
| [Frontend](frontend/expert/README.md) | Integrate accepted contributions into one accessible, self-contained HTML page. |
| [Visual artist](visual-artist/expert/README.md) | Create p5.js/D3 artwork and artistic data experiences, with purposeful optional sensor inputs. |
| [Canvas artist](canvas-artist/expert/README.md) | Make lightweight Canvas/WebGL pieces using the original page specialist contract. |
| [Blender artist](blender-artist/expert/README.md) | Produce editable Blender sources, a preview and a web asset. |
| [Image editor](image-editor/expert/README.md) | Preserve originals and produce an editable GIMP master and optimized export. |
| [Image concept](image-concept/expert/README.md) | Prepare a checked image request and composition notes for a separately selected generator. |
| [Page planner](page-planner/expert/README.md) | Propose bounded tasks against an explicitly supplied Bench Manage snapshot. |
| [Page reviewer](page-reviewer/expert/README.md) | Assess supplied page observations and produce a structured review with limitations. |

The [Product Owner design brief](../docs/PRODUCT-OWNER-DESIGN.md) uses SIPOC to
guide worker development. It is maintained separately from exported definitions
and job deliverables.

The [Enterprise Architect design brief](../docs/ENTERPRISE-ARCHITECT-DESIGN.md)
uses the same source-only design method. Its
[specialization guide](enterprise-architect/expert/SPECIALIZE.md) explains how
to turn a clean pinned base into a focused architect with Hire.

From the repository root:

```sh
python3 scripts/workers list --all
python3 scripts/workers check
git rev-parse HEAD
python3 scripts/workers export visual-artist /absolute/path/to/artist \
  --ref FULL_COMMIT --allow-experimental
```

Use the full reviewed commit printed by Git. Export requires a new destination
outside the checkout and creates `expert/` plus `worker.lock.json`. Its bytes
come from that commit, even when the working tree has edits. The lock records
source identity, requirements, hashes and executable modes. Agent uses the
expert folder; it does not load the lock as instructions.

Install dependencies only in the exported copy, following its README. Supply
fresh work and evidence folders and explicit current inputs. Exports exclude
prior deliverables, briefs, conversations, runtime memories and development
files. Review source content as well as filenames before promotion.

Use [teams](../teams/README.md) to assemble these definitions. A team owns a
roster and wiring; workers are not stored underneath another worker in source.
The older large Architect, Writer and Present applications remain outside this
library; Enterprise Architect is a new thin definition. Historical showcases
stay under `examples/` and never enter exports.

Only active entries appear in the default list. Experimental entries require
explicit opt-in; deprecated and retired entries cannot be newly exported at
that source revision. Older pins retain their historical status. See the
[library guide](../docs/WORKER-LIBRARY.md) for maintenance and promotion.
