# Hire UI

A conversation-led workbench for creating, teaching and improving Bench workers.
Talk to the reusable `bench-hire` worker while the UI keeps selected context,
evidence and concrete controls visible. Catalogs and direct forms remain
accessible. One independent Go executable serves embedded HTML and assets.

For Web-based workers on hosts where Chrome cannot launch inside Cage, the
operator can start a dedicated browser outside Cage and select its loopback
HTTP or WebSocket endpoint with `-web-attach ENDPOINT`. The run form then offers
**Use connected browser**, unchecked by default. Selecting it explicitly enables
networking and passes the endpoint and tab-ownership instructions in the retained
Agent goal. The browser's session is available to that run. The interface does
not launch, discover, authenticate or close browsers, or grant Web click approvals.
A closed endpoint is rejected before creating a job; a TCP connection alone does
not prove a compatible or authenticated browser. Browser lifetime remains the
operator's responsibility; update the configured endpoint after replacing it.

A local browser interface for finding workers and teams, exporting reusable
source and building expert folders with Hire. A small Go executable serves the
application and its assets; there is no frontend install or build step.

## Build and start

Build with Go 1.26 or newer:

```sh
cd interfaces/hire
make build
./bin/hire-ui version
./bin/hire-ui \
  -source /absolute/path/to/bench-tools \
  -data "$HOME/.local/share/hire-ui"
```

Open [Hire UI](http://127.0.0.1:8787). The source must be the explicitly
selected Bench checkout. The data directory must be outside that checkout;
it holds local workspaces and job records. The server accepts loopback
addresses only. Stop it with Ctrl-C.

| Option | Default | Purpose |
| --- | --- | --- |
| `-source PATH` | Required | Select the Bench source checkout |
| `-data PATH` | Required | Select private local application state outside source |
| `-addr ADDRESS` | `127.0.0.1:8787` | Choose a loopback HTTP listener |
| `-hire PATH` | `hire` | Select the public Hire executable |
| `-moniker PATH` | `moniker` | Select the public name generator for flash teams |
| `-python PATH` | `python3` | Select Python for the source catalog/export utility |
| `-model PROVIDER/MODEL` | `ASK_MODEL` | Prefill the editable model selection |
| `-ask PATH` | `AGENT_ASK` or `ask` | Select public Ask or the configured wrapper for analysis |
| `-record PATH` | `record` | Record selected analysis evidence and model session |
| `-web-attach ENDPOINT` | Unset | Offer an explicitly selected connected browser |
| `-agent PATH` | `agent` | Select Agent for worker and conversation runs |
| `-assistant PATH` | Source’s `workers/bench-hire/expert` | Select the advisory worker definition |
| `-allow-run` | Off | Enable individual worker execution |
| `-allow-build` | Off | Enable conversation, analysis and model-backed authoring |

Startup needs the source checkout, Python 3.9+, Git and an installed Hire
executable. Scaffolding and verification need Hire's documented dependencies.
Model authoring additionally needs a selected model, an existing Ask account
connection and Agent/Cage setup. The form defaults to `-model` (or `ASK_MODEL`)
and passes its selected value to Hire as literal `-m` arguments. A missing or
malformed selection keeps the brief in the form without starting a command.
Selecting a model does not authenticate: credentials and any `AGENT_ASK`
wrapper remain configured through the installed tools, outside the UI. See [Hire's manual](../../tools/hire/README.md)
and [Bench setup](../../START-HERE.md) for those independent programs.

For `openai-codex`, Ask requires an authorization header on an inherited
descriptor. A configured OAuth profile can supply it to an Ask wrapper via
`oauth with PROFILE -- /absolute/path/to/ask -header-fd 3 "$@"`.
Select that wrapper with `AGENT_ASK` when starting the interface. Create and
validate the profile using [OAuth's setup instructions](../../tools/oauth/README.md)
first; selecting a model does not create that profile.
Never paste tokens into the model field. A missing-header failure is shown as
“Connect your model account,” with the original exit status and logs retained.

To enable model-backed authoring explicitly:

```sh
./bin/hire-ui \
  -source /absolute/path/to/bench-tools \
  -data "$HOME/.local/share/hire-ui" \
  -allow-build
```

## Access from your phone with Tailscale

The listener stays on loopback. To use a local Tailscale Serve HTTPS proxy,
select its exact origin and the one Tailscale login allowed to operate Hire:

```sh
hire-ui -source /absolute/bench-tools -data /private/hire-ui \
  -tailscale-origin https://your-machine.your-tailnet.ts.net:10443 \
  -tailscale-user you@example.com

tailscale serve --bg --https=10443 http://127.0.0.1:8787
```

Retain your existing model, worker, and delivery options. Open that HTTPS address
on a device signed into the selected Tailscale account. This grants that account
access to the same conversations, outputs and explicitly enabled worker actions;
it is not a separate read-only view. No additional login page is needed.

Hire accepts the selected remote Host only from a loopback proxy carrying the
exact `Tailscale-User-Login`. Tailscale Serve strips client-supplied identity
headers and supplies its authenticated identity. Missing or different identities
are rejected before any page or file is served. The backend must stay loopback
only and must not be reached through a proxy that passes caller-provided identity
headers unchanged. Ordinary local access remains available. Host, exact unsafe
request origin, browser cross-origin checks and form tokens stay enforced.

Use Serve, not Funnel: this interface is intended for the selected user's private
Tailscale access. The Mac and Tailscale must be running. See [Tailscale's identity
header contract](https://tailscale.com/docs/features/tailscale-serve).

## Talk to Hire

Choose **Working on** to attach a worker or run, describe your desired outcome,
and send. Worker and run pages offer contextual conversation links. Prior text
is retained within that conversation. The sidebar shows catalog counts, recent
conversations and observed command availability. Presence does not prove
credentials or health; Hire never discovers credentials or browser accounts.

Agent runs a frozen copy of `bench-hire` for each message with the selected model
connection, Cage and Record: six turns, two minutes per action, network off.
Inputs include the catalog, explicit focus, diagnostics and at most 192 KiB of
conversation text. Local workers include a frozen definition baseline; runs
include log tails and a file inventory, not implicit artifact contents. The
explicitly selected checkout supplies current Bench reference material. Cage
does not isolate host reads; worker instructions prohibit searching homes or
credentials. Twenty turns and 2 MiB per request bound context growth.

A reply asks one question or offers up to three editable proposals:

- **Build this worker** calls Hire authoring when you select the button.
- **Save team draft** saves members and handoffs; building stays separate.
- **Prepare this revision** authors a separate local-worker candidate for review.
- **Choose evidence** opens the selected run’s investigation controls.
- **Open** navigates to a known catalog item or activity record.

The interface independently checks exact JSON shape, bounds, duplicate/null
fields, input-byte binding and allowed targets before displaying or applying
proposals. It checks the current worker baseline again before revision. A receipt
links each applied card to its activity across restarts; an interrupted dispatch
with an unconfirmed receipt cannot repeat that action. Failed replies retain the
exact outcome and evidence without automatic retry. Format checks do not prove
diagnosis accuracy or output quality.

Forms work without JavaScript. Live status uses the existing activity endpoint
and reports observed state, not generated reasoning or estimated progress. Logs,
files and exact exit status are available from each turn’s evidence link. Run the
[worker contract cases](../../workers/bench-hire/tests/test_check.py) separately
from paid model evaluations; the library evaluation catalog includes fresh
conversation-quality cases.

## Use the interface

1. Browse or search the catalog, then inspect a worker's description and
   README or a team's declared roster.
2. Export suitable source at a full commit. Experimental source requires its
   explicit export selection. Exported definitions retain their source locks.
3. Use a brief to create a blank expert without a model, or start an enabled
   model build. The latter calls Hire with a 50-turn bound and a ten-minute
   timeout per action, and keeps its default Cage behavior.
4. Inspect the local result and the command's stdout, stderr and exit status.
   Structural verification calls `hire verify`; it does not run generated
   checks or prove the worker performs its job well.

Completed builds and successfully verified local drafts appear automatically in
Workers as **local, experimental** entries, alongside the source catalog. Their
names are searchable, each links to its current guide and latest successful
activity, and repeated verification does not create duplicates. Retained job
records restore these entries after restart; no separate registration is needed.
A more recent unfinished, failed or running build/verification hides the local
entry until it succeeds again. Missing or unreadable guides are also omitted.
A direct command outside the UI needs a successful **Verify structure** action
in the UI to register its outcome. Local entries are not published or exported
as pinned source, and verification does not establish live job quality.

The source catalog describes the selected checkout's current files. Export reads the
selected committed revision, which can differ from uncommitted catalog edits.
The interface uses the repository's export command rather than copying a live
worker folder into a supposedly pinned result.
The catalog and commit are selected at startup; restart to refresh them.

Unfinished worker builds, revisions and staged team builds offer **Continue for 50 more turns** on their activity page. Each click uses the same model, original brief and current draft, with a separate activity record and another 50-turn allowance. New builds use Hire’s `build` checkpoint to retain conversation context. Older builds without a checkpoint keep their files but start fresh context on their first continuation. Continue again from the latest unfinished activity as needed; nothing continues automatically. Completed, cancelled, failed and unknown outcomes are not eligible. Team continuation never repeats source exports or staging.

Only one command runs at a time. Page navigation does not cancel it; use the
job's cancel action. Hire forwards interruption to Agent, and the interface
records the process's actual outcome. Hire's exit 2 means unfinished work and
130 means interruption. No failed, interrupted or unknown operation is retried
automatically.

One server owns a data directory at a time. Logs retain up to 8 MiB per stream,
with a 32 KiB preview and explicit truncation notices. After three seconds of
unresponsive cancellation the process group is killed and the outcome is
marked unknown. A server crash also leaves unobserved commands unknown; inspect
their evidence before running anything again.

Workspaces, controller evidence and retained job output live under the
selected data root, outside library source. Treat that directory as private:
briefs and logs may contain job information. Source selection does not grant a
worker permission to modify the library. Teaching and deployment remain separate
operations through the existing Bench commands.

## Develop and verify

```sh
make test
make check
```

`make check` runs ordinary tests, race tests and vet without a live model. Build
the directory on its own with `GOWORK=off` to verify source independence.
Executable integration should exercise real public tools in isolated
temporary directories; paid model evaluation is a separate selected operation.

Run the explicit, offline public-command integration from the repository root:

```sh
python3 scripts/build hire agent brief ask ply cage record
BENCH_UI_INTEGRATION_SOURCE="$PWD" \
BENCH_UI_INTEGRATION_BIN="$PWD/.build/bin" \
  make -C interfaces/hire integration
```

This checks actual catalog reads, pinned worker and team exports, blank
scaffolding, and structural verification. It uses a fresh home and an
allowlisted environment without credentials. It proves generated checks are
not executed during verification. Model authoring is tested with a fake public
executable in the ordinary tests; these checks make no paid model calls.

`make build VERSION=0.1.0` sets the executable's reported version.
`make clean` removes this application's build output.

See [DESIGN.md](DESIGN.md) for the HTTP, subprocess, state and accessibility
decisions. This is a standalone local application, not part of the installed
headless Hire command or a multi-user hosted service.

## Run a worker

Start the interface with `-allow-run` and, if needed, `-agent /absolute/path/to/agent`.
Use **Run worker** on a local worker’s page or a successful individual worker
export. Review its guide and check, enter the task, optional text input and its
filename, expected result filename, model and turn limit (1–100, default 50).
Internet access is off until selected; live UPS lookups need it. Account and
service credentials stay configured through the existing tools.

Submitting starts public `agent run` in a fresh workspace with sibling evidence,
a private goal file, selected input/output recording, and a `run` checkpoint.
Agent executes the worker’s check outside its action sandbox; the form makes
that boundary explicit. Only one command runs at a time and nothing retries
automatically. The result page shows the text artifact and a download link,
plus logs and exact status. An unfinished run may have a partial artifact.
Unfinished runs offer **Continue run for 50 more turns**. Agent resumes the same
workspace, original task, current inputs/output and named conversation checkpoint,
with unchanged network permission. Each invocation keeps its own logs and exit
status. Use the latest activity to continue again; duplicate clicks and stale
pages cannot start another continuation. Runs ending with exit 125 require an
explicit review of existing effects and diagnostics before continuation. An
unknown outcome after a crash or forced stop is not resumable from this control.
Missing input or a broken check needs correction, not just more turns; use
**Talk through this** or **Analyze issue** to work through those blockers.
The worker folder remains the originally selected path; this does not freeze
its instructions or turn an incomplete recording into a completed one.

This first version accepts one text input (128 KiB) and previews/downloads a
selected text result up to 2 MiB. Teams retain their documented entry commands.

## Investigate and correct a worker

Open a finished local-worker run and choose **Analyze issue**. Select the relevant
text evidence and a model connection. The definition, log tails and selected files
are sent to Ask; Record retains the snapshot and analysis session. Analysis is
a diagnosis to review, not a successful repair.

Choose **Review analysis**, describe the correction, then **Prepare fix**. Hire
authors a separate candidate. **Review proposal** shows each changed file before
and after, including any checker changes. After review, **Verify and save revision**
uses Hire's structural inspection and adds a separate experimental local worker.
The original remains available. From the saved result, use **Run worker** with a
fresh case; structural verification does not establish job quality. Stale or
unreviewed proposals cannot be saved. There are no automatic retries.

Analysis uses `-allow-build`, `-ask` (the existing `AGENT_ASK` wrapper by default),
and `-record`; credentials remain with these commands. Definitions must contain
bounded regular UTF-8 files, at most 128 files / 8 MiB. Evidence selection lists
at most 64 files of at most 256 KiB, excluding hidden directories and runtime state;
at most 2 MiB may be selected. Larger/raw binary artifacts remain on disk.

## Create and adapt teams

Open **Teams → Create team**. Name the outcome, choose at least two roles and
workers, describe each deliverable, the handoffs, and acceptance criteria.
**Add another role** and **Remove role** work without JavaScript (up to 16 roles). Saving a draft
calls `hire new` without a model. It copies selected local worker definitions;
library selections are pinned to the displayed full source revision.

Choose **Build team** with your existing model connection. A retained, fixed
command script exports selected source workers through `scripts/workers export`
and, when applicable, the starting team through `export-team`. Source locks stay
beside the draft. The script stages the selected definitions, then replaces
itself with public Hire for bounded authoring. Every argument is quoted as data;
the brief is a separate file. Export failure stops before the model call and its
exact exit status is retained. This is command composition, not a team runtime.

Inspect the saved roster, selected member guides, handoffs, generated guide,
entry command and check, then **Verify and save team**. Hire checks structure
without executing generated checks. Member definitions must still match their
saved copies; added/removed/modified members and stale reviews block saving.
The assembled definition must contain an executable `bin/team` with documented
usage. Follow that guide to install prerequisites and evaluate fresh work.
The UI does not execute arbitrary team entry commands or treat a team as an
individual worker.

**Customize team** starts from an existing catalog team's roster and wiring.
**Edit membership** on a local team creates a new draft/revision, preserving the
original. A failed build retains its evidence; edit it into a new draft before
another build. Drafts and verified experimental teams appear in Teams and survive
restarts. Local teams are not automatically published to the source library.

Review currently supports bounded regular UTF-8 definitions (128 files, 8 MiB
per reviewed assembly; 1 MiB per file). Symlinks and runtime metadata are refused.
Larger or binary assemblies need the existing command-line workflow. Offline
HTTP tests exercise validation, pinning, review and recovery. Public-command
integration uses real exports and Hire scaffolding/verification with an explicitly
fake model author, and does not establish real team output quality.

## Ask for an outcome

The home page is now **Your work**. Describe what you want, answer any essential
question, review the returned files and previews, and request changes in the
same conversation. Hire selects a specialist, prepares its inputs, creates missing
expertise (or a team adapter) with Hire, and runs it through the existing Bench
commands. Users do not need to name input/result files, choose a model each turn,
or switch between authoring and execution screens. The configured model and
both -allow-build and -allow-run enable this flow.

The work companion is `workers/bench-hire/expert/task`; the original advisory
profile and its learned tool knowledge remain available under **Manage your
bench**. Work conversations retain earlier output versions. Each send is a
bounded plan/prepare/execute/present composition, not an unlimited background
agent. Missing facts are asked in conversation. Missing access or dependencies
remain honest blockers; the local work flow does not silently deploy, publish,
send messages, purchase, or install software. Technical activity records remain
available under each reply's Details control.

Prepared expertise is retained in the private library for reuse. Refinement keeps
previous versions intact; unfinished preparation returns through Hire before the
specialist can run. A missing-information question does not discard earlier work.

The initial artifact viewer supports text, raster images and PDFs for preview, plus
safe downloads of documents, editable SVG, HTML, text, tables and archives.
Results are limited to 32 flat files, 25 MiB each, 100 MiB total. Inputs are
materialized by the planner from the supplied conversation (12 files / 1 MiB),
not facts invented to satisfy a check. File uploads and executing arbitrary
third-party content are not part of this version. Individual task actions currently
run without network access; online research remains outside this initial path.

## Finished work and sharing

Select `-plonk /absolute/plonk -plonk-url https://work.example.com
-plonk-token-file /private/plonk.token` to enable delivery controls in work
conversations. All three are operator settings; users need only **Open results**,
**Create a share link**, **Share this update**, and **Stop sharing**. The token file
must be private. Publication uses the public Plonk executable, with literal argv,
a fixed output snapshot, exact command status, bounded time and cancellation.
It shares the existing one-command admission gate. An interrupted step stays
unfinished; a user-requested retry reuses the original idempotent request.

Opening results prepares a private remote snapshot; it does not create a share.
The controller authenticates every private gallery, image, PDF and download
request. Credentials never enter browser pages or the worker's execution goal.
Shared links advance only on the explicit share-update action. Revoking a link
also revokes its historical file URLs; downloaded copies cannot be recalled.

Workers may produce `delivery.json` alongside flat native outputs. It provides
title/summary and logical artifacts with `id`, `title`, `description`, `original`,
and optional `preview`, `thumbnail`, and `pages`. Every referenced filename must
already be in the controller's bounded output snapshot. Invalid metadata falls
back to the native returned files rather than selecting outside content. The
interface handles the manifest; people never write one themselves. Native files
remain downloadable when a browser preview is unavailable. Static pages use
relative flat CSS/JS/assets; Plonk supplies a separate sandboxed content origin.

Plonk remains independently buildable and deployable. See its `DELIVERY.md` for
the HTTP/CLI/MCP contracts and content-host deployment. Localhost preview links
are only local demonstrations, not links a remote recipient can open.

The explicit integration check uses a locally built Plonk binary and temporary
server data, never personal credentials or a paid model:

```sh
BENCH_UI_PLONK_BIN=/absolute/plonk GOWORK=off go test \
  -run TestDeliveryPublicExecutableIntegration -v .
```

### Continuing stopped work

The latest stopped turn offers a next step in the conversation. **Continue
working** resumes confirmed unfinished individual-worker execution or authoring
in the original workspace with the original goal, inputs and public Agent/Hire
checkpoint. Each click admits another bounded 50-turn invocation and has its own
activity, presentation and immutable file snapshot. Old download/share versions
remain unchanged. A later continuation can continue again; a stale button cannot
branch an older attempt or replay one that already finished.

When the outcome was not observed, evidence is incomplete, the saved plan is
unavailable, or a team needs its own entry contract, **Try again with saved work**
starts a fresh attempt using the conversation and saved files. It never claims
that an uncertain command succeeded or silently replays it. A normal message
remains available to change direction or supply missing facts. No automatic
retries, model calls on page load, permission expansion or paid live tests occur.
Dependencies must be present and selected in the service environment; giving a
run more turns cannot repair a missing renderer or missing source material.


## Keep the conversation going

The conversation remains open during and after work. While a worker runs,
**Send update** saves a short message for the next step using public Agent/Hire
`-steer` and Ply's existing steering boundary. It cannot interrupt an action
already running or expand permissions. Notes remain visible and become context
for later refinements, even if they arrive after the process finishes. Saving is
not a delivery acknowledgement. Existing team entry commands without steering
keep notes for follow-up and say so explicitly.

Live **Done / Now / Next** notes show the current plan and optional worker updates.
A stopped result also shows remaining work and **Continue working** when its
checkpoint is resumable. Each continuation gives another 50-turn invocation,
keeps its own logs/results and retains the conversation. A normal follow-up
message uses the saved files for a fresh refinement. Earlier versions remain
available.

Conversation steering needs Agent and Hire versions supporting `-steer`.
The UI passes one controller-owned append-only file outside action workspaces.
Messages are bounded and nonce-deduplicated. A late note never starts work
silently; sending the next message or clicking Continue is explicit.


## References and assigned specialists

Use **Attach files** in the conversation to send manuals, photographs, PDFs,
Word/PowerPoint/Excel files, or text. Up to five files per message, 25 MiB per
file and 50 MiB total; a conversation retains up to 40 files / 200 MiB. Originals
remain private, outside worker write roots, and available in later refinements
and checkpoint continuations. Active updates supply a committed file manifest
through the existing steering channel. Files are not automatically published
with deliverables. Reading a format still depends on available worker tools.

An assigned-specialist card shows the actual definition name, worker/team,
selection provenance, purpose, available references and web access. Known team
rosters describe assigned members, not inferred per-member execution. Expand
**About this specialist** for the selection explanation and activity link.
The planner has explicit **adapt:** catalog targets to copy an existing worker
and adapt it through Hire before execution; source definitions remain unchanged.

**Allow web research for this message** explicitly enables network access for
individual-worker execution. It is off by default and visible in the worker
card; the next message prefills the previous choice. Continuation preserves the
existing choice. Guidance cannot expand access during a running invocation.
Teams retain their own member access contracts. Model access remains handled by
the configured public tool connection.

Presentation summaries use cleaned copies of process output, keeping raw logs
and exact statuses unchanged. The companion's execution-object contract is
preserved. Known worker-reported blockers remain visible even if a mechanical
check passed or the summary stage fails.


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
