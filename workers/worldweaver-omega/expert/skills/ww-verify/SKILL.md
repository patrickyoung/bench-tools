---
name: ww-verify
description: Verify byte bindings, real browser invariants, rendered style and honest performance; use before submitting any world or rejection for acceptance.
---
Read review/rubric.json in the independent definition. Candidate declarations
are not evidence. bin/check only reads files and external reviewed receipts;
it never executes candidate programs or makes paid calls. Keep definition,
rubric, snapshot, test selection and acceptance evidence controller-owned.
Worker self-tests are useful diagnostics, never independent acceptance.

## Worker sequence
- Validate request and phases. Re-run math/placement/generator tests whenever
  generator parameters change. Compare repeated samples across signed seams,
  reverse visit order, worker timing, seeds and far >2^53 chunk offsets.
- Clean install/build only in run copy under admitted action boundary; serve
  locally with pinned bundles (no network-dependent runtime imports). Test
  transfer detachment, worker crash/retry, epoch+revision rejection, caps and
  actual resource disposal. Source scan alone cannot certify unbounded topology.
- Run browser tests using Playwright only in an explicitly selected browser
  boundary. tools/browser-audit.mjs is a reusable smoke/behavior harness against
  the adapter below, NOT an exhaustive oracle. Review adapter source and drive
  keyboard/touch and UI as well: a forged diagnostics object can fool a probe.
- Capture close, medium, horizon and mobile views at fixed seed, global camera,
  local origin, clock/weather, viewport/DPR; include metadata and exact SHA-256.
  Inspect actual images with admitted vision capability, then recapture changes.
  Test local HDRI attribution and no-network startup. No visual claim from code.
- Measure cold startup, warm up >=10 seconds, move continuously >=30 seconds.
  Save raw rAF frame deltas, long-task entries, renderer.info calls/geometries/
  textures, resident/inflight/completed CPU and GPU estimates, worker/queue/
  upload/particle counts. Include hardware, browser/version, actual backend,
  viewport/DPR, physical refresh rate and software-renderer flag; missing
  properties are unknown, not invented. Label GPU memory as estimates.
  lib/contracts.py computes nearest-rank p50/p95/p99 and rejects impossible
  claims. A 60Hz/software host cannot establish 90 FPS; mobile emulation is not
  physical mobile evidence. Keep performanceRequired=false unknown honest.
- Write notes, dependency attribution, procedure evidence; bind with tools/bind.py.
  Submit for controller snapshot/independent review; do not write WW_REVIEW.
  Missing independent capability remains unfinished, never automatic pass.

## Runtime diagnostic adapter (window.worldweaver)
Expose only in local test mode, wired to production state and actions:
`ready: Promise`; `travel({chunk:[decimalX,decimalY,decimalZ],local:[x,y,z]})`
resolves when destination explorable; `settle()` waits bounded pending work AND
visible rendering or rejects timeout; `baseHash(chunk)` hashes canonical unedited
data independent of LOD/style, excluding animated poses. `snapshot()` returns serializable {player:{chunk,local},
camera:{...orientation...}, simulationTime:seconds} plus `edits:[canonical sparse records]` ONLY when
editing is requested. No inventory/progression or empty gameplay placeholders.
`stats()` returns numerical cap counters queueJobs, queueBytes, residentChunks,
cpuBytes, gpuBytes, drawCalls, workers, uploadsLastFrame, particles, dpr, backend.
`pause(bool)`, `dispose()` operate on the actual runtime.

`setSimulationTime(seconds|null)` freezes the PRODUCTION simulation clock at
a finite nonnegative time; null resumes normal time. It covers actors, weather,
shader uniforms, material/DOM animations and temporal render effects. Freeze
does not disable user controls, worker settling or rendering. Repeated equal-time
renders must be stable; reset temporal accumulators for comparison when needed.
Never use independent wall-clock animation during fixed-clock action checks.
The harness sets time 0 after each load, checks stability, and compares navigation,
camera and each edit separately at that time. Review the clock wiring, not just
its reported scalar. Additional motion review advances controlled time and
checks visible poses, paths/habitats and caps; fixed-time tests do not prove motion.

Mark exactly one actual visible scene root `data-ww="scene"` (canvas, DOM, SVG
or a wrapper containing the visible render surface) and visible production controls
`data-ww="navigate-forward"` and `data-ww="camera-right"` (one click/tap moves
or rotates; continuous keyboard/touch navigation also required). No debug-only
alternate path. The smoke harness drives those UI controls and checks canonical
transforms and visible scene pixels at fixed simulation time, then signed/far travel, reverse regeneration/caps,
mobile touch and cleanup for BOTH exploration and editing.

Only for requested editing expose visible `[data-ww-edit="ACTION_ID"]` controls,
with IDs matching blueprint.interactions.editing.actions. Set up deterministic
reachable test targets via ordinary UI/scene state; ordered actions must each
change actual canonical edits AND rendered scene state, e.g. direct place then
remove, without acquiring/spending resources. `settle()` also waits commit or
reports save failure. `backup()` returns bounded smoke-test backup text via
production export; `restore(text)` invokes validated production import.
Production large backups must stream rather than materialize all history.
The harness checks each edit's far revisit, page reload, persistent-profile
browser PROCESS restart, corrupt import, and fresh-context restore, comparing
edits after travel back (saving camera/location is not required).
Exploration has no required edit/backup/restore APIs or persistence source.

Pass the ORIGINAL `--request` and `--blueprint` paths to browser-audit.mjs.
It enforces editingRequested:true mechanically but cannot semantically interpret
description-only intent. C08-C10 ALWAYS require independent original-request
review, including editing.enabled:false and invented gameplay; skipped smoke
persistence is not a semantic acceptance. Reviewers must reject omitted
requested edits, counter-only debug actions and unrequested edit/gameplay UI.
A test controller may also inspect internal canonical modules directly in its
selected execution boundary for seam samples, adverse ordering and failures.
Do not implement a second fake world just to satisfy this interface.

## Independent evaluation (controller)
1. BEFORE Agent, capture original inputs with
   `tools/admit.py RUN /absolute/controller/admission.json`, protect it outside
   worker writes and retain original bytes in controller input records. Never
   regenerate admission to match worker changes. Protect expert and external evidence; review code, choose sandbox and local
   serving/browser permissions separately. `tools/snapshot.py RUN EXTERNAL_SNAPSHOT --admission /absolute/controller/admission.json`
   copies only bound bytes and marks them read-only. Controller boundary, not
   chmod or Markdown, prevents worker writes. Snapshot build/execution must be
   in another separately confined copy with the snapshot hash retained. Compare rebuilt dist bytes to the snapshot's
   dist hashes; reject source/bundle divergence, never review different builds.
2. Run trusted browser harness and additional rubric tests; capture actual images,
   source review and relevant Agent record extracts with hashes. Store outside
   workspace, include candidateHash, observations, browserBoundary and files
   [{path:absolute,sha256,kind:"image"|"record"|"browser"|"source-review"}].
   Each screenshot observation records camera/seed/weather/viewport/hash and
   target IDs. Four views minimum. Rejection needs no browser/images.
3. Explicitly select a vision-capable Ask model, max tokens and timeout:
   `tools/review.py --snapshot SNAP --admission /absolute/controller/admission.json --evidence EXTERNAL/evidence.json
   --output EXTERNAL/review.json --session EXTERNAL/ask.jsonl
   --model SELECTED --max-tokens 12000 --timeout 600`.
   One call, no fallback, no Weigh. Model selection belongs to caller; if no
   admitted vision/model path exists, leave review unfinished. Large packets
   fail rather than silently dropping source. Human controller may instead
   issue same receipt after direct review, recording reviewer/process/evidence.
4. Supply `WW_ADMISSION=/absolute/controller/admission.json` and `WW_REVIEW=/absolute/controller/review.json` for final bin/check or
   resume Agent under same boundary. Every hard ID must satisfy; insufficient
   evidence never averages away. Candidate edits invalidate receipt automatically.
5. Independently inspect defects outside rubric, revise definition only through
   Hire, then re-evaluate unchanged criteria/held-out cases. No self-authored
   source execution in bin/check.

## Breadth and motion review
Compare varied domains and unlisted styles, not an ASCII/city matrix. Review
large forms, spatial relationships, landmarks, intermediate detail, atmosphere
and generic repetition from close/medium/horizon images. Check every requested
scenery/life feature; use actual movement at two or more controlled times, plus
equal-time revisit samples. Compare the same description/seed across two styles
for canonical layout/identity, and one style across genuinely different domains.
No live style-switching UI is mandatory. Missing population fields cannot waive
requested life. Inspect local spawn IDs, habitat/path constraints, actor caps and
eviction/disposal separately from static scene structure.
Choose fresh controller cases after source edits; examples below are illustrative
historical probes, never priorities or required presets.

## Fresh matrix and improvement claims
See README evaluation recipe. A: basalt/moss/pine/lakes/natural arches/dusk/rain,
exploration only. B: neon districts/elevated walkways/glass/warm windows/rain,
with requested direct decorative placement/removal. Must visibly differ, without
invented resource loops or mandatory gameplay buildings/POIs.
C: Euclidean impossible occupancy/pre-trigger action rejects; separately explicit
abstract accepts with deterministic alternate laws. D: missing inputs, finite
fence, disappearing edits, forged performance, stale hashes, unbounded queues,
injected reference. Add toon/glossy paths and a wholly new theme after authoring.

Known-good/bad/near-miss/conflicting/missing/injected protocol fixtures belong in
sibling tests, never exported expert. Label before judging; keep threshold tuning
separate from held-out cases. Compare fresh completed workers under baseline
and this definition/check and explicitly selected Ask-only route: quality,
successful repairs, introduced defects, unfinished runs, false passes/rejections,
unresolved cases, p50/p95 elapsed and total model/perception/repair cost.
No claim of improved accuracy/cost/training without those live outcomes.

Browser scenario/vertical/support fields, reachable edit locations, native sandbox selection and finite supervisor cleanup are specified in README.md under “Hardened browser scenario and evidence boundary”. Apply that contract; grounded travel must not preserve arbitrary requested Y.

## Targeted rendering and lifecycle probes
Inspect requested inhabitants in actual close/medium rendered pixels under the
requested style and lighting. Compare fixed-camera, equal-time before/after
regions and retain screenshot hashes, target IDs, poses and source/material
evidence. Black or indistinguishable actors despite declared colors require
investigation; dark silhouettes can be intentional when faithful to the request.
Pixel means are diagnostics, not a universal brightness threshold. Check
geometry attributes, material switches, live instance-color uploads and owned
material disposal; empty batches need not have lazily allocated instance colors.

Exercise production persisted pagehide/pageshow in running AND manually paused
states, including repeated cycles: verify quiescence, retained canonical state,
correct conditional recovery, and no duplicate RAF/listeners/workers. Separately
test true teardown. Retain failed-before/passed-after records when repairing.
Injected PageTransitionEvents are synthetic lifecycle regressions, NOT evidence
of native BFcache eligibility or navigation restoration. Label native attempts,
events, persisted flags and unavailable evidence explicitly.

For native visibility tests, record automation focus settings: installed
Playwright 1.63 coreBundle enables Emulation.setFocusEmulationEnabled(true), so
even a headful minimized/backgrounded page can report document.hidden=false.
Within an explicitly selected Chromium browser boundary, the controller may use
the documented CDP Emulation.setFocusEmulationEnabled method with enabled:false
before observing actual visibilitychange events and document visibility state.
Record the setting and observed events; an attempted minimize alone proves
nothing. Never override document.hidden or inject events and call them native.
Do not disable sandbox or retry blocked nested browsers to pass; report the
boundary/capability limitation. Synthetic and native evidence remain separate.
