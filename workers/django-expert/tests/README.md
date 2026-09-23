# Offline contract tests

Run `python3 -B workers/django-expert/tests/test_contract.py` from repository root.
The suite resolves this worker's own expert and uses temporary synthetic files.
No model, application dependencies or network calls. It tests current hash bindings,
status/evidence, minimal build files, exact environments, malformed data and unsafe
paths. It does not prove security, usable UI or working Django code.

For model quality, evaluate fresh plan/build cases with PostgreSQL and OIDC/admin
negative tests. Review real browser flows and separately validate live identity
integration. Use the catalog quality cases and worker README. Keep case outputs,
logs, packages and evidence outside source.
