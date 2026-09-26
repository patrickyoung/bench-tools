# Hire UI design

## Purpose and home

Hire UI makes reusable expertise visible and authoring approachable. Its
primary objects are workers, teams and local authoring results. It is a local
application at `interfaces/hire`, with its own Go module and `hire-ui`
executable. This location keeps human interfaces discoverable while preserving
the independent Unix programs under `tools/`.

The visual starting point is the supplied [manager-console reference](https://claude.ai/artifact/Fc7LMQtJmHEF1Toga5dYvm):
a quiet navigation rail, compact catalog, readable details and a focused brief.
The implementation should use typography, spacing and state clarity to make
the work easy to understand. Product copy describes worker capabilities and
user actions; process details belong in expandable records and documentation.

## Small Go application

Use Go 1.26 or newer and standard-library packages. `net/http` owns routing and
request handling, `html/template` escapes rendered values, and `embed` packages
templates and static assets into the executable. Templates are parsed at
startup. There is no JavaScript build step, frontend package installation,
database or mandatory background service.

HTML navigation and form submissions are the baseline. Small JavaScript may
improve filtering or status refresh, but it must not become a second copy of
the application's state machine. CSS handles responsive layout. Forms have
labels, controls have visible focus, status text is understandable without
color, and motion respects user preferences.

These choices use current Go primitives, not a claim that one web framework is
the universal Go standard:

- [HTTP server and method-aware routing](https://pkg.go.dev/net/http)
- [Context-sensitive HTML escaping](https://pkg.go.dev/html/template)
- [Embedded static files](https://pkg.go.dev/embed)
- [Cross-origin request protection](https://pkg.go.dev/net/http#CrossOriginProtection)
- [Subprocess execution](https://pkg.go.dev/os/exec)
- [Accessible forms](https://www.w3.org/WAI/tutorials/forms/)

## Commands are the integration boundary

The caller selects the source checkout, data directory and executable paths
at startup. The application does not discover authority from a sibling path.
It uses literal argument arrays, never a constructed shell command.

| Operation | Public boundary | Meaning |
| --- | --- | --- |
| Worker and team catalog | `python3 SOURCE/scripts/workers list --all`, with `--teams` for teams | Read current catalog metadata |
| Details | Selected catalog metadata and bounded README files | Explain the source and declared roster |
| Worker export | `scripts/workers export ID DEST --ref FULL_COMMIT` | Create a clean pinned worker copy |
| Team export | `scripts/workers export-team ID DEST --ref FULL_COMMIT` | Assemble the pinned roster and source |
| Blank expert | `hire new DEST DESCRIPTION` | Scaffold without a model |
| Model authoring | `hire build -C WORKSPACE -evidence EVIDENCE ...` | Build through Hire and Agent |
| Structural inspection | `hire verify EXPERT` | Inspect definition structure through Agent |

Experimental exports require an explicit user selection. Retired or deprecated
source retains the export tool's refusal. A current catalog can include edits
that are absent from a selected commit; the UI must show which commit it
exports instead of implying that a working-tree preview is the exported source.

Only `-allow-build` enables model-backed authoring. A build is bounded to 50
turns with a ten-minute timeout per action and retains Agent's default Cage behavior. The form requires an explicit provider/model and passes it through Hire’s
`-m` option. Startup `-model`, defaulting to `ASK_MODEL`, prefills the form.
Credential configuration remains with the installed commands. The interface
offers explicitly enabled individual worker runs through Agent. Deployment and policy changes remain outside this interface. Structural
verification never runs generated checks; a worker run delegates them to Agent.

## State and outcomes

The explicitly selected data root is outside the source checkout. Workspaces
hold generated or exported definitions; job records and controller evidence
are kept separately. Files are local private state, not reusable library
content. Logs can contain the submitted brief and model output.

One public command may run at a time. A submitted operation is independent of
the HTTP request that displayed it. Navigating away does not interrupt it.
The interface retains the selected operation, paths, separate stdout and
stderr, completion state and exact process exit status. It must distinguish
success, unfinished work, failure, cancellation and an unknown outcome.

Cancel sends a signal to the selected process; Hire already forwards signals
to Agent. The application must wait for the process outcome rather than mark
a signal request as successful cancellation. Server shutdown signals active
work and waits within its shutdown bound. A retained unfinished job after a
restart is not retried or declared successful. This is a single local command
supervisor, not a scheduling system.

Unfinished authoring (exit 2) may explicitly continue for another 50 turns. The adapter validates the latest workspace activity and reconstructs only its known Hire command. Each invocation gets separate logs and status; the existing workspace, brief, model and public Hire checkpoint persist. Legacy eight-turn builds can continue with current files and a new conversation checkpoint. No arbitrary argv or team staging script is replayed. Team assembly publishes controller-owned continuation arguments only after all staging succeeds. Stale forms cannot branch an older build, including after restart.

## Local worker discovery

Workers combines the selected source catalog with completed local builds and
successfully verified local drafts. Local entries are derived from retained job
records on each request, use stable IDs based on the expert path, and remain
experimental. The latest build/verification for an authored folder determines
eligibility; unsuccessful or unobserved outcomes cannot inherit an older success.
A readable guide must remain under the selected data root. Exported source does
not become another local worker merely because it was verified. Local detail
pages show current files and the dated outcome separately. They do not publish
source, modify the checkout, or claim a live evaluation occurred.

## Local HTTP boundary

Listen only on loopback. Check the request Host and reject unsafe cross-origin
requests. Bound form input and process output. Source text, READMEs and logs
are escaped content. File views resolve only selected catalog entries or local
results; the browser does not receive an arbitrary filesystem file server.
These application checks do not replace Agent/Cage's execution boundary or
turn a generated definition into controller authority.

## Explicit Tailscale access

An optional exact HTTPS `.ts.net` origin plus one selected Tailscale login allows
mobile access through a loopback Tailscale Serve proxy. It does not relax the
listener constraint or admit arbitrary Hosts. Requests to that origin require
an immediate loopback peer and exactly one matching Serve identity header;
identity-free and other-user requests cannot read pages, files or form tokens.
Only the local Serve proxy may supply that identity. Caller-supplied forwarding
headers are not used to derive authority. Unsafe requests also retain the exact
selected origin, standard-library cross-origin checks, and controller form token.
Local access retains its existing semantics. This adds no session service or
provider login, and does not enable public Internet access through Funnel.
App pages use `Referrer-Policy: same-origin`: native Safari forms retain their
same-origin POST origin, while links to other origins receive no referrer.
Opaque (`null`) and mismatched unsafe origins remain rejected; the fix is not an
origin-check bypass. Denials log only a reason category, never request bodies,
identities, paths or credentials.

## Verification

The module's offline gate is ordinary Go tests, race tests and vet. HTTP tests
cover routes, form validation, display escaping and origin/Host protection.
Fake public executables exercise literal arguments, separate streams, exact
status and interruption. Executable integration exercises actual catalog and
pinned exports plus Hire scaffolding and structural inspection in temporary
directories, without live model calls.

Browser checks cover the catalog, details, brief, local results and narrow
layouts. A standalone build proves there is no source dependency on a sibling
module. Neither structural checks nor fake model responses establish generated
worker quality; a selected live job evaluation is separate evidence.

## Worker runs

An optional operator-selected `-web-attach` loopback endpoint is offered as a
per-run browser checkbox, initially unselected. Selecting it grants networking
and adds explicit Web attachment and tab ownership instructions to the private
goal file before Agent starts. The caller owns that browser outside Cage; the
interface neither implements CDP nor starts browsers. A bounded TCP preflight
rejects stale endpoints before model execution, without claiming protocol or
session validity. Web retains its interaction gates. This selection never turns
off Cage or discovers an ambient profile.

`-allow-run` enables explicit runs through the caller-selected `-agent` binary.
Completed local workers and single-worker exports expose Run worker. Teams keep
their own entry commands and are not sent to Agent as individual workers.
Each run uses fresh work/state and sibling evidence, a private goal file, one
optional named text input, a required named text result, an explicit model,
1–100 turns (default 50) and a ten-minute per-action timeout. Network is off unless selected.
Agent owns Cage, recording, checkpoints and check execution. The form exposes
the guide and check and requires consent to execute both. The check runs outside
the action sandbox. Input names are bounded basenames; artifacts are read under
the data root, bounded to 2 MiB, escaped in HTML, and downloadable as plain text.
Runs share the one-command guard and exact cancellation/outcome contracts.
Latest unfinished runs can explicitly continue for 50 more turns using the same
Agent checkpoint, task, workspace and recorded input/output selections. Only the
UI’s known argv shape and original recorded worker path are accepted; network
permission cannot expand. Exit 125 retains its uncertain outcome and requires
explicit user review of effects/diagnostics before a new invocation. Unknown,
active, cancelled and successful runs cannot be resumed this way. Admission
rechecks latest activity and the single-command guard, including after restart.
Original activity logs remain separate from current workspace files.

No model calls or real service requests are made by the offline verification.

## Issue analysis and reviewed revisions

Finished local-worker runs offer explicit evidence selection. The interface
snapshots the current definition, bounded stdout/stderr tails and selected UTF-8
files, then composes public `record run -- ask` for read-only model analysis.
Record snapshots the selected input and sealed child Ask session; a ten-minute
timeout bounds the command. `-ask` defaults to the existing `AGENT_ASK` wrapper
or `ask`; `-record` selects Record. Model work requires `-allow-build`.

The user chooses a correction after reading the analysis. Hire builds a separate
candidate under the existing Cage/turn limits. Exact before/after files and modes
are shown before saving; candidate and original fingerprints must still match
the reviewed proposal and baseline. Saving calls public Hire verify, then makes
a new experimental local revision discoverable. An unreviewed candidate cannot
enter through the generic verification route. The original remains unchanged.

Only saved local definitions are editable here. Symlinks, non-text content and
oversized evidence are refused; runtime state is excluded. Controller snapshots
and review metadata stay outside authoring work. No automatic repair, replay,
Hone admission or acceptance claim follows an analysis. Improve experiments need
separately selected cases and acceptance policy; Agent fresh-case evaluation is
explicit after saving. Offline HTTP tests exercise the review gate; live model
analysis and worker retention evaluations are separate from that gate.

## Team authoring

Team drafts are controller-owned rosters and requirements beside an authoring
workspace. Local member copies are frozen when the draft is saved; source
members and a selected base team use public pinned exports before authoring.
The retained assembly script is a finite composition of literal-quoted commands,
with stop-on-failure and an `exec` handoff to Hire. It contains no user brief
text and does not interpret an arbitrary workflow. Hire authors or adapts team
wiring; Agent, Tend and Weave retain their own execution boundaries. Generated
entry commands and checks never execute during UI authoring or verification.

Catalog entries derive from retained team jobs and roster metadata. A draft is
not a verified worker. Source export locks and selected member copies remain
outside the Hire-writable work root. Review requires the exact roster's unchanged
member definitions, a guide, instructions, executable check and `bin/team`.
Explicit save binds a fingerprint and calls Hire verify; a later edit invalidates
the displayed saved review. A membership edit creates a separate revision and
may reuse the parent's wiring. Source-team customization preserves the selected
base lock but never claims the adapted assembly is an unchanged source export.

The team runtime is intentionally its documented public entry command, not the
individual-worker Run form. No universal team input protocol, scheduler, implicit
permissions or shared model context is introduced by these authoring pages.


## Conversation and reviewed actions

The home page combines a retained conversation with context information and
concrete controls. `bench-hire` is a reusable library worker. The adapter freezes
its definition outside each Agent work root and composes public `agent run` with
input/output recording and default confinement. It adds no provider client,
action loop, scheduler or automatic retry. Assistant turns use the existing
single-command admission gate.

Controller-owned `assistant.json` identifies the conversation and records model,
request/definition digests and an optional local-worker baseline. Its sibling
`snapshot.json` holds exact supplied context. Agent receives `request.json` and
produces `response.json`. Both the worker checker and independent Go decoder
validate the response. The UI trusts its controller snapshot, not a modified work
request. Replies are bounded to 128 KiB and contain plain text, one optional
question, and at most three allowlisted proposals. Missing, duplicate, unknown or
null fields, unsupported actions and unlisted targets are rejected. html/template
escapes model strings; only controller mappings produce routes and argv.

Users may edit proposal fields before choosing an action. Existing handlers own
scaffolding, authoring, team drafts, revision review and evidence selection. An
action cannot add a model, credential, network permission or arbitrary target
path. Task knowledge such as a tracking URL can occur in a goal without executing
a request. Revisions compare the current definition with their frozen baseline
and remain separate candidates until review and save.

A pending receipt precedes dispatch. A known admission result replaces it with
a job link or not-started record. An interrupted pending/unreadable receipt is
an unconfirmed outcome and cannot repeat that proposal. Receipts and deterministic
nonces prevent duplicate clicks. Conversation display derives from retained jobs,
not another process/session store. Restart never silently resumes a turn.

Instrumentation reports selected source revision, enablement, availability and
observed execution state. It does not equate command presence with authentication
or health. Explicit source reads are scoped by instructions, not filesystem
secrecy: Cage limits writes/network while host reads remain unrestricted. Runtime
histories, facts, replies, candidate copies and recordings stay outside source.

## Outcome-first work conversations

The default home is a work conversation: one message, observed progress, returned
artifacts, then natural-language refinement. It does not ask users to choose
models, filenames, input formats, roles, tools, or technical recovery modes.
The former advisory bench-management conversation remains at /assist, with the
existing direct controls under Details.

One message admits a finite plan → prepare → execute → present composition in a
child invocation of this binary. Planning and presentation use the reusable
bench-hire task companion through public Agent. The controller validates target
IDs and bounded materialized inputs; it exports pinned source or copies selected
local expertise, uses public Hire for missing expertise/team adapters, and runs
public Agent or the team's documented entry through its narrow bin/task adapter.
It implements no provider client, delegated action loop, scheduler or automatic
retry. A single invocation is bounded to 45 minutes; authoring and execution are
50-turn public tool calls. Signals, separate logs and exact status propagate.

Input originals stay outside execution write roots; only worker-specific *_INPUT
bindings to those files are admitted. Execution roots, generated definitions and
records remain separate. Generated individual acceptance checks get a Cage write
boundary too. Team adapters execute under Cage with a selected writable run root;
network there supports member model connections while member Agent action policy
stays independent. Actions never gain authority from logs, artifacts or source.
Dependencies, publication, purchases and messages require separately authorized
work; this first work flow does local deliverables rather than deploying them.

Each conversation turn uses a fresh workspace and private frozen companion copy.
Refinement copies earlier bounded deliverables into a new workspace, keeps all
old versions and supplies recent conversation plus the previously prepared expert.
Unobserved outcomes after restart never trigger an automatic retry. A new explicit
message may start fresh local work with an uncertainty warning in its context; it
does not resume or confirm the unobserved process. Prepared definitions are retained
as private catalog entries, deduplicated by their content. Unfinished definitions
return through Hire before execution. Tool failures
remain failures; a presentation reply cannot promote the tool's exit status.
Artifact snapshots are bounded regular flat files, outside model write roots.
The UI serves only indexed artifacts with nosniff/CSP, forces active HTML/SVG to
attachment, and offers inline raster/PDF previews. No arbitrary filesystem route
or model-provided executable argv is exposed by the conversation contract.

## Outcome delivery through Plonk

The task controller owns delivery snapshots and sharing receipts outside worker
write roots. Workers optionally describe logical originals and their preview
files; the controller validates that description against returned artifacts.
The chat shows logical work, keeping technical supporting files out of its cards.
Native originals retain their download indices. A missing or invalid description
uses a safe file-based fallback.

The optional Plonk connection is explicitly selected at startup. A delivery is
one bounded child composition: freeze selected output files, publish a private
snapshot through public `plonk publish`, and only for a user-selected share action
create or update a link. Stop sharing invokes public revocation. This uses the
existing process manager, not a new agent loop or scheduler. Each operation has
a retained request, private receipt, command status and cancellation. Retry uses
the same request/content/base after uncertain outcomes. New shares after
revocation use new capabilities.

Private gallery requests are proxied along one fixed owner/version path, including
all relative previews and downloads. Bearer headers never reach the browser or
model. Active pages open on Plonk's separate sandboxed origin; owner-authorized
private website previews receive a short-lived snapshot capability rather than
creating a public share. Generated content cannot choose publishing credentials,
external argv, unreturned files, or whether sharing happens. Publication and
conversation progress remain visible without requiring technical modes.

## Conversation continuation

The latest stopped task exposes recovery beside its reply. Controller-only
resume records point to the original runtime workspace and retain the validated
plan and previous result. The public command is reconstructed from known Agent
or Hire arguments (50 turns, original goal/input bindings/checkpoint and default
Cage policy), never copied from submitted argv. Presentation and artifact
snapshots go into the new invocation's directory; earlier results stay immutable.
Worker acceptance checks remain confined to the original execution write root.
Admission rechecks that the selected task is latest, has a known resumable exit,
and no other command is active. It revalidates original task/thread membership,
plan shape and required controller files. Duplicate/stale/successful requests
cannot replay work. Team entry contracts and uncertain outcomes use an explicit
fresh attempt instead, retaining prior files and the uncertainty warning.

Regression tests exercise repeated continuation to success from the same fake
checkpoint, unfinished authoring, unchanged earlier outputs, no replanning,
stale/successful rejection, and fresh recovery after an unobserved outcome.
These tests compose public-process fixtures and make no paid model calls.


## Short updates and continuous conversation

The active composer remains enabled for updates to that conversation. Its
message route appends bounded, nonce-identified JSON lines to a controller file
outside all mutable worker roots; it never admits a process. Public Hire and
Agent forward `-steer` to Ply, which owns consumption at its existing boundaries.
The same file spans the finite sequential stages; old notes are retained as
context across continuation and refinement. Existing team adapters have no
universal steering contract, so their execution explicitly marks notes as saved
for follow-up. Saved notes are not model acknowledgements. Stale attempts cannot
accept new notes; duplicate submissions cannot duplicate a message, including
after reload. No scheduler, provider client, automatic replay or new grant is
introduced. Messages after completion stay pending in the conversation until
an explicit next message or continuation.

Controller stage labels are complemented by optional worker hire-status.json
strings: done, now, next, blocked (each <=240 characters). These are bounded,
escaped display data, never completion evidence or authority. Missing, partial,
invalid, symlinked or old-attempt status falls back to observed stage information.
Execution captures the update in its immutable per-attempt result; the status
file is excluded from deliverables. Presentation requests ask for a short summary
of finished work, remaining work and next steps, grounded in execution evidence.
Polling reads files and adds no model calls.

Offline tests cover mid-run steering through a public executable fixture,
duplicate/late/stale notes, retained refinement context, status escaping and
bounds, immutable earlier results, forced termination and early resume failures.
Real source-built Hire→Agent→Ply→Ask integration separately verifies the public
steering seam using an isolated loopback fixture, without paid calls.


## Conversation attachments and specialist transparency

Only work-message routes accept bounded multipart forms. Exact host/identity
and standard cross-origin checks happen before body parsing; CSRF precedes
persistent storage. Those routes receive bounded five-minute read/write
deadlines for mobile transfers. Temporary multipart files are removed. A full
validated upload batch is atomically renamed into a controller-owned
conversation/nonce directory with its size/hash manifest. Retries reuse the
nonce. Original filenames are display data, never storage paths. Downloads
check the selected indexed file's bytes/hash and force attachment disposition.

Planner requests and selected execution goals carry the uploaded metadata and
read-only original paths. Initial execution records those files through public
Agent. Mid-run attachments use the existing append-only steering seam to name
the committed manifest; saving is not proof of model consumption. Refinement,
clarification and continuation retain conversation files. They are not copied
into automatically published deliverables.

Controller selection records drive the specialist card, with the frozen guide's
actual title and declared catalog roster where available. No child-member
activity is inferred from a team assignment. Legacy runs reconstruct selection
from retained validated plans. Explicit adapt targets map to existing catalog
paths plus NeedsBuild: the controller copies the worker and invokes public Hire
before execution, rather than assuming a prepared worker can satisfy every new
request. The current model's selection is still a decision, not a capability
proof; acceptance and remaining requirements remain separately visible.

Web research is an explicit per-message user choice. Only the individual
execution Agent receives -net; its checker retains the established local
boundary. Continues cannot expand the earlier choice. Existing team adapters
retain their independent contracts. No provider client or shared runtime is
introduced by these UI options.


## Library-first task selection

New conversation plans use selection protocol version 1. Before dispatch, the
controller requires a disposition and reason for every base worker and team in
the supplied catalog. Existing tested expertise is the starting point: reuse
unchanged definitions where they fit, adapt specific missing contracts, and
build new expertise only when the recorded review identifies no reusable or
adaptable fit. Distinct complementary specialties belong in an existing or
newly assembled team, rather than a generic replacement worker. The planner
has up to 50 turns for this selection; output presentation retains eight.

A new team must name 2–16 concrete roles, selected worker keys, responsibilities,
and any capability gap. The controller exports selected source members at the
recorded revision or snapshots selected local definitions before Hire starts.
It retains selection.json and selected-members.json outside the authoring root.
Hire may author new members and explicitly selected adaptations; unchanged
members must retain their content and executable modes. Missing, replaced,
extra or changed reused members stop dispatch. Existing assembled teams retain
their member definitions while their UI entry adapter is prepared. Assigned
specialist cards display the concrete selected roster for new teams too.

These gates validate declared selection and preserved source, not the truth of
model reasoning or proof of member execution. Team controllers must invoke
members through the existing public execution commands and retain their actual
run evidence. Offline executable fixtures exercise selection, pinned exports,
member dispatch and rejected replacement without a model. Legacy saved plans
remain readable and resumable under their original protocol; new requests
cannot silently fall back to it. Builds and runs never start from an invalid
selection, and export cancellation/boundary statuses remain unchanged.


When planning adds team members, creates a specialist, or adapts one, the
conversation shows a short staffing explanation before authoring starts. It
names the selected members and distinguishes existing, adapted and new expertise.
The planner supplies a plain-language capability gap; existing-only team
assembly uses its selection explanation. This is a nonblocking update, not an
approval gate. **Add a preference** focuses the ordinary conversation composer
without replacing the user's draft. Optional focus/style/priority notes reach
Hire through the existing steering channel during preparation. Once a team is
executing without steering, the notice says notes are for follow-up. Historical
notices describe the recommendation for that attempt, not work still underway.


## Flash teams and names

When no existing team matches a job with complementary specialties, `new:team`
assembles a temporary **flash team**. A declared reusable team blocks that
fallback. Selected existing members, independent workspaces, handoffs, review
and the normal public team entry command remain required. Temporary membership
does not relax access or claim features its entry command cannot support.

Flash teams live with their conversation and retained work. Their name survives
reuse and follow-ups, and they remain selectable within that conversation even
when another turn used an individual worker. They are excluded from the shared
local worker/team catalog; creating one never promotes it into reusable source.

Select the standalone public name generator with `-moniker /absolute/moniker`
(default `moniker` on PATH). Before assembly the controller calls
`moniker -dir DATA/team-names -json`, validates and saves its reservation, and
introduces the named team with the existing optional staffing guidance. Failed
or invalid naming stops preparation; no model-generated fallback invents a
name. A successful saved reservation is not rerolled. Moniker owns atomic name
uniqueness within this explicitly selected private directory. It uses no model,
network, shared runtime, or hidden state. The same tool can be exposed by the
existing public `mcpserve` executable using its supplied MCP manifest/dispatcher.
