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

Run the evaluator from the repository root:

```sh
sh eval/run.sh
```

It copies the corpus to a private temporary directory, so mutable roots and
evidence are never written in the source fixtures. It requires the installed
public `brief`, `ply`, `cage`, `trail`, and `ask` programs. The report is TSV on
stdout; diagnostics stay on stderr. A failed invariant exits nonzero.

This is an integration corpus, not a benchmark score. It measures deterministic
boundaries: offline validation, exact compiled bytes, zero-model gates, recursive
homes, and Ask-owned replay. Model quality and false-success experiments require
separate frozen tasks and an external oracle.
