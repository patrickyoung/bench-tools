# Machine-observable interaction contract

These data attributes support independent tests; do not display implementation
details in the UI. Use native accessible controls, meaningful labels and focus.

For the EPL showcase expose at least four illustrative player cards with varied
positions and ages. Each visible card has `data-player-card`, `data-player-id`,
`data-player-name`, `data-position`, and numeric `data-age`. Filtering can hide
cards; keep metadata in the DOM. Search input: `data-search`. Position select:
`data-position-filter`, with empty value for all. Maximum-age input:
`data-age-filter`, editable numeric input with min/max matching the dataset.

Card profile button: `data-open-profile="ID"`. Profile dialog:
`data-profile-dialog`, native dialog preferred, with `data-close-profile`.
Include `data-strengths`, `data-uncertainties`, and `data-next-action` sections
containing actual profile-specific content. Close restores focus to the opener.

Comparison toggles: `data-compare="ID"` with aria-pressed state.
`data-open-comparison` opens `data-comparison`, containing two or more
`data-compared-player="ID"` sections and metrics with visible units.
`data-close-comparison` closes it. Selection must survive opening profiles.

Shortlist buttons: `data-shortlist="ID"`, aria-pressed true/false.
`data-shortlist-count` displays the exact saved count. No backend is necessary;
in-memory state is sufficient, with local storage as an optional enhancement
that must not break an offline file or privacy-restricted browser.

Animation fallback: show `data-visual-fallback` when the selected rendering
context is unavailable. Reduced motion stops nonessential animation scheduling;
a static composition remains. Functional content must work without a canvas.

For other briefs, retain the animation contract and include a nonempty
`<script id="page-tests" type="application/json">` with representative journeys:
`[{"name":"Open details","steps":[{"action":"click","selector":"#details-button"},
{"action":"expectVisible","selector":"#details-panel"}]}]`.
Supported actions: click, fill (value), select (value), expectVisible,
expectHidden, expectText (value substring), expectCount (numeric value).
Each journey must include an assertion of a meaningful state change.

Embed selected image-generation `generated.png`, Blender `preview.png`, and
GIMP `composite.png`/`composite.webp` bytes unchanged when those specialists were
requested. This binds page use to checked sources. Preserve an accepted page's
exact bytes when copying it after a passing review. Changing the page requires
a new review; reports for a prior hash cannot approve a new result.
