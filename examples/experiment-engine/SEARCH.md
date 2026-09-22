# Run a bounded instruction search

**Historical research recipe.** Use [native Improve](../../tools/improve/README.md)
for new experiments. This earlier Python controller is retained to understand
past studies and run its offline regressions; do not extend it as a parallel
controller. Its old acceptance/search policy is not the current Improve router
gate. Commands below document the historical workflow.


`search.py` composes the existing trial and scoring filters into a finite research
loop. The first target is the existing naive support router. Only `AGENTS.md`
can change; this is not yet a general-purpose installed Bench tool.

Supply a JSON spec outside the checkout. All paths can be absolute; expert/cases
paths are otherwise relative to the spec, and case input/label paths to the
cases file. The proposer is literal argv, never a shell command string.

```json
{
  "version": 1,
  "expert": "/selected/expert",
  "cases": "/selected/cases.json",
  "model": "openrouter/deepseek/deepseek-v4.1-flash",
  "effort": "off",
  "iterations": 2,
  "repeats": 2,
  "max_trials": 14,
  "max_seconds": 1200,
  "proposer_argv": [
    "python3", "/checkout/examples/experiment-engine/propose.py",
    "--model", "openrouter/deepseek/deepseek-v4.1-flash"
  ],
  "agent": "/selected/bin/agent",
  "ask": "/selected/bin/ask",
  "record": "/selected/bin/record"
}
```

The model above is an explicit example, not a baked-in default. Proposer and
worker models may differ. Load your selected Bench environment and credential
profile; preserve approved wrappers. `propose.py --hire` and `--ask` select its
public commands. Credentials belong only in the process environment or existing
private wrappers, never in the spec, argv, input cases or definition.

A cases file selects development and held-out case batches:

```json
{
  "development": [
    {"id": "current-request", "family": "historical-keywords",
     "input": "development.json", "labels": "development-labels.json"}
  ],
  "holdout": [
    {"id": "active-priority", "family": "multiple-active-issues",
     "input": "holdout.json", "labels": "holdout-labels.json"}
  ]
}
```

Inputs and labels use the [existing router contract](README.md#run-one-trial).
Freeze them before the run. IDs, families and normalized note text cannot overlap
between splits. Family names are caller assertions: this detects accidental
reuse, not semantic similarity. These are small family probes, not proof of
broad generalization. The proposer receives development inputs, labels, scores,
current instructions and prior decisions; held-out contents/paths are omitted.
Host reads remain possible, so this is not an adversarial secrecy boundary.

Validate the spec and preview the maximum workload without executing commands:

```sh
python3 search.py --spec /selected/spec.json --out /selected/research/run-01
```

Run explicitly, with a new output directory outside source:

```sh
python3 search.py --spec /selected/spec.json --out /selected/research/run-01 --live \
  > /selected/research/run-01-summary.json
```

One development batch, one held-out batch, two repeats and two proposals permit
14 worker invocations: two initial diagnostics, eight fresh search comparisons,
and four final comparison trials. Each worker invocation retains the existing
three-turn, one-cycle, 40-second command and 180-second outer limits. The supplied
Hire proposer receives the selected evidence directly in its brief and permits four turns and one cycle per proposal, with a 230-second
outer limit. Agent’s `-timeout` bounds action/check commands, not provider
requests; The recipe’s outer deadline bounds the whole model-backed invocation and
sends termination through Record, Agent and Ply before escalating.
Thus this configuration permits at most 50 model requests under
the supplied runners. Different proposer commands own their internal limits.
The total wall-clock deadline sends termination to the selected process group;
cleanup may add up to ten seconds. Record is given five seconds to forward
cancellation before escalation, allowing Ply to stop separately grouped model
commands. Remote providers may still bill work already started. Failed work is not retried. Remaining trial capacity is
reserved for the final comparison. A dollar ceiling is not promised; actual
reported provider costs are recorded, and missing usage stays unknown.

The frozen `judge.py` rule accepts either:

- More usable correct results in **every repeat**, no case regression and no
  unfinished candidate trial; or
- Perfect results in both arms, shorter instructions, fewer measured input
  tokens in every pair and no additional model calls.

A failed worker invocation contributes zero usable correct rows. Broken receipts
invalidate the study. A baseline without model evidence stops the search before
proposal generation. If all proposals fail before comparison, the result is
inconclusive. A shorter answer alone is not lower total cost. Costs,
output tokens and elapsed time remain visible tradeoffs for the quality gate.
This search gate is separate from the earlier strict `compare.py` experiment.

A successful development candidate becomes the next local baseline automatically.
The final candidate is frozen before one held-out comparison against the original.
No subsequent proposal sees that result. If it fails, no export is produced.
Do not repeatedly tune on these held-out cases in later studies.

`result.json` contains the verdict, all decisions, budgets used and the cost
ledger. `known_provider_cost` is a subtotal; `total_provider_cost` is null if any
stage lacks complete billing. The supplied Hire adapter verifies its Ask sessions
before reporting usage. Other trusted proposer adapters own their usage reporting;
Record proves streams/process outcomes, not the truth of arbitrary cost claims.

Exit 0 means the held-out comparison supported the candidate; 1 means no supported
proposal or exhausted budget; 2 means invalid invocation/evidence. Progress goes
to stderr and one JSON result to stdout. Interruptions retain evidence and yield
an inconclusive result. There is deliberately no automatic resume or retry.

On success, `proposal/expert`, `proposal/change.diff` and `proposal/manifest.json`
contain the exact evaluated source and original rollback copy reference. They
are proposals: the source worker and shared library are unchanged. Follow the
existing review and promotion procedure to adopt them. Do not invent a Hone
recovery from an offline score comparison.

Weigh is absent from the default proposer and worker path. The controller sets
`BENCH_WEIGH=0`. A caller-selected proposer may explicitly compose Weigh for a
separate ablation, but must supply its own authorization/configuration and complete
cost accounting; this switch is a preference, not command confinement.

## Verify without hosted models

```sh
python3 -m unittest discover -s . -p 'test_*.py' -v
BENCH_TEST_BIN_DIR=/selected/bin python3 -m unittest discover -s . -p 'test_search.py' -v
```

The second command exercises real Agent/Ply/Ask/Cage/Record composition against a
loopback provider fixture. It proves the keep/discard/promotion plumbing, not
model quality. Live results and credentials never belong in this example tree.
