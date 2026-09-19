---
name: ww-compile
description: Compile a fresh world description and style into a typed, traceable, causally consistent blueprint before writing runtime code.
---
Read references/contract.md, references/research.md and references/capabilities.md in this skill. For installed
Brief, use `brief cat ww-compile/references/contract.md` and
`brief cat ww-compile/references/research.md` and
`brief cat ww-compile/references/capabilities.md`; direct paths are the fallback.

1. Validate request with the independent `tools/validate.py --request request.json`.
   Only roughDescription + style are required. Apply the documented deterministic
   technical defaults in memory; never rewrite admitted request.json. Explicit
   abstract/non-Euclidean style supplies opt-in, not a guess from unusual scenery.
2. Default to exploration; no invented resource/survival/progression loop or
   blocky aesthetic. Scenic buildings/landmarks remain valid. Enable direct edits
   only for original requested intent or editingRequested:true. False or absent
   flags cannot erase edits requested in prose; quote their spans. Independent
   C08/C09/C10 review checks applicability even for editing.enabled:false.
   Interpret each requested noun, interaction, mood and law; quote the relevant
   request span in feature records. Separate observed reference traits, artistic
   guesses, defaults, unsupported inputs and contradictions. For visual references
   render/read actual admitted pixels with an available perception tool; filenames
   and alt text are not perception. Record path/hash and uncertainty.
3. Interpret the domain before choosing geometry. Urban briefs need street/block/
   building/access relationships; natural environments need terrain/water/
   vegetation relationships; interiors, orbital habitats and other domains need
   their own coherent organization. Noise is a tool, not a universal landscape
   template. Compose large forms, focal landmarks, intermediate detail, negative
   space, routes/views and atmosphere; generic repetition with keyword-colored
   props is not enough. Record composition and causal spatial relationships.
   The legacy biomes field can describe spatial regions, not mandatory islands.
   Include requested people/vehicles/wildlife with coherent silhouettes, visible
   movement and path/habitat constraints; optional population is not gameplay. Choose units, seed streams, rational frequencies,
   heights, ranges, occupancy and access constraints. Reject mutually exclusive
   simultaneous positions and pre-trigger effects in ordinary physics. In abstract
   mode explain alternative state transitions, event ordering and stable edit IDs.
4. Separate canonical content from presentation. Hash/seed layout, placement and
   stable object IDs independently of style, renderer, verification flags, LOD
   and JSON key order. Same description/seed across styles must retain canonical
   layout even if geometry treatment changes; do not seed from whole blueprint
   or request JSON. Style-only changes need no live style-switching UI.
   Map any nonempty style to materials, lighting, geometry treatment, color,
   camera composition and/or full-scene effects, not just a label:
   - Toon: quantized gradient bands, strong silhouettes, restrained palette,
     optional compatible outlines; rough dielectric base.
   - Low-Poly Diorama: flat normals/faceted meshes, warm key/cool fill, matte
     stone/foliage, silhouette groves, water material distinct from ground.
   - Hyper-Realistic Cyberpunk: layered dense districts, wet roughness variation,
     emissive signage, metallic facades, coherent access routes, neon reflected
     cues and rain; do not call low-detail blocks photorealistic.
   - Hyper-Gloss Minimalist: sparse large precise forms, polished roughness,
     high-value contrast, carefully placed reflected lighting and transmission
     only for plausible translucent solids; preserve readable affordances.
   These are examples, not an enum, prioritized features or content presets.
   In particular cyberpunk styling must not turn an unrelated world into a city.
   Arbitrary unlisted styles need intentional technique choices and actual
   rendered review; ASCII is neither the product nor a required shader.
   Route each capability through capabilities.md, selecting only needed admitted
   tools and compatible backend/profile. Do not conflate animation with layout.
5. Write phase 01, run `tools/validate.py --phase blueprint.json lighting.json`
   after phase 02 exists. Use `templates/core/placement.mjs` for deterministic support/
   overlap corrections when its AABB assumptions fit. Record original and repaired
   constraints; non-resolvable access/art choices return to intent, never silently
   erase a landmark. Numerical validity does not establish composition.
6. Map every feature to an output parameter JSON pointer, implementation path,
   test observation and status. Add units/relationships, inferred defaults and
   reference observations. This prevents dead blueprint fields. Reviewer must
   perturb representative parameters and inspect actual effects.
