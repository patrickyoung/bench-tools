# Gather first, review in parallel

These records demonstrate readiness, without running a model or executing work.
The observation is deliberately fabricated example data; its reference is not
evidence of a completed business task. A real adapter must verify actual results.

Install Weave, then run from its source directory (`cd tools/weave` from the
monorepo root). No model, Python caller, or Tend queue is needed:

```sh
weave examples/quickstart/tasks.jsonl < /dev/null
weave examples/quickstart/tasks.jsonl < examples/quickstart/observations.jsonl
```

The first command prints `baseline`. The second prints `operations` and
`controls`, preserving each complete input record. `compare` remains blocked
until both reviews have verified accepted observations.

Inspect `tasks.jsonl`: the reviews name `baseline` in their `needs` lists, and
`compare` names both reviews. The observation snapshot binds its accepted
baseline to the exact task bytes. Editing those bytes makes that observation
stale and Weave refuses it.

In a real workflow, a caller sends ready tasks to ordinary programs or Agent
experts, checks the results, and supplies the next observations. Tend can keep
those attempts durable. Empty output alone never establishes that the whole
job is finished; inspect whether tasks are accepted, running, or blocked.
