# Capability routing, not a fixed stack
Read during intent compilation and runtime selection. Supplied primary-doc/npm
findings were checked 2026-09-19; adoption is not proof of fit. Keep vanilla
Three 0.186.0. Libraries assist composition; none supplies our infinite-world,
identity, bounded population or durability contract.

Start with the brief's relationships and intended views, then record in generated
DEPENDENCIES.md: capability, chosen tool or core technique, rejected alternative,
backend, exact profile/pins, license/local assets, measurable benefit and cost.
Use existing compatible implementations before hand-writing mature helpers.
Do not install everything. Base is a complete permitted dependency choice.

| Need | First route; evaluate additions only when useful |
| --- | --- |
| Geometry/models/composition | Official Three GLTFLoader, animation and geometry utilities. Compose access, scale and silhouette yourself. Inspect selectively admitted Kenney/Quaternius packs for actual contents/license; glTF Transform 4.5.0 is a build-time optimization candidate. Check optimized pixels, decoders and local attribution. |
| Materials/light/atmosphere | Official lights/materials, Sky and backend-matching water. WebGL uses Water; WaterMesh requires WebGPURenderer. Shorelines/habitat remain world logic. Admitted Poly Haven textures/HDRIs need per-asset provenance. tsl-textures 3.0.5 requires evaluated WebGPU/TSL integration; three.quarks 0.17.1 VFX does not establish WebGPU parity. |
| Ambient people/vehicles/wildlife | Official AnimationMixer and coherent geometry first. For requested life, choose bounded path following/steering or animation appropriate to habitat. Candidate Yuka 0.7.8 has an older published release; recast-navigation / @recast-navigation/three 0.43.1 adds navmesh/crowd support when needed. Evaluate tile/worker/origin handoffs. No LLM per actor, full offscreen simulation or invented game loop. |
| Navigation/spatial queries | Official controls/raycasting, simple local collision when adequate. Reviewed optional three-mesh-bvh 0.9.15 accelerates geometry queries, not world generation/physics. Keep BVHs chunk-local and disposed. Rapier compat 0.20.0 is a later-evaluation option for slopes/character collision, not required rigid bodies for all scenery. |
| Style/effects | Materials, lighting, geometry treatment, color and camera may suffice. Official compatible addons or reviewed postprocessing 6.39.5 for WebGL2 effects such as outlines, bloom or pixelation. A watercolor-like brief needs its own visual technique and review, not relabeling bloom. ASCII is just one possible request: official DOM AsciiEffect or postprocessing ASCIIEffect/ASCIITexture; do not merge the latter with convolution effects in one EffectPass. |
| Performance | Core InstancedMesh/BatchedMesh, workers, local chunk bounds, disposal first; measure the bottleneck. BVH only for meaningful queries. @three.ez/instanced-mesh 0.3.16 is a later candidate for culling/LOD/skinning; WebGPU compatibility is not established here. |

R3F 9.7.0 / Drei 10.7.8 is an optional React architecture, not a requirement
or admitted profile here. Do not add React solely for a helper available in vanilla.
No WebGL composer/pass attached to WebGPURenderer. Do not assume WebGPU support
from a package's popularity or an experimental branch.

## Reviewed source profiles
Copy package.json AND package-lock.json from exactly one definition directory:
- `templates/runtime`: base; Three 0.186.0, esbuild 0.28.2, Playwright 1.63.0.
- `templates/profiles/effects`: base + postprocessing 6.39.5 (Zlib), WebGL2 only.
- `templates/profiles/spatial`: base + three-mesh-bvh 0.9.15 (MIT).
- `templates/profiles/effects-spatial`: both, only when both are justified.

Optional templates set package.worldweaverProfile; absence means base. Preserve
exact dependency declarations, complete lock graph/integrities and local bundles.
Scripts can implement the build/serve lifecycle. No unchecked dev/optional/peer
dependencies, overrides or workspaces. Other tools in this reference are
evaluation candidates, NOT installation authority or already admitted profiles.
A new profile requires explicit source review, pinned lock/license review,
backend/build/browser evaluation through the controller and Hire amendment.

Supplied controller evidence for the optional packages: Chromium 153/Metal,
Three 0.186.0, ordinary/BVH hit/miss agreement, effect pixel differences and
glyph-cell response, camera/resize/cleanup without errors (six library checks).
This proves library operation, not world composition, crowd navigation,
streaming, WebGPU effects or mobile 90 FPS. Asset catalogs are not bundled.
