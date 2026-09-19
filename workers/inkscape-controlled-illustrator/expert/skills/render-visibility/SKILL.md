---
name: render-visibility
description: Use for every illustration plan to keep declared focal and required objects visible through the final Inkscape paint order.
---

# Render visibility review

Before drawing, identify the one primary visible object and the other visibly
essential objects implied by the request. In `review.targets`, name an actual
visible drawing object for each such item: exactly one `primary`, plus
`required` or `support` targets as appropriate. Target the exposed final
surface, not a temporary construction shape that will intentionally be painted
over. The target list is a review aid, not a substitute for understanding the
brief.

Construct scenes in paint order. A field, window, opening, aperture, frame, or
foreground plane can conceal earlier geometry. When a scene must remain visible
through an opening, build the surround from border pieces or separated planes;
do not place an opaque panel over the scene and assume a named layer makes its
contents visible. Use deliberate overlap only when hiding a form is part of the
picture.

After each Inkscape render, read `output/composition-audit.json`. A primary or
required target reported as fully covered by a later opaque rectangle is a
repair condition: revise the plan's geometry or paint order, rerun the trusted
authoring adapter, and render again. Do not edit generated SVGs or audit files.

The audit catches only one mechanical class of total occlusion. It cannot
recognize the brief, judge partial visibility, inspect text metrics, or assess
composition and polish. Inspect the PNG at full size and thumbnail size when
image viewing is available; otherwise record that this wider visual review is
still outstanding. This is a general construction and review method, never a
stored scene, layout, palette, or object library.
