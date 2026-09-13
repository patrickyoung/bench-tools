# Machine-observable interaction contract

These data attributes support independent tests; do not display implementation
details in the UI. Use native accessible controls, meaningful labels and focus.

Animation fallback: show `data-visual-fallback` when the selected rendering
context is unavailable. Reduced motion stops nonessential animation scheduling;
a static composition remains. Functional content must work without a canvas.

Retain the animation contract and include a nonempty
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
