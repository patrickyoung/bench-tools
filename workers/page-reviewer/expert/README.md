# Page reviewer

Writes a candid review from the current page, brief and supplied browser/visual observations.

Export this worker with the source-library utility, then use its own definition:

```sh
agent run -C /path/to/current-work -evidence /path/to/control \
  /path/to/export/expert -- 'The bounded current assignment'
```

Supply the candidate page and review_observations from a selected external review process. Missing evidence must be stated, never invented. Each run receives only its explicit inputs and its own context/workspace.
Dependencies are listed in worker.json in the source catalog. Keep credentials,
model selection, work, evidence and installed dependencies outside source.

Deliver `output/` containing review.json and review.md, plus `handoff.json`. The public handoff
uses specialist name `review`; an assembly binds that role to this worker ID.
The deterministic helpers are carried by this definition and resolve locally;
no parent team folder or PAGE_TEAM_ROOT setting is required.

The check validates review structure and the bound handoff. A well-formed report can describe a failing page; it does not approve a page for publication.

For a positive contract case, supply the required artifacts and run
`tools/make-handoff review SUMMARY output/...`, then `bin/check` from the
workspace. A missing required file or a changed manifested byte must fail.
Run source, browser or native-tool review appropriate to the actual task before
accepting quality. The repository has synthetic regressions for these boundaries.

This source was extracted from the existing page-team specialist, with its
small existing handoff utility copied for independent use. Hire adapts the
definition; Agent supplies the execution loop. No prior page or run is included.
