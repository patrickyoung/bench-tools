# Blender artist

Creates an editable Blender master, a production script, a PNG preview and a GLB web export.

Export this worker with the source-library utility, then use its own definition:

```sh
agent run -C /path/to/current-work -evidence /path/to/control \
  /path/to/export/expert -- 'The bounded current assignment'
```

Supply a bounded brief and select the installed BLENDER executable. Each run receives only its explicit inputs and its own context/workspace.
Dependencies are listed in worker.json in the source catalog. Keep credentials,
model selection, work, evidence and installed dependencies outside source.

Deliver `output/` containing asset.blend, build_asset.py, preview.png and asset.glb, plus `handoff.json`. The public handoff
uses specialist name `blender`; an assembly binds that role to this worker ID.
The deterministic helpers are carried by this definition and resolve locally;
no parent team folder or PAGE_TEAM_ROOT setting is required.

The check verifies required files, basic format signatures and the bound handoff. It does not open the Blender project, prove render quality or certify the exported geometry.

For a positive contract case, supply the required artifacts and run
`tools/make-handoff blender SUMMARY output/...`, then `bin/check` from the
workspace. A missing required file or a changed manifested byte must fail.
Run source, browser or native-tool review appropriate to the actual task before
accepting quality. The repository has synthetic regressions for these boundaries.

This source was extracted from the existing page-team specialist, with its
small existing handoff utility copied for independent use. Hire adapts the
definition; Agent supplies the execution loop. No prior page or run is included.
