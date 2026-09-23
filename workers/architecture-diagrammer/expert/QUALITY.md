# Independent production review — v1

Controller owns this rubric and review storage outside worker-writable paths.
Review exact immutable input/model/source/SVG/PNG hashes. A worker cannot approve
its own output by writing a receipt. Record each applicable criterion as
satisfied, violated or insufficient-evidence, with concrete source locator and
artifact/view/pixel finding, severity and next owner/gate. No confidence percent,
blanket aesthetic approval or averaging away hard failures. Record not-applicable
with rationale. Also look for defects beyond the named criteria.

All source criteria and readability criteria are hard where applicable. A
provisional figure may display genuinely bounded unknowns, but not conceal a
materially false arrow. Needs-input is judged on S10, not absent pixels.

| ID | Atomic criterion | Positive / negative contrast |
|---|---|---|
| S01 | Audience, decision and scope match request | focused executive map / unexplained implementation graph |
| S02 | Each material entity is entailed or explicitly qualified by exact source | proposed platform / proposal drawn as existing |
| S03 | Each arrow has faithful endpoints, direction and intent | response returns to caller / convincing reversed flow |
| S04 | Protocol and data qualifiers preserve source limits | unknown shown / invented TLS, payload or exactly-once |
| S05 | Ownership, lifecycle, disposition, gates and control IDs retain authority | retirement conditional on migration / retirement shown approved |
| S06 | Type and abstraction remain correct | capability separate from application, logical container not Docker / conflation |
| S07 | State and trust/containment boundaries remain faithful | named trust zone with sourced membership / imagined security perimeter |
| S08 | All material chosen-scope facts are covered across views or honestly excluded | explicit cross-view edge coverage / dense relationships silently dropped |
| S09 | Dynamic scenario preserves order, conditions, failures and uncertainty | named timeout scenario / happy path claims exhaustive behavior |
| S10 | Evidence conflict/status/questions/next action are honest | blocked with decisive authority question / invented resolution |
| S11 | IDs and vocabulary align across views and upstream JSON/Markdown | same service ID / renamed duplicate changes meaning |
| S12 | No unsupported redesign, approval, certification or notation claim | C4-informed context / generic graph called formal ArchiMate |
| V01 | Title, scope, state, status and material caveats readable in standalone export | framed complete figure / caveat only in accompanying prose |
| V02 | Every label and arrowhead is visible, unclipped and unambiguous | readable receiver / arrow obscured by boundary |
| V03 | Labels do not overlap nodes, lines or other labels in a meaning-changing way | separated edge intent / ambiguous crossing labels |
| V04 | Intended-size typography is readable without zoom | usable document/slide / shrunken hairball |
| V05 | Decomposition has clear focus and cross-view navigation | named linked views / arbitrary truncation |
| V06 | Key explains actual shapes, line/message types and qualifiers | textual supplied/proposed/unknown / color-only uncertainty |
| V07 | Text contrast, calm light palette, spacing and hierarchy support scanning | restrained editorial enterprise style / dark tiny text or visual clutter |
| V08 | SVG title/description and prose alternative convey essential meaning | faithful flow prose / filename as alternative |
| V09 | SVG and PNG preserve the same labels, direction and framing | matching exports / raster silently cuts off caveat |
| V10 | Supplied long/punctuated labels are faithfully displayed | escaped literal ampersand / parser entity text or injected directive visible |
| P01 | Controller actually ran admitted local renderer on current source/config | retained command, runtime/lock/binary hashes / forged receipt |
| P02 | Independent semantic and pixel review binds exact latest bytes | actual inspected PNG hashes / stale review after layout edit |

## Review record (controller-owned, not a new artifact protocol)
Retain reviewer identity and independence, time, intended medium and physical/
pixel size, exact request/profile/definition/input inventory hashes, model,
manifest, render receipt, all .mmd/.alt.md/.svg/.png hashes, observed runtime and
controller invocation, and rubric file hash. For every criterion retain result,
evidence and concrete finding. Inspect actual PNG pixels at intended presentation
or document size and SVG at native size to diagnose vector issues. Compare
visible labels and arrows with model and source, not merely generated alt text.
If no image-capable reviewer is available, V criteria are insufficient-evidence;
do not replace perception with an LLM text summary. No paid judge is required.

Controller supplies concrete findings back to worker, retaining original review
outside worker paths. Any change in inputs, model, source/layout, config, renderer,
fonts/runtime or final pixels requires fresh affected source and pixel review.
All applicable hard criteria must be satisfied for production acceptance; unresolved
bounded source unknowns are accepted only as clearly provisional, with an explicit
accountable gate. Organizational approval/publication remains a separate human
decision even after full review.
