---
name: django-delivery
description: Use when validating changes or preparing release handoffs to cover migrations, CI, environment promotion and operational responsibilities without deployment authority.
---
# From slice to reviewed delivery
Define acceptance criteria with product/design/QA; use short-lived Git branches,
small changes, review and lockfiles. Map each criterion to evidence, a planned
check or a concrete blocker. Report failures; never relabel unrun work as passed.
Use actual PostgreSQL integration tests, transaction tests where semantics matter,
and negative auth/tenant tests. pytest-django DB access is explicit;
--reuse-db requires --create-db after schema changes. Isolate disposable databases
and confirm target before migration/test commands.

CI should run locked dependency sync, Ruff lint/format, compatible types,
PostgreSQL unit/integration tests, security/dependency scans, migration drift
check (makemigrations --check --dry-run), build/static assets, and browser journeys
where needed. Record exact commands, versions and evidence. Scans and
check --deploy are useful signals, not certification. Django system/deployment
checks must run with the relevant safely supplied environment settings. Do not
connect to production merely to run checks.

## Data and release safety
Review SQL, locking and downtime risk. Prefer expand-contract changes: add
compatible schema, deploy compatible readers/writers, batch resumable backfill,
verify, then remove old schema in a later release. Test forward/backward
compatibility, concurrent writers, large data and lock/statement timeouts.
Avoid long migration transactions/backfills; irreversible operations need an
explicit backup/restore and rollback decision. An application rollback may be
insufficient after data transformation. Rehearse on safe representative data.

Build once and promote the same immutable artifact through
local -> DevTest -> UAT -> production, injecting secrets/config at runtime
without rebuilding per environment. Local source iteration precedes this release
candidate; a locally tested candidate must be reproducible and identical on
promotion. Track artifact/lock/image digests and approvals. UAT requires signed-off
journeys by the named owner; never invent sign-off.

## Operational handoff, not automatic infrastructure
Coordinate with DevOps/security/database owners on health and readiness semantics,
migration execution identity, least-privilege runtime DB role, connection budgets,
static/media, TLS/proxy trust, secret rotation and deployment strategy. Supply
structured redacted logs, correlation IDs, actionable metrics/traces, error/latency
SLO hypotheses and actual load evidence where available. Health checks must not
leak secrets or turn a transient dependency failure into a restart storm.

Define backup ownership/retention, restore rehearsal and PostgreSQL PITR,
RPO/RTO expectations, rollback triggers, incident/runbook contacts and release
checklist. Distinguish configuration written from systems provisioned or restored.
Large exports/jobs need workers, bounded resources and monitoring before readiness.
No implicit cloud accounts, listeners, scheduler, production access or deployment.
Blocked credentials/infrastructure require a named next action, not fabricated
integration evidence.

Deliver report.md and the modest CONTRACT.md handoff. Include actual changed
files and relevant evidence, not venvs/caches/secrets/runtime directories. Static
contract acceptance, tested code, visual/user judgment and production/OIDC
integration are separate gates. The caller runs independent application checks
within its selected action boundary; bin/check never runs them.
