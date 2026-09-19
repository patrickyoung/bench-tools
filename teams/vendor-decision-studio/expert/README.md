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
The local status published means publication-ready; the command sends nothing.

To re-render explicitly selected prior content with fresh review:

    /absolute/export/expert/bin/rebuild-publication /absolute/old-publication /absolute/new-publication

An optional third argument names comma-separated workers to revise, such as
information-designer,executive-writer,presentation-designer. Selected roles receive
prior specs and critiques in fresh Agent contexts; unselected roles reuse content
with explicit lineage. New facts or weights require a fresh analytical run.
A revision verdict is a completed review, not a tool execution failure.

## Outputs and dependencies

publication/result contains report.docx, report.pdf, report.md, presentation.pptx,
presentation.pdf, comparison.xlsx, SVG/PNG graphics, narrative and review JSON,
comparison-data JSON/CSV, statistics/source-notes JSON, index.md and a hash manifest. Analytical results and all private records remain
separate. Documents contain sustained prose; workbooks retain complete rationales; verbatim source quotes remain in the data JSON;
PowerPoint tables paginate at four candidates per slide. Graphics support up to
12 candidates; effects panels support up to three comparisons and fail explicitly
for more. New encodings require a reviewed renderer extension.
Native criterion tables explain weighted contributions. Statistical interval
slides use editable vector shapes with units, sample counts, uncertainty and
sourced practical thresholds. They are not native chart objects; score bars
remain native charts. Word decision diagrams use native text and table cards.
Every inspected image has target-specific observations, including passes.
Hash-bound production manifests link source, authored specs, final artifacts,
page previews and extracted text. Basic native semantics are inspected; formal
PDF/UA or WCAG certification is not requested or claimed by the default profile.

The shared story owns status_label and eligibility_labels, preserving decision
scope and the evidence basis of each gate across formats. Parallel decision cards
have no automatic sequence numbers; sensitivity uses distinct marker shapes.
Native contribution tables paginate at three criteria and four candidates per
slide for readable type. Exact statistical profiles/tests and gate records remain
in the required audit JSON files, prepared and bound before publication review;
the workbook's five sheets cover the comparison and its scoring evidence.

For a narrowly scoped revision, PUBLICATION_REVISION_BRIEF may select a JSON
object mapping only the named revision roles to instructions of at most 6000
characters each. Each role receives its selected text in inputs/revision-brief.txt.
The brief narrows the assignment; it cannot replace authoritative facts. To
migrate an older story missing qualified status labels, include editorial-director
among the roles to revise. Unchanged downstream content can retain explicit
lineage while being rendered and reviewed afresh.

Required environment: the existing analytical team's selectors plus
PUBLICATION_NODE, PUBLICATION_NODE_MODULES, PUBLICATION_PYTHON,
PUBLICATION_PLOT_PYTHON, PUBLICATION_SOFFICE, PUBLICATION_PDFTOPPM,
PUBLICATION_PDFTOTEXT, PUBLICATION_DOCX_RENDERER and PUBLICATION_SLIDE_SKILL.
Use the selected bundled Artifact Tool/LibreOffice runtime and document/slide
skill helpers. Install matplotlib requirements outside source. The host sets request.production=controller and PUBLICATION_AUTHORING_ONLY=1
for production Agent assignments, which write complete bound specs. The trusted
controller then renders, runs full artifact validation and supplies fresh pixels
for review. The default check still requires artifacts. Standalone production
uses the same sequence: explicitly select that request/environment for authorship,
then run tools/render from the host and bin/check without the authoring flag.
No worker installs packages, executes generated code, calls another agent, edits
its definition or redirects workspace paths to bypass a production boundary.
PUBLICATION_TURNS defaults to 30; PUBLICATION_REVIEW_TURNS defaults to 40. PUBLICATION_TIMEOUT defaults to 15m per Agent
action. Ask image review batches contain at most five pages and have a five-minute
timeout each. Source pins reproduce definitions, not deterministic model judgments.

The workbook is an auditable saved evaluation. Its points formulas expose the
arithmetic; changing scores/weights requires a fresh evaluation. Native Office
package checks and LibreOffice previews do not certify behavior in Microsoft
Office or Google Slides. A model quality pass is not human certification.
