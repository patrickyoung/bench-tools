# Architecture Diagrammer

[Home](../README.md) · [Workers](../workers/README.md)

The [Architecture Diagrammer](../workers/architecture-diagrammer/expert/README.md)
turns selected architecture evidence into consistent Mermaid views and local SVG
and PNG figures. It is an experimental independent specialist. Enterprise
Architect retains strategy, platform and portfolio decisions; Solution Architect
retains solution design and standards traceability. The diagrammer preserves
those decisions, their provisional status and their source qualifications.

## Handoff and production

Stage a current request and only the relevant architect handoff, narrative,
platform/catalogue and standards files under a separate workspace's `inputs/`.
Specify the audience, question, intended medium and selected source authority.
Copied manifests describe the original workspace; they do not authorize scanning
sibling runs or certify missing artifacts. The worker can also use a standalone
architecture brief without another worker.

Agent authors an evidence-linked model and compiles editable Mermaid, prose
alternatives and cross-view coverage. The controller renders with explicitly
selected local dependencies under Cage, checks the bound outputs, and reviews
actual images against the worker's [quality rubric](../workers/architecture-diagrammer/expert/QUALITY.md).
Source/model agreement and successful rendering are separate from architectural
truth, legibility and organizational approval. Changed figures need fresh review.

Use focused enterprise landscape/capability/transition views, solution boundary
and integration views, or named dynamic scenarios. Dense maps should be split
without silently dropping relationships. Current, proposed and unknown facts
remain distinguishable. Conflicting decisive authority produces questions rather
than a fabricated definitive diagram. Generic flowcharts are not certified C4,
UML or ArchiMate models.

## Tool selection, researched September 2026

| Tool | Appropriate use and limits |
|---|---|
| [Mermaid CLI](https://github.com/mermaid-js/mermaid-cli) | Implemented default: editable text plus local SVG/PNG. The selected CLI 11.17.0 uses Mermaid 11.17.2; pin actual packages, browser and fonts. Current core 12 documentation is not automatically the CLI's feature set. |
| [Mermaid layouts](https://mermaid.js.org/config/layouts.html) | Explicit Dagre or ELK for supported graph views; actual layout must be rendered and reviewed. Mermaid's [C4 syntax](https://mermaid.js.org/syntax/c4.html) remains experimental, so this worker uses mature flowchart and sequence primitives. |
| [Structurizr DSL](https://docs.structurizr.com/dsl) | Consider for a long-lived governed architecture model with multiple views. [Mermaid export](https://docs.structurizr.com/export/mermaid) is available, but layout/style equivalence needs validation. Not an implemented backend here. |
| [D2](https://d2lang.com/tour/layouts/) | Consider when richer layout control is needed; engine capabilities and licensing differ. Not installed or benchmarked here. |
| [PlantUML](https://plantuml.com/deployment-diagram) | Consider for established UML/deployment notation requirements. Separate runtime and input admission are needed; not an implemented backend here. |

No diagram tool establishes source accuracy by itself. The selection favors
Mermaid portability and an auditable rendering workflow; it does not claim a
universal tool ranking. [Simon Brown's C4 checklist](https://c4model.com/diagrams/checklist)
informs the original review method (CC BY 4.0 attribution), alongside the worker's
[dated method references](../workers/architecture-diagrammer/expert/references.md).

Follow the worker README for exact authoring, rendering and review commands.
Dependencies, current cases, images and controller records stay outside the source
library. [Library exports](WORKER-LIBRARY.md) pin reviewed source deliberately;
existing jobs never update automatically.
