# Bench tools

**Small programs for building tools that use language models.**

Turn support tickets into a digest. Give a model a failing test and let it work
until the test passes. Keep the evidence behind a report. Make a useful one-off
task into a job you can run again.

Bench tools gives you the pieces: a model connection, reusable instructions,
an action loop, checks, records, and durable jobs. Use one program on its own
or connect several through files and ordinary process input/output. You can
write the connecting code in Python, a shell script, or with your coding agent.
There is no required server or shared runtime.

## Start with something useful

[**Get your first result →**](docs/GETTING-STARTED.md) Install Ask and turn
meeting notes into a short update. Then save the method for next time.
The walkthrough explains every bit of shell syntax it uses.

Start from the public source checkout (Go 1.26+, Python 3.9+, and Git required):

```sh
git clone https://github.com/patrickyoung/bench-tools.git
cd bench-tools
python3 scripts/install ask
export PATH="$HOME/.local/bin:$PATH"
```

[Configure a model](docs/GETTING-STARTED.md) before your first Ask request.
Install selected tools or the whole toolkit with the [installation guide](docs/INSTALL.md).

Already have Ask configured? Give it the actual notes:

```sh
ask 'Summarize the decisions and next steps. Flag missing owners.' < notes.txt
```

`< notes.txt` supplies the file's contents. Ask prints an answer; it cannot
open files or run commands from a prompt. Install Ply from the checkout root
with `python3 scripts/install ply`, then use it with a check:

```sh
ply -sh -turns 8 -check 'go test ./...' 'Fix the failing tests.'
```

Run this second example in a Go project you intend the model to edit. `-sh`
allows commands with your user permissions; `-turns 8` bounds model turns.
Ply runs the tests first, works if they fail, and tests a proposed result again.
A passing check proves only what those tests cover.

## Choose by the job

| I want to… | Start with | Add when needed |
| --- | --- | --- |
| Explain, summarize, or transform supplied material | [Ask](tools/ask/README.md) | [Brief](tools/brief/README.md) for a reusable procedure |
| Let a model edit files or use programs until a check passes | [Ply](tools/ply/README.md) | [Cage](tools/cage/README.md) for write/network limits |
| Produce a report with traceable sources | [Context](tools/context/README.md) + Ask | [Cite](tools/cite/README.md) to check citation identities |
| Give a repeatable job a directory of instructions and checks | [Agent](tools/agent/README.md) | [Tend](tools/tend/README.md) for durable queued attempts |
| Control a proposed external change | [Action](tools/action/README.md) | [May](tools/may/README.md) for a recorded human decision |
| Build a tool with help from an LLM | [The builder guide](docs/BUILD-WITH-AN-LLM.md) | [Draft](tools/draft/README.md) for a structured design workflow |

[The full tool guide](docs/TOOLS.md) covers all **17 components and 20 public
commands**, including Rules, Hone, Trail, MCP, OAuth, and Weave. Install only
the components you need; their runtime companions are selected separately.

## Learn enough to build

| Guide | What you will get from it |
| --- | --- |
| [Getting started](docs/GETTING-STARTED.md) | A first result, provider setup, and basic terminal notation |
| [Recipes](docs/RECIPES.md) | Small, concrete workflows with inputs and checks |
| [Runnable starters](examples/README.md) | Copy a complete meeting brief, evidence answer, or durable job and run it |
| [How it works](docs/HOW-IT-WORKS.md) | What a model, loop, verifier, and durable job each contribute |
| [Choose your tools](docs/TOOLS.md) | Each tool's role, boundaries, and detailed reference |
| [Build with an LLM](docs/BUILD-WITH-AN-LLM.md) | A prompt and workflow for making your own useful tool |
| [Compare with modern agent tooling](docs/COMPARISONS.md) | Where coding agents, SDKs, workflows, MCP, and Bench fit in 2026 |
| [Installation](docs/INSTALL.md) | Whole toolkit, selected tools, updates, and removal |
| [Source and releases](docs/RELEASES.md) | Which installation to use and how to pin a reproducible toolset |

You do not need all the pieces for every task. A fixed transformation may need
no model; a summary may need only Ask. Add a loop when the next step depends on
what happened, and a durable queue when the work must survive the caller exiting.

## Working on this repository

Build prerequisites are Go 1.26+, Python 3.9+, Git, and a Unix shell; checks
have [additional prerequisites](docs/DEVELOPING.md#choose-the-right-check).
From this checkout:

```sh
make build                    # all 20 commands in .build/bin
make test                     # ordinary checks, no paid model calls
make check                    # full standalone and process integration checks
```

Make is optional: `python3 scripts/build` and `python3 scripts/check --quick`
provide the first two operations. See [development and verification](docs/DEVELOPING.md)
for prerequisites, isolated checks, individual builds, and source exports.

Each directory under `tools/` keeps its own commands, module or script, manual,
tests, license, and release identity. There is no root Go module or umbrella
`bench-tools` command. The [Bench application](https://github.com/patrickyoung/bench)
provides an interactive workspace and maintains its own pinned suite releases;
use this repository to build and compose the standalone tools.
The [migration decision](docs/DECISION.md) and [original verification](docs/VERIFICATION.md)
record the source history. Root tooling and documentation are [MIT licensed](LICENSE);
each component retains its own license.
