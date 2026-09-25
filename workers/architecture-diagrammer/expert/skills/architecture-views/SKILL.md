---
name: architecture-views
description: Use when selecting enterprise or solution views. Select and evidence-bind enterprise or solution Mermaid views, preserving architecture role boundaries and decomposing dense maps.
---
# Select the smallest useful story

Start with who must understand what. For executives show capabilities, enterprise
platforms and portfolio lifecycle/transition gates, not a cloud icon inventory.
For solution teams show context, logical containers, named integration intent,
data/trust boundaries and evidenced deployment. Do not conflate business,
logical and physical views. Use matching IDs and a source-qualified attribute
to relate distinct current/target representations.

Read the staged EA or SA JSON handoff alongside selected Markdown/catalogs.
Preserve upstream requirement/control/standard IDs as attributes and source
locators. Conflicts go to their accountable architect via controller, not a
new diagrammer decision. A supplied recommendation remains proposed.

Sketch 6–12 core elements; split by audience, boundary, state or scenario.
Every selected edge needs both endpoints and an intent label. Count relationship
coverage, not just boxes. Use omissions only for explicit out-of-view facts,
never to hide relationships causing layout trouble; another view can carry them.
Keep unknown owners/protocols/data explicit, even when visually inconvenient.

Prefer flowchart LR for linear integration and TB for layered/landscape views;
try Dagre or ELK only through the admitted renderer. Neither engine proves
correctness. Boundaries are named containment, not unproven security guarantees.
Sequence views are focused ordered scenarios, not complete behavioral models.
Separate success, timeout, retry and conflict scenarios when branching cannot be
represented honestly. Distinguish asynchronous send from request and response.
Source every retry/acknowledgment; never infer exactly-once delivery.
Sequence headers show wrapped names/kinds and material state/evidence marks;
full attributes remain visible in frame notes attributed by name and stable ID,
as for boundary attributes. Flowchart non-boundary attributes stay in graph labels.

Author the closed model, compile deterministically, and check arrows against
sources before handing to controller rendering. No arbitrary Mermaid escape hatch.
