---
name: quality-security-operations
description: Specify measurable quality attributes, threat and failure controls, and accountable operations for a solution. Use when defining acceptance targets, risk controls or support handoffs.
---
# Quality, security and operations
Use Azure/AWS Well-Architected trade-off lenses where relevant, not as company
policy. For material availability/SLO, performance, capacity, RTO/RPO,
security/privacy, accessibility, supportability, cost and sustainability define
load/environment, measure/target basis, evidence method and owner. Mark other
attributes not applicable with rationale; avoid fake precision. Compare proposed
targets with platform capability evidence and cost/complexity.

Apply NIST SP 800-207's resource/identity trust principle: network location is
not authorization. Show identity federation, least privilege, human/service
authn/authz, tenant isolation, network zones and trust crossings. Specify
encryption, key/secrets ownership/rotation, data controls, audit access and
retention. Use scoped supported OWASP ASVS control IDs when useful, not an
unsupported compliance certification. Threat-model abuse and failure modes:
outage, overload, dependency loss, credentials, data disclosure/corruption and
recovery; tie mitigations to C- IDs and tests.

Use OpenTelemetry's signals lens for traces/metrics/logs and safe correlation,
without logging secrets or personal data by default. Telemetry is not an SLO:
name indicators, alert threshold provenance, routing/on-call ownership,
runbooks, failure drills, backup restore proof and support escalation.
Record design intent separately from completed tests and retained evidence.
