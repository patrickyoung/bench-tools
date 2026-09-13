# Bench tools

**Give a digital worker a job, a workspace, and a way to check its work.**

Turn a customer's question into a reply grounded in your policy. Give a model
a failing test and let it repair the code. Save the method in an expert folder
and use it again tomorrow, with different inputs.

Bench tools is a collection of focused Unix programs. **Hire builds an expert
folder; Agent runs it.** Ask connects to the model, Ply drives the action/check
loop, and the other commands supply skills, evidence, permissions, records,
and durable execution. They cooperate through files, streams, and exit status.
The shell and operating system still arrange the work.

Think of a worker with an **ID, a badge, and a computer**. Its Unix account is
the ID. Credentials and operator-selected permissions determine its access.
Its workspace is the desk where it leaves the result. Markdown teaches the
job; it cannot issue the badge. A Linux box, a suitable model, and deliberately
chosen access are enough to start. macOS is supported too.

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

## Make the job reusable

You should not need to write another agent runtime for each specialty. Give
Hire a job description, inspect the folder it builds, then give Agent that
folder and a workspace:

```sh
python3 scripts/install hire agent ask brief ply cage
mkdir authoring customer-work
hire build -C authoring -evidence authoring-records -- \
  'Build a support expert that reads question.txt and policy.txt, writes
reply.md, and flags anything the policy does not answer. Include a check
and examples of a good reply and a reply that should be rejected.'
```

The artifact is `authoring/expert`. Review its instructions and generated
check, then put your `question.txt` and `policy.txt` in `customer-work/`:

```sh
agent show -C customer-work authoring/expert
agent run -C customer-work -evidence customer-records authoring/expert -- \
  'Draft the reply from the supplied policy.' > run-summary.txt
cat customer-work/reply.md
```

These commands use your configured model. Hire checks the generated folder's
structure; you still need to evaluate whether its check tests the right thing.
Agent uses Cage to limit worker writes and deny worker networking by default;
host reads remain available. Use a fresh workspace for each new case so an old
accepted result cannot satisfy the new job's pre-check.

Want a complete example before asking a model to build one? The
[support-reply starter](examples/support-reply/README.md) includes an expert,
sample policy, question, and a citation check. Copy it and run Agent directly.

## Choose by the job

| I want to… | Start with | Add when needed |
| --- | --- | --- |
| Explain, summarize, or transform supplied material | [Ask](tools/ask/README.md) | [Brief](tools/brief/README.md) for a reusable procedure |
| Let a model edit files or use programs until a check passes | [Ply](tools/ply/README.md) | [Cage](tools/cage/README.md) for write/network limits |
| Produce a report with traceable sources | [Context](tools/context/README.md) + Ask | [Cite](tools/cite/README.md) to check citation identities |
| Build and run a reusable expert definition | [Hire](tools/hire/README.md) builds; [Agent](tools/agent/README.md) runs | [Tend](tools/tend/README.md) for durable queued attempts |
| Call or expose an agent on another machine | [A2A](tools/a2a/README.md) | Agent runs local work; Tend supervises its attempt |
| Control a proposed external change | [Action](tools/action/README.md) | [May](tools/may/README.md) for a recorded human decision |
| Build a tool with help from an LLM | [The builder guide](docs/BUILD-WITH-AN-LLM.md) | [Draft](tools/draft/README.md) for a structured design workflow |

[The full tool guide](docs/TOOLS.md) covers all **19 components and 23 public
commands**, including Rules, Hone, Trail, MCP, OAuth, and Weave. Install only
the components you need; their runtime companions are selected separately.

## Learn enough to build

| Guide | What you will get from it |
| --- | --- |
| [Getting started](docs/GETTING-STARTED.md) | A first result, provider setup, and basic terminal notation |
| [Recipes](docs/RECIPES.md) | Small, concrete workflows with inputs and checks |
| [Runnable starters](examples/README.md) | Run an expert folder, a small text filter, or an ordinary durable job |
| [How it works](docs/HOW-IT-WORKS.md) | What a model, loop, verifier, and durable job each contribute |
| [Choose your tools](docs/TOOLS.md) | Each tool's role, boundaries, and detailed reference |
| [Build with an LLM](docs/BUILD-WITH-AN-LLM.md) | Build an expert with Hire; write code only for the parts that need it |
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
make build                    # all 23 commands in .build/bin
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
