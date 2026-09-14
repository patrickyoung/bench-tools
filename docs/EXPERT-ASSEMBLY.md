# Assemble experts with Hire

For the current user workflow, read [Find, build and use workers and teams](BUILD-WITH-AN-LLM.md)
and [the library guide](WORKER-LIBRARY.md). This document records the original
assembly design and example evidence. Current reusable definitions live in
`workers/`; rosters and wiring live in `teams/`. Committed source export can
assemble an unchanged team without invoking Hire again.

Status: implemented and evaluated locally; the reusable example remains experimental.

Hire should build the worker the job requires, including a manager and focused
specialists when their distinct capabilities justify delegation. Agent should
run each expert. The reusable artifact is a directory of instructions, skills,
tool bindings, checks, and interface descriptions. It is not another provider
client or orchestration framework.

The first requested application is a **single-file web page builder**. Its
first live case is an **EPL scouting showcase**. The builder must be reusable
for a different page brief, and additional specialists must be addable without
changing the core runner.

## What Hire should own

Hire turns a job description into a reusable worker definition. Assembly means
selecting existing expertise and capabilities, specifying explicit handoffs,
and leaving inspectable checks and evaluation cases. It should prefer an
existing worker or a single expert when that is enough. A team is justified by
distinct contributions, not by the number of agents available.

The build output must describe both its capabilities and its demonstrated
limits. A well-written persona is not proof of competence. Structural checks
establish a valid definition; task evaluations establish observed behavior.
Failures feed a later Hire revision, while Agent and the existing controller
continue to own execution. This separates improvement of the worker from the
worker doing its assigned job.

Avoid promising a perfect worker. Aim for a worker fitted to explicit acceptance
criteria, with retained evidence and regression cases. New specializations
should normally add files and admitted tool bindings, not another runtime.

## Required result and proof

| Requirement | Evidence required before completion |
| --- | --- |
| Hire assembles a worker from existing capabilities | A real Hire build produces or revises the team definition; generated output passes structural and behavioral checks |
| Independently reusable specialists | Each definition runs through Agent in its own workspace with explicit input, context, and check |
| Manager plans and assigns work | Retained model-produced backlog with task IDs, dependency edges, specialist selections, expected outputs, and limits |
| Checked progress and integration | Actual child/check outcomes release dependencies; deliberate failure blocks acceptance; the integrated page passes the root check |
| Blender specialist | An actual background Blender run retains its scene and rendered output, then the asset appears in the resulting page |
| Image-generation specialist | An actual configured generation call retains the brief, image, and provenance; a missing generator remains an explicit failure |
| GIMP specialist | An actual GIMP operation retains its editable master and exported asset, then the page uses that asset |
| Frontend specialist | One distributable HTML file with embedded CSS, JavaScript, libraries, and assets; responsive, accessible interactions work |
| Visual-animation specialist | A purposeful Three.js or suitable equivalent animation, with reduced-motion handling and a usable rendering fallback |
| CLI and subagent interfaces | The same expert definitions work through ordinary public process calls with explicit streams and outcomes |
| MCP interface | Actual discovery and invocation through MCPserve, including a rejected or malformed request |
| Manager API/A2A interface | Actual client/listener invocation through the existing A2A REST or JSON-RPC interface, artifact export, and appropriate authentication tests |
| Extensibility | Add a further specialist by configuration/definition and execute it without rewriting the runner |
| Real demonstration | Inspect the rendered EPL page and its interactions; distinguish illustrative scouting data from verified real football facts |

## Existing machinery to use first

Agent supplies definitions, Brief context assembly, separate child contexts,
Ply's loop, and Ask evidence. Weave selects ready work; Tend owns attempts.
MCPserve and A2Aserve already provide protocol edges. Hire's embedded
`assembling-experts` skill now describes this assembly procedure.

The adjacent Bench Manage application already implements bounded manager
proposals, admission, Tend execution, Weave readiness, contribution checks,
integration, and a derived backlog view. Its current default worker is
report-only. Any use for creative artifact production needs explicit Agent
worker adapters and demonstrated artifact/check boundaries; its previous
research evaluation is not proof of that extension.

Present already contains working image-generation and graphics guidance.
Preserve useful production capabilities while removing any temptation to copy
its application runtime into the new worker. The existing Bench workers
checkout has unrelated uncommitted work; this task must preserve it.

Reusable example definitions belong with the monorepo examples. Actual
authoring attempts, execution workspaces, and controller evidence stay outside
the source checkout. No public deployment or recurring schedule is implied.

## Execution boundary to settle in implementation

Default Agent actions are confined, and ordinary nested Agent calls cannot
rebind that inherited boundary. A manager writing a backlog must not grant
itself broader network or evidence access. Prefer an existing external
controller that starts each Agent with separate work and control roots.
Do not put execution into a completion checker as a workaround.

Source manifests, expected answers, and check programs do not become trusted
merely by being stored in a different subdirectory. Native confinement and
observed checks must support the authority claims made by the final worker.

## Requirement audit

The [page-team example](../examples/page-team/README.md) and its
[evaluation](../examples/page-team/EVALUATION.md) retain the exact result,
commands, observed failures and limitations. The local execution roots remain
outside the monorepo; the public example contains reusable source, fixtures,
the accepted demonstration, an actual interaction GIF and sanitized results.

| Requirement | Observed result |
| --- | --- |
| Hire builds the assembly | Actual Hire draft and focused revisions, followed by operator repair and successful structural verification; embedded assembly instructions improved from the failures |
| CLI / subagent / separate context | Real small fixture and full EPL continuation through public Agent processes; separate work/evidence; native and child-context integration tests pass |
| Manager backlog and assignment | Model-produced complete dependency graph, checked admission, two failed review attempts replaced with new IDs and their blocked final-copy closure, then a checked finish decision |
| Blender / image generation / GIMP | Actual native scene creation/reopen, original image generation, and five-layer XCF composition/reopen; their checked raster hashes are visibly used in the final HTML |
| Frontend / visual artist | Accepted offline 3.7 MB HTML, purposeful Canvas animation, reduced motion and rendering fallback; scouting controls pass six browser scenarios |
| Checks and final integration | Real low-contrast rejection, review schema failures, browser regressions, source/hash gates and exact reviewed final bytes; no failed contribution was promoted |
| MCP | Discovery, accepted calls, malformed input and changed-brief rejection; final EPL HTML returned exactly |
| API / A2A / authentication | Fresh fixture build over mTLS JSON-RPC, same task through REST, matching text/file artifacts, missing-certificate refusal, wrong-Origin 403 and other-owner 404 |
| Extensibility | A copy-editor definition and binding executed through unchanged core adapters; portable extension recipe included |
| Real demonstration | Accepted page, actual browser-interaction GIF, desktop/mobile visual review and operator inspection of profile/comparison states |

No new provider client, action loop, scheduler, server, shared root runtime or
Go module was added. Hire and Agent remain separate Go commands. The example's
application adapters and deterministic checks compose the installed programs;
Bench Manage remains a separately installed existing application. Present's
image capability was invoked without copying its provider implementation or
changing its unrelated working tree.

The final creative case reused previously checked artwork after harness and
instruction repairs. It was not an unattended first attempt. The final worker
also retains Ply's existing correction cycles; a real public-command offline
fixture proves rejection and repair within one worker attempt. A passing local
showcase establishes observed behavior, not universal quality or a production
Linux/remote-host deployment.
