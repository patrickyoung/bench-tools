# Image editor

Finishes images with editable GIMP masters, a production script and preserved originals.

Export this worker with the source-library utility, then use its own definition:

```sh
agent run -C /path/to/current-work -evidence /path/to/control \
  /path/to/export/expert -- 'The bounded current assignment'
```

Supply admitted images under inputs/ and select GIMP_CONSOLE. Each run receives only its explicit inputs and its own context/workspace.
Dependencies are listed in worker.json in the source catalog. Keep credentials,
model selection, work, evidence and installed dependencies outside source.

Deliver `output/` containing composite.xcf, finish.py, finish-notes.md, preserved originals and one composite.png or composite.webp, plus `handoff.json`. The public handoff
uses specialist name `gimp`; an assembly binds that role to this worker ID.
The deterministic helpers are carried by this definition and resolve locally;
no parent team folder or PAGE_TEAM_ROOT setting is required.

The check verifies the required source/output forms, basic XCF signature and bound handoff. It does not open or render the XCF or prove visual quality.

For a positive contract case, supply the required artifacts and run
`tools/make-handoff gimp SUMMARY output/...`, then `bin/check` from the
workspace. A missing required file or a changed manifested byte must fail.
Run source, browser or native-tool review appropriate to the actual task before
accepting quality. The repository has synthetic regressions for these boundaries.

This source was extracted from the existing page-team specialist, with its
small existing handoff utility copied for independent use. Hire adapts the
definition; Agent supplies the execution loop. No prior page or run is included.
