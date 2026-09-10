# Managed Bench integration for Cowork

Cowork cloud sessions run the agent loop and code execution on Anthropic-managed
infrastructure. A local folder connection does not imply local binary execution,
and local MCP servers do not supply a cloud integration.

## Minimum managed contract

A managed Bench integration should expose typed, high-level operations rather
than unrestricted remote shell:

- runtime/version and capability preflight;
- contract draft/show/admit/run;
- Draft new/check/admit/build/prove;
- Agent new/check/show/run/tick/history/proposal review;
- read-only artifact retrieval and exact evidence export;
- controller-side connection/effect operations with explicit user decisions.

Each operation returns literal status, bounded stdout/result, stderr/evidence,
artifact identifiers, version, definition/contract/check digests when available,
and terminal reason. The adapter must not translate status 1, 2, 75, 125, or 130
into generic success.

## Runtime and state

- Pin one Bench suite version and checksum.
- Use a dedicated per-organization and per-agent workspace.
- Keep controller state, checks, oracles, credentials, policies, schedules, and
  receipts outside worker write roots.
- Define persistence across fresh Cowork sessions and export/backup behavior.
- Use workload identity, quotas, default-deny egress, and narrow capability
  tokens rather than a shared host identity.
- Record every operation and authority decision in inspectable controller
  evidence.

## Connector boundary

Cowork reaches custom connectors through Anthropic's cloud. A custom remote MCP
or service must be publicly reachable from the configured egress ranges, use
strong transport and workload authentication, and scope every operation. Do not
expose a raw local daemon through an ad hoc tunnel as the production answer.

Connector credentials remain server-side. The model receives typed operations
and resource/identity/scope metadata, not secret values.

## Fresh-session test

Before calling the integration durable:

1. End the authoring session.
2. Start a fresh Cowork cloud session with the plugin enabled.
3. Perform read-only version and Agent-home inspection.
4. Retrieve prior controller evidence by approved identifier.
5. Run an already-passing fixture pre-check with zero model calls.
6. Verify no host-only file, conversation memory, local MCP, or ambient secret was
   required.

Failure means design-only or steward-required, not a retry with broader access.
