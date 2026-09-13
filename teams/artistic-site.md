# Artistic site team

Choose this assembly when the site benefits from an original artistic visual:
data made into an expressive form, a simulation, or an interactive art piece.
The Visual artist favors artistic visualization over charts and dashboards.
The caller supplies the current purpose and inputs in ordinary language.

Export `page-team` and `visual-artist` separately at full reviewed commits using
the [library exporter](../workers/README.md). Keep both source locks. The page
team already has a `visual-artist` slot and a public command adapter; no new
runner is necessary.

Use Hire on the fresh page-team authoring directory to replace only its
`expert/agents/visual-artist` definition with the clean standalone artist
export. Supply both export locations and this recipe in the build goal. Copy
the source files, including its checker, handoff utilities and pinned package
files, without symlinks or references back to the source checkout. Inspect the
result and record the adaptation and revised digests alongside both original
locks. The original page-team lock does not describe the adapted files.

Install dependencies in the assembled root expert and in its new artist
definition with each one's `npm ci --ignore-scripts`. Do this before Agent
runs. The artist resolves p5, D3 and esbuild from its own `AGENT_HOME`.

The manager assigns the admitted visual brief and current data to Visual
artist. Its `page-team.handoff/v1` manifest retains specialist `visual-artist`
and the existing `visual.js`, `visual.css`, `fallback.svg` and notes filenames.
Additional handoff files preserve editable source, preview, dependency notices
and `bench.visual/v1` encoding/input metadata. Existing manifest validation and
materialization can consume these ordinary files.

Frontend embeds the accepted bundle and styles unchanged, mounts
`BenchVisual.mount(root, options)` with the admitted data, and preserves its
controls and textual explanation. A single-file page must inline the local
bundle, CSS and fallback; it must not depend on the artist's private workspace
or preview files at runtime. It calls `destroy()` when removing the component.
Select only additional specialists that this particular site needs.

Reviewer checks the actual artwork in the final page: artistic intent and
legible data mapping, mobile layout, keyboard interaction, independent mounts,
update/resize/cleanup, reduced motion and fallback. For a sensor-enabled brief,
use browser mocks to cover opt-in, denial, pending-then-stop, release and manual
alternatives. Structural checks alone cannot establish artistic or browser
quality. Actual device testing remains a separately authorized check.

Keep job inputs, art, previews, screenshots, installed packages and run records
outside reusable source. This is a documented assembly recipe, not a prebuilt
or automatically installed team; evaluate the specific assembly before use.
