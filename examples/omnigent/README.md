# Deploy a Bench worker or team with Omnigent

Package a worker/team once, install it on a local or SSH-accessible machine,
and open it in Omnigent. Each job runs in a separate Docker container. Agent
still runs workers; each team keeps its documented entry command. Omnigent is
the chat interface, and the existing `mcpserve` supplies the protocol edge.

For local or remote deployed work, use the recommended
[durable service](SERVICE.md): authenticated SSH/MCP access, private inputs and
credentials, Tend queueing, status/cancellation and explicit recovery.

Omnigent is optional: `./install --without-chat` installs the execution service
without `uv` or the Omnigent environment. [Matterbridge chats](../matterbridge/README.md)
use this path for a persistent builder and a Telegram topic per deployment.

## Prepare and install

From the Bench checkout, select a full reviewed commit and a new directory
outside the checkout:

```sh
python3 scripts/deploy-omnigent worker product-owner "$HOME/deployments/product-owner" \
  --ref "$(git rev-parse HEAD)" --allow-experimental
cd "$HOME/deployments/product-owner"
./install
```

For a team, replace `worker product-owner` with `team vendor-comparison-team`.
The package exports clean definitions and member copies through `scripts/workers`.
It never copies your home, credentials, previous jobs or uncommitted definitions.
The deployment adapter comes from the current checkout; its hashes are recorded
separately from the pinned Bench source and original worker/team lock.

The target needs macOS or Linux, Python 3.9+, Go 1.26+, Git, `uv`, and a working
local Docker daemon. Allow several GB for the image/build and Python environment.
`install` compiles MCP and Tend for the target and builds independent Bench
commands into the job image. It installs **Omnigent 0.14.0** into a private
Python 3.12 environment, freezes the resulting image ID in `installed.json`,
and probes read-only source plus writable job storage. It does not start a
public listener, change shell profiles, or configure model accounts.

Worker and team images contain the execution components Agent, Ask, Brief,
Ply, Record, MCP, Tend and Weave. They omit Hire, Draft and Hone. Only a
deployment explicitly packaged as `builder` (`entry: hire`) adds Hire.
The host prefix contains MCP and Tend. These are ordinary independent
programs, and neither the exporter nor a compiler is needed to execute an
installed job. Source and Go are used during installation/image building;
the final job image contains compiled tools and the exported definition.
For a direct host installation, see [runtime-only setup](../../docs/INSTALL.md#deploy-only-the-runtime).

Builds fetch dependencies. Go modules and worker requirements retain their
source pins; image base tags and Omnigent's transitive dependencies are not a
fully locked supply chain. Keep the installed image ID for repeat runs, or pin
base digests and Python dependencies in a reviewed deployment adaptation.
Keep this directory at its installed absolute path; rerun installation after
moving it. A failed installation can be rerun; prior jobs are retained.

## Synchronous local utilities

The [durable service](SERVICE.md) is the recommended path, including on a local
machine. These original synchronous commands are useful for local diagnostics
and one-off work whose lifetime the caller manages.

Configure Ask's provider/model on the target. For example, with an OpenAI key
already supplied to the shell by your credential manager:

```sh
export ASK_MODEL='openai/YOUR_MODEL'
./chat
```

Omnigent has its own model connection. The chat defaults to its `openai-agents`
harness; `./chat --harness claude-sdk` selects another configured harness.
Its settings and sessions are private to `omnigent-state/`. For interactive
provider setup with that same state:

```sh
OMNIGENT_CONFIG_HOME="$PWD/omnigent-state" \
OMNIGENT_DATA_DIR="$PWD/omnigent-state" ./omnigent/bin/omnigent setup
```

Ask does **not** inherit an Omnigent subscription login. `installed.json` lists
the environment variable **names** forwarded to the job container, initially
`ASK_MODEL`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_BASE_URL` and
`ANTHROPIC_BASE_URL`. Add names required by your chosen Ask provider. No values
are written to the spec, image, source bundle or command arguments. Credentials
selected for a job are available to that job; this is not a secretless proxy.

Place explicitly selected supporting files in `inputs/` before starting a job.
The adapter copies regular files into a fresh job; it refuses symlinks. The
name `inputs/request.txt` is reserved for the invocation request. The
chat has two tools: `run_job(job, request)` and `read_job(job)`. It cannot select
arbitrary host commands. You can also bypass chat and use the same sandbox:

```sh
./run intake-001 < current-goal.txt
```

Use a new job ID for new work. Duplicate IDs are refused, including after a
timeout or interruption. Runs live under `runs/JOB_ID/`: terminal status and
stdout/stderr are outside the container mount; `sandbox/` contains inputs,
workspace, evidence and team outputs. The CLI preserves the child exit status
and stream bytes; chat returns status and bounded output tails. Read complete
files on the execution machine. `finished` only means the process ended;
only exit 0 reports command success. Codes such as 2, 3, 75, 125 and 130 retain
their existing Bench meanings. Do not retry unknown outcomes automatically.

These are synchronous calls (two-hour MCP limit). A disconnected chat may
leave an unknown result. `read_job` includes the deterministic container name.
Inspect retained files and `docker ps` on that machine
before explicitly deciding whether to stop or retry. This adapter does not add
a background job scheduler or automatic continuation.

## Teams and dependencies

| Selection | Request | Entry command | Additional setup |
| --- | --- | --- | --- |
| Any worker | Goal text; selected files are copied into its workspace | `agent run` | Worker README prerequisites |
| `vendor-comparison-team` | Its existing job JSON; material paths relative to `inputs/` | `bin/compare-team` | Analyst requirements and PDF text tools installed by the image |
| `vendor-decision-studio` | Its existing job JSON | `bin/compare-studio` | Publication rendering dependencies from its README |
| `page-team` | Brief text | `bin/page-team` | Separately pinned Bench Manage, browser and chosen native design tools |

The base image includes Python 3.12, Node 22, PDF text extraction and core Bench
commands. It is sufficient infrastructure for standard-library workers and
the comparison team. It does not magically provide every specialty's browser,
rendering tool or external service. Follow `definition/expert/README.md`; extend
the template Dockerfile in your reviewed checkout and package a new deployment
for those requirements. The installer refuses changed packaged adapter hashes.
Do not claim a specialty is
ready until its actual entry command passes its checks in that image. New teams
need an explicit input/entry mapping in the adapter rather than a guessed command.

The source packager also accepts an explicit `--candidate FILE` generated by
the sandboxed Hire builder. Those worker or team snapshots select the ordinary
root Agent entrypoint (`entry: agent`); a team must implement its manager,
specialists and handoffs as a runnable expert assembly. This does not infer
commands for arbitrary team wiring. Candidate bytes, file hashes and modes are
recorded separately from the pinned toolkit source. Installation and a smoke
run remain necessary before connecting a deployment conversation.

## Remote machine

Transfer the **uninstalled** bundle, then install on the target so binaries,
Python paths and the image match that machine. For example, from the parent of
the prepared `product-owner` directory:

```sh
scp -r product-owner user@machine:deployments/
ssh -t user@machine 'cd deployments/product-owner && ./install'
ssh -t user@machine 'cd deployments/product-owner && ./chat'
```

Create `deployments/` on the target first and choose a new destination; never
merge a new bundle into an existing deployment. Configure credentials on the
target rather than copying a live local environment. Use SSH port forwarding
for the loopback web address Omnigent prints, or use its terminal chat over SSH.
Do not expose an unauthenticated server to the network. A public multi-user
Omnigent server is a separate, explicit hosting choice covered by upstream.

Docker must be local to the execution machine. Remote Docker contexts are
refused because bind paths would resolve on a different host. SSH to that host
and run the bundle there instead. Download selected results with `scp` when done.

## Sandbox boundary and Cage

Docker encloses the **whole job**: the worker, team controller, subprocesses and
checks. The container runs as your uid/gid with a read-only image, dropped
capabilities, no privilege escalation, bounded memory/CPU/process count, and a
temporary `/tmp`. Only the current job's `sandbox/` directory is mounted
writable. No host home, source checkout, other run, or Docker socket is mounted.
Model access uses ordinary Docker networking, so egress is allowed. This is a
container boundary, not a claim of VM isolation or an egress allowlist.

The image's small `sandbox-agent` wrapper selects Agent's existing `-no-cage`
option explicitly. There is no sandbox fallback: Docker failure never launches
the job on the host. Native Agent defaults remain untouched outside this image.
Cage is useful as a second layer if you need narrower per-worker write/network
grants inside a shared team sandbox. This starter uses one trust boundary for
the team: members share job files and can access job credentials. Its internal
evidence is therefore not protected from other members by a second kernel
boundary. Host-retained terminal status and streams are outside that mount.

Deleting a deployment is manual after preserving desired results. Containers
use `--rm`; source, images and run files are retained until you remove them.
No paid model evaluation is performed by packaging, install or offline tests.

## Verification and upstream contracts

From the Bench checkout:

```sh
python3 -m unittest discover -s scripts/tests -p 'test_omnigent.py' -v
python3 scripts/check-docs.py
```

To also exercise the generated spec through Omnigent's real MCP connection,
set `BENCH_OMNIGENT_PYTHON` to the absolute Python interpreter in an installed
Omnigent 0.14.0 environment when running that test command. This optional check
still uses synthetic Docker responses and makes no model call.

The offline suite checks packaging, actual MCP discovery/calls with both
lifecycles, duplicate refusal, command/status mapping and sandbox launch flags.
Its Docker stand-in cannot establish kernel isolation. `./install` performs
the real image build and a boundary probe; run a representative worker/team
job separately to evaluate model output and specialty readiness.

Upstream references: [Omnigent](https://github.com/omnigent-ai/omnigent),
[agent specifications](https://github.com/omnigent-ai/omnigent/blob/main/docs/AGENT_YAML_SPEC.md),
and [server deployment](https://github.com/omnigent-ai/omnigent/blob/main/deploy/README.md).
