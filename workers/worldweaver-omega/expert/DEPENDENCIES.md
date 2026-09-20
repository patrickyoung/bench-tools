# Selected dependencies and attribution

Reusable instructions/tools: MIT, see LICENSE. No third-party implementation
source or installed packages are vendored here. Runtime template lock v3 pins:
- Three.js 0.186.0 — MIT, Three.js authors (mrdoob and contributors).
  https://github.com/mrdoob/three.js
- esbuild 0.28.2 — MIT, Evan Wallace and contributors.
  https://github.com/evanw/esbuild
- Playwright 1.63.0 — Apache-2.0, Microsoft Corporation.
  https://github.com/microsoft/playwright

Preserve actual installed LICENSE/NOTICE files in generated distribution
attribution, including esbuild platform packages and playwright-core. Chromium
has its own bundled third-party licensing; Playwright license is not a relicensing
of the browser. No HDRI/reference asset is included or implicitly licensed.
Locally supplied assets require provenance, license and redistribution decision.

Public host tools: Bench Hire/Agent/Brief/Ask/Record (existing controller boundary),
Python >=3.10 standard library, Node >=22/npm, and an explicitly admitted
Playwright Chromium installation for browser tests. Review requires an explicitly
selected vision-capable Ask route supporting schema and attachments, or direct
independent human perception/review. No provider SDK, Weigh, scheduler or custom
agent runner is added. Inspect installed APIs for backend-specific implementation.


Optional reviewed profiles (never mandatory): `effects`, `spatial`,
`effects-spatial`, under templates/profiles. These add postprocessing 6.39.5
(Zlib, pmndrs, https://github.com/pmndrs/postprocessing) and/or three-mesh-bvh
0.9.15 (MIT, Garrett Johnson/contributors,
https://github.com/gkjohnson/three-mesh-bvh). Preserve their actual license files.
Their exact npm resolved/integrity records come from the controller's separately
tested library lock; base graph is unchanged. postprocessing is WebGL2-only in
these profiles. The routed ww-compile/references/capabilities.md explains choices,
backend limits and which additional libraries still require evaluation/admission.
