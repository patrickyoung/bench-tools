---
name: data-ai
description: "Use when defining governed data products or pragmatic AI with contracts, evaluation slices, isolation, human authority, and cost evidence."
---

# Data and AI

Use for data products, mesh/federated ownership, data platforms, analytics,
machine learning, generative AI, or AI governance.

## Procedure

1. Identify the business decision and data/AI capability gap. Include reuse and
   non-AI/do-nothing alternatives; AI is not the default.
2. Define each data product's stable ID, domain owner, consumers, purpose,
   schema/semantic contract, quality measures, lineage, classification, access
   policy, retention, change compatibility, support, and review.
3. Separate domain accountability from platform enablement. Provide
   discoverability, interoperability, policy automation, observability, and
   self-service without transferring data quality ownership to a central team.
4. For AI, define intended and prohibited uses, decision subject, human
   authority/override, failure harm, escalation, logging, privacy, supplier
   boundary, tenant isolation, data/model version, and rollback/stop control.
5. Design evaluation slices representing relevant populations, edge cases and
   harms. Specify baseline, metric, threshold, uncertainty, red-team/adversarial
   evidence where relevant, runtime monitoring, drift/change trigger, and owner.
6. Account for inference/training/license/storage costs and capacity. Pilot with
   bounded data and an adopt/hold/stop gate; never imply correctness from a demo.
7. Express enforceable controls as reviewable specifications and tests where
   appropriate. Humans retain legal, ethical, safety, and business authority.

## Evidence

Use supplied lineage, quality results, contracts, access logs, data
classifications, evaluation datasets/results, incident records, model cards,
supplier terms, cost records, and owner decisions. Never fabricate evaluation,
tenant isolation, interviews, certification, or regulatory compliance.

## Common failures

- A “data product” with no owner, consumer, contract, quality promise, or change
  policy.
- “Mesh” as decentralization without interoperability and decision rights.
- Average AI scores hiding weak slices or harmful failure modes.
- Human-in-the-loop with no actual authority, time, or escalation.
- Vendor benchmark treated as workload proof.
- AI adoption for trend alone.
