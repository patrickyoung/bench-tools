---
name: cloud-edge
description: "Use when choosing hosting, managed service, SaaS, build, cloud-native, distributed, or edge approaches without fashion-driven prescriptions."
---

# Cloud and edge strategy

Use for hosting placement, managed/SaaS/build choices, cloud-native patterns,
microservices, serverless, mesh, distributed locations, or edge constraints.

## Procedure

1. Establish workload outcomes and constraints: consumers, latency, availability,
   data location, connectivity, offline behavior, safety, scale pattern,
   integration, skills, support horizon, and exit requirements.
2. Compare do nothing/reuse, SaaS, managed platform, hosted product, and build.
   Assess control needed, differentiation, operating burden, portability,
   contract constraints, total cost evidence, and reversibility.
3. Choose architecture characteristics before named technology. Justify
   decomposition by independent change, scaling, ownership, fault isolation, or
   policy need. A modular monolith or purchased service may fit better.
4. For edge/distributed operation, specify intermittent connectivity,
   store-and-forward/synchronization, identity and key lifecycle, remote
   observability, fleet update/rollback, physical exposure, data consistency,
   degraded mode, and central/edge decision rights.
5. Use serverless, containers, microservices, or service mesh only where their
   value exceeds complexity. Define service contracts, ownership, SLO/capacity
   floors, recovery, and failure boundaries.
6. Set proof criteria and migration/exit gates. Hand implementation choices to
   accountable platform and delivery teams.

## Evidence

Use measured demand and latency, outage history, data classifications,
connectivity observations, contracts, support matrices, skills inventory,
architecture dependency evidence, bills, and pilot results. Treat vendor
statements and untested prices/specifications as unverified.

## Common failures

- “Cloud native,” microservices, mesh, or serverless as goals.
- Ignoring egress, licenses, operations, skills, or exit portability.
- Assuming reliable connectivity or strong consistency at the edge.
- Central identity with no disconnected/degraded design.
- Detailed technology prescription before outcome and constraints.
