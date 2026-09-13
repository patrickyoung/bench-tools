---
name: p5-d3-craft
description: Use when implementing or bundling a p5.js or D3 artwork; produce a robust multi-instance visual for the BenchVisual browser contract.
---

# p5 and D3 craft

Choose p5 for immediate-mode drawing, noise, vectors, and simulations; D3 for scales, geometry, hierarchy, force, spatial indexing, data joins, and scoped DOM/SVG work. Combining them must clarify the design, not inflate it.

- Use p5 instance mode only. Scope D3 selections to the mount root. Prefix every DOM/SVG ID per mount and rewrite fragment references and ARIA links too: IDs are document-global even inside a scoped component. Keep all mutable state per mount.
- Expose `globalThis.BenchVisual.mount(root, options = {})` synchronously. Validate the root and treat option/data strings as text. Return update, resize, and idempotent destroy functions; all calls after destruction are no-ops.
- Seed every random/noise source deterministically. Document seed coercion. Rebuild or update predictably when data or seed changes.
- Bundle p5/D3 into `output/visual.js` with the definition-owned esbuild package, resolving imports via `AGENT_HOME/node_modules` or esbuild `nodePaths`. No CDN, dynamic remote import, telemetry, or runtime asset fetch. Keep dependency license notices.
- Use esbuild's JavaScript API `build({nodePaths: [process.env.AGENT_HOME + '/node_modules'], ...})`, or set the CLI's `NODE_PATH` environment variable. There is no `--node-path` CLI flag. Load the build API from `AGENT_HOME/node_modules/esbuild`; use its public API, not package internals.
- Scope CSS with a unique component class and avoid selectors for bare elements, `html`, `body`, or host layout.
- Use `ResizeObserver` with a safe fallback. Cap DPR and object counts; clamp elapsed simulation time. Use a quadtree or comparable index when neighborhood work would otherwise become quadratic.
- Suspend RAF/p5 loops when the document is hidden or the root is offscreen. Resume only if motion is allowed. Explicit and media-query reduced motion stop continuous animation and settle to a meaningful composition.
- Tear down observers, media-query listeners, DOM listeners, timers, RAFs, p5 instances, and generated nodes.
- p5 setup can complete asynchronously. Guard setup, queued observer callbacks, and every awaited continuation against destruction; a mount immediately followed by destroy must not create a late canvas or restart a loop.
- Prefer native button/range/select controls with labels and visible focus. Supply keyboard equivalence and concise status text.
- Preserve the static SVG and textual equivalent when Canvas, WebGL, ResizeObserver, or animation is unavailable.
- Verify library initialization in the absence of media APIs as well as your own sensor functions. A library imported at bundle load can fail before mount exists. Defer optional renderer initialization and catch its failure while preserving the synchronous mount API and static/manual view. Do not fake permission APIs to conceal unavailable hardware.
- Verify the fallback is visibly replaced once the renderer is ready. SVG elements do not share every HTML property: set/remove an actual attribute or use CSS rather than assuming `svg.hidden = true` changes rendering. Test expanded controls and details on a narrow host without relying on the preview page's CSS reset.

Test with at least two simultaneous instances, update before and after resize, repeated destroy, a call after destroy, changing reduced-motion preference, hidden/offscreen transitions, mobile and wide sizes, and empty/dense data. Browser tests evaluate behavior; source keyword searches do not.
