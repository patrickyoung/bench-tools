# Incidents and effect uncertainty

## First response

1. Pause new schedules, ticks, and effect proposals.
2. Preserve the Agent home, controller evidence, supervisor events, connector
   receipts, and relevant external record identifiers.
3. Record what is known, observed, inferred, and unknown.
4. Identify whether an effect request was prepared, released, sent, completed,
   or left without a trustworthy terminal result.
5. Assign one incident owner and one business-effect reconciliation owner.

Do not “clean up” state before evidence is captured.

## Diagnose by owning layer

| Layer | Evidence to inspect | Typical correction owner |
| --- | --- | --- |
| Outcome | Admitted contract and GOAL | Business owner |
| Procedure | Brief skill and fail→pass evidence | Process owner |
| Source | Context records, freshness, conflict order | Data/source owner |
| Tool | Admitted descriptor, command output | Platform steward |
| Authority | Action policy, May decision, scopes | Security/business owner |
| State | work/state/checkpoint and supervisor events | Operator |
| Check | Check code, mutation and oracle results | Independent evaluator |
| Runtime | Cage/Tend/host evidence | Platform steward |

## Unknown external effect

When Action, MCP, a connector, or an interrupted command reports an unknown
outcome:

- do not send the request again;
- use the original idempotency or business key to inspect the external system;
- compare external state with attempt/sent receipts;
- record one of: observed completed, observed absent, still unknown;
- only the controller or business owner may resolve to done, fail, or explicit
  retry;
- preserve the original unknown event after resolution.

## Prompt injection or unexpected authority use

Stop the run. Preserve the source record and proposed action. Determine which
admitted instruction or capability allowed the behavior. Remove or narrow the
capability at the owning layer, add an adversarial regression case, rerun proof,
and do not blame the text source for an authority the system should never have
granted.

## Recovery closeout

Close only when:

- current external effects are reconciled;
- the owning layer is corrected or the agent remains paused;
- checks and regression cases cover the failure;
- evidence replay passes;
- authority and budgets are re-reviewed when changed;
- owner, date, impact, and residual unknowns are recorded.
