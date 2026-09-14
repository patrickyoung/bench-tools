---
name: security-design
description: "Use when guiding zero-trust identity, resource, policy, supplier, data-access, compliance-evidence, and exception decisions."
---

# Security by design

Use for trust boundaries, access, sensitive data, platform guardrails, supplier
risk, compliance mapping, or exceptions.

## Procedure

1. Identify protected resources, data classifications, actors/workloads,
   locations, actions, harms, accountable risk owner, and applicable supplied
   obligations. Do not begin with a network perimeter alone.
2. Map identity lifecycle and strength for people, workloads and devices:
   issuance, authentication, authorization context, least privilege, separation,
   rotation/revocation, recovery, privileged access, and audit evidence.
3. Define policy decisions and enforcement points close to each resource.
   Include default deny where justified, session/change context, segmentation,
   encryption/key ownership, telemetry, failure mode, and emergency access.
4. Trace each obligation to a control objective, implementable specification,
   evidence producer, evidence consumer, cadence/trigger, exception owner, and
   acceptance criterion. Mapping is not certification.
5. Assess suppliers using supplied contracts and evidence: data use/location,
   subprocessors, incident notification, access, assurance, support, continuity,
   portability, deletion, exit, and concentration risk. Mark claims unverified.
6. Design exceptions with narrow scope, rationale, compensating controls,
   accountable approver, evidence, expiry, and remediation. Expiry must be
   testable and auditable.
7. Recommend a prioritized, reversible risk treatment and escalate legal,
   regulatory, privacy and risk acceptance to authorized owners.

## Evidence

Use current asset/data inventories, threat analyses, identity/access records,
control tests, incident history, contracts, attestations, exception records,
logs and owner decisions. State source dates, coverage, and gaps.

## Common failures

- “Zero trust” treated as a product, network redesign, or slogan.
- Controls with no resource, decision, enforcement point, or evidence.
- Supplier marketing accepted as assurance.
- Permanent exceptions or expiry with no enforcement.
- Claiming compliant, secure, certified, or risk-accepted from architecture
  prose or a checker result.
