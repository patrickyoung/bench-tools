---
name: bitmap-style
description: Use when restyling an existing PNG from a bounded brief; author faithful image editing requests with expressive anime treatments and protected-raster handoffs.
---
# Reference-led art direction

Separate invariants from treatment. The source owns layout, factual content,
subject identity, relative scale, pose, margins, background palette and the
location of every label. The style brief owns rendering choices. Do not
reimagine the infographic as a poster with rearranged elements. Build a prompt
that names both sets explicitly and lets a reference-capable generator see
the actual PNG. Never infer new facts from decorative shapes.

For an expressive anime request, translate the requested mood into varied
contour weight, confident gesture-following linework and readable silhouettes;
cel-shaded light/shadow planes that describe form rather than muddy gradients;
selective hand-painted lighting, textured highlights and restrained atmospheric
detail. Preserve silhouettes and spatial anchors so identities remain legible.
Push expression through contour rhythm and lighting without changing the
meaning, identity, count or placement of subjects. Respect the original
background palette instead of substituting a generic neon scene. This is
aesthetic vocabulary, not a claim of association with any studio.

For other aesthetics, choose equally concrete material, edge, lighting, texture
and color-handling directions appropriate to the caller's style. Do not force
anime vocabulary onto watercolor, engraving, collage or another bounded style.

Protected rectangles are axis-aligned original pixel boxes (x,y,width,height),
not percentages or generator-resolution coordinates. Include their labels for
audit; labels are not new text to draw. Keep the same negative space and adjacent
palette. The compositor restores source pixels after size normalization, so it
cannot repair changed geometry just outside a rectangle. Exact rectangles may
leave visible seams or hard borders. Overlap is harmless; full union protection
is invalid. With zero rectangles, every pixel is editable and no text/data
preservation can be claimed mechanically.

Review rubric for a human viewing source, raw and final together:
- Substantial intended aesthetic change in editable artwork, not only noise.
- Composition, margins, identity, scale, pose and factual reading preserved.
- Background palette continuous; no seams, halos or abrupt texture cutoffs.
- Protected typography/data exactly retained; unprotected facts checked.
- No invented labels, pseudo-data, signatures, logos or endorsement.

A changed-pixel count cannot answer these questions. Notes must distinguish
observations from intentions and explicitly leave this review pending.
