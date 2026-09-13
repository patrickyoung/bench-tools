---
name: p5-d3-craft
description: Use when implementing or bundling a p5.js or D3 artwork; produce a robust multi-instance visual for the BenchVisual browser contract.
---

# p5 and D3 craft

Choose p5 for immediate-mode drawing, noise, vectors, and simulations; D3 for scales, geometry, hierarchy, force, spatial indexing, data joins, and scoped DOM/SVG work. Combining them must clarify the design, not inflate it.

- Use p5 instance mode only. Scope D3 selection and generated IDs beneath the mount root. Keep all mutable state per mount.
- Expose `globalThis.BenchVisual.mount(root, options = {})` synchronously. Validate the root and treat option/data strings as text. Return update, resize, and idempotent destroy functions; all calls after destruction are no-ops.
- Seed every random/noise source deterministically. Document seed coercion. Rebuild or update predictably when data or seed changes.
- Bundle p5/D3 into `output/visual.js` with the definition-owned esbuild package, resolving imports via `AGENT_HOME/node_modules` or esbuild `nodePaths`. No CDN, dynamic remote import, telemetry, or runtime asset fetch. Keep dependency license notices.
- Scope CSS with a unique component class and avoid selectors for bare elements, `html`, `body`, or host layout.
- Use `ResizeObserver` with a safe fallback. Cap DPR and object counts; clamp elapsed simulation time. Use a quadtree or comparable index when neighborhood work would otherwise become quadratic.
- Suspend RAF/p5 loops when the document is hidden or the root is offscreen. Resume only if motion is allowed. Explicit and media-query reduced motion stop continuous animation and settle to a meaningful composition.
- Tear down observers, media-query listeners, DOM listeners, timers, RAFs, p5 instances, and generated nodes.
- Prefer native button/range/select controls with labels and visible focus. Supply keyboard equivalence and concise status text.
- Preserve the static SVG and textual equivalent when Canvas, WebGL, ResizeObserver, or animation is unavailable.

Test with at least two simultaneous instances, update before and after resize, repeated destroy, a call after destroy, changing reduced-motion preference, hidden/offscreen transitions, mobile and wide sizes, and empty/dense data. Browser tests evaluate behavior; source keyword searches do not.
