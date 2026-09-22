# Bounded instruction search

**Historical research recipe.** Use [native Improve](../../tools/improve/README.md)
for new experiments. This earlier Python controller is retained to understand
past studies and run its offline regressions; do not extend it as a parallel
controller. Its old acceptance/search policy is not the current Improve router
gate. Commands below document the historical workflow.


Start with the existing support router. Search changes only `AGENTS.md`.
The worker's check, cases, labels, runner, scorer, comparison rule, model and
limits stay frozen. This is an experimental recipe, not a new Bench program.

`search.py --spec SPEC --out NEW_DIRECTORY --live` runs a finite search. Without
`--live` it validates and prints the maximum workload; it executes no commands.
The spec selects public executables and a literal proposer argv. The supplied
`propose.py` adapter uses Hire; the controller never calls a provider itself.

1. Snapshot the expert, evaluation inputs and local Python helpers; fingerprint
   selected executables and proposer files. Keep evidence outside source.
2. Measure the baseline on development cases with at least two fresh repeats.
3. Send the current best instructions, development outcomes and previous
   experiments to the proposer. No held-out case contents or paths are sent.
4. Accept a bounded replacement instruction string. Copy the current best
   definition and replace only AGENTS.md. Reject unchanged or repeated proposals.
5. Run fresh matched baseline/candidate pairs, alternating execution order.
   `judge.py` retains only complete, nonregressing candidates with either more
   correct outputs or equal perfect quality and fewer instruction bytes/input
   tokens without more model calls. Correctness means accepted usable output.
6. Continue from the retained candidate, preserving every rejected attempt.
7. Freeze the final candidate and compare it with the original on held-out
   cases once, using the same repeats and gate. Never feed that result back
   into this search. Export exact bytes and a diff only when supported.

Budgets cap proposals, worker invocations, per-run turns/command time, and total elapsed
admission time. A process timeout first signals its process group and allows nested Record /
Agent / Ply cancellation to finish before escalation; cleanup has its own bounded
grace. A completed parent alone does not prove model descendants stopped. There are no
hidden retries or resume: interrupted evidence is retained; run a new experiment
explicitly. Unknown billing remains unknown. Model randomness, provider aliases,
caches and transitive dependencies are not controlled by file fingerprints.

Use development results for search, and held-out results for the final proposal.
These are procedural holdouts, not secrets: Cage confines writes/network but
allows host reads. The proposer is a trusted, caller-selected command. Integrity
checks detect ordinary drift; they do not authenticate hostile controller output.
Externally accessible labels require stronger isolation for adversarial studies.

stdout is one JSON result; stderr is progress. Exit 0 means supported on this
experiment's held-out cases, 1 means no supported proposal/incomplete budget,
and 2 means invalid invocation or broken evidence. A successful experiment never
writes the source worker or deploys it. `proposal/expert`, its exact diff and
manifest are the handoff to existing review/promotion tools. No synthetic Hone
recovery is created. Weigh is absent from the default path. An optional proposer
can compose it separately and must account for its cost.
