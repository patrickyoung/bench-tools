# Security

Action is an authorization and receipt boundary, not a sandbox, credential
store, risk classifier, or guarantee that a remote service behaved correctly.

Connectors, deterministic policy, May, and Ask execute with the controller's
operating-system authority. Keep their executables, paths, configuration,
credentials, and state outside model-writable directories. `ACTION_PATH`
should contain only reviewed connectors. A connector descriptor and its digest
identify observed bytes; they do not attest to dependencies, interpreters,
environment, credentials, or remote state.

Action never trusts a proposal to choose an executable path, policy, approval
mode, or credential. It rejects unknown and duplicate JSON fields, bounds all
streams, hashes the selected executables, and rechecks the connector and
descriptor after authorization. Deterministic policy results and May results
must be canonical, exact, and consistent with their exit status.

The effect boundary is the complete request release to the connector. A
connector must read and validate all stdin before causing an effect. It must
return 125 after its own remote-send boundary whenever it cannot establish a
trustworthy terminal outcome. Automatic retry after 125 can duplicate an
effect.

Ask seals prove that the recorded local prefix replays and that Action
attributed those receipt bodies. They are not signatures by a remote service,
and a valid earlier prefix can still be truncated. Protect session files with
the same care as other audit evidence.

Agent runs scrub Action, May, and connector-path environment from Ply before
the worker starts. Cage is primarily a write/network boundary, not a read or
process-identity boundary. For adversarial workers, run the controller and
worker under separate operating-system identities, containers, or machines so
the worker cannot discover or execute controller capabilities by other means.

Report vulnerabilities privately to the repository owner with the exact
proposal, policy result, event prefix, connector behavior, and exit status.
