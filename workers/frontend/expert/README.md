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
