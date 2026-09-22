# The Road

A standalone walkable Three.js world created with the experimental WorldWeaver
worker: a rural mile with a great oak, stone bridge, stream, old paving, canopy
passages, a long climb and distant hills. Chunk streaming allows continued
exploration. There is no gathering, mining, inventory or persistence.

## Run

Use Node.js 22 or newer, npm, Python 3 and a WebGL2-capable browser:

```sh
cd examples/the-road/phase-03/runtime
npm ci
npm run build
npm run serve
```

Open http://127.0.0.1:8080/. The built `dist/` directory is standalone; serve it
with any static HTTP server. All runtime assets load locally. Running the world
requires no Bench installation, model credentials or model calls.

WASD/arrows walk; drag to look; Shift walks briskly; Escape/Pause pauses.
Guide provides landmark jumps, key remapping, reduced motion and optional
wind, leaves and stream sound. Sound starts only after you enable it.

## Source and rendering

`phase-01/blueprint.json` contains the authored layout and generation parameters.
`phase-02/lighting.json` contains the lighting profile. `phase-03/runtime/`
contains source, the exact dependency lock, assets and regression checks.

Three.js 0.186.0 provides WebGL2 rendering, HDR/PMREM lighting, scanned PBR
materials, ACES tone mapping, shadows and instanced foliage. esbuild 0.28.2
bundles the runtime. Playwright 1.63.0 is pinned for the original browser
verification; it is not imported by the running world. Third-party license
texts are copied into `dist/licenses/` by the build. Poly Haven images are
CC0; asset credits, source URLs and hashes are in `assets/` within the runtime.
The remaining source uses this example's MIT license.

SSR, SSGI, advanced screen-space AO, WebGPU, GPU compute, bloom, depth of field
and compression are not implemented. Geometry is visibly procedural. Featured
scenery covers one mile with simpler terrain beyond; distant hills form a
painted panorama that follows the viewer, not physical twenty-mile terrain.

## Verify

From `phase-03/runtime/`:

```sh
npm test
node tests/resize-regression.mjs
node tests/panorama-regression.mjs
npm run build
```

These are offline production-module and geometry checks. They do not replace
browser rendering, interaction or audible-output verification. Build products,
installed packages and temporary test bundles are ignored by Git.

Before publication, the delivered source passed native Chromium navigation,
keyboard and touch interaction, audio consent/lifecycle, deterministic particle
motion, chunk travel/resource bounds, graphics recovery, native visibility and
BFcache checks. Ultrawide and portrait resizing and five panorama directions
were exercised. On the tested Apple M4/Metal host, a 30-second moving sample
had p95 approximately 16.7 ms; 90 fps and physical-mobile performance are not
certified. GPU byte totals are estimates.

Independent review satisfied visual/runtime criteria C03–C15, but full brief
acceptance remains open: the description combines westward travel, a north
hedge and the hedge on the left. This preview preserves the emphasized
closed-green-left/open-gold-right view and omits compass labels. That choice
has not received a user clarification. The blueprint's empty conflict list
is not proof that the original directions were consistent.

## Provenance

This is an explicitly published example, separate from the reusable worker
library. Bench Agent produced it using the selected `openai-codex/gpt-6-astra`
model and unchanged experimental WorldWeaver source
`c1df477c82def4e995fe15064419d34496c6a4b5`
([worker PR #22](https://github.com/patrickyoung/bench-tools/pull/22)).
The reviewed source candidate was
`39e43fbdee180739e4f43f28a77087d0a3edef055dde9878d69b3f21b3d15217`.
Runtime source, layout, lighting and lockfile are copied unchanged apart from
removing one trailing space; the built application bundles reproduce exactly. Publication
adds this guide, ignore rules, license and portable asset provenance. Generated
bundles, job requests, model histories and controller evidence stay outside
this example. It runs independently of the worker PR.
