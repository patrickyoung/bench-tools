# django-expert
A portable single-worker definition for Django **6.1.1**, not an application or
runtime. Agent supplies execution; four focused Brief skills supply foundations,
environment/auth security, UX/admin and delivery guidance. MIT, Patrick Young 2026.

## Setup and invocation
Keep this definition outside the mutable workspace. Required for worker execution:
the existing Bench `agent` installation and its configured Ply/Ask, Cage, Record
and Brief companions, model credentials selected by the operator, and Python 3.10+
on PATH for the standard-library structural checker. No pip packages needed for
bin/check. Python 3.14 is the *application* default, not a requirement to inspect
the definition. Brief skills are ordinary frontmatter SKILL.md files; no custom
loader. No sibling worker, provider client or network service is required here.

For application work, provide Git, uv and an authorized compatible Python,
PostgreSQL/psycopg environment (Compose-capable container engine or explicitly
supplied DB). Ruff, typing, pytest-django, allauth and optional browser dependencies
belong in the application's verified lockfile, not the expert. Inspect before
installing; dependency resolution, database startup and network access require
the operator's boundary. Do not install packages to validate this definition.

Example from a directory holding `expert/` (prepare request and inputs first):
```sh
DEF="$(pwd)/expert"
WORK="/absolute/path/to/task-workspace"
EVIDENCE="/absolute/path/to/task-evidence"
mkdir -p "$WORK" "$EVIDENCE"
# Write $WORK/request.md and copy only selected inputs/ before invoking.
agent run -C "$WORK" -state "$WORK/state" -evidence "$EVIDENCE" \
  -goal-file "$WORK/request.md" \
  -record-input request.md \
  -record-output report.md -record-output handoff.json \
  "$DEF" >"$EVIDENCE/stdout.txt" 2>"$EVIDENCE/stderr.txt"
rc=$?
printf '%s\n' "$rc" >"$EVIDENCE/exit-status.txt"
```
Record exit status without hiding it behind a pipe. Add repeatable
`-record-input inputs/NAME` and `-record-output app/CHANGED_FILE` flags for selected
inputs and important deliverables; report/handoff alone do not retain code bytes.
Full command recordings and selected files belong in the separate evidence
directory. Secret credentials are not record-input artifacts. Use operator secret
injection, never plaintext job materials. Default Cage restricts writes and denies
network; do not silently use `-no-cage`. Operator-selected `-net` or other authority
is a separate decision, not granted by this definition. No production effects
by default.

For subagent use, invoke the same Agent definition in a separate workspace with
actual bounded request/inputs and explicit artifact handoff; no recursive runner
or team required. Optional native-harness reuse must include AGENTS.md, PROFILE.md,
CONTRACT.md, references and applicable full skills and retain the checker and
authority limits; an abbreviated persona is not equivalent. Agent remains the
supported runner. No MCP/API/listener interface is provided or required.

## Sample request schema
Use Markdown fields like these (template, not an installed application):
```markdown
# Request
Mode: plan | build | iterate | review_debug (choose one)
Outcome: bounded task and intended users
Application: app (explicit workspace-relative directory), or none for a plan
Selected inputs: inputs/brief.md, inputs/constraints.md (or none)
Acceptance criteria:
- Observable user/business outcome
- Required functional, auth/tenant and accessibility checks
Constraints:
- Existing conventions, supported browsers, data limits, exact version pins
Allowed effects:
- Files/directories allowed to change
- Commands/network/DB targets explicitly authorized, or read-only
Environments:
- local: DB source and optional test-login decision
- DevTest: OIDC test tenant and shared-environment owner
- UAT: identity/integration owner and approval needs
- production: identity/operations owner; no deployment permission implied
Review/feedback: available evidence, owners and outstanding questions
```
For a bootstrap, select the intended new `app/` path in the request; create it
only when authorized and record it once it exists. If blocked before creation,
handoff.application can be null and the report must explain. For existing apps,
the chosen path must already exist inside the workspace. Credentials go through
the operator, not these files.

## Deliverables and checks
See CONTRACT.md for exact JSON fields, bounds and path/hash semantics.
Every mode produces report.md and handoff.json plus appropriate app/code/docs.
The report covers decisions, changes/findings, criterion coverage, actual checks,
limitations and next actions. `ready_for_review` is not a release sign-off.
`blocked` requires concrete blockers, limitations and next actions; passed partial
validations are allowed only with existing manifested evidence, not as a claim of
overall task or external integration success. `ready_for_review` rejects known
failed/blocked validations. A ready_for_review build must name an existing
application directory and manifest at least one file below it; blocked builds
may have application=null or partial files. This proves artifact existence only,
not correctness or completeness. Selection fidelity and task completeness need review.

Independent check, with no Agent/model/application dependencies:
```sh
(cd "$WORK" && "$DEF/bin/check")
```
It verifies strict bounded JSON, enums, four exact environments, real relative
non-symlink files, current SHA256 request/selected inputs/artifacts, manifested
report and evidence for claimed validations. It never runs command strings,
project imports, network calls, dependency installs or generated application code.
No keyword scanning stands in for security or UX quality.

Separate gates:
1. **Structural acceptance:** this checker. Worker-authored logs and hash bindings
   are not independent runtime proof. A changed request invalidates old handoff.
2. **Actually tested implementation:** caller-selected authorized actions, trusted
   command receipts, code review, PostgreSQL/auth tests and CI.
3. **Visual/user judgment:** browser/manual accessibility review and actual user
   feedback; WCAG 2.2 AA is a target, not a certificate from static validation.
4. **Production/OIDC integration:** real approved IdP configuration, runtime
   negative tests for every AdminSite, UAT sign-off and operational readiness.
No model-based verifier is necessary or included. Missing live infrastructure
means blocked integration, never invented credentials or a weaker auth fallback.

## Generalized implementation lessons
OIDC-only shared environments require server-side password lifecycle restrictions,
not just login guards: inspect all AdminSites, UserAdmin forms/routes and custom
account flows, with identity-owned provisioning and unusable local passwords.
Preserve explicit local testing. A residual password form is a UI/policy defect,
not by itself an authentication bypass or a general Django vulnerability.
Test actual environment startup in fresh processes, role-based direct GET/POST,
unchanged hashes after denied writes and no fallback sessions. Preserve graceful
allauth login/callback/error/logout; simulated sessions and invalid-callback tests
are not live IdP validation. Implement typed public service/view boundaries or
record them as incomplete; missing typing/browser coverage leaves setup partial.
These are source-independent guidance refinements, not application validation;
the static checker and contract remain unchanged.

## Synthetic examples and reproducible validation
The authoring bundle's `tests/test_contract.py` is OUTSIDE this definition.
It constructs temporary fixtures, runs only bin/check, and cleans them up:
```sh
PYTHONDONTWRITEBYTECODE=1 python3 -B tests/test_contract.py
hire verify expert
```
Good: a hashed plan with report and all four environment decisions; a blocked
report with concrete missing IdP details, limitations, next actions and evidenced
passed local tests; a ready build with a manifested application file.
Bad: report-only ready build, blocked without blockers, missing report/output, stale request or selected input, artifact tampering,
invalid environment/status, escaped or symlink paths, and a passed result without
manifested evidence. Extra cases cover parser bounds, malformed shapes and
failure/readiness inconsistency. Fixtures are synthetic, not model quality evidence.
Observed authoring outcomes are recorded alongside tests, outside expert/.

No live app, package install, live worker model evaluation or real OIDC integration
is part of authoring validation. To extend expertise, add a focused Brief skill,
update the AGENTS.md selection procedure and extend independent cases when the
contract changes; do not add a runner or hidden dependency.
