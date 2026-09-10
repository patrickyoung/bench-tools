# Connection and effect provisioning

## Read connections

Provision sanitized fixtures first. For every live source record resource,
record type, identity, scopes, freshness, business authority, conflict order,
sensitivity, egress, rate/data limits, owner, and revocation.

Context connectors normalize evidence but do not establish source authority.
Cite checks exact reference/URL identity but not semantic support.

## MCP admission

Discovery is read-only inventory. Review endpoint identity, transport,
authentication, descriptor, schemas, annotations, data reach, effect behavior,
timeouts, redirects, task semantics, and result limits. `mcpbox admit` pins the
endpoint/capability descriptor and rechecks before use.

Admit read capabilities into a narrow toolbox only when direct invocation is
intended. Admit effectful MCP tools as Action connectors. Do not admit the same
effectful tool through both paths by accident.

MCP status 75 is input-required/unfinished. Status 125 may mean an effect exists
without a trustworthy terminal result. Requests are never retried implicitly.

## OAuth

Use a public-client PKCE or device flow where appropriate, or an approved
confidential/workload client. Secrets enter only via protected stdin/file or a
secret service, never argv. Bind profiles to the exact resource and smallest
scopes. Prefer `oauth with`, which refreshes and passes one header to one child
through descriptor 3 without placing it in argv or environment.

Record owner, client type, identity, scopes, refresh policy, profile state path,
rotation, revocation, and incident response. Do not copy profile files into an
Agent home or skill ZIP.

## Action and May

An Action proposal names a reviewed connector and typed input, not a command,
policy, credential, or approval mode. The controller supplies those authorities.

Review the connector bytes and descriptor, then define deterministic policy:

- 0 allow;
- 3 deny;
- 75 exact May review.

May decisions bind one exact action digest and job, then spend once. Action
rechecks connector identity after authorization, records attempt before release,
then sent and result evidence. Define reconciliation for status 125 before the
first effect.

## Connection acceptance test

- Read-only fixture succeeds with expected identity and scope.
- Out-of-scope resource is denied.
- Changed descriptor is rejected before use.
- Expired/refresh-failed OAuth stops without exposing tokens.
- Policy allow, deny, and review paths behave distinctly.
- May grant cannot be reused or applied to changed bytes.
- Duplicate request uses idempotency/reconciliation key.
- Interrupted/unknown effect is observed and resolved without automatic retry.
- Revocation removes access in a fresh session.
