---
name: django-auth-environments
description: Use when configuring environments, authentication or permissions to enforce fail-closed Django settings, OIDC and independently guarded admin and tenant authorization.
---
# Environment and identity boundary
Use a shared explicit base with small overrides selected from an exact mapping.
Unknown/missing environment must refuse startup, not fall back to local. Importing
production settings must not depend on mutable local defaults or a local .env.
Validate required secrets and OIDC configuration at startup, with actionable
redacted errors; never substitute invented credentials. Shared environments use
DEBUG=False, deliberate ALLOWED_HOSTS, CSRF trusted origins, HTTPS, secure
HttpOnly session cookies, intentional SameSite, CSP, and proxy trust restricted
to the known terminating proxy. Prevent spoofed forwarding headers. Separate
static/media serving and untrusted uploads; do not execute user media.

| Name (case-sensitive) | Database/data | Authentication and exposure |
| --- | --- | --- |
| local | Local PostgreSQL; synthetic data; disposable separate test DB | Loopback only; explicit optional password login using standard Django hashing, CSRF and permissions, or test OIDC |
| DevTest | Separate shared test database/roles and synthetic or approved data | Default OIDC test tenant, production-like controls; no shared password fallback |
| UAT | Separate DB, roles and appropriately sanitized data | Required OIDC authorization-code + PKCE, distinct client/secret/callback; DEBUG false |
| production | Separate least-privilege DB/roles and protected real data | Required OIDC authorization-code + PKCE, production client/secret/callback; DEBUG false |

DevTest isolated test-fixture override is allowed only on explicit request,
bounded to an isolated nonshared test process/data with assertions and no deployed
fallback. It is not a shared password login feature. In UAT/production missing
OIDC config MUST refuse startup. No local password/password-reset/signup route
or admin backdoor. Required secrets never appear in logs, reports or commits.

## Authentication implementation
Prefer maintained django-allauth after checking compatibility with exact pins,
or document why a maintained compatible alternative is required. Explicitly
enable PKCE; do not assume default. Use maintained validation for state, nonce,
issuer, audience, signature (approved algorithms/keys) and expiry; reject replay
and mismatched callback/issuer. Exact registered callbacks and allowlisted
post-login redirects; no implicit/password OAuth grant. OIDC authenticates over
OAuth, which alone is not identity. Keep browser auth in secure server sessions;
never put tokens in localStorage, logs or application URLs. The short-lived
authorization code in the protocol callback must be promptly exchanged and
removed by redirect, with query logging redacted; no token-bearing URLs.
Avoid provider-token storage unless needed and protected.

Key identity by issuer+subject. Define provisioning, disabled-user policy,
verified-claim trust and deliberate account linking; never auto-link on untrusted
email or let users self-grant staff/superuser. Deny by default. Authentication,
Django model permissions and object/tenant authorization are different controls.
Require staff MFA at the IdP, safe rate limits (including admin), session lifetime,
rotation at login, deprovisioning, revocation latency, logout/local session
invalidation and provider logout expectations. Test expired/revoked sessions and
cross-tenant attempts. Missing IdP means blocked integration, not weaker controls.

## Password lifecycle in OIDC-only environments
In DevTest, UAT and production, inventory and disable password lifecycle surfaces
as well as password login: AdminSite.password_change/password_change_done,
UserAdmin per-user password routes and add/change forms capable of setting a
password, every custom AdminSite, and accounts/reset/signup/recovery URLs.
Enforce policy server-side on direct GET and POST, not just navigation or template
links. Reject forbidden password writes regardless of staff/superuser privilege.
Provision OIDC-only users with unusable local passwords; define deliberate
provisioning UI consistent with the identity owner, rather than exposing generic
password-setting user creation. Existing credentials need an explicit reviewed
migration policy, not an unrequested destructive bulk rewrite. Preserve legitimate
explicit local password testing, without enabling a shared-environment fallback.

A password form remaining reachable despite a rejecting authentication backend
is an OIDC-only UI/policy defect that can mislead users or leave unmanaged local
credentials. It is not by itself a login bypass or a general Django vulnerability.

## Every admin entrance
Inventory each AdminSite, including custom sites and direct URLs. The allauth
social-only setting does NOT secure Django admin by itself. Use the chosen
release's supported secure_admin_login or equivalent independently enforced
guard on EACH site, and ensure inactive/nonstaff/unauthorized principals cannot
enter. Do not simply hide login forms. No reusable password backdoor/break-glass
account unless a separately authorized design replaces this task's policy.

Test direct anonymous/authenticated GET and POST to every admin login and admin
view; test account login/signup/password/reset routes in all environments. Verify
password auth cannot create a session in UAT/production, forged claims cannot
grant staff, wrong issuer/audience/state/nonce fails, unsafe next redirects fail,
missing config/unknown env fails startup and permission/tenant scope is enforced.
Use mock protocol cases for unit coverage, but label live IdP tests separately.
All configuration and logs must redact credentials; review source and actual
runtime evidence independently of the structural handoff checker.

## Environment startup and negative coverage
Start fresh processes with the actual settings selection for local, DevTest, UAT
and production, using authorized isolated test data and configuration. Assert the
effective authentication backends, SOCIALACCOUNT_ONLY, URL configuration and
environment policy agree. Flipping only LOCAL in an already-loaded local settings
process is supplemental unit coverage, not shared-environment startup evidence.
Missing required configuration must fail closed; unavailable integration remains
explicitly blocked.

For each environment exercise anonymous, ordinary, staff and privileged users
against inventoried login and password lifecycle routes, with direct GET and POST
(including authorized CSRF-bearing requests so CSRF rejection cannot mask a policy
failure). Confirm forbidden writes leave persisted password hashes unchanged and
cannot create users with usable passwords; verify no accidental authentication
fallback grants sessions. Test the permitted local flow separately.
Minimize allauth routes without breaking graceful login initiation, callback,
error or logout behavior. Test missing state and invalid callbacks for safe
failure without a session, redirect loop or unhandled error. Simulated authenticated
sessions test authorization/UI only; mock protocol negatives and live OIDC protocol
tests are separate evidence. Neither simulated sessions nor invalid callback tests
establish live IdP validation.
