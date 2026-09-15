---
name: evidence-design
description: Use when designing bound-data charts, graphs, explanatory infographics and auditable workbook tables from checked findings and a shared publication message plan.
---

# Encode the reader's question
- Identify whether the task is comparison, change, distribution, composition,
  relationship or a decision condition. Choose aligned bars/dots for comparable
  magnitudes, lines only for real ordered progression, distribution displays
  only with supplied distribution evidence, and tables when exact lookup matters.
  A relationship plot does not establish causation. Label hypotheses explicitly.
- Bind every chart to supplied data references through CONTRACT.md. Never retype,
  embellish, impute, interpolate or manufacture values. Keep missing scores
  unknown, distinct from zero and ineligible. Retain population, denominator,
  units, time horizon, weights and eligibility alongside the interpretation.
- Use honest baselines and scales. Bars start at zero; a restricted axis on a
  suitable non-bar plot must be explicit and justified. Align scales for
  comparisons, disclose transformations and avoid dual axes or area/volume
  encodings that exaggerate differences. Do not imply rank from alphabetical
  or arbitrary ordering. Respect supplied precision; display rounding must not
  change the conclusion, and underlying editable values remain intact.
- Treat score bounds as bounds, preference sensitivity as dependence on weights
  or assumptions, and statistical confidence as the specified statistical
  statement. Use separate labels and visual forms. Never turn a score range
  into a confidence interval or proposed weights into approved priorities.

# Explain, then refine
Use a clean focal read with supporting annotation. Put informative titles,
direct labels, units, nearby sources/claim IDs and useful alt text with each
figure. Do not rely on hue alone: combine position, text, symbols or line style.
Specify legible sizes at intended export dimensions and adequate contrast.
Use restrained semantic color consistently with the shared plan.

An infographic should explain a tradeoff, mechanism supported by evidence, or
decision condition. Map each shape, arrow, scale and spatial relation to a
supplied concept; no invented causal arrows, measured architecture or roadmap.
Prefer a few clear value groups, intentional negative space and controlled
shape/line rhythm. Avoid decorative dashboards, generic icon grids, tiny labels,
unmotivated texture and size differences that imply unsupplied quantities.
Keep factual labels editable; supplied illustration principles do not authorize
image generation or converting all text to outlines.

# Publish auditable quantitative detail
Plan a summary-first workbook with clear navigation to complete detailed
scores, evidence and sources. Use professional headers, consistent formats,
aligned numeric columns, explicit units and unknowns, readable widths, sensible
freeze/filter/print behavior where the host supports it, and no merged data
cells that obstruct analysis. Preserve supplied formulas/values and their
provenance; request missing computations from the statistical owner.
Do not silently drop alternatives or detailed evidence to make a clean summary.

Specify editable sources, chart/graph exports and infographic treatment using
only CONTRACT.md's supported content. Retain the declarative master, production
inputs and source mappings through the host. Unsupported output/editability is
a blocker. Review fresh supplied render critiques at final size and in context:
check clipped labels, axes, tables across page breaks and color-independent
reading. Successful file creation or byte checks cannot prove visual craft.

# Publication clarity gates
Keep the shared conditional recommendation intact in the workbook opening,
figures and closing decision path. Use explicit if/then language for alternate
conditions. A status badge must carry its material restriction rather than
overpower it. Write failure gates as failures with supported consequences, not
as compressed lists of positive requirements.

Keep workbook_intro at most 450 characters, identifying the conditional
recommendation and its essential caveat. Put navigation in a short separate
note within the existing supported content; do not add a schema field. If the
renderer cannot separate that note, flag the limitation rather than expanding
the opening or modifying helpers.

When supplied, state signed practical thresholds, units and favored direction
in the practical-importance figure's caption or subtitle; bind their meaning
to the supplied facts and never invent thresholds. A statistical interval and
a missing-score bound are different evidence and need distinct labels.
Separate categorical scenarios use unconnected points, never polylines:
ordering scenario names does not create a continuous trajectory. If the fixed
renderer cannot express the required encoding, report the defect to the
controller instead of claiming that wording repaired the visual.

Ownership headings in infographics must fit their rendered blocks. Prefer
short role names and move qualifications into the detail, preserving material
conditions within contract limits. Keep supplied fact IDs in fact_ids/source
mapping; do not expose raw implementation IDs in reader-facing prose.

Read supplied selected prior review and prior spec, and correct all applicable
material content defects, not merely old bindings. Preserve exact quantitative
detail in the checked workbook/data while making summaries readable. Require
actual fresh renders and independent quality review at final size; authorship
and a file checker alone cannot establish publication quality.

Provenance: Generalized publication review guidance supplied 2026-09-15;
authoring feedback, not Hone learning, certified recovery or new case facts.

# Controller production handoff
When request.json says production=controller, apply all creative and evidence
requirements above to full authorship of output/spec.json. Read all selected
relevant evidence and complete every contracted content element, not just an
outline or old bindings. Bind exact request/input bytes, run the host-supplied
absolute bin/check from the runtime workspace, correct authoring defects and
finish. The host selects an authoring-only check for this phase. Do not invoke
tools/render or claim rendered files exist. The controller subsequently runs
the trusted renderer, full artifact checks and fresh independent image/semantic
review; spec acceptance is not publication readiness. Fresh-render requirements
above are publication gates, not a requirement to render during this phase.

Without that mode, manually operated standalone work retains the default full
bin/check requirement for real artifacts. The authorized host may run the same
exported trusted tools/render after spec authorship. On a rendering permission
failure, report the specific production issue to the controller; do not change
dependencies, redefine renderers, weaken checks or link outside the workspace.

Avoid repeated dumps of duplicated JSON or raw SVG/XML; inspect purposefully
and reserve time to finish the complete authored spec and its check. Conclude
with exactly what was authored, the observed check result and the production
and independent review still owed by the controller, not a readiness claim.

Provenance: Host architecture correction supplied 2026-09-15; not Hone learning.
