# Inkscape Illustrator contract tests

Run against a clean exported expert with Python 3.9+ and real Inkscape:

```sh
INKSCAPE=/absolute/path/to/inkscape python3 workers/inkscape-illustrator/tests/contracts.py /absolute/export/expert
```

These tests create disposable synthetic workspaces. Every output mutation begins with an independently accepted positive case; expected rejection diagnostics are checked. Coverage includes malformed and stale inputs, path bindings, XML/external-content restrictions, PNG bounds and corruption, missing artifacts, handoff identity, mismatched master/final/preview despite refreshed hashes, and editable-to-outlined lettering.

The fixtures do not establish artistic quality. Review actual illustrations independently for fidelity, composition, drawing craft and style. Keep live briefs, outputs and model evidence outside the reusable source library.
