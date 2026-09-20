---
name: django-foundations
description: Use when planning, bootstrapping or changing Django 6.1.1 projects to establish reproducible Python/PostgreSQL setup, simple domain design and measured ORM behavior.
---
# Foundations and data
Read request and repository first. Separate observed state from desired setup.
For existing apps preserve patterns unless a measured problem justifies change.
For new apps use a modular monolith with domain apps and a custom user configured
before first migration; never retrofit user model lightly after deployed data.

## Reproducible development
- Inspect Git root/status/diff before initializing. Initialize only a new selected
  application directory when authorized. Agree a short-lived branch; preserve
  uncommitted work. Keep changes small with reviewable rationale.
- Pin initial Django==6.1.1. Refresh security advisories and propose subsequent
  patch updates for approval. Verify compatible Python 3.14 latest patch,
  PostgreSQL 18 supported patch and psycopg 3, plus allauth, typing and test
  dependencies; no speculative installed-version claims. Document fallback
  tradeoffs and blockers. No Python 3.15 prerelease default.
- Put .python-version, pyproject.toml and uv.lock in the app. Use uv sync --locked;
  record exact versions and actual resolution/sync status. Pin direct dependencies,
  lock transitives; pin container image versions/digests. Never use floating latest.
- Supply .gitignore, placeholder-only .env.example, editor config, concise setup
  commands and safe secrets guidance. Exclude .env, venv, caches and credentials.
  Use Ruff lint/format, compatible mypy/django-stubs or justified typing alternative,
  pytest-django, and browser tests for journeys that need them.
- Local DB is PostgreSQL via Compose with healthcheck or an explicitly supplied DB.
  Bind local services to loopback. Use least-privilege credentials, persistent dev
  volume if appropriate, separate isolated disposable PostgreSQL test DB with a
  bounded test-only role. Never reuse shared or production DB for tests.
  Refuse ambiguous destructive DB targets; no default SQLite even tests.
- Document commands for DB startup/readiness, locked install, migrations, test DB,
  tests and server startup. Distinguish prepared commands from execution; never
  claim provisioning, migrations or successful tests that were not run.

## Models and services
Use database constraints for invariants; choose on_delete deliberately. Django
6.1 database-level cascade bypasses delete signals: audit effects before use.
Model forms validate input; services encapsulate justified multi-model business
operations, not a universal repository wrapper. Keep transactions short and atomic.
Design concurrent writes with constraints, conditional updates/locking and tested
idempotency; retries cannot double-charge or duplicate effects. Include object/
tenant scope in every query, relation chooser, export, action and background job.

Measure query counts for lists/admin and representative datasets. Use
select_related/prefetch_related, EXPLAIN, indexes fitting actual filters/orders,
bounded querysets and stable pagination with tie-breaking keys. Avoid N+1s and
unbounded exports/uploads; stream or background large work with resource limits.
Profile before caches, replicas or partitioning. Cache keys include user/tenant/
permission dimensions where needed; specify invalidation and prevent private
responses entering public caches.

Synchronous transactional code is the default. Async transactions remain
unsupported: keep transaction work in one synchronous function, use sync_to_async
only at an actual async boundary. CONN_MAX_AGE=0 in async deployments; otherwise
choose connection lifetime deliberately. psycopg pools require pool support and
are per process/alias: budget total web+worker+replica connections and headroom,
do not casually combine persistent connections and pools.

Django native tasks define/enqueue work; they do not supply durable production
execution. Select an actual backend/workers with ownership and monitoring. Use
after-commit dispatch or outbox for stronger guarantees; specify retries/backoff,
dedupe/idempotency, timeouts, poison-job handling and visibility. Immediate backend
is not durable production work. Adopt new fetch modes/mailers only for a reason
and after testing pinned dependencies.
