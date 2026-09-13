# Visual artist verification

The synthetic suite checks the definition's file boundary without generating
or executing an artwork. It covers accepted local artifacts, tampering, stale
declared inputs, symlinks and escaping paths, complete manifests, invalid
metadata and library declarations, JavaScript syntax, static SVG restrictions
and unavailable checker dependencies.

From the repository root:

```sh
python3 -m unittest discover -s workers/visual-artist/tests -v
```

Set `VISUAL_ARTIST_EXPERT` to test a clean exported expert instead of the source
definition. The suite requires Node 22+ and Python 3.9+, makes no model calls,
installs no packages and removes its temporary fixtures. GitHub checks run it
against an export of the exact checked commit.

For a release evaluation, separately run the exported worker through Agent on
at least two fresh briefs: a p5 simulation/art piece and an expressive D3 data
piece. Install pinned dependencies only in that export. Keep goals, input data,
outputs and evidence outside this source tree. Record the model, limits, source
lock, failed attempts and checks. Run Ask replay verification on the resulting
records. Replay integrity and structural acceptance do not prove useful art.

Review the actual generated source and rendered desktop/mobile output. Check
the intended metaphor and data mappings, missing/extreme inputs, literal text
handling, keyboard details, host CSS isolation, independent mounts and IDs,
update, resize, immediate/repeated destroy, offscreen/hidden behavior, static
fallback, and both initial and changing reduced-motion preferences. Inspect
network requests. Sensor cases use synthetic browser API mocks for permission
granted/denied/unavailable, overlapping requests, late grants after stop,
source-ended, pagehide and cleanup. Actual hardware testing is a separate
authorized check. A broad skill description is not a claim of universal device
support or successful artistic judgment on every brief.
