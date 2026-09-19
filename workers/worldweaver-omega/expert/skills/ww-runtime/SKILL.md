---
name: ww-runtime
description: Implement a modular explorable Three.js world with deterministic bounded streaming and sparse edits; use after blueprint and lighting compilation.
---
Use R5-R10 and the run contract from ww-compile. Keep simulation separate from
camera/LOD/shaders. Read installed package exports and implementations for any
uncertain API. Pinned package/lock templates select Three 0.186.0, esbuild 0.28.2,
Playwright 1.63.0. Read ww-compile/references/capabilities.md and choose base or
one reviewed optional profile; copy its package AND lock. Document capability,
backend, license and measured justification in DEPENDENCIES.md. No additional
unchecked dependencies. Install only in the run copy (`npm ci`, optionally
`--offline`) when admitted.
No CDN imports. Copy required dependency licenses/asset attributions into output.
The templates contain a dependency/build skeleton, NOT a runnable world.

## Vertical slice and invariants
1. Implement coordinates/noise/generation, then a worker + scheduler, then
   meshing/renderer, then real navigation/camera inputs. Test travel, look and
   revisit before adding far-distance style detail. Only for requested editing,
   add direct placement/removal or specified actions and durable restart tests,
   without acquiring/spending resources. Do not stop at that slice: the final must stream.
2. Optional templates/core modules supply BigInt coordinates, 3D Perlin, reservation
   ledger, deterministic AABB placement and sparse IndexedDB operations. Copy
   into run source if used. They do not supply meshing, collision, backup UI,
   scheduling or the complete runtime. Their limits are explicit. BigInt hash
   collisions are not identities: use full canonical strings as persistence keys.
3. Canonical chunk components are signed decimal strings/BigInts; positions are
   chunk+bounded local fraction. Negative floor division, rational-frequency
   neighbour samples and fixed seed stream labels make seams continuous at
   0, -1 and far offsets beyond Number precision. Do not convert a global
   coordinate to Number even temporarily. No modulo-world or finite X/Z fence.
   Sample terrain in full XYZ; heightfields alone must still use continuous 3D
   noise for density/biomes/caves where prompted. Physics uses canonical data,
   never disposable geometry or random instance ordering.
4. Radially enumerate bounded desired XYZ offsets, prioritize near/horizon,
   retain hysteresis ring; cap desired list as well as pending/active/completed
   tasks. Reserve worst-case transferred array bytes BEFORE dispatch. Worker
   reply includes coordinate, seed/version, epoch and edit revision. Transfer
   ArrayBuffers; source is detached, no reuse. Stale jobs remain charged until
   completion/discard; on worker failure release exact reservations, bounded retry.
   Bound completed queue and uploads/frame. Keep CPU+GPU+in-flight accounting,
   worker heaps/geometry/index/material/texture/particle estimates explicitly.
5. Mesh terrain/architecture/props in combined geometry or InstancedMesh. Stable instance-to-ID
   maps survive rebuilding; update flags/bounds after changes. Share materials
   with refcounts, dispose geometry on eviction and shared assets only on final
   release. Adaptive DPR/particles/decoration/view radius use frame hysteresis;
   never change canonical world/optional-edit/collision identity. Stop enqueue/upload on hidden/pause.
6. Choose explicit default WebGL2 WebGLRenderer + compatible composer passes
   for broad support, or WebGPURenderer with awaited init and node materials/
   post-processing. Report actual backend visibly. WebGLRenderer is not WebGPU.
   WebGPU optional fallback must use compatible nodes or reconstruct a separately
   implemented WebGL pipeline. No ShaderMaterial/onBeforeCompile/EffectComposer
   attached to WebGPURenderer. TSL/compute is an optional separately tested tier.
   Handle init failure, context/device loss, recovery and no-GPU UI. Rebuild from
   canonical data after loss; remove RAF/listeners/workers/IDB on teardown.

## Optional requested editing: durable changes
This section applies only when edits are requested. Exploration needs no edit
UI, persistence module or empty gameplay state. Camera/preferences saves are
optional convenience. Do not add mining, crafting, inventory, survival or
progression. Chunk streaming requires neither voxel aesthetics nor shelters/POIs;
scenic architecture is valid when it fits the request.
Use sparse paged deltas, not full terrain. Namespace worldId + generationVersion;
verify seed+semantic generation fingerprint before opening existing history.
Changing material-only presentation need not change geology; define this boundary.
Commit chunk revision and placed/removed/modified edits atomically, plus any
related optional location/preferences that actually change together. Display Saved only on transaction
complete; pending and failures remain visible with retry/export advice.
CAS revisions plus BroadcastChannel refresh handle conflicting tabs; never
overwrite a newer edit with an old worker response. Do not silently fall back to
RAM storage while showing saved. Bound one chunk's loaded edit page and resident
edit cache; millions of historical chunks must never be loaded wholesale.
Core store's page() caps pages, not total chunk history: runtime must consume
bounded pages, apply backpressure and define per-chunk edit-size limits honestly.

Add user-triggered navigator.storage.persist()/estimate() feedback, streaming
NDJSON backup export and staged import into a new generation-specific slot.
Validate each page, format/version/namespace/fingerprint/IDs/checksum/size before
activation. A corrupt/interrupted import must leave prior active save usable.
Do not merge foreign world/version data silently. Backup may need user file
permission; Blob-only fallback must have a documented finite size cap.
Test actual IndexedDB across page close/reopen in a persistent browser profile;
a new ephemeral context intentionally has different storage. Test explicit
backup restore into a new context, multi-tab conflicts and aborted transactions.

## UI and diagnostics
Navigation/camera controls must drive actual runtime transforms from keyboard
and touch. Requested editing uses the same production path from UI and diagnostics,
with applicable reach/support checks and no resource costs or fake counters.
Explain alternate physics. Show seed/version/backend, pause and bounded telemetry;
only requested editing requires save state and backup controls. Verify actual
scene/canonical changes via UI, then revisit, reload and restart.
Responsive touch targets, key remapping instructions, focus behavior, reduced
motion and hidden-tab handling are required. Expose read-only measurements plus
the testing adapter described in ww-verify; no test-only alternative world state.


## Composition, presentation and bounded ambient life
Build spatial rules first, then domain-specific geometry and coherent assets.
Streamed chunks do not prescribe blocks, terrain-first islands or repeated props.
Content IDs, layout and collision are canonical; materials, lights, outline/
postprocessing/DOM effects, camera framing and visual geometry are presentation.
Same description/seed retains layout across styles and verification options.
Style-only geometric simplification must not silently move access or landmarks.

For requested actors, blueprint population groups specify habitat/routes,
representation, motion and active limits. Use visible bounded movement: path
following for vehicles, traversable routes for walkers, confined habitats for
wildlife, or domain-appropriate alternatives. Give bodies readable silhouettes.
Use animation/steering/navigation helpers only where useful. Active local actors,
update frequency, animation mixers, nav tiles, spawn queues and disposal must be
bounded separately from static chunk geometry. Evicted actors retain stable
spawn identity; derive motion from seed + simulation time or explicitly bounded
local state, not wall-clock arrival/order. Never simulate the entire unseen world.
Compare static layout separately from animated poses; compare poses at equal
simulation time, and verify plausible motion at different times. Reduced motion
is a user preference, not an excuse to omit requested life. No LLM-per-actor loop.

## Rendered inhabitants and retained-page lifecycle
For requested inhabitants, match material inputs to each geometry and populated
instance batch. Instance colors multiply the material base and, when enabled,
vertex colors: setColorAt alone cannot compensate for absent geometry color
attributes under vertexColors:true. Inspect the installed renderer path; use
compatible attributes or a separately owned appropriate material when needed.
Mark changed instance colors for upload. Do not mandate a shared palette,
lighting model or material across styles. Dispose separately owned materials
exactly once on final release; never dispose assets still shared by residents.
Verify actual inhabitant pixels at fixed camera/time, not JSON color declarations
or nonempty population counts; preserve equal-time canonical IDs and poses.

Do not bind unconditional dispose to pagehide. On persisted pagehide, suspend/
quiesce retained rendering, enqueue/upload and worker activity without losing
canonical state or the user's manual pause. On persisted pageshow, resume or
reconstruct from retained canonical state only when visibility and manual pause
permit. Keep manual pause distinct from lifecycle suspension. Make repeated
hide/show and recovery idempotent: no duplicate RAF, listeners or workers.
Dispose on true teardown (including non-persisted pagehide), not BFcache retention;
do not rely on teardown delivery to commit requested durable edits.
