---
name: delivery-migration
description: Turn a solution design into thin-slice proof, staged migration, rollback and operational handoffs with accountable owners. Use when planning proof, cutover, rollback or delivery handoffs.
---
# Delivery and migration
Capture compact ADRs: choice, alternatives, evidence, rationale, trade-offs,
requirement/standard references, reversibility and review triggers. Define a
thin vertical slice proving risky assumptions with load/data conditions,
measurable pass/fail acceptance, owner and retained evidence. A planned proof
is not a successful test.

Plan dependencies and accountable handoffs across product, EA, security, data,
platform, engineering, QA and operations. Mark unsupplied personal owners as
role nominations pending. Give CI validation, IaC or managed-platform
configuration, environment separation, secrets handling and drift/release
guidance; implementation teams own actual pipelines and code.

Stage migration with data mapping, cleansing, rehearsal, count/value
reconciliation, in-flight transactions and consumer compatibility. Define
cutover authority, prerequisites, stop/rollback triggers and evidence. Rollback
must handle data writes and side effects as well as application versions;
identify irreversible steps and recovery alternatives before crossing them.
Do not invent dates, capacity or delivery commitments.

Hand over telemetry, alerts, runbooks, support escalation, backup/restore and
service acceptance. Exit/decommission requires migrated consumers, reconciliation,
retention/export, contractual and access cleanup, dependency removal and owner
signoff. No spend/risk/release approval or deployment by this architect.
