# Hire UI

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
| `-python PATH` | `python3` | Select Python for the source catalog/export utility |
| `-model PROVIDER/MODEL` | `ASK_MODEL` | Prefill the editable model selection |
| `-agent PATH` | `agent` | Select Agent for explicit worker runs |
| `-allow-run` | Off | Enable individual worker execution |
| `-allow-build` | Off | Enable model-backed Hire authoring |

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

## Use the interface

1. Browse or search the catalog, then inspect a worker's description and
   README or a team's declared roster.
2. Export suitable source at a full commit. Experimental source requires its
   explicit export selection. Exported definitions retain their source locks.
3. Use a brief to create a blank expert without a model, or start an enabled
   model build. The latter calls Hire with an eight-turn bound and a ten-minute
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
filename, expected result filename, model and turn limit (1–100, default 20).
Internet access is off until selected; live UPS lookups need it. Account and
service credentials stay configured through the existing tools.

Submitting starts public `agent run` in a fresh workspace with sibling evidence,
a private goal file, selected input/output recording, and a `run` checkpoint.
Agent executes the worker’s check outside its action sandbox; the form makes
that boundary explicit. Only one command runs at a time and nothing retries
automatically. The result page shows the text artifact and a download link,
plus logs and exact status. An unfinished run may have a partial artifact.
This first version accepts one text input (128 KiB) and previews/downloads a
selected text result up to 2 MiB. Teams retain their documented entry commands.
