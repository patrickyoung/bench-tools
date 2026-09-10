# Evidence, authority, and effects

## Contents

1. Keep three planes separate
2. Evidence connections
3. Credentials
4. External effects
5. Authority ladder
6. Shell, toolbox, and Cage
7. Budgets and stop conditions

## Keep three planes separate

- **Evidence plane:** facts and records the worker may read.
- **Work plane:** model and deterministic transformations that produce a
  candidate or an effect proposal.
- **Effect plane:** controller policy, human decision when required, credentialed
  execution, and durable receipts.

A file permission, a Claude approval dialog, and a Bench effect receipt answer
different questions. Do not describe them as interchangeable.

## Evidence connections

Begin with fixtures, then read-only sources. Use Context when heterogeneous
sources need one normalized evidence record with stable references. Set source
authority, freshness, and conflict order in the business design; Context does
not invent them.

Use Cite for exact Context reference-to-URL identity in Markdown. Add separate
checks for factual support, coverage, and source authority.

For MCP, discovery grants nothing. A steward reviews the endpoint and descriptor,
then digest-admits only required tools, prompts, resources, or templates through
an MCP capability folder. Prefer direct read capabilities for evidence. Admit
effectful tools as Action connectors instead of placing them in the worker's
toolbox.

## Credentials

Credentials remain controller-owned and outside chat, the Agent home, argv,
ordinary environment variables, logs, and case fixtures. OAuth binds a profile
to one resource and passes one Authorization header to one exact child through a
file descriptor. Prefer `oauth with`; `oauth header` exposes secret material and
is an expert-only edge.

During discovery report only:

- capability present or absent;
- resource URL or business system;
- account or workload identity;
- scopes;
- expiry/refresh responsibility;
- owner and revocation path.

## External effects

The worker writes a typed proposal. It must not receive Action, May, policy,
connector, or credentials as ordinary tools. The controller:

1. validates the proposal against the admitted schema;
2. resolves and hashes the reviewed connector;
3. evaluates deterministic policy;
4. obtains an exact one-shot May decision when policy requires review;
5. releases the request once;
6. records attempt, sent, result, and uncertainty receipts outside worker write
   authority.

Never retry automatically after status 125 or another effect-unknown result.
Observe the external system, reconcile by idempotency key or business record,
then explicitly resolve or retry.

## Authority ladder

Grant only the current rung:

1. read sanitized fixture;
2. read one reviewed source;
3. analyze or recommend;
4. draft inside a bounded workspace;
5. write a typed external-effect proposal;
6. execute one approved effect through Action;
7. schedule or supervise recurring work.

At each increase, rerun the regression set and review new prompt-injection,
exfiltration, duplicate-effect, denial, interruption, and retirement cases.

## Shell, toolbox, and Cage

- A Ply toolbox chooses executable names but is not a sandbox. Shell builtins and
  redirects can have authority without invoking a named external command.
- Full shell is an explicit high-authority grant. Do not make it the default for
  a business worker.
- Agent normally confines model actions with Cage. By default Cage denies
  network and limits writes, but the child may read host files visible to the
  same operating-system identity.
- Use a narrow OS identity, container, or VM for sensitive or adversarial work.
  Treat per-task sealed runtime, workload identity, credential proxy, quotas,
  and signed capability locks as a stronger production boundary than current
  Cage.

## Budgets and stop conditions

Admit finite limits for:

- model turns and rejected cycles;
- per-command time and retained output;
- total elapsed time and cost;
- records, files, and bytes read or written;
- external proposals and effects;
- retries after observed failure;
- compactions and supervised restarts.

Stop on missing policy, changed connector digest, stale authority, verifier
breakage, confinement failure, human denial, unknown effect, budget exhaustion,
or evidence conflict without an admitted rule.
