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

Only `-allow-build` enables model-backed authoring. A build is bounded to eight
turns with a ten-minute timeout per action and retains Agent's default Cage behavior. The form requires an explicit provider/model and passes it through Hire’s
`-m` option. Startup `-model`, defaulting to `ASK_MODEL`, prefills the form.
Credential configuration remains with the installed commands. The interface
offers explicitly enabled individual worker runs through Agent. Teaching,
deployment and policy changes remain outside this interface. Structural
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

`-allow-run` enables explicit runs through the caller-selected `-agent` binary.
Completed local workers and single-worker exports expose Run worker. Teams keep
their own entry commands and are not sent to Agent as individual workers.
Each run uses fresh work/state and sibling evidence, a private goal file, one
optional named text input, a required named text result, an explicit model,
1–100 turns and a ten-minute per-action timeout. Network is off unless selected.
Agent owns Cage, recording, checkpoints and check execution. The form exposes
the guide and check and requires consent to execute both. The check runs outside
the action sandbox. Input names are bounded basenames; artifacts are read under
the data root, bounded to 2 MiB, escaped in HTML, and downloadable as plain text.
Runs share the one-command guard and exact cancellation/outcome contracts.
No model calls or real service requests are made by the offline verification.
