# Add expertise through files

Add `expert/agents/NAME/AGENTS.md`, a focused `bin/check`, any selected skills
and tool bindings, and `expert/bin/workers/NAME`. The worker binding is only:

```sh
#!/bin/sh
export PAGE_TEAM_SPECIALIST=copy-editor
exec "$(dirname "$0")/../run-worker"
```

Make it executable. Bench Manage discovers executable worker names and supplies
them to the planning manager. The same worker adapter starts the new expert
through Agent, validates its handoff and materializes only accepted inputs.
No runner, scheduler, registry or server change is needed.

For a concrete extension test, copy `tests/copy-editor` into the copied expert's
`agents/` directory and create that binding. This deliberately narrow fixture
corrects one spelling error while preserving an uncertainty statement. It is
a proof of extension mechanics, not a general copy-editing evaluation.

The fixture manager and root check below arrange a single real Agent task.
They are deterministic test programs, not replacements for the page manager.
`EXTENSION_BUNDLE` selects the absolute copied expert directory:

```sh
export EXTENSION_BUNDLE=/absolute/path/to/page-team/expert
printf '%s\n' 'Exercise the copy-editor extension and preserve uncertainty.' |
  "$BENCH_MANAGE" run -C /absolute/path/to/runs/extension-001 \
  -manager /absolute/path/to/page-team/tests/extension-manager \
  -workers "$EXTENSION_BUNDLE/bin/workers" \
  -task-check "$EXTENSION_BUNDLE/bin/task-check" \
  -check /absolute/path/to/page-team/tests/extension-check \
  -m "$PAGE_TEAM_MODEL" -turns 60 -attempts 12 -decisions 8 \
  -reserve-attempts 2 -reserve-turns 6 -task-seconds 3m -deadline 10m \
  -pass-env AGENT -pass-env EXTENSION_BUNDLE
```

Pass any additional configured provider or credential-wrapper variable names
with the existing `-pass-env` option, as in the interface runbook. The observed
extension run returned:

> The player shows composure under pressure. Further live observation is needed.

The worker adapter, dependency transfer, task check and page filter had
identical file hashes before and after adding this specialist. The actual
controller and Agent ran it, and its contribution and integration checks passed.
