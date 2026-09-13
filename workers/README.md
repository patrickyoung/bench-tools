# Reusable workers

Source definitions for new work. Exports contain no previous deliverables,
briefs, run history, installed dependencies or development files.

| Worker | Purpose | Authoritative record |
| --- | --- | --- |
| [Page team](page-team/expert/README.md) | Build and review one self-contained web page; select only useful specialists | [Status, owner, requirements and approved files](page-team/worker.json) |
| [Visual artist](visual-artist/expert/README.md) | Create p5.js/D3 artwork and artistic data experiences; add purposeful optional sensor interactions | [Status, owner, requirements and approved files](visual-artist/worker.json) |

From the repository root:

```sh
python3 scripts/workers list --all
python3 scripts/workers check
git rev-parse HEAD
python3 scripts/workers export page-team /absolute/path/to/team \
  --ref FULL_COMMIT --allow-experimental
```

Use the full reviewed commit printed by Git. Export requires a new destination
and creates `expert/` plus `team.lock.json`. Its source comes from that commit,
even when the local working tree has edits. The lock records source identity,
requirements and every exported file's digest and executable mode. Agent uses
the expert folder; it does not load the lock as instructions.

Only active entries appear in the default list and export normally.
`--allow-experimental` permits deliberate evaluation. Deprecated and retired
entries are excluded from new exports at the selected source revision. An older
pin retains its historical status; this is source management, not revocation.

Install dependencies in the exported copy, using its setup instructions.
Never install them into this source library. Keep work and evidence elsewhere.
Use [a team recipe](../teams/README.md) to describe the reusable assembly,
then supply a new goal and current inputs separately.

Add or modify workers through ordinary GitHub pull requests. Update metadata,
source and relevant synthetic tests together. Review actual source content:
an allowed Markdown filename is not proof that it contains no prior job data.
See the [lifecycle and packaging guide](../docs/WORKER-LIBRARY.md).

Architect, Writer and Present are outside this library. Existing example
showcases remain under `examples/` and are never included by worker exports.
