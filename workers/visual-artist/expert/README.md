# Bench Visual Artist

A portable filesystem expert for original p5.js/D3 website artwork and artistic data visualization. It creates source and a local browser bundle, not a complete website. It deliberately has no provider client, scheduler, server, or nested worker.

## Setup and invocation

Requires Node 22+, Python 3.9+, Agent with its installed Bench companions and configured Ask provider, and the pinned packages in `package-lock.json`. After exporting the definition, the operator runs `npm ci --ignore-scripts` in the exported definition directory, never in a run workspace or source-library checkout. Playwright is an optional development dependency for independent browser review; the checker does not launch it. Supply a permitted installed browser, or install Playwright's Chromium in the operator's environment.

Create an empty workspace containing only admitted inputs, then run:

    agent run -C /path/to/workspace -evidence /path/to/evidence /path/to/visual-artist -- goal text...

Describe the intended experience in plain language and identify any current input files. Purpose, audience, mood and embed constraints help; let the worker infer routine design choices. It requests missing meaning or units only when essential to an honest result. Agent provides `AGENT_HOME`; the worker resolves pinned libraries there and does not install during a run.

A page team consumes `handoff.json` using the existing `page-team.handoff/v1` contract. Each manifest entry binds an `output/` relative path to its absolute workspace source, byte length, digest, and media type. The specialist is `visual-artist`. Copy or inspect only after running the definition's check from that same workspace.

## Output and validation

The worker writes the required artifact beneath `output/` plus `handoff.json`. `output/visual.js` exposes the synchronous `BenchVisual.mount` contract; `visual-source.js` remains editable; notes document the build and measured review.

Run from the artifact workspace:

    /path/to/visual-artist/bin/check

The check validates regular non-symlink required files, metadata's exact field types, supported pinned library declarations, current input digests and safe paths, JavaScript syntax without executing authored JavaScript, safe static SVG XML, and complete handoff coverage/digests. Exit 0 is structural acceptance, exit 1 is unfinished or invalid output, and another nonzero status indicates a broken or unavailable verifier dependency.

Structural acceptance does **not** prove aesthetics, truthful or perceptually effective encoding, sensor privacy, accessibility, or browser behavior. Input bindings cover declared files; they cannot prove the worker declared every relevant source or used it correctly. Always use a fresh workspace for a new goal. Those claims require independent source review, visual review, and browser tests. The checker never activates sensors and never executes the generated bundle. Fallbacks use a deliberately small static SVG subset: geometry, text and local gradients/definitions with presentation attributes; no CSS, images, animation or foreign content.

## Cases

**Positive case:** a brief admits `inputs/tides.csv` with provenance and units and asks for a calm explorable tidal tapestry. The worker maps time and height to documented woven channels, bundles an appropriate pinned library, records the exact input digest, supplies static/text alternatives, tests lifecycle and reduced motion, and manifests every output file. The check accepts its structure after independent artistic/browser review.

**Negative case:** an artifact uses a CDN script, claims live location without an intentional enable/stop/manual fallback, declares a stale data digest, includes an SVG event handler, omits an extra source file from the handoff, or replaces the brief with a dashboard. Digest, SVG, metadata, or manifest faults are rejected structurally; CDN use, misleading art, and faulty browser behavior must also be rejected by source/browser/visual review.

To reproduce checker counterexamples, copy a completed run into a disposable workspace and regenerate its handoff there, since source paths bind to the original workspace. Verify that copy first, then alter one declared input byte or add an unmanifested file under `output/`; each altered copy must exit 1. Do not test against production artifacts in place.

## Source reuse

`tools/make-handoff` and `bin/validate-handoff` are unmodified copies of the MIT-licensed page-team utilities from Bench commit `27d191fa0528a10fcf42b0ca26a8b8ea04b14bc9`. This expert carries its own copies so it exports independently, without a sibling checkout. The existing Agent/Hire tools provide all model execution and construction. The three small skills provide artistic judgment, p5/D3 craft and optional sensor procedures. No generated artwork, previous job content, memories of prior runs or installed dependencies belong in this definition.
