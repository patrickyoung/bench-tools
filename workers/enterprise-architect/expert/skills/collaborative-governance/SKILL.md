---
name: collaborative-governance
description: "Use when designing paved roads, peer RFC or ADR decisions, automated controls, explicit decision rights, and expiring exceptions."
---

# Collaborative governance

Use for standards, guardrails, decision rights, exceptions, RFC/ADR practice,
evidence automation, or platform enablement.

## Procedure

1. Identify the decision class, outcome/risk, accountable owner, consulted
   expertise, affected consumers, delegated authority, and escalation path.
   Governance is a decision system, not necessarily a board.
2. Define the smallest useful standard: scope, rationale, safe default, required
   result, evidence, owner, version, effective/review dates, and conditions for
   change. Avoid prescribing implementation where outcomes suffice.
3. Provide a paved road with discoverability, templates/reference patterns,
   tested controls, feedback, support, observability, and a documented escape
   path. Measure adoption friction and outcomes.
4. Use peer RFCs for consequential proposals and ADRs for accepted local
   decisions. Capture context, options including no-op, trade-offs, decider,
   status, consequences, and review trigger. Do not use documents as ceremonial
   approval queues.
5. Automate deterministic controls where valuable. Specify pass/fail semantics,
   positive/negative fixtures, CI placement, evidence retention, ownership, and
   failure escalation. Hand executable implementation to delivery/platform
   owners.
6. Make exceptions explicit: bounded scope, rationale, risk owner, compensating
   controls, evidence, expiry, remediation, and renewal criteria. Test expiry
   and keep an auditable record.
7. Review standards against outcomes, incidents, false positives, delay, and
   consumer feedback; retire controls whose costs exceed demonstrated value.

## Evidence

Use decision records, policy/test results, exception registers, change and
incident history, platform adoption/onboarding data, audit evidence, consumer
feedback, and named owner decisions.

## Common failures

- Defaulting to a central architecture board or universal sign-off.
- “Best practice” with no context, owner, evidence, or expiry.
- Policy code without fixtures, CI behavior, exception path, or delivery owner.
- Paved road as mandate without support or feedback.
- Architecture advice presented as authorization.
