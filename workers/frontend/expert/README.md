# Frontend

Builds and integrates a self-contained, accessible web page from the current brief and accepted inputs.

Export this worker with the source-library utility, then use its own definition:

```sh
agent run -C /path/to/current-work -evidence /path/to/control \
  /path/to/export/expert -- 'The bounded current assignment'
```

Supply the brief and any accepted visual contributions under inputs/. Each run receives only its explicit inputs and its own context/workspace.
Dependencies are listed in worker.json in the source catalog. Keep credentials,
model selection, work, evidence and installed dependencies outside source.

Deliver `output/` containing index.html, plus `handoff.json`. The public handoff
uses specialist name `frontend`; an assembly binds that role to this worker ID.
The deterministic helpers are carried by this definition and resolve locally;
no parent team folder or PAGE_TEAM_ROOT setting is required.

The check validates the static single-file contract and the bound handoff. Browser behavior, accessibility and visual quality require separate review.

For a positive contract case, supply the required artifacts and run
`tools/make-handoff frontend SUMMARY output/...`, then `bin/check` from the
workspace. A missing required file or a changed manifested byte must fail.
Run source, browser or native-tool review appropriate to the actual task before
accepting quality. The repository has synthetic regressions for these boundaries.

This source was extracted from the existing page-team specialist, with its
small existing handoff utility copied for independent use. Hire adapts the
definition; Agent supplies the execution loop. No prior page or run is included.

## CSS-first judgment amendment

`skills/css-first-layout/SKILL.md` and its compact semantics reference teach
content-first flow/Grid/Flex/subgrid selection, intrinsic/container-responsive
layout, accessible fluid type, logical spacing, modest cascade/tokens, native
state, color and progressive static-safe motion. AGENTS.md explicitly loads
these for construction/layout/refactor work in a fresh context. Examples are
illustrations, not practice deliverables. Small JS remains appropriate for
application state, persistence, data, playback and focus.

The evidence packet was reviewed 2026-09-17; source maturity is not browser
support. Offline runs retain conservative baselines and disclose unsupported
or untested assumptions rather than fetching pages. The reference corrects
article shortcuts about layers, scope, nesting, focus-visible and transitions.
For passing-review copy assignments this teaching defers completely: preserve
approved bytes, do not restyle. Existing exact-byte creative integration,
INTERACTIONS.md, dependencies and controller-owned review remain unchanged.

Structural amendment checks (no worker execution):
`brief lint -strict expert/skills` and `hire verify expert` from the definition's
parent directory. These inspect format/readiness only, not CSS quality,
browser support, interaction behavior or fresh-case success. The operator
evaluates real cases separately with the existing acceptance/review contract.
