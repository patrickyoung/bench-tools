---
name: api-ecosystem
description: "Use when stewarding APIs, events, or schemas as products with owners, partner access, compatibility, migration, and lifecycle."
---

# API ecosystem

Use for integrations, API/event/schema catalogs, reuse, partner ecosystems,
contract standards, compatibility, or retirement.

## Procedure

1. Inventory existing interfaces before proposing new infrastructure. Give each
   API, event, or schema a stable ID, capability, accountable owner, consumers,
   purpose, sensitivity, protocol/format, contract location, lifecycle, support,
   dependencies, and dated review.
2. Define the product contract: consumer outcomes, access/onboarding, semantics,
   examples, errors, idempotency/order guarantees where relevant, service
   promise, quotas, observability, support and feedback.
3. Separate interface contract from implementation. Use a machine-readable
   contract supported by consumers; do not upgrade versions solely because a
   newer specification exists.
4. Specify partner/workload identity, authentication, authorization scopes,
   tenant isolation, key/credential lifecycle, abuse controls, audit and
   offboarding. Escalate legal and commercial terms.
5. Set compatibility policy for additive/breaking change, schema evolution,
   deprecation notice, parallel operation, consumer test evidence, migration,
   and removal. Identify producer and consumer responsibilities.
6. Prefer descriptors for the existing catalog/portal and federated ownership.
   Improve discovery and reuse rather than building another portal app.
7. Define adoption and retirement measures and a review trigger. Sunset only
   after consumer migration and dependency/decommission acceptance.

## Evidence

Use current descriptors/contracts, gateway or broker records, consumer lists,
usage telemetry, compatibility tests, incidents, access reviews, support
records, and owner decisions. Mark undocumented consumers and stale contracts
as risks, not facts.

## Common failures

- API counted as reusable merely because it exists.
- Contract with no owner, lifecycle, consumer, or migration policy.
- Breaking change disguised as a version bump.
- Human partner credentials or tenant boundaries left implicit.
- A new portal application substituted for accurate catalog metadata.
