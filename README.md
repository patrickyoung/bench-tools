# Bench tools

**Build a digital worker once. Use it on its own or assemble it into a team.**

Describe the result you want, supply the current inputs, and give the worker a
workspace. Its reusable definition teaches the job and checks the result.
Another job uses the same definition with fresh inputs and work.

**Hire builds. Agent runs. Unix connects the commands.** The existing tools
provide model access, correction loops, skills, evidence and durable execution.
Workers cooperate through explicit inputs, checked files, streams and exit
status. The monorepo keeps their source and team rosters together in Git.

## Install with your AI app

Give Claude Code, Codex or Claude Cowork this request:

> Install and set up Bench from https://github.com/patrickyoung/bench-tools.
> Read START-HERE.md and complete setup for this environment.

The app follows its native plugin route, installs the commands where it can
run them, checks the installation and saves a setup record for future tasks.
It preserves existing settings and makes no additional model calls for setup.
Then ask it to find, build or run a worker for your job.

You can also add the GitHub URL directly through your app's plugin installer:

| App | Install Bench |
| --- | --- |
| Claude Code | `claude plugin marketplace add https://github.com/patrickyoung/bench-tools`, then `claude plugin install bench-tools@bench-tools` |
| Codex | `codex plugin marketplace add https://github.com/patrickyoung/bench-tools`, then `codex plugin add bench-tools@bench-tools` |
| Claude Cowork | **Customize → Plugins → Add → Add marketplace → Add from a repository**. Paste the URL, sync, then install **bench-tools**. |

Start a fresh task and ask **“Use Bench to finish setup.”** The plugin supplies
the skill; its first setup installs the separate Unix programs. Setup needs
macOS, Linux or WSL, Python 3.9+ and Git. It uses verified GitHub packages on
supported Intel/AMD and ARM64 hosts; source builds need Go 1.26+. Linux also
needs system Bubblewrap. The app checks its actual execution environment;
a mounted folder alone is not a host shell.
Running workers later needs an [Ask model connection](docs/GETTING-STARTED.md#2-connect-a-model).
[Host details and verification](START-HERE.md#your-harnesss-setup-instructions).

## Start with workers and teams

| You want to… | Start here |
| --- | --- |
| Find, build and run a worker or team | [The practical walkthrough](docs/BUILD-WITH-AN-LLM.md) |
| Ask Claude, Codex or another harness to do it | [START-HERE.md](START-HERE.md) |
| See the available specialties | [Worker catalog](workers/README.md) |
| Select a ready assembly | [Team catalog and recipes](teams/README.md) |
| Add, improve, version or retire reusable source | [Library and GitHub guide](docs/WORKER-LIBRARY.md) |
| Call or expose a worker on another machine | [Optional A2A](tools/a2a/README.md) |

For a harness, start with a request like:

> Read START-HERE.md and use Bench to create and evaluate a worker or team for
> my job: [describe the outcome and where the current inputs live]. Look for
> reusable workers and teams first. Leave a clean definition and a command I
> can run again with new inputs.

The harness is the authoring environment. A worker runs through Agent with
its own configured model connection. The [harness instructions](START-HERE.md)
cover setup for Codex, Claude Code, Cowork, Pi and OpenClaw.

## What is a worker? What is a team?

A **worker** or **expert** is a reusable folder of instructions, skills, tools
and an acceptance check. **Agent** is the program that runs that definition.
A **team** selects workers and supplies their execution wiring. A **run** is
one assignment's current inputs, work, results and evidence.

```text
One worker:   goal + current inputs → Agent + expert folder → checked result
Managed team: goal + current inputs → manager → selected workers → checked result
```

The same worker can run independently or belong to multiple teams. Each team
assignment has its own model context and workspace. In the page team, the
manager proposes work and existing Bench Manage, Tend and Weave admit and
execute it. A roster describes membership; its team wiring determines how
those workers cooperate.

The [current catalog](workers/README.md) includes Frontend, Visual Artist,
Canvas Artist, Blender Artist, Image Editor, Image Concept, Page Planner and
Page Reviewer. Their READMEs explain required inputs and output contracts.
For example, Image Concept prepares a request for a selected generator; Page
Reviewer needs supplied observations. Independent use still needs those inputs.

## Find and export clean source

Start from the public checkout. Source builds require Go 1.26+, Python 3.9+,
Git and a supported Unix environment:

```sh
git clone https://github.com/patrickyoung/bench-tools.git
cd bench-tools
python3 scripts/workers list --all
python3 scripts/workers list --teams --all
```

Listings are local metadata reads and need no model. `--all` includes
experimental and retired entries; the default shows active entries only.
Read the selected definition's requirements before installing its companions.

```sh
git rev-parse HEAD
python3 scripts/workers export frontend /absolute/path/to/frontend \
  --ref FULL_COMMIT --allow-experimental
python3 scripts/workers export-team page-team /absolute/path/to/page-team \
  --ref FULL_COMMIT --allow-experimental
```

Replace `FULL_COMMIT` with a reviewed full commit. Choose fresh destinations
outside the checkout. The experimental option permits deliberate evaluation.
One commit pins the team roster, wiring and every selected worker. Locks record
source identities and exported hashes. No prior jobs, artwork, sites, memories
from runs, or development leftovers enter the exports.

Install selected commands with the [existing installer](docs/INSTALL.md),
[connect Ask to a model](docs/GETTING-STARTED.md#2-connect-a-model), and follow
the exported README to install any definition-specific dependencies. The
[walkthrough](docs/BUILD-WITH-AN-LLM.md) covers standalone use, a managed team,
Hire authoring, evaluation and promotion back to GitHub.

## The command is also the integration point

A worker's command is `agent run` plus its definition and workspace. It reads
stdin, writes its report to stdout and retains the declared deliverables as
files. The page team's command instead returns its accepted HTML on stdout.
Use each command's documented stream and exit contract.

For remote use, the existing `a2aserve` can expose that same command. Standard
stdin/stdout workers need a card and server configuration; file outputs need
explicit artifact declarations. A custom dispatcher is needed only to translate
an application contract. The page team already includes one. `a2a` is the Unix
client; the optional listener is an OS-managed network service with TLS and
authentication. Local use needs neither. See [A2A](tools/a2a/README.md).

## Learn the underlying tools when you need them

| Guide | Purpose |
| --- | --- |
| [First model call](docs/GETTING-STARTED.md) | Ask setup, a small result and basic shell notation |
| [Runnable starters](examples/README.md) | Explicit practice inputs and executable examples |
| [Recipes](docs/RECIPES.md) | Small compositions of existing tools |
| [How it works](docs/HOW-IT-WORKS.md) | Model, loop, check and durable job responsibilities |
| [Tool reference](docs/TOOLS.md) | All 20 components and 24 public commands |
| [Installation](docs/INSTALL.md) | Selected tools, updates and removal |
| [Source and releases](docs/RELEASES.md) | Pinning a reproducible toolset |

An ordinary program may solve a fixed transformation; Ask may suffice for one
answer. Use a worker when the job benefits from reusable expertise and checked
actions, and a team when separate roles and explicit handoffs help.

## Working on this repository

Worker and team authors should start with the [library guide](docs/WORKER-LIBRARY.md).
Keep definitions under `workers/`, rosters and wiring under `teams/`, and job
content outside source. Review changes with GitHub pull requests.

For toolkit development, from the checkout:

```sh
make build                    # all 24 commands in .build/bin
make test                     # ordinary checks, no paid model calls
make check                    # standalone and process integration checks
```

See [development and verification](docs/DEVELOPING.md) for prerequisites and
focused checks. Each `tools/` component keeps its own build, manual, tests,
license and release identity. There is no root Go module or umbrella command.
The [Bench application](https://github.com/patrickyoung/bench) provides its own
interactive workspace and pinned suite. [Source history](docs/DECISION.md) and
[original verification](docs/VERIFICATION.md) describe the monorepo migration.
Root documentation and tooling are [MIT licensed](LICENSE); components retain
their own licenses.
