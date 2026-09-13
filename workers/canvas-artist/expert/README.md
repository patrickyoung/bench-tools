# Canvas artist

Creates compact Canvas/WebGL art with a static fallback. The separate Visual Artist worker specializes in p5.js and D3 artistic data experiences.

Export this worker with the source-library utility, then use its own definition:

```sh
agent run -C /path/to/current-work -evidence /path/to/control \
  /path/to/export/expert -- 'The bounded current assignment'
```

Supply a bounded visual brief and embed constraints. Each run receives only its explicit inputs and its own context/workspace.
Dependencies are listed in worker.json in the source catalog. Keep credentials,
model selection, work, evidence and installed dependencies outside source.

Deliver `output/` containing visual.js, visual.css, fallback.svg and visual-notes.md, plus `handoff.json`. The public handoff
uses specialist name `visual-artist`; an assembly binds that role to this worker ID.
The deterministic helpers are carried by this definition and resolve locally;
no parent team folder or PAGE_TEAM_ROOT setting is required.

The check validates required nonempty files, JavaScript syntax and the bound handoff. It does not prove browser behavior or artistic quality.

For a positive contract case, supply the required artifacts and run
`tools/make-handoff visual-artist SUMMARY output/...`, then `bin/check` from the
workspace. A missing required file or a changed manifested byte must fail.
Run source, browser or native-tool review appropriate to the actual task before
accepting quality. The repository has synthetic regressions for these boundaries.

This source was extracted from the existing page-team specialist, with its
small existing handoff utility copied for independent use. Hire adapts the
definition; Agent supplies the execution loop. No prior page or run is included.
