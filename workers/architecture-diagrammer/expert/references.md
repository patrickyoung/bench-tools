# Method provenance and original synthesis
Build research date: 2026-09-22. This definition distills primary-source research
and reviewed enterprise/solution worker contracts supplied during authoring.
No private source paths, organizations or case facts are runtime dependencies.
These citations are method provenance, not claims of browsing during a job.

- Mermaid CLI, https://github.com/mermaid-js/mermaid-cli — local editable source
  plus SVG/PNG is useful for review. Selected CLI 11.17.0 resolves Mermaid
  11.17.2 and Puppeteer 25.11.0 in the tested runtime. Dependency metadata,
  documented public renderMermaid API and implementation were inspected at
  authoring. Lock graph, browser and fonts remain operator responsibilities.
- Mermaid layouts, https://mermaid.js.org/config/layouts.html — ELK can help
  compound graphs; no engine guarantees clarity. Core 12 guidance must not be
  assumed for CLI 11.x. Use CLI's bundled ELK registration; validate real renders.
- Flowchart, https://mermaid.js.org/syntax/flowchart.html and sequence,
  https://mermaid.js.org/syntax/sequenceDiagram.html — stable primitives suffice
  for restrained architecture views. Explicit entity escaping and constrained
  generation are preferable to executing source-provided Mermaid.
- Accessibility, https://mermaid.js.org/config/accessibility.html — accessible
  title/description complement, not replace, readable labels and prose alternative.
- C4 Mermaid syntax, https://mermaid.js.org/syntax/c4.html — experimental notation
  and styling limits favor stable flowchart primitives here. Architecture-beta
  (https://mermaid.js.org/syntax/architecture.html) focuses resource topology and
  is not a universal capability/portfolio language. Neither is defaulted here.
- Simon Brown, C4 model and diagram review checklist,
  https://c4model.com/diagrams/checklist — audience, abstraction, directional
  intent, meaningful boundaries and notation keys guide this original synthesis.
  C4 guidance attribution: Simon Brown, CC BY 4.0. No copied diagram/checklist.
- Structurizr DSL/export, https://docs.structurizr.com/dsl and
  https://docs.structurizr.com/export/mermaid — a model plus multiple views helps
  governance at estate scale; an exporter does not guarantee identical visual
  quality. This worker intentionally uses a smaller closed JSON contract and
  never executes DSL includes/scripts/plugins.
- D2 layouts, https://d2lang.com/tour/layouts/, and PlantUML deployment,
  https://plantuml.com/deployment-diagram — alternatives for layout control or
  formal notation are escalation candidates, not installed backends or claims
  of comparative performance. Check runtime/licensing and authority separately.

Role synthesis: EA retains strategy, platforms, capability/portfolio lifecycle,
standards and transition gates. SA retains end-to-end solution design,
requirements/control traceability and delivery handoffs. Diagramming translates
their selected artifacts without inheriting their design/approval authority.
“2026” denotes disciplined current methods, not invented future standards.
