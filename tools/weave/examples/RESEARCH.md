# Bounded invoice research recipe

[research.py](research.py) compares parameter search, one adaptive researcher,
and a small adaptive team on the same fixed invoice simulator. Agents propose
concrete interventions; ordinary programs measure them. This is a runnable
experimental recipe over explicitly synthetic, uncalibrated evidence. It ships
as example source, not as an installed or supported research command. It cannot
establish a real business improvement or a general advantage for teams over one
agent.

The [domain contract](RESEARCH-DOMAIN.md) specifies the evidence, interventions,
simulator and acceptance rules. The [trial run record](RESEARCH-RESULTS.md)
describes the completed comparison and links its portable aggregate summary;
this guide makes no live-outcome claim.
The earlier [business prototypes and results](RESULTS.md) remain separate
historical demos.

## What is being compared

The default protocol has **nine experiment slots per arm, repeated in three
trials**. A shared deterministic baseline precedes all trials. The baseline and
one final confirmation per arm/trial are additional fixed evaluations, outside
the nine search slots.

| Arm | Default procedure | Model proposal allowance per trial |
| --- | --- | --- |
| `search` | Shuffle the 35 non-baseline configurations using seed `7301 + trial_index`; test the first nine without replacement | None |
| `single` | Nine sequential proposals; each researcher call sees that arm's previous measured feedback | Nine Ask invocations |
| `team` | Three rounds of three parallel roles; join their measurements before the next round | Nine Ask invocations |

The three team roles examine queue flow, receipt handling and total effort, and
missed exceptions/service/stress failures. Roles in a round receive the same
prior feedback and do not see one another's current proposals. A deterministic
scorer chooses the incumbent; there is no additional coordinator model call.
The single arm receives an instruction to consider all process tradeoffs.
Histories are separate between arms and trials. The seed controls parameter
search, not provider sampling.

Every slot is spent once admitted to the protocol. A malformed or failed
proposal consumes its slot and blocks its dependent evaluation. A duplicate
configuration also consumes a slot and is measured without earning a replacement.
Failed evaluations consume slots. Negative results are valid completed
experiments: checker acceptance means the measurement matches the frozen
contract, not that the intervention succeeded. An unknown execution outcome
stops the run for inspection; the recipe never automatically retries it.

All agent proposals use **one Ask invocation** with a closed JSON schema and no
file or action tools. Each reply must provide the exact four-field candidate,
a hypothesis and an expected tradeoff. This makes the single/team proposal
allowances comparable. Provider transport retries can still occur and are
metered separately; one invocation is not a promise of one network attempt.
Ply remains available in the general business recipe for checked report revision.
This comparison does not spend extra Ply turns repairing proposals.

Weave itself remains unchanged: a finite task graph and observation snapshot go
in, ready original task records come out. The Python recipe creates each next
finite round. Tend owns execution, Ask owns model evidence, and ordinary domain
procedures own measurement and selection.

## Evidence, feedback and the final test

The fresh fixture has three disjoint splits, each with 24 cases:

1. **Development:** agents receive the cases, their retrospective adjudication,
   fixed observations, intervention meanings and complete candidate space.
2. **Feedback-validation:** agents receive aggregate evaluation feedback during
   search, including baseline scores. They do not receive its case rows or
   labels. Because results influence later proposals and selection, this is
   validation feedback, **not a true holdout**.
3. **Final holdout:** no rows or scores enter proposal prompts. Every arm's
   selection in every trial is frozen before any final confirmation is admitted.
   Final results cannot change a selection within this experiment.

Each next proposal receives its arm's entire history of tested configurations
and outcomes. The last six entries include full public score summaries: selected
per-scenario metrics, feasibility checks and relative comparisons. Older entries
retain their aggregate objective, feasibility and relative findings. Recent
hypotheses/tradeoffs are shortened for the prompt; the result artifacts and
experiment ledger retain the original accepted text. This bounds repeated
feedback without erasing the list of configurations already tested.

The controller's selection rule first prefers a candidate eligible on both
development and feedback-validation, then one feasible on both, then applies
the domain's declared validation ranking. It starts with the baseline and keeps
only a strictly better incumbent. Prose is never part of the score. Final
`confirmed` requires both search eligibility and final-holdout eligibility.
A retained candidate can still be infeasible or unconfirmed; always read those
fields with its metrics.

The simulator runs normal demand, 40% faster arrivals, and one staff member
absent on every split. Each scenario must meet at least 95% deadline service,
at most 5% rework and zero backlog at the fixed horizon. Relative improvement
separately requires at least a 10% reduction in the equally weighted mean of
scenario medians, with no p95 or labor increase in any scenario. Equal weights
are a declared synthetic stress-test choice, not observed business frequencies.
These rules are frozen before the trial; an all-negative outcome is acceptable.

Agents choose queue order, receipt precheck, review strategy and single/pair
batching from 36 configurations. They cannot edit processing times, detection
effects, staffing, deadlines, ground truth or thresholds. Fixed per-case effects
are stipulated synthetic counterfactuals, not calibrated causal estimates.
The source and fixture are public; withholding here is enforced by the
report-only prompt path, not by claiming that the dataset is secret.

## Running it

Use the tools already built in the Bench monorepo. From its root:

```sh
python3 scripts/build ask ply tend weave
export WEAVE="$PWD/.build/bin/weave" TEND="$PWD/.build/bin/tend"
export ASK="$PWD/.build/bin/ask" PLY="$PWD/.build/bin/ply"
cd tools/weave
```

The commands below run from that Weave directory. They record the selected
executable hashes, so a trial using this checkout is distinguishable from one
using earlier pinned builds. Keep the resulting records with any comparison.

For a standalone Weave checkout or reproduction with the pinned public tools,
install the pinned Ask, Ply and Tend
revisions and build Weave, then run the offline checks. Go 1.26 or later, Git,
Python 3.9 or later on Unix, and network access for bootstrap are required.
Sibling checkouts are unnecessary:

```sh
make bootstrap
make check-examples
export WEAVE="$PWD/var/tools/weave" TEND="$PWD/var/tools/tend"
export ASK="$PWD/var/tools/ask" PLY="$PWD/var/tools/ply"
```

The pins live in [dependencies.json](../scripts/dependencies.json).
[bootstrap.py](../scripts/bootstrap.py) installs the tools into `var/tools` by
default. For a different directory, use `python3 scripts/bootstrap.py --bin DIR`
and `./tests/check --bin DIR`, then point the four variables above at that
directory. Bootstrap may download source and Go dependencies; the checks use
local inference fixtures and make no live model calls.

Run a provider-free parameter-search control:

```sh
python3 examples/research.py var/research-search --arms search \
    --evaluations 9 --trials 3 --team-size 3 --seed 7301 --seconds 1800 \
    > var/research-search-report.json
```

Run the full comparison after configuring Ask authentication:

```sh
# Pass only the environment names needed by your provider or Ask wrapper.
export TEND_PASS='ANTHROPIC_API_KEY ANTHROPIC_BASE_URL'
python3 examples/research.py var/research-comparison \
    --model anthropic/YOUR_MODEL \
    --evaluations 9 --trials 3 --team-size 3 --seed 7301 --seconds 1800 \
    > var/research-comparison-report.json
```

Python 3.9 or later on Unix is required. `ASK` may name an ordinary executable
credential wrapper; authentication remains Ask's concern. The common worker
setup also resolves and pins `PLY` in model mode, although research proposals
invoke only Ask. Use a dedicated directory and its Tend root, without unrelated
jobs or additional Tend workers.

`--arms` accepts a distinct subset of `search single team`; all three is the
default. Only `--arms search` runs without a model. `--model` defaults to
`ASK_MODEL`. `--evaluations` accepts 1–35 slots, `--trials` 1–5, `--team-size` 1–3
and `--seconds` 1–86400. Team size also bounds active workers per finite round;
changing it changes the team protocol. The documented comparison uses nine
slots, three trials and three roles. A changed protocol needs a new directory
and must be identified in the results.

`--dataset path.json` accepts only the exact versioned **synthetic, uncalibrated**
benchmark contract validated by `research_domain.validate_benchmark()`. It is
not a historical-data importer. Extra or missing fields, altered evaluator
rules, unsupported provenance and invalid observations fail validation.
Real company evidence requires a separately reviewed ingestion, intervention
and calibration design before it can support business conclusions.

The run freezes the benchmark, protocol manifest, source digests, executable
paths/hashes and creation time. Its deadline is creation time plus the declared
seconds, so resuming the same command does not buy more time. Each Tend job is
limited to five minutes and each model command to two minutes. Changed inputs,
sources, tools or protocol options are refused in an existing directory.

Stdout is one final JSON report; progress goes to stderr. Exit 0 means the
comparison and final confirmations completed, including unsuccessful scientific
outcomes. Exit 2 indicates a controller/input/execution failure. Exit 130
indicates an interrupted or expired experiment. An interrupted run can be
revisited with the identical command to reconstruct completed evidence; an
expired deadline prevents admission of further work.

## Inspecting and reproducing the evidence

```text
study/
  experiment.json, benchmark.json       immutable protocol and complete fixture
  baseline/                            shared measured baseline
  trial-01/{search,single,team}/
    round-001/                         one finite plan and dedicated Tend root
      manifest.json, tasks.jsonl
      recipe/                          frozen worker/domain source copies
      tend/                            execution events, attempts and outputs
      tasks/<stable-job-id>/
        input.json, result.json
        session.jsonl                  Ask events for a model proposal
        verified.jsonl                 disposable verified Ask snapshot
      observations.jsonl               disposable input to Weave
  experiments.jsonl                     derived slot decisions and evidence refs
  selections.json                       immutable selections before confirmation
  confirmation/                        final evaluations bound to selections
  report.json                          derived comparison and usage summary
```

Tend's events and bound attempt outputs establish execution outcomes. The
adapter checks task, input, dependency, source and artifact identities. Ask
sessions are checked using `ask replay -check -json`; accepted proposal results
cite matching sealed business-check notes. The experiment ledger and report
are derived views of these authorities, not a second conversation or job log.
Manifest and selection files are immutable inputs to later work.

Inspect a round and then a session named by its result evidence:

```sh
TEND_ROOT="$PWD/var/research-comparison/trial-01/team/round-001/tend" "$TEND" list
TEND_ROOT="$PWD/var/research-comparison/trial-01/team/round-001/tend" "$TEND" events
"$ASK" replay -check -json /path/from/evidence/session.jsonl
```

An independently installed Trail can also read those Ask logs; it is not a
dependency installed by this recipe's bootstrap.

The report records spent slots, accepted evaluations, failures, duplicates,
verified request/retry counts, available token usage and timing completeness.
Usage from failed or partial calls, or receipts without usable counts and
duration, is marked incomplete. Cache reads and cache writes have separate
counters; reasoning is recorded separately but must not be added again to output
tokens. Interpret provider-specific cache accounting before combining counters.
Execution spans come from Tend attempt timestamps; model duration comes from
Ask events. These measures are not a hard dollar cap, a reservation of provider
spend, or a complete hardware-cost comparison. Equal proposal/evaluation caps
do not imply equal realized tokens or wall time.

Three trials explore search variability on one fixed benchmark. They are not
three independent business samples, and a synthetic advantage does not establish
a causal operational benefit. Preserve failed and inconclusive runs alongside
successful ones when deciding whether a larger, calibrated trial is warranted.

After the comparison report completes and all selections are frozen, a separate
diagnostic may enumerate all 36 configurations on development and
feedback-validation using ordinary computation. It distinguishes failure to find
a qualifying candidate from the absence of any qualifying candidate in this
finite space. This diagnostic is outside each arm's nine-slot allowance and
excluded from the comparison. It makes no model calls, changes no selections,
and must not be used to tune the contract or rerun the arms. Report its results
separately from the budgeted trial.

## Relationship to autoresearch

This recipe adapts the narrow experiment scope, fixed evaluator, measured
baseline and retained experiment evidence from Karpathy's
[autoresearch README at commit `228791f`](https://github.com/karpathy/autoresearch/blob/228791fb499afffb54b46200aca536f79142f117/README.md).
That project restricts agent changes to a training file and uses a fixed training
time budget. Here the editable object is a four-choice intervention, while all
arms share evaluation allowances and frozen domain rules.

Its pinned [research program](https://github.com/karpathy/autoresearch/blob/228791fb499afffb54b46200aca536f79142f117/program.md)
records kept, discarded and crashed experiments and directs an indefinite
improve/evaluate loop. We retain measured retain/discard decisions and failed
experiments, but use finite rounds, no replacement slots, a fixed deadline and
confirmation after all selections freeze. The recipe does not adopt an endless
agent loop, agent-authored evaluator changes, or extrapolate language-model
training gains into business improvements.
