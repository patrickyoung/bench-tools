# Reusable workers

For finding, building and running definitions, follow the
[worker/team walkthrough](../docs/BUILD-WITH-AN-LLM.md); harnesses start at
[START-HERE.md](../START-HERE.md).

Each entry owns one focused definition that can be exported on its own or
selected by a team. Status, owner, requirements and approved files live in its
`worker.json`; source changes go through ordinary GitHub pull requests.

Run worker and team evaluations with `make check-workers`. The
[evaluation runbook](../docs/WORKER-EVALUATIONS.md) covers prerequisites, focused
runs, retained evidence, native/browser suites and separate model-quality cases.

| Worker | Purpose |
| --- | --- |
| [Django expert](django-expert/expert/README.md) | Build Django 6.1.1/PostgreSQL applications with professional admin, iterative accessible UX, four environments, OIDC and measured scaling. |
| [Excel workbook designer and editor](excel-workbook-designer/expert/README.md) | Create and prompt-edit reports and text-first team workbooks; exact lookups, RAG, WBS and time-off coverage; native charts/simple pivots; verify behavior and preserve content. |
| [Product Manager](product-manager/expert/README.md) | Frame SAFe-informed product strategy and evaluations, then synthesize checked evidence into business decisions with explicit economic and lifecycle tradeoffs. |
| [Product Owner](product-owner/expert/README.md) | Clarify customer problems, test outcome assumptions, recommend backlog priorities and delivery trade-offs, and review AI product decisions. |
| [Enterprise Architect](enterprise-architect/expert/README.md) | Guide enterprise platforms, portfolio investments and sunsets, capability reuse, emerging-product evaluation, standards and transition gates. Delivery teams implement. |
| [Polars analyst](polars-analyst/expert/README.md) | Audit observational data, assess design and compute defensible descriptive or Welch/paired statistics with uncertainty. |
| [Vendor comparison](vendor-comparison/expert/README.md) | Produce evidence-linked technical comparisons with business weights, anchored scores, mandatory gates and visible uncertainty. |
| [Editorial Director](editorial-director/expert/README.md) | Own one audience-specific argument, message map and visual direction across publications. |
| [Information Designer](information-designer/expert/README.md) | Design factual charts, explanatory infographics and complete auditable Excel tables. |
| [Executive Writer](executive-writer/expert/README.md) | Transform expert evidence into sustained Word reports and executive summaries. |
| [Presentation Designer](presentation-designer/expert/README.md) | Pace executive PowerPoint narratives with native editable charts, tables and text. |
| [Publication Reviewer](publication-reviewer/expert/README.md) | Audit evidence fidelity, writing, design and cross-format coherence using current rendered-image critiques. |
| [Frontend](frontend/expert/README.md) | Integrate accepted contributions into one accessible, self-contained HTML page. |
| [Visual artist](visual-artist/expert/README.md) | Create p5.js/D3 artwork and artistic data experiences, with purposeful optional sensor inputs. |
| [Canvas artist](canvas-artist/expert/README.md) | Make lightweight Canvas/WebGL pieces using the original page specialist contract. |
| [Blender artist](blender-artist/expert/README.md) | Produce editable Blender sources, a preview and a web asset. |
| [Inkscape illustrator](inkscape-illustrator/expert/README.md) | Create editable vector illustrations from a description and optional style; export portable SVG and an Inkscape-rendered preview. |
| [Inkscape-controlled illustrator](inkscape-controlled-illustrator/expert/README.md) | Create editable vector illustrations through a bounded drawing plan, trusted document adapter, and native Inkscape export. |
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
