# Executable-check and runtime security review

## Check review

Record:

- exact check bytes/digest and interpreter;
- dependencies and their pinned versions;
- read roots, write roots, network, identity, and credential reach;
- input contract and candidate stdin handling;
- deterministic outputs and exit meanings;
- timeout, output cap, file/data bounds, and concurrency behavior;
- side effects, cleanup, and repeated-run safety;
- positive, negative, broken-check, first-run, and mutation cases;
- what the check proves and explicitly does not prove.

The check is controller code. A malicious or careless verifier can read secrets,
alter state, or approve everything even when worker actions are confined. Run it
under its own least-authority identity where the risk warrants it.

Reject:

- constant success or failure;
- model-as-judge for the same model's output;
- expected answers inside worker-readable roots;
- checks the worker can modify;
- production effects during verification;
- unbounded network, time, output, or input traversal;
- structure-only checks marketed as business correctness;
- hidden manual steps that turn exit 0 into an unsupported claim.

## Cage and host boundary

Cage currently owns model-action writes and network. It may still expose
host-readable files, processes, programs, and ambient identity. For sensitive or
adversarial work, add a narrow OS account, container/VM, read allowlist, secret
broker, process/resource quotas, and default-deny egress outside Cage.

Status 125 means confinement did not establish; no child should have run. Treat
it as a failed boundary, not permission to fall back to ordinary shell.

## Agent home ownership

- Governed definition and tools are reviewed and versioned.
- Worker writes only mutable work/state/temp.
- Check and `bin/wake` ownership are explicit.
- `.agent/runs`, Ask logs, May, Tend, Action policy, connector definitions, and
  credentials remain controller-owned.
- Backups and restoration preserve the separation rather than flattening all
  paths into one writable archive.

## Security cases

Test:

- instructions embedded in evidence;
- path traversal and symlink changes;
- stale or swapped connector descriptors;
- secret-like strings in output/logs;
- network denial and egress allowlist;
- full disk/output/resource exhaustion;
- concurrent run/check and stale definition;
- interruption before and after an effect release;
- replay tampering;
- retirement with lingering schedules or credentials.
