# Django expert

The [Django expert](../workers/django-expert/expert/README.md) is an independently
reusable Hire-authored worker for Django 6.1.1 and PostgreSQL. It can plan,
bootstrap/build, iterate and review/debug a selected application, producing code
when implementation is requested. It remains experimental: contract tests do
not establish application, security or design quality.

Its defaults are a small modular monolith, compatible supported Python, uv and
locked dependencies, Git, actual PostgreSQL development/testing, native staff
admin and purpose-built customer views. Server-rendered templates, native Django
partials, semantic HTML and modern CSS come first; selective htmx or richer UI
is justified by user tasks. Discovery, prototypes, feedback, browser/accessibility
checks and small working slices form the iterative UX process.

The four explicit environments are local, DevTest, UAT and production. Local
password accounts are for testing. Shared environments default to OIDC over
OAuth with PKCE, separate identity clients and fail-closed configuration; UAT
and production have no password fallback, including direct admin login. Identity
integration and object/tenant authorization are separate responsibilities.

The dated source references cover Django 6.1.1, Python, PostgreSQL, allauth,
OAuth security, uv/Ruff, browser testing, accessible UI and progressive
interaction. Recheck advisories and dependency compatibility for each real
project. Later patch upgrades require an explicit decision; no floating latest
versions or unsupported feature promises.

Export a committed definition with `scripts/workers export django-expert` and
`--allow-experimental`; see the [library guide](WORKER-LIBRARY.md). Supply a fresh
workspace with request.md and explicitly selected inputs, then use Agent and the
worker's documented contract. Keep credentials, applications, dependency installs,
model records, current briefs and evaluations outside the source library.

The worker owns application engineering and reviewable handoffs. DevOps,
identity, design and QA collaborators retain their own infrastructure and
acceptance responsibilities. No team, runtime, provider client or deployment
service is added. The static checker reads bounded files only and never executes
generated application code. Run meaningful application tests through the selected
action boundary and evaluate user experience in the browser.
