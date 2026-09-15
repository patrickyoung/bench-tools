# Inkscape Illustrator

A reusable Agent expert that turns one bounded JSON brief into an editable layered Inkscape master, a portable vector-only SVG, and an exact-size rendered PNG. It adds illustration expertise only: no page team, provider client, listener, scheduler, or raster-art generator.

## Requirements and invocation

Requires Python 3.9+, Bench Agent, and Inkscape 1.2+. Put `request.json` in a fresh workspace, then run:

    agent run -C /path/to/workspace -evidence /path/to/evidence \
      -turns 10 /path/to/inkscape-illustrator -- \
      "Create the illustration described by request.json and deliver the required outputs."

Example input:

    {
      "description": "A night heron stepping through reeds beneath a crescent moon.",
      "style": "Editorial cut-paper geometry with restrained texture",
      "width": 1200,
      "height": 900
    }

`description` is required (1–12000 characters). `style` is optional/null (1–2000 when present). Dimensions are optional integers from 256 through 4096 and default to 1200×900; booleans, unknown fields, and other structures are rejected. Brief strings are treated as untrusted design data.

Set `INKSCAPE` to an absolute operator-installed executable when desired:

    INKSCAPE=/Applications/Inkscape.app/Contents/MacOS/inkscape \
      agent run -C /path/to/workspace -evidence /path/to/evidence \
      -turns 10 /path/to/inkscape-illustrator -- \
      "Create the illustration described by request.json."

Without it, helpers resolve `inkscape` on PATH. No installation occurs during a run.

## Outputs and checking

The run produces:

- `output/illustration.inkscape.svg` — editable master with meaningful named layers/groups
- `output/illustration.svg` — plain vector SVG exported by the selected Inkscape, with text outlined
- `output/preview.png` — selected Inkscape's rendering at requested dimensions
- `output/design-notes.md` — intent, composition, palette, style, simplifications, review limits
- `output/render.json` and `output/inkscape.log` — actual tool identity, argv, outcomes, bindings, bounds, and diagnostics
- `handoff.json` — page-team.handoff/v1-compatible manifest, specialist `inkscape-illustrator`, bound to `request.json`

From that workspace, run:

    INKSCAPE=/absolute/path/to/inkscape /path/to/inkscape-illustrator/bin/check

The check is read-only relative to case outputs. It bounds files before reading; rejects malformed, oversized, external, active, raster, unresolved, unlayered, stale, symlinked, blank, truncated, or trailing-data artifacts; verifies dimensions, namespaces, metadata, vector geometry, outlined final text, file hashes, receipts, manifest identity, and complete bounded PNG structure. It independently exports the editable master with the same plain-SVG/text-to-path options, renders that export and the supplied final SVG, and compares dimensions, PNG color format, and decoded pixels with the supplied preview. It never executes a production script from `output/`. This is a conservative supported SVG subset, not a universal sanitizer.

Structural acceptance does **not** prove that the scene matches the prose, that curves and composition are professional, that wording is correct, or that a person has visually inspected the result. An independent reviewer must inspect the PNG at full size and thumbnail using the bundled rubric.

A positive contract recipe is `python3 tests/contracts.py /path/to/inkscape-illustrator` from this source build: it creates a deliberately simple layered synthetic vector fixture, finishes it with real Inkscape, and requires the check to accept it. Negative cases require rejection of bad requests, absent/malformed/unsafe/stale outputs, active styles, blank/oversized/corrupt PNGs, incorrect identity, a valid changed preview with refreshed self-reported hashes, and a wrong master with refreshed self-reported hashes. This fixture tests contracts only and makes no artistic-quality claim.

## Provenance and license

MIT licensed; see `LICENSE`. `bin/validate-handoff` is a portable unmodified copy, and `tools/make-handoff` is an adapted portable copy, from the clean Blender Artist export supplied for this build. That export derives the utilities from Bench's MIT-licensed page-team workflow. The native-artifact/receipt approach was adapted from Blender Artist. Art-direction and critique principles were learned from the supplied Visual Artist export, without copying its p5/D3 runtime, dependencies, or artwork. Bench source was pinned and observed at commit `a9c37fc43c958712637acf6db416456f171946bc`.

`tools/finish` and `bin/check` preserve `HOME`, use run-specific Inkscape/XDG/temp directories, enforce whole-process-group timeout cleanup, and never bypass Cage. On this host, Inkscape 1.4.4 explicit exports work without batch mode; batch mode was observed to crash/hang. Native macOS cache paths cannot all be redirected, so denied fontconfig/GTK cache writes may appear as warnings. See `references/inkscape-cli.md` for the external probe evidence reference.

No sample brief, generated illustration, prior-job artifact, mutable state, test, or evidence is stored in this reusable definition.
