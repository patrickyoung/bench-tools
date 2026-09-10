# Platform-steward handoff

Create `STEWARD-HANDOFF.md` when design can continue but trustworthy execution
cannot. Replace “ask IT” with the smallest bounded request another owner can
accept, decline, or revise.

## Required sections

### Request

- Business job and accountable owner
- Current readiness state
- Smallest missing capability or decision
- Why it blocks a specific claim

### Claude surface

- Claude Code, Cowork cloud/local, or unknown
- Observed execution location, OS, and architecture
- Connected workspace and its read/write scope
- Skill and plugin version

### Runtime

- Desired Bench suite and Agent versions
- Observed command/version report
- Missing or mismatched commands
- Controller state/evidence location outside worker writable roots
- Proposed source-free suite archive or managed integration

### Evidence connections

For each source:

- business resource and authoritative record type;
- account or workload identity;
- read scope and freshness;
- sensitivity and prompt-injection exposure;
- expected Context/MCP connector boundary;
- owner and revocation path.

Never include a token, cookie, secret, authorization header, or raw environment
value.

### Effects and approvals

- proposed effect and typed input shape;
- connector owner and idempotency/reconciliation key;
- deterministic Action policy;
- conditions requiring May review;
- expected receipts and effect-unknown response;
- explicitly forbidden direct shell, browser, or connector paths.

### Proof

- executable check owner and authority;
- what the check proves and omits;
- teaching/regression/withheld case counts;
- external oracle owner and location class, not secret answer bytes;
- mutation and adversarial cases;
- pilot threshold and stop rule.

### Operations

- cadence and quiet-state behavior;
- time, turn, cost, data, and effect budgets;
- schedule/supervision owner;
- incident and unknown-effect owner;
- pause, rollback, retention, and retirement path.

### Decision requested

End with one or more literal decisions:

1. approve/decline the exact runtime installation or managed integration;
2. approve/decline each connection and identity scope;
3. accept/revise the verifier authority;
4. accept/revise the effect policy and operating budgets.

No work begins merely because the handoff was created.
