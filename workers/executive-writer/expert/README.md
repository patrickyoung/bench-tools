# Independent publication definition

This is a thin judgment layer, not an application or a renderer. Its role is
specified in role.json; AGENTS.md and the single skill define its responsibility.
PROVENANCE.md identifies the exact reused knowledge bytes and adaptations.

## Independent invocation
Requires public Agent (and its configured model, recording and confinement
companions), local file/hash utilities, and host-owned CONTRACT.md, bin/check,
dependency selectors and, for production, tools/render. The host selects a new
workspace with request.json and only the relevant regular inputs/ files.
Downstream roles need actual shared-plan and evidence bytes, not references to
another agent's conversation. Definitions and evidence stay outside workspaces.

Operator template (variables must be selected explicitly; not an authoring test):

    agent run -C "$WORKSPACE" -m "$MODEL" -effort high -turns 20 \
      -timeout 15m -evidence "$EVIDENCE" \
      -record-input request.json -record-input "$SELECTED_INPUT" \
      -record-output output/spec.json "$DEFINITION" \
      -- 'Read request.json and selected inputs; follow CONTRACT.md and author output/spec.json.'

Repeat -record-input for EVERY selected inputs/ file; SELECTED_INPUT is one
workspace-relative path, not a directory or glob. Use an explicit supported
provider/model, not an implicit default. Keep stdout, stderr, rendered artifacts,
previews and exit status in the external evidence root as well as Agent's
recordings. Agent's -timeout bounds each action, not the complete multi-stage job. Do not silently disable confinement.
The same invocation is the subagent interface, launched by the external
controller, never by another definition. No network interface is supplied.

## Outputs, checks and limits
The required worker artifact is output/spec.json (bench.publication-spec/v1);
CONTRACT.md owns exact content shape, binding and verdict/render conventions.
Production files and previews come only from the reviewed host renderer.
The host's executable check is authoritative for mechanical acceptance, not
proof of craft, factual truth, business approval or image inspection.

The exported definition includes the host-reviewed contract and production helpers.
Set the PUBLICATION_* runtime selectors described by the decision studio runbook.
No runtime packages or current job data are bundled. Checks run from the selected
workspace. Production roles call the definition's tools/render with the workspace
path; the renderer archives prior owned versions before generating fresh outputs.
When request.production=controller, the host selects PUBLICATION_AUTHORING_ONLY=1
for the Agent assignment. It writes the complete bound spec; the host then runs
this definition's tools/render and the full bin/check without that flag. Default
checks require all artifacts. The reviewer consumes actual fresh image critiques;
it must not invent them. Never redirect workspace paths around a write boundary.

## Host-owned evaluation recipe, not observed results
Positive: in two fresh fictional runtime cases, test a full statistical
comparison and a partial-evidence comparison with proposed weights and untrusted
vendor demands. Carry the same checked evidence/message plan through a complete
editable Word report, executive deck with at least one native editable chart or
table, XLSX tables, chart/graph exports and explanatory infographic. Inspect every
final page, slide and figure, including workbook publication views; pass fresh
bound visual critiques to the independent reviewer and assess package coherence.

Negative: separately change a fact, remove a material caveat or required format,
stale a source/spec/image-review binding, or omit inspection/introduce poor craft.
Each must fail readiness. No run or evaluation result is stored in this definition.
The host records source requirements, fixtures and actual evaluations externally.
Add another specialty by supplying a separate definition and host-selected
contract/dependencies; do not modify Agent or introduce a second runtime loop.
