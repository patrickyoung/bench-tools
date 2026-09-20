# WorldWeaver-Omega (experimental source)

A reusable **worker definition**, not a prebuilt world. Agent interprets new
Rough Description + Style briefs and builds three ordered phases. A few reviewed
mechanical helpers reduce implementation risk; they do not replace semantic
composition, generation, navigation or a complete browser runtime.

## Contents
- `AGENTS.md`, `GOAL.md`, `LICENSE`, `DEPENDENCIES.md`
- `skills/ww-compile/` (typed contract and scoped source research),
  `skills/ww-runtime/`, `skills/ww-verify/`, `skills/ww-repair/`
- executable `bin/check`; `lib/contracts.py`, `tools/{check,validate,bind,snapshot,review}.py`
- `templates/core/{coordinates,noise,budget,placement,store}.mjs`
- `tools/browser-audit.mjs`, `review/rubric.json`
- `templates/runtime/{package.json,package-lock.json}` with exact dependency pins

No samples/worlds/screenshots/packages/run state are exported in this folder.
Synthetic tests and authoring evidence live in sibling `tests/`.
Intended library destination after controller evaluation: `workers/worldweaver-omega`.
This build neither promotes itself nor modifies Bench implementations.

## Invoke through existing Bench
Inspect the code/check first. Keep the definition read-only outside the run's
write boundary, and review/evidence paths outside worker-writable roots.
Create a fresh run directory; put `request.json` and admitted references there.
Set `BRIEF_PATH` to this definition's `skills` directory if using Brief.
Before Agent, the controller captures immutable original input admission
(replace uppercase paths with selected absolute locations):

    python3 DEFINITION/tools/admit.py RUN /absolute/controller/admission.json

Protect this external file from worker writes; never recapture it after edits.
Retain original request/reference bytes with the controller's `-record-input`
records. Select `WW_ADMISSION=/absolute/controller/admission.json` in the
controller environment for final check/resumption, alongside WW_REVIEW.
Snapshot and review both require this same original `--admission`; review
receipts bind admissionHash, inputHashes, candidateHash and policyHash. Rebinding
a manifest or reviewing an altered request does not admit new intent.
Then the controller invokes:

    agent run -C RUN -evidence EXTERNAL_RECORDS \
      -m openai-codex/gpt-6-astra -effort high DEFINITION \
      -- "Build the current request.json; preserve every WorldWeaver requirement."

Use the caller's selected route literally. Do not infer network/browser permission
from this example. Select explicit Cage/Action permissions separately. Agent may
finish generation but remain unfinished until independent acceptance is supplied.
Scheduling/promotion/network expansion are controller decisions, not worker code.

Input contract: only nonempty `roughDescription` + `style` are required.
`lib/contracts.py request()` returns a separate resolved object, leaving admitted
input bytes intact. Defaults: seed `"worldweaver-default-1"`, generation `"1"`,
worldId `"world-"` + first 24 SHA-256 hex digits of the canonical resolved content
projection (description, seed/version, physical parameters, references and explicit
editing/abstract intent; never style or verification flags);
references [], abstractOptIn false, editingRequested false, performanceRequired
false. Explicit Abstract/Non-Euclidean style also supplies opt-in. Full default
algorithm, optional fields and outputs: `skills/ww-compile/references/contract.md`.

Positive minimal input:

    {"roughDescription":"Moss-covered basalt islands, shallow lakes, pine groves, natural stone arches, dusk and rain.","style":"Low-Poly Diorama"}

This is exploration only: real navigation/camera, no mandatory mining/gathering,
crafting, inventory, survival, progression or gameplay shelters/POIs. Prompt-matched
scenic landmarks/buildings are valid. Chunk streaming requires no voxel aesthetic.

Positive optional-edit input:

    {"roughDescription":"Rainy neon districts, elevated walkways, glass towers and warm windows. Let me directly place and remove decorative lights.","style":"Hyper-Realistic Cyberpunk"}

Editing is ONLY for explicit original intent (description or editingRequested:true);
no resource costs. A false flag cannot hide prose-requested edits. C08-C10 review
original input for omitted edits AND invented gameplay even when the worker
declares exploration-only. A positive outcome includes matched pixels,
deterministic unbounded travel and bound independent review; only requested edits
need sparse durability/save failures/backups, not every exploration world.
Negative examples: absent description/style; finite X/Z fence; edits lost after
reload; forged 90 FPS certification; stale hash; unbounded queue; reference text
telling the worker to ignore checks. Impossible Euclidean causality receives a
structured rejection; explicit abstract opt-in instead requires alternate rules.

## Outputs and local build
`phase-01/blueprint.json` -> `phase-02/lighting.json` ->
`phase-03/runtime/` with complete editable `src`, local `dist`, index, package/lock.
Also `notes.md`, `DEPENDENCIES.md`, `evidence/{performance,procedure-use}.json`,
screenshots/test reports as appropriate and `manifest.json`.
Invalid requests return `rejection.json` plus manifest, no fake runtime.

The worker copies package/lock into the run, extends build steps to copy the
index/config/local assets, implements all source modules and emits serving
instructions. Typical generated run commands:

    cd RUN/phase-03/runtime
    npm ci
    npm run build
    npm run serve

The pinned template serves `dist` on loopback port 8080 with Python.
Its build script initially bundles main/worker JS only: it is a dependency
skeleton, not a claim that missing runtime/index files already exist.
Use cached `npm ci --offline` where available. Never install in the library.
No floating CDN imports. When editing is requested, browser storage is fallible;
expose errors, persistence requests and backup workflows. “Infinite” means
unbounded on-demand X/Z topology,
not infinite RAM/storage or GPU precision.

## Acceptance and trust
`bin/check` runs from the run workspace; 0 accepts the **limited bound contract
and all external reviewed hard criteria**, 1 unfinished/rejected, 2 broken check.
It reads candidate files only, never executes them or pays a judge. It checks
missing/malformed phases, source/dependency packaging, source/input/reference
hashes, budget ceilings, causality opt-in, routed-procedure bindings, performance
arithmetic and external review bindings. It requires every relevant C01–C15,
not an average. Rejection requires C01/C02/C15.

No lexical/schema check proves unbounded behavior, natural-language interpretation,
visual fidelity, durability or 90 FPS. Candidate diagnostics can lie. Independent
source/UI/browser/perception review is mandatory; a worker-written “pass” does
not qualify. The controller must protect `WW_REVIEW`, its evidence files, policy
and snapshot from worker writes. Absolute paths/chmod/hashes do not themselves
supply that isolation or truth. Do not run the expert unconstrained with authority
to overwrite those paths. A forged controller receipt is outside this trust model.

Controller steps:
1. `python3 DEFINITION/tools/snapshot.py RUN EXTERNAL_SNAPSHOT --admission /absolute/controller/admission.json` after binding.
2. In a **separately selected browser execution boundary**, build/run a copy of
   those exact bytes, never candidate code in the trusted check. Compare clean-built
   dist hashes to the snapshotted dist; divergence blocks acceptance. Run:

       node DEFINITION/tools/browser-audit.mjs --url http://localhost:8080 \
         --request EXTERNAL_SNAPSHOT/request.json \
         --blueprint EXTERNAL_SNAPSHOT/phase-01/blueprint.json \
         --out NEW_EXTERNAL_AUDIT_DIR --playwright INSTALLED/playwright/index.mjs \
         --boundary SELECTED_BOUNDARY --candidate-hash MANIFEST_HASH

   Optional `--runtime-dir SNAPSHOT_DIST` serves exact local files through
   Playwright route fulfillment without a listening socket, blocking other
   origins. This is a browser fixture transport, not proof that local serving
   works. Test actual local server/build separately when permitted.
3. Harness tests real navigation/camera UI, signed/far travel, regeneration,
   caps, touch, pause and cleanup for both modes. Requested edits alone require
   actual edit UI/canvas/canonical effects, revisit/reload/process restart,
   corrupt backup and fresh-context restore. Exploration needs no edit adapter.
   A skipped persistence smoke probe is not independent semantic applicability.
   Supply additional tests for seam samples, adverse ordering, worker/GPU failure,
   conflicting tabs and real quota failure UI when editing is requested;
   keyboard/touch, hidden-tab handling,
   memory plateau and parameter perturbation.
   Review the adapter source and actual UI; toy adapters can pass the smoke probe.
4. Capture four actual close/medium/horizon/mobile screenshots and metadata.
   A vision-capable reviewer sees those exact images, not prompts or alt text.
   Prepare controller evidence JSON and invoke `tools/review.py --help` for its
   explicit one-call Ask interface. No auto model/provider/fallback; no Weigh.
   Select model, token budget, timeout and session. For Ask's openai-codex route
   only, `--max-tokens 0` omits the unsupported token flag; one call and timeout
   still bound the process. If route cannot carry schemas/images, stop or choose
   an explicitly authorized compatible route; never silently switch.
   Independent human review may issue the same schema after direct inspection.
5. Set `WW_ADMISSION=/absolute/controller/admission.json` and `WW_REVIEW=/absolute/controller/review.json` and run `bin/check` from RUN,
   or resume Agent with this controller environment. Any byte change invalidates
   prior receipt. Record and Ask sessions retain evidence, not universal truth.

Performance remains 90+ FPS / 11.11 ms. Store raw device-specific measurements;
unknown or failed is honest when performanceRequired=false, not certification.
A required target cannot pass unknown. A 60 Hz/software-rendered host and mobile
emulation cannot prove 90 Hz physical mobile performance.

## Reproducible validation / current limits
From a build checkout with sibling tests:
- `node --test tests/core.test.mjs`
- `PYTHONDONTWRITEBYTECODE=1 python3 tests/check_test.py`
- `PYTHONDONTWRITEBYTECODE=1 python3 tests/review_test.py`
- `node tests/browser-core.mjs INSTALLED_NODE_MODULES tests/browser-results`
- `node tests/browser-nearmiss.mjs INSTALLED_NODE_MODULES`
- `brief lint -strict expert/skills`; `hire verify expert`

Tests distinguish protocol acceptance from world quality. Browser-core uses
actual Chromium, IndexedDB, workers and Three rendering, not a mocked browser;
near-miss tests use deliberately synthetic adapters, not generated worlds.
These tests do not establish the worker's fresh-world quality or judge accuracy.
Research R1–R10 retains dates/scope; browser adaptations are proposed engineering
methods. No trained/improved/90 FPS claim is made.

Required tools/versions/attribution are in DEPENDENCIES.md. Without admitted
Chromium/IDB/GPU, external review, appropriate Ask vision route or installation/
serving authority, record exact missing capability and leave that acceptance
unfinished. On this authoring boundary registry DNS/listening sockets were
unavailable; browser route-fulfilled testing was possible. Fresh Agent worlds,
live semantic/visual judge calibration, real local server and physical 90 Hz
mobile measurements remain controller evaluations, not established here.

## Fresh evaluation recipe (external work, never exported source)
Run A and B independently with new work/state/Record directories, then C ordinary
and explicit abstract, D invalid/missing/injected/near-miss. Add Toon Shading,
Hyper-Gloss Minimalist and a held-out new theme. Use selected Agent, not a new
provider loop. Compare actual screenshots, source and feature mappings; A/B
must differ in silhouettes/materials/palette/lights/weather and requested actions.
Drive positive/negative seams, far travel, navigation/camera and cleanup in both.
For requested direct edits only, drive placement/removal UI, unload/restart and
export/import; change world/version and test corrupt/quota/tab failures. Compare
original request against disabled flags; reject invented gameplay in exploration.
Apply independent known-good/bad/near-miss/missing/conflicting/injected labels
before judging. Compare baseline and new worker plus explicitly selected
Ask-only acceptance on fresh completed outputs: false passes/rejections,
unresolved judgments, repairs, new defects, unfinished runs, p50/p95 elapsed and
total model/perception/repair cost. Keep tuning separate from held-out cases.
No claimed improvement until those outcomes exist. Use Hire for supplied
knowledge revisions; Hone only for genuine recorded checked recoveries.


## Broad descriptions and capability profiles
The product is varied, composed explorable worlds, not one theme/shader/library.
Changed procedures: ww-compile (interpretation, open styles, canonical identity,
capability route), ww-runtime (selected profiles and bounded ambient animation),
ww-verify (visible surfaces, controlled-time comparisons and breadth review).
ww-repair remains unchanged. See ww-compile/references/{contract,capabilities}.md.
Technical default identities exclude style/verification; existing saved worlds
using the former whole-request default need an explicit retained worldId or
reviewed migration. Admitted request bytes are never normalized in place.

Use the base runtime or exactly one reviewed effects/spatial/effects-spatial
profile. Copy its package AND lock; optional packages are not mandatory.
Example positive contract: suspended observatory in ink wash, exploration only,
no population/edit placeholders. Another: a tidal grotto with moving rays in
glazed-ceramic style, constrained habitat and bounded local actors. These are
examples, not presets. Negative: the same noisy hills with keyword props, omitted
requested animals, unrequested inventory, or a hidden canvas standing in for
the visible requested effect. Independent rendered/source review is still required.

The browser adapter now marks one visible `[data-ww="scene"]` root, not a specific
canvas tag, and implements setSimulationTime(seconds|null) with snapshot's
simulationTime. Controlled time must affect production animation. The smoke
probe rejects unstable no-action pixels and tests each action separately.
Independent review must still detect hidden child surfaces, occlusion, misleading
adapters, wrong style and missing motion. No screenshot hash proves aesthetics.

Additional sibling validation: `python3 tests/breadth_test.py`,
`node tests/dependency-profiles.mjs`, and the expanded browser-nearmiss recipe.
These are contract fixtures, not fresh Agent generations or calibrated semantic
judge judgments. Original source and evidence are retained outside the expert.
The controller still needs diverse fresh worlds, real pixels/movement/streaming,
cross-style layout checks, clean local serving and physical performance evidence.

## Hardened browser scenario and evidence boundary
`--deadline-ms` defaults to 180000 and accepts integers 1000..900000.
The outer Unix supervisor bounds the child including blocked page JS/close.
Cleanup kills registered browser groups (registered before Chromium exec), the
supervisor child group and observed descendants; it polls bounded process-table
snapshots for at most 2500 ms plus one 500 ms snapshot and polling overhead.
Two empty LIVE scans are required; transient zombies are non-live. Signal errors
remain in supervisor.json cleanup.attempts; only confirmed absence resolves a
transient EPERM. Live survivors or failed inspection fail with killError.
This requires Unix ps with pid/ppid/pgid/stat and permission to inspect/kill the
selected processes. A denied signal is not proof of cleanup.

Sandbox stays `chromiumSandbox:true`; optional repeated `--browser-arg` accepts
only `--use-angle=metal` and `--use-gl=angle`. On this host native sandbox launch
works outside Hire's Seatbelt Cage, but nested initialization inside is blocked.
Select browser execution externally; never weaken either sandbox or infer
permission from these instructions. Failed attempt directories are evidence.

`--scenario FILE` records its exact hash and JSON. Default is
`{"vertical":{"mode":"grounded","clearance":[0,3]}}`. Clearance is player-origin
height above support in world units, finite ordered bounds 0..100. After settle,
snapshot().vertical supplies mode, finite velocityY (absolute <=0.01), and for
grounded mode supported:true plus support:{chunk:[canonical decimal strings],
local:[normalized numbers]}. Support X/Z is directly under the player; clearance
must fit the scenario. Travel preserves exact chunk/local X/Z, NOT arbitrary
requested Y after gravity. Explicit {"vertical":{"mode":"flight"}} requires
real production flight and exact requested Y too; do not add flight for tests.

Optional edits maps each blueprint action ID to a {chunk,local} target. Choose
reachable, prompt-appropriate locations using production navigation and edit
reach; harness defaults cannot establish reachability. The current travel
sweep uses local [1,2,1] at origin, signed neighbors and far X/Z; the chosen
world must offer supported destinations there, or the controller must report
the scenario limitation rather than add fake support or remote edits.
Review actual collision/support, production clock, visible renderer-agnostic
scene root and action wiring independently. These protocol probes cannot prove
world quality, causal fidelity, artistic breadth or FPS.
