# Support-router experiment

This is a small example of the [Improve protocol](../../README.md), with two
independently labeled notes per split. It demonstrates composition and a targeted
case probe, not broad quality or statistical confidence.

The source checker validates output shape. `bench.py trial` independently
compares final output with frozen labels and preserves failure, stop reasons,
reasoning usage and retry observations. `judge.py` requires perfect outputs,
no regressions or additional calls, complete billing, at least 10% lower total
cost by default and savings in a majority of pairs. Set `--min-savings-percent 7`
when generating a spec for a 7% experiment. The validated
`settings.min_savings_percent` value drives both the authoring brief and judge;
it is adapter policy, not an Improve core flag. Freeze it before comparison. The proposer edits only AGENTS.md.

The trial score separates `json_valid`, `shape_valid`, `ids_valid` and
`order_valid`. `semantic_rows` compares queues by unambiguous ID regardless of
order; an unidentifiable answer has null semantic correctness, not invented
wrong labels. `rows` and `correct` retain the strict ordered-output score.
`accepted` means the entire job is correct and its runner exited successfully;
valid format alone cannot pass. Semantic diagnostics do not repair the output
or weaken the frozen gate.

Use the synthetic fixture first:

```sh
python3 spec.py --offline > /tmp/router-offline.json
improve -n < /tmp/router-offline.json
improve -o /tmp/router-offline-001 < /tmp/router-offline.json
improve verify /tmp/router-offline-001
```

Fixture costs and quality are invented. The fixture calls no model, but the
controller still uses real Record/Ask local recording and replay.

For an explicitly authorized paid experiment, put independently installed
`improve`, `record`, `ask`, `hire`, `agent`, `ply`, `brief` and `cage` on PATH.
Configure the selected provider through Ask's existing mechanism. From here:

```sh
python3 spec.py --model PROVIDER/RUNNER --proposer-model PROVIDER/AUTHOR > /tmp/router-live.json
improve -n < /tmp/router-live.json
improve -o /tmp/router-live-001 < /tmp/router-live.json
```

Replace both model placeholders. Generating/planning the spec makes no model
calls; executing it can make paid calls. Maximum workload: 15 worker jobs,
one four-turn Hire invocation and two deterministic judges. Worker jobs use
three turns, one cycle, a 40-second command timeout and the experiment's
230-second per-adapter deadline. These adapters select native Ask without an
extra token-cap wrapper; configure any additional provider cap explicitly.

`--source /absolute/expert` selects another compatible support router.
`--effort` selects Ask's effort value, default low. Program paths and adapter
files are pinned in the generated spec. Keep the same PATH when executing:
Record discovers its own Ask dependency independently. The adapters explicitly
select Agent's Ask/Ply/Record/Brief/Cage and Hire's Agent. Weigh is disabled in
these adapters. No provider clients or alternative agent loops are implemented.

The Hire adapter is deliberately narrow. Its model-written worker/check files
are never executed by the authoring stage; only AGENTS.md changes are returned
for evaluation. The model receives development inputs and diagnostic scores,
including stop reasons and reasoning usage. Its hypothesis is still a proposal,
not an established diagnosis. The trial adapter treats native retries or
missing response usage as unknown billing; the gate rejects those costs.

The published cases are development material for anyone who has read them.
Replace both splits and freeze fresh reserved cases before claiming a new
worker improvement. Promotion uses the exact exported source and retained
comparison; no source library or default runtime model changes automatically.
