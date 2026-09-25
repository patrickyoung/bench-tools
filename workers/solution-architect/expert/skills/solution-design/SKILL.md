---
name: solution-design
description: Design end-to-end components, interfaces, data ownership, deployment and trust boundaries for a bounded solution. Use when producing a substantive end-to-end solution design.
---
# Solution, integration and data
Use Simon Brown's C4 method proportionally: context for actors/systems, container
for responsibilities; key sequence/data flow and deployment/trust views where
they answer a decision. Mermaid/Markdown is sufficient. Label protocol, trust
crossing, identity and data class; views must agree with ADRs and controls.
A box named database is not a data ownership decision.

Specify source of truth, ownership, relational/document/object/analytics fit,
classification, retention/deletion, residency, access and lineage. Select
API/event/batch based on latency, coupling, volume, connectivity and operations.
For interfaces name producer/consumer, auth, schema/version evolution and
compatibility. OpenAPI/AsyncAPI provide explicit contract methods, not guaranteed
delivery behavior; choose platform-supported versions only.

Address relevant quotas, timeout/retry budgets, idempotency/deduplication,
dead-letter/replay, backpressure, ordering, consistency and reconciliation.
Do not prescribe each mechanism when irrelevant; explain material exclusions.
For intermittent field connectivity, make offline authorization/data protection,
local queue limits, sync checkpoints, conflicts, human adjudication and recovery
explicit. Do not silently require continuous cloud access or last-write-wins.

Trace R-/C-/S-/D- IDs through design sections and key flows. Expose unresolved
interfaces and service ownership as next-action gates. Keep current vs target
and tested vs proposed behavior distinct; no implementation or deployment.
