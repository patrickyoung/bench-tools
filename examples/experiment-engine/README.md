# A small experiment engine

**Historical research recipe.** Use [native Improve](../../tools/improve/README.md)
for new experiments. This earlier Python controller is retained to understand
past studies and run its offline regressions; do not extend it as a parallel
controller. Its old acceptance/search policy is not the current Improve router
gate. Commands below document the historical workflow.


The independently packaged successor is [Improve](../../tools/improve/README.md).
This directory retains the original support-router research recipe; new
experiments should use the tool and caller-selected adapters.

Start with one worker, one hypothesis and a fixed way to measure its output.
This example routes short support notes to four queues. It deliberately starts
with a crude keyword instruction. It demonstrates how to remove that instruction
and test the resulting policy-driven worker. The recipe now includes a [bounded autonomous search](SEARCH.md) around the
original trial, scoring and comparison filters. It remains an example, not an
installed Bench executable or runtime service. Research runs outside production;
fresh model trials are paid calls, while replay and scoring are offline.

The host conducts a bounded experiment:

```text
selected records → optional observations → Hire authors one change
                                                        ↓
fresh input → Agent/Ply/Ask → Record → score.py → compare.py
                                                        ↓
                                          reviewed local promotion
```

Weigh can extract small semantic observations or select from an explicit menu;
it does not generate arbitrary patches or establish correctness. Prefer the
[atomic event feature recipe](../../tools/weigh/examples/event-features/README.md)
for learning from logs. A broad question asking which change will improve a
worker bundles diagnosis and prediction; a plausible choice is not measured
improvement. The host writes a focused teaching brief, Hire authors
the change, and independent labels measure actual worker outputs. No source is
changed by `score.py` or `compare.py`. A supported result is evidence for a local
review, not authority to modify global prompts or unrelated workers.

## Programs and their boundaries

| Program | Job |
| --- | --- |
| Hire | Scaffold the naive worker; author a focused revision in a separate copy |
| Agent, Ply, Ask, Cage | Execute the worker with existing context, checks, model access and action boundary |
| Record | Retain each process's exact streams and outcome; verify them offline |
| Weigh | Classify selected observations or choose an experiment from a bounded menu |
| Trail | Inspect and validate the retained Ask sessions |
| Hone | Prepare a reusable lesson only when a real recorded recovery qualifies |
| Brief | Load procedures if a future experiment introduces a skill |
| Git or ordinary versioned directories | Review and retain exact source; adopt or revert deliberately |

`trial.py` invokes public Agent and Record executables once, snapshots the expert
and request, and retains a manifest. It uses no provider API. `score.py` verifies
the outer Record receipt and Ask sessions, then compares returned queue labels.
`compare.py` reads paired scores from stdin and prints a decision. Each source
fingerprint includes file content and executable modes. Labels and reports are
caller-controlled evidence, not signed attestations against a hostile author.
Paths, installed executables, environment and provider aliases remain external
dependencies; a source hash is not proof of identical deployed model weights.

This worker's `bin/check` checks output structure and IDs only. The independent
scorer checks routing accuracy. A structural pass can accompany a poor result.

## Run one trial

Use an installed Bench environment, Python 3.9+, a working Cage boundary and an
explicit Ask model/credential profile. Preserve any approved `AGENT_ASK` wrapper.
All live runs use the caller's provider account; offline tests make no model calls.
Keep work, inputs, labels, model records and experiments outside the checkout.

The caller supplies `request.json` with a `policy` string and `notes`, an ordered
array of `{ "id": "n1", "text": "..." }` objects with unique string IDs.
The separate label file is an ordered array of `{ "id": "n1", "queue": "billing" }`.
Allowed queues are `security`, `billing`, `technical`, and `general`.
Write the policy and independent labels before running either arm. Use a new
trial directory every time; the runner refuses an existing destination.

```sh
python3 trial.py --expert naive --input /selected/cases/request.json \
  --out /selected/trials/baseline-1 --model "$WORKER_MODEL"
python3 score.py --trial /selected/trials/baseline-1 \
  --labels /selected/cases/labels.json > /selected/scores/baseline-1.json
```

Each trial permits three turns, one cycle, a 40-second command timeout and a
180-second outer timeout. `--effort` passes an explicit Ask effort (default
`low`; use `off` to omit reasoning configuration for the selected model).
It returns the child's status, or 1 for broken
recording/source integrity and 2 for invalid invocation. It never retries.
`score.py` returns 0 for a valid score, including failed work, or 2 for invalid
evidence. Unknown usage stays null; it is never replaced with zero cost.

## Propose and compare one revision

Inspect a selected episode using Trail and the retained streams. Supply selected
observations to the existing
[select-fix.py](../../tools/weigh/examples/improve-checks/README.md#select-one-fix-from-check-results)
adapter. Set `BENCH_WEIGH=1`, select `--backend weigh`, a model, the Weigh/Record
executables and a private credential environment or approved wrapper. Keep keys
out of argv, snapshots and records. Retain the complete native distribution.
Weigh errors stop that selection; no implicit retry or substitute model runs.

For this experiment, an appropriate menu is simplify instructions, add a skill,
retain the baseline, or investigate the check. The observed result determines
the teaching brief; do not tell the selector that a conjecture is established.
This menu is an exploratory hypothesis generator. Evaluate atomic observations
and deterministic candidate selection as a separate arm before adopting the
menu selector as the improvement engine's default.

Copy only the expert into `/selected/authoring/expert`. Ask Hire to remove the
keyword shortcut while preserving the output contract and checker. Inspect the
diff, run `hire verify`, and test the check before executing it. Freeze the
candidate's fingerprint before preparing fresh evaluation cases.

Use at least two matched pairs, reversing execution order in the second pair.
Every arm gets the same input, independent labels, selected model and limits.
Fresh cases are not secret: Cage limits writes/network, not host reads. Do not
claim an adversarial holdout without a separate inaccessible environment.

Collect scores in one JSON object with `baseline` and `candidate` arrays, in
matching pair order. This example's predeclared efficiency gate requires:

- Perfect candidate accuracy in each pair, with no case-level regressions.
- Fewer provider-reported input tokens in each pair and no extra model calls.
- A smaller instruction file and a distinct, frozen source definition.
- Successful execution, complete verified evidence and matched settings/labels.

```sh
python3 compare.py < /selected/evaluation.json > /selected/comparison.json
```

Exit 0 is supported, 1 keeps the baseline, and 2 rejects invalid evidence. Reusing
one trial as two repeats is invalid. An unchanged source is never an improvement.
Elapsed time and output tokens remain separate tradeoffs; fewer input tokens do
not prove lower total cost. Preserve rejected trials and initial reports.

Review the exact diff, recompute scores from retained receipts, rerun comparison,
and verify that candidate bytes still match the evaluated fingerprint before
copying them to a **new** local release directory. Retain a manifest naming the
old/new fingerprints, protocol, comparison, source diff and rollback directory.
Do not overwrite the old release. Git review and a new pinned export are needed
to promote an improvement into a shared worker library.

The existing Weigh `evaluate.py`/`review.py` helpers target comparisons of checks
on fixed candidates. This experiment changes the producing worker instead, so
it uses its small task-specific scorer and comparison filter rather than
misrepresenting different generated outputs as one fixed candidate.

## What can grow later

Change one surface per experiment: instructions, skill, subagent definition,
checker, or rejection feedback. Each needs a suitable independent outcome metric
and frozen limits; do not let a proposed checker grade its own promotion. Keep
each candidate, measurements and decision as ordinary files. Scheduling and
retained queues can use existing tools when there is a real need.

Run `hone -why SESSION` on an actual Agent session before attempting a lesson.
A first-try success or a separate offline score disagreement is not a checked
recovery. If no recovery qualifies, use Hire for a justified method change and
say so. Never manufacture a failure to make Hone accept it.

## Offline verification

```sh
python3 -m unittest discover -s . -p 'test_*.py' -v
```

Tests cover regression rejection, missing measurements, stale/mismatched labels,
duplicate evidence, source/mode changes and the structural check. They do not
establish model quality. Retain live evaluation evidence outside this directory.

The experimental discipline is inspired by
[Karpathy's autoresearch program](https://github.com/karpathy/autoresearch/blob/master/program.md):
a baseline, a small mutable surface, fixed evaluation and measured keep/discard
decisions. The composition follows
[Kernighan and Pike's Program Design in the UNIX Environment](https://www.in-ulm.de/~mascheck/various/uuoc/kp.pdf):
ordinary files, processes and filters. These are design influences, not claims
that their authors participated in this experiment.
