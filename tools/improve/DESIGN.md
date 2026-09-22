# Improve

Read an experiment specification, measure one proposed source change against a
baseline, and print whether fresh reserved cases support keeping it.

## Contract

`improve -n < experiment.json` validates and prints the maximum workload without
running commands or creating files. `improve -o NEW_DIRECTORY < experiment.json`
executes that workload as needed and emits one JSON result. `improve verify DIR`
checks retained files and replays Record receipts without running the original
commands. Version 0.1.0 is an independent Go program using the standard library.

The specification selects a source directory, an explicit list of mutable
existing UTF-8 files, development and holdout case files, three literal command
vectors, declared dependency files, Record, settings, repeats and time limits.
Each trial command composes execution and independent scoring. Case contents and
score payloads are opaque to Improve; the selected judge owns the acceptance
rule. The router example requires perfect quality and lower measured cost.
The acceptance example instead consumes a caller-owned use-case policy; it
can accept a justified quality/cost trade-off without changing this protocol.
The experiment-creating application owns that policy and its rationale. Neither
Improve nor its judge infers the business's willingness to pay from run scores.

The sequence is fixed: development baseline; one proposal from development
observations; fresh matched development pairs; freeze; fresh matched holdout
pairs; export supported bytes. A failed development comparison stops before
holdout. The proposer receives no holdout contents or paths. Three repeats are
the default, with baseline/candidate order reversed on odd repeats. An external
caller can start another experiment with new reserved cases.

## Files and process boundary

Snapshot the complete source tree and cases; pin selected executables and
explicit dependency files. Require new output disjoint from every selected
input. Reject symlinks in source/case/dependency paths; resolve selected command
symlinks once and pin their targets. Preserve original source and modes. A
proposal replaces only enumerated existing text files and cannot change modes.
This supports prompts, skills, subagent definitions and checks, while the
independent scoring/judging commands remain frozen. Adding/deleting files is
outside this first version.

Every proposer/trial/judge invocation gets a separate cwd, work and evidence
directory. The trial receives a fresh source copy. Record wraps the selected
command and preserves stdin, stdout, stderr and outcome. Improve verifies Record
replay matches the observed streams and status before consuming the JSON. It
never reads the Ask session format. Selected adapters verify nested evidence
through its owning tools. There are no provider clients, model loops, retries or
fallbacks in Improve.

Commands return versioned JSON envelopes with caller-defined score or decision
data and an explicit cost (nonnegative dollars or null). Null propagates into
study totals; the judge decides which measurements its rule requires. All
stages, including diagnostics and authoring, appear in the cost ledger. Failed
or interrupted commands leave unknown cost rather than zero. A timeout signals
the process group, permits bounded cleanup, then kills remaining group members;
no resubmission follows an uncertain outcome. Detached descendants remain the
selected command's responsibility.

Planning rejects unavoidable source-copy workloads beyond the study inventory
bound and rejects call ledgers above 1 MiB before executing commands. Single trees allow 4096 files/128 MiB; the whole study allows 8192
files/512 MiB. Runtime stream writers cancel the command group at 2 MiB,
retaining incomplete evidence without a retry. Caller-written files remain
subject to final inventory validation, not a filesystem quota. An admitted run
still emits invalid-evidence JSON if finalization fails; no proposal survives.

The output directory retains requests, raw streams, process receipts, scores,
decisions, pins, exact candidate, result and a file inventory. Success exports
`proposal/source` and a manifest binding baseline/candidate hashes and final
decision. It does not overwrite a worker, amend a skill or publish a release.
Exit 0 means supported; 1 means no supported proposal or incomplete execution;
2 means invalid invocation/evidence or operational failure. A bad trial's job
result is still a valid observation when the trial adapter successfully scores
it; a broken adapter is never silently treated as a failed job.

## Limits of the claim

The caller trusts selected programs and controls the output directory. Hashes
detect drift, not malicious controllers. Holdout separation is procedural;
host-readable files are not secret. Pins do not capture environments, remote
aliases, caches or undeclared transitive dependencies. Replay proves retained
process consistency, not correct labels, causal improvement or deployment
authority. Weigh is optional inside a selected research adapter, never an
implicit live-run dependency. Hone retains its separate recovery contract.

## Verification

Unit/process fixtures cover acceptance, both rejection stages, malformed
evidence, unknown costs, immutable inputs, path separation, cancellation and
replay tampering. `scripts/check-improve.py` in the containing Bench checkout
checks the built Improve → Record → Ask process boundary without inference.
Standalone tests use fake public executables; the leaf builds without siblings.
