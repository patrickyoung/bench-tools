---
name: finops
description: "Use when shaping cost allocation, shared and unallocated cost, unit economics, budgets, capacity floors, licenses, egress, or exit costs."
---

# FinOps and unit economics

Use for cloud/SaaS spend, allocation, showback/chargeback, budgets, unit
economics, commitments, scaling, licenses, egress, or exit cost.

## Procedure

1. Define the decision and scope: provider/product, accounts/contracts, period,
   currencies, business owners, allocation consumers, and supplied cost source.
2. Establish allocation dimensions and ownership: capability, product, service,
   environment, team/cost center, tenant/customer, and required tags or account
   hierarchy. State validation and treatment of missing tags.
3. Separate direct, shared, and unallocated cost. Document each allocation rule,
   driver, source, denominator, residual, owner, confidence, and review cadence.
   Do not hide unallocated cost to make totals appear complete.
   Reconcile every total to its named service/account scope. Distinguish a
   subtotal from the enterprise total and explain excluded services or possible
   overlap rather than silently omitting or double-counting their costs.
4. Select business-relevant units such as transaction, active tenant, order, or
   model inference. Define numerator, denominator, quality/SLO context, period,
   and exclusions. Units support decisions only when volume and service level
   are comparable.
5. Compare options with supplied prices and observed usage. Include licenses,
   commitments, support, people/operations, data transfer/egress, migration,
   portability, contract exit, stranded cost, and uncertainty.
6. Distinguish waste reduction from capacity needed for SLO, resilience,
   security, demand spikes, or regulatory retention. Scale down only above
   explicit capacity and service floors.
7. Set budget/forecast variance triggers and owner actions. Recommend a small
   reversible adjustment with acceptance evidence; do not authorize spend.

## Evidence

Use invoices, billing exports, contracts, usage/volume telemetry, tag coverage,
service levels, capacity tests, license inventories, forecasts, and owner-approved
allocation rules. Never invent prices, discounts, savings, or ROI.

## Common failures

- Treating all untagged spend as zero or arbitrarily charging one team.
- Unit cost without a stable denominator or quality context.
- Cost optimization that violates SLO/capacity floors.
- Ignoring licenses, egress, migration and exit.
- Presenting a forecast or recommendation as a budget approval.
