# Agent-home evaluation corpus

This frozen corpus exercises lifecycle states that are easy to blur together:

- `done`: its executable check already accepts; re-entry must finish without
  calling the deliberately failing model fixture, and its archived Ask session
  must replay exactly;
- `quiet-watch`: its nonempty heartbeat has a quiet `bin/wake`; a tick must
  return zero before resolving Ply or calling a model;
- `broken-watch`: its wake probe exits outside the 0/1 protocol; a tick must
  return the controller-broken status 2;
- `delegator`: a valid parent with one complete nested specialist home, proving
  recursive validation and separately rooted evidence.

Run the evaluator from Agent's source directory (`cd tools/agent` from the
monorepo root):

```sh
sh eval/run.sh
```

It copies the corpus to a private temporary directory, so mutable roots and
evidence are never written in the source fixtures. It requires the installed
public `brief`, `ply`, `cage`, `trail`, and `ask` programs. The report is TSV on
stdout; diagnostics stay on stderr. A failed invariant exits nonzero.

Go builds the current Agent into the temporary directory by default; set
`AGENT_TEST_EXECUTABLE` to an already built Agent to select it explicitly.
Archive validation calls Trail directly. Authoring and home maintenance now
belong to Hire, so the evaluator does not use removed Agent builder commands.

For an example that actually drafts and repairs a result, use the monorepo's
[support-reply check](https://github.com/patrickyoung/bench-tools/blob/main/examples/README.md).
It drives the real Agent/Brief/Ask/Ply/Cite commands with a local model fixture;
this corpus focuses on lifecycle boundaries that need no model response.

This is an integration corpus, not a benchmark score. It measures deterministic
boundaries: offline validation, exact compiled bytes, zero-model gates, recursive
homes, and Ask-owned replay. Model quality and false-success experiments require
separate frozen tasks and an external oracle.

## Read-only handoff input proof

Run `python3 eval/protected-inputs.py AGENT CAGE PLY BRIEF ASK RECORD [OUTPUT_DIR]`
with physical public executable paths and a fresh output directory. This never
calls a model. Real Agent/Ply/Cage/Record verify a passing precheck; a controlled
Ply process driver exercises the real Agent action interpreter and checker
wrapper, including unchanged checker rejection 1 and interruption 130. Both
boundaries attempt overwrites, unlinking, input-directory renames, and rewriting
the frozen policy, then write legitimate output. Results, streams, argv and
policy are retained. This proves executable composition and OS enforcement,
not real-model quality or full Ply loop behavior.
