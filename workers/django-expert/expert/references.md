# Dated primary references
Evidence date: **2026-09-19**, supplied research verified by authoring host.
No browsing or dependency compatibility execution during definition authoring.
Latest pages may change; refresh versions, advisories and plugin compatibility
at real kickoff. Recommendations are defaults, not claims of universal popularity.

- Django 6.1.1 (2026-09-02): related-field admin display and mixed DB/Python
  deletion fixes. Exact requested initial pin.
  https://docs.djangoproject.com/en/6.1/releases/6.1.1/
- Django 6.1: Python 3.12–3.14; fetch modes, DB deletion (DB_CASCADE bypasses
  signals), mailers require deliberate evaluation.
  https://docs.djangoproject.com/en/6.1/releases/6.1/
- Python 3.14: modern supported language; free threading/JIT not a default need.
  https://docs.python.org/3.14/whatsnew/3.14.html
- Django PostgreSQL >=15, psycopg 3 recommended; pools per process/alias.
  https://docs.djangoproject.com/en/6.1/ref/databases/
- PostgreSQL major support lasts five years. Recommend supported 18 patch when
  available, verify exact patch/image and use same major across environments.
  https://www.postgresql.org/support/versioning/
- Profile ORM, EXPLAIN, relation loading and indexes before scaling machinery.
  https://docs.djangoproject.com/en/6.1/topics/db/optimization/
- Async transactions unsupported; sync transactional boundary, async persistent
  connections disabled.
  https://docs.djangoproject.com/en/6.1/topics/async/
- Native tasks define/enqueue, not a durable production execution engine.
  https://docs.djangoproject.com/en/6.1/topics/tasks/
- Admin is internal/model-centric; permissions do not imply tenant isolation.
  https://docs.djangoproject.com/en/6.1/ref/contrib/admin/
- Native partialdef/partial and fragment loader syntax since 6.0.
  https://docs.djangoproject.com/en/6.1/ref/templates/language/
- Native CSP middleware, report-only and nonces; test cache interactions.
  https://docs.djangoproject.com/en/6.1/howto/csp/
- Deployment checks: secrets, hosts, HTTPS, cookies, static/media, DEBUG/logging.
  https://docs.djangoproject.com/en/6.1/howto/deployment/checklist/
- allauth OIDC: explicitly enable PKCE, verify chosen release compatibility.
  https://docs.allauth.org/en/latest/socialaccount/providers/openid_connect.html
- Social-only is not protection for every route; safe linking/provisioning.
  https://docs.allauth.org/en/latest/socialaccount/configuration.html
- Admin bypasses allauth by default; secure_admin_login must cover every site.
  https://docs.allauth.org/en/latest/common/admin.html
- OAuth security BCP: authorization code + PKCE, exact redirects/replay defenses.
  https://www.rfc-editor.org/rfc/rfc9700.html
- uv project locking and locked sync.
  https://docs.astral.sh/uv/guides/projects/
- Ruff integrated lint/format, align Python target.
  https://docs.astral.sh/ruff/
- pytest-django explicit DB access and transaction tests; reuse-db schema caveat.
  https://pytest-django.readthedocs.io/en/latest/database.html
- Playwright pytest browser integration; use real journey/manual review too.
  https://playwright.dev/python/docs/test-runners
- htmx progressive enhancement, CSP-safe settings, private history and focus.
  https://htmx.org/docs/
- CSRF on unsafe htmx requests; prefer core versioned native-partial docs over
  older package advice.
  https://django-htmx.readthedocs.io/en/latest/tips.html
- WCAG 2.2 AA target; keyboard, labels, focus, contrast, recovery and status.
  https://www.w3.org/WAI/WCAG22/quickref/

Engineering recommendations: modular monolith, server rendering/native partials,
selective htmx, modern responsive CSS/design tokens, custom user early, compatible
typing/pytest on PostgreSQL, iterative UX, explicit four-environment controls and
measured scaling. These choices are reasoned defaults, not facts proven by URLs.

Additional UI sources from the supplied research packet:
- Supported container size queries with fallback; style/scroll-state support varies.
  https://developer.mozilla.org/en-US/docs/Web/CSS/Guides/Containment/Container_queries
- Native dialog semantics/focus when a modal serves the task, not by default.
  https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/dialog
- Core Web Vitals: starting field p75 budgets LCP <=2.5s, INP <=200ms,
  CLS <=0.1. Distinguish representative field evidence from lab diagnostics;
  source inspection cannot prove scores.
  https://web.dev/articles/vitals
