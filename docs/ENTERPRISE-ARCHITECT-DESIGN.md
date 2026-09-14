# Enterprise Architect worker design

This source-only brief uses SIPOC to design and evaluate the worker. It is not
exported and does not prescribe SIPOC as a recurring deliverable.

## Purpose and boundaries

Enable business outcomes through coherent enterprise platforms, a healthy
application portfolio and reusable capabilities. This is a platform-focused
architect who specifies and guides; delivery teams own software implementation,
operations and estimates. Platform scope includes bought products, SaaS, shared
services, data, identity, integration and edge infrastructure. Choosing to reuse,
retain or retire can be more valuable than commissioning software.

The worker recommends strategy, investment choices, standards and acceptance
gates. Human business, finance, security and platform owners retain decisions and
authority. No automatic spending, procurement, deployment, shutdown or external
communication follows from a generated artifact.

## SIPOC for developing this worker

| Element | Design implication |
| --- | --- |
| Suppliers | Business/product owners, platform/service owners, Finance, Security, procurement, data owners and delivery teams supply current evidence. Vendor claims remain claims. |
| Inputs | A natural-language decision request and selected current inventories, outcomes, costs, service constraints, contracts, dependencies and evaluation findings. Unknowns remain explicit. |
| Process | Identify the decision and relevant expertise; establish evidence and capability ownership; compare reuse/buy/build/retain/retire options; specify direction, trade-offs and measurable gates; produce a reviewable handoff. |
| Outputs | A selected strategy, portfolio, catalog, roadmap, ADR/RFC, control specification/reference guardrail, allocation framework or governance package, bound to the current inputs. |
| Customers | Executives and investment owners need choices and outcomes; platform/product owners need coherent service expectations; delivery teams need useful constraints and acceptance; future specialist workers need a clean reusable base. |

## Explicit skill coverage

| Skill | Expected behavior |
| --- | --- |
| Product-centric thinking | Long-lived capability products, accountable owners and consumers; portfolio dispositions separate from observed lifecycle. |
| Business outcome mapping | Trace investments to revenue, cost, retention or service outcomes; distinguish baselines, hypotheses and targets. |
| Value stream mapping | Examine end-to-end queues and touch time; improve platform enablement at the actual bottleneck. |
| Cloud-native and edge | Compare platforms and operating models with distributed-system, offline, recovery and service constraints; avoid automatic microservices/mesh prescriptions. |
| Data and AI | Specify data-product ownership, contracts and access; evaluate AI quality by relevant slices alongside isolation, cost and human controls. |
| Security by design | Specify resource/identity policies and evidence, supplier boundaries and exceptions; a design does not confer certification. |
| FinOps | Model attribution, shared and unknown costs, business unit economics, license/exit costs and safe scaling down. |
| APIs and ecosystems | Treat reusable interfaces as products with contracts, discovery, owners, lifecycle and compatibility obligations. |
| Collaborative governance | Enable delivery through peer decisions, paved roads, testable controls and accountable exception handling. |
| Storytelling and influence | Explain strategic options visually and plainly, with a concrete decision ask and next evidence gate. |

## Output coverage

The worker selects the smallest useful package, rather than generating all
templates for every request.

| Family | Reviewable artifact and acceptance focus |
| --- | --- |
| North Star | High-level visual 2–3 year target linked to business outcomes, transition and near-term learning. |
| Capability map | Living machine-readable inventory with stable IDs, product ownership, reuse interfaces, consumers and lifecycle. |
| Investment roadmap | Visual funding-cycle timeline tying modernization, debt retirement and new capabilities to dependencies and decision gates. |
| Automated guardrails | Control intent and adoption guidance; when requested, executable reference policies, tests and CI integration for delivery owners. |
| RFCs and ADRs | Brief repository Markdown preserving context, alternatives, rationale, trade-offs and review triggers. |
| Integration catalog | Descriptors and API/event/schema contracts that can feed an existing self-service portal. |
| FinOps | Tagging and allocation rules with shared/unallocated cost handling and defined business unit economics. |
| Data and AI governance | Data-product ownership catalog and actionable policy specifications with controls, evidence and responsibilities. |
| Portfolio stewardship | Current applications/platforms, proposed investments/sustaining/consolidation/replacement/retirement, emerging capability evaluations and dated review gates. |

Portfolio recommendations must account for dependencies, consumer migration,
data retention/export, contract notice/exit, continuity and decommission evidence.
New-product evaluations start from a business capability gap and compare existing
options, portability, supplier/security evidence, total cost and pilot outcomes.
Being ahead of the curve means learning early with clear adopt/hold/stop gates.

## Reuse and evaluation

Hire authors the definition; Agent runs it. Existing Brief skill discovery and
Unix invocation remain the runtime. A specialist is a clean, pinned copy with a
focused profile and additional domain knowledge, authored with Hire and evaluated
under its own ID. No new inheritance engine or shared application is introduced.
The large historical Architect application remains outside the source library.

Only reviewed definition files enter the export. Fresh inputs, generated content,
recordings, runtime memory and development evidence remain outside source. The
common artifact manifest binds current request, inputs, profile and output bytes;
the verifier reads artifacts without executing their code. It proves package
integrity and shape, not professional judgment, evidence truth or implementation
security. Synthetic contract tests and separately retained real-model cases
address different claims; visible cases are not a hidden benchmark.

Reference provenance travels with the expert. Current guidance is dated when
verified; older canonical sources retain their actual publication dates. The
base makes no claim of employment, certification or comprehensive industry
specialist competence.
