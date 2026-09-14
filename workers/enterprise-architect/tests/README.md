# Enterprise Architect acceptance

These synthetic cases exercise the exported worker's artifact boundary without
a model. They cover all ten output families, required roles, unknown inputs,
complete input/output/profile bindings, wrong JSON types, symlinks, nonregular
and oversized files, bounded trees, and non-execution of generated policy code.
Each case copies the selected definition into a disposable directory.

From the repository root:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s workers/enterprise-architect/tests -v
```

CI exports the exact committed source first, then runs the same suite with
`ENTERPRISE_ARCHITECT_EXPERT=/absolute/export/expert`. The environment variable
selects the source for tests; the worker's runtime check always uses its own
profile. Tests and fixtures are never exported with the worker.

These checks establish package integrity, not architecture competence. Evaluate
real model runs separately, in fresh workspaces with explicit current inputs,
controller evidence and independent review criteria. Review visual readability,
catalog semantics, outcome calculations, platform guidance and decision quality.
Inspect generated policies before running them in a separate execution boundary;
the artifact verifier deliberately never executes output code.

The source-only [design brief](../../../docs/ENTERPRISE-ARCHITECT-DESIGN.md)
maps the ten skills and requested outputs to acceptance expectations. Keep
real inputs, generated artifacts, evaluation findings and model records outside
the reusable library. Retain failures and distinguish visible regression cases
from a genuinely inaccessible benchmark.
