# Vendor decision studio

General reusable comparison and publication team. All nine members are independently
exportable workers. The Product Manager acts twice: framing and decision synthesis.

## Flow and responsibility

1. Product Manager frames business need, criteria and decision boundaries.
2. Polars Analyst tests admissible data and states inference limits.
3. Vendor Comparison applies weights and produces a cited matrix.
4. Product Manager synthesizes the recommendation. Product Owner reviews it.
5. Editorial Director creates one argument, message map and visual direction.
6. Information Designer specifies charts, an explanatory infographic and workbook.
7. Executive Writer authors a complete Word report. Presentation Designer creates
   the executive deck with native editable evidence.
8. The Publication Reviewer's expertise inspects every actual rendered image through
   Ask. A separate Agent context audits the complete argument and those critiques.
   All rubric dimensions must be >=4/5, with no material defects, to publish.

## Run

After exporting the team at an explicit full Git commit and selecting dependencies:

    /absolute/export/expert/bin/compare-studio /absolute/job.json /absolute/new-run

Add --offline to prohibit analytical source fetching. The model provider still
requires its configured connection. The analytical input contract is unchanged
from vendor-comparison-team; reuse its business brief, candidates and materials.
Set PUBLICATION_PROFILE to an explicitly selected JSON file with audience and/or
brand text. Defaults request an executive package with supporting long-form detail.
Brand text can guide palette and wording; template/logo ingestion is not implemented.

To publish an explicitly selected previously completed, checked analysis:

    /absolute/export/expert/bin/publish-decision /absolute/checked-analysis /absolute/new-publication

An optional third argument is the profile JSON. This produces fresh publication
workspaces and records, without rerunning or claiming to rerun the earlier analysis.
Check with PUBLICATION_RUN=/absolute/new-publication /absolute/export/expert/bin/check.
Exit 0 means the publication gates passed. Exit 2 preserves a needs-revision package.
Other failures retain their stage records. No automatic retry or weakened gate.

## Outputs and dependencies

publication/result contains report.docx, report.pdf, report.md, presentation.pptx,
presentation.pdf, comparison.xlsx, SVG/PNG graphics, narrative and review JSON,
index.md and a hash manifest. Analytical results and all private records remain
separate. Documents contain sustained prose; workbooks retain complete evidence;
PowerPoint tables paginate at four candidates per slide. Graphics support up to
12 candidates; effects panels support up to three comparisons and fail explicitly
for more. New encodings require a reviewed renderer extension.

Required environment: the existing analytical team's selectors plus
PUBLICATION_NODE, PUBLICATION_NODE_MODULES, PUBLICATION_PYTHON,
PUBLICATION_PLOT_PYTHON, PUBLICATION_SOFFICE, PUBLICATION_PDFTOPPM,
PUBLICATION_PDFTOTEXT, PUBLICATION_DOCX_RENDERER and PUBLICATION_SLIDE_SKILL.
Use the selected bundled Artifact Tool/LibreOffice runtime and document/slide
skill helpers. Install matplotlib requirements outside source. No worker installs
packages, executes generated code, calls another agent or edits its definition.
PUBLICATION_TURNS defaults to 20. PUBLICATION_TIMEOUT defaults to 15m per Agent
action. Ask image review batches contain at most five pages and have a five-minute
timeout each. Source pins reproduce definitions, not deterministic model judgments.

The workbook is an auditable saved evaluation. Its points formulas expose the
arithmetic; changing scores/weights requires a fresh evaluation. Native Office
package checks and LibreOffice previews do not certify behavior in Microsoft
Office or Google Slides. A model quality pass is not human certification.
