# Choose the tools for your job

Bench tools separate asking a model, doing work, checking results, and keeping
evidence. Start with the part your task needs and add others as the task grows.

This guide covers all **19 components and 23 public commands**. MCP supplies
four commands; A2A supplies two; each other component supplies one. [Get started](GETTING-STARTED.md)
or [install selected tools](INSTALL.md). Examples assume commands are on `PATH`;
model calls also need [Ask setup](../tools/ask/README.md#install).

| Your immediate need | Start with |
| --- | --- |
| Explain, summarize, classify, or draft text | **Ask** |
| Edit files or run programs until a check passes | **Ply** |
| Give a standing job its own instructions, files, and history | **Agent** |
| Work out what to build and how to test it | **Draft** |
| Sort, copy, calculate, or fetch by a fixed rule | An ordinary program; add a model where judgment helps |

Stdout is the usable output stream; stderr carries diagnostics; the exit status
is the numeric outcome. [How it works](HOW-IT-WORKS.md) explains these interfaces.

## Ask, Ply, and Agent: answer, work, or standing job?

**[Ask](../tools/ask/README.md)** takes instructions plus supplied material and
prints the model's answer. Use it to draft a release note from a change list:

```sh
printf '%s\n' 'Added CSV export. Fixed duplicate notifications.' |
  ask -q 'Write a two-sentence release note.' > release-note.md
```

The file contains the answer; errors remain visible in the terminal. Ask
cannot run commands or read a path merely mentioned in your prompt. Give it
contents through stdin or `-a FILE`. Plain `ask` starts fresh; `ask -c` continues
the current conversation; `ask -f review.jsonl` uses only that named session.
Use [the manual](../tools/ask/ask.1) for attachments, JSON schemas, and replay.

**[Ply](../tools/ply/README.md)** adds a loop around Ask: ask for a step, execute
one shell command, return its actual output, then ask again. A final report
goes to your check. From a Go project you intend it to edit:

```sh
ply -sh -f repair.jsonl -turns 12 \
  -check 'go test ./...' 'Fix the failing tests without changing the public API.'
```

The deliverable is the changed project; stdout is the final report. `-sh`
permits ordinary shell execution with your permissions. A toolbox selected by
`-t DIR` narrows program discovery, but does not confine file access.

The check runs before the first model call and after each candidate report.
Exit **0 accepts**, **1 returns feedback**, and another status means the checker broke.
A passing check proves what that check covers: tests must cover the API
constraint too. Without `-check`, Ply's exit 0 only means the model stopped.
See [Ply's manual](../tools/ply/ply.1) for limits, checkpoints, and confinement.

**[Agent](../tools/agent/README.md)** runs that work.
**[Hire](../tools/hire/README.md)** builds its definition. For example, a
weekly report needs stable instructions, inputs, a check, and earlier runs:

```sh
hire new -home report-worker 'Maintain the weekly project report'
agent show report-worker
```

This scaffolds files and prints their composition without calling a model.
Edit `GOAL.md` and `bin/check`, put inputs in `work/`, then use `agent run report-worker`.
For a reusable expert, `hire new EXPERT` creates just its definition.
`agent run -C WORKSPACE -evidence EVIDENCE EXPERT -- GOAL` binds it to a separate
workspace and invokes the same native runner. The definition stays unchanged;
stdout carries the answer and files carry the deliverables.
The [first-worker tutorial](../tools/agent/README.md#build-your-first-worker) includes an exact expected result.
`agent check` validates the home's structure; `bin/check`, run through Ply, judges the task outcome.

Agent keeps definition files, writable `work/` and `state/`, and controller
evidence under `.agent/` separate. Cage confines worker writes and disables
worker networking by default; host reads remain unrestricted. `agent tick`
runs a wake probe once. An external scheduler decides when to invoke it.

## Brief, Rules, Context, and Cite: four different inputs and checks

An instruction tells the model **how to work**. Evidence tells it **what was
observed**. Keep that distinction when constructing a prompt:

| Tool | What you supply | What you receive |
| --- | --- | --- |
| [Brief](../tools/brief/README.md) | A task or named Agent Skill | Matching skill names or the selected procedure |
| [Rules](../tools/rules/README.md) | A repository directory | Applicable `AGENTS.md` and `CLAUDE.md` instructions |
| [Context](../tools/context/README.md) | A named source connector and query | Validated evidence records with source and citation identities |
| [Cite](../tools/cite/README.md) | Evidence file plus a candidate answer | The unchanged answer if its citation identities match |

Use **Brief** for a repeatable method such as writing release notes:

```sh
brief ls
brief find 'release notes'
brief cat release-notes
```

The last command requires a `release-notes` skill; [create one with this tutorial](../tools/brief/README.md#make-your-first-skill).
Finding normally stays offline. `brief find -ask` asks a model to choose
from names and descriptions, without sending skill bodies. Reading a skill
does not execute it; `ply -s release-notes ...` explicitly loads its procedure.

Use **Rules** when instructions depend on where a change is made:

```sh
rules -list .
rules .
```

Run inside a Git repository. The first lists instruction paths; the second
prints complete bodies in root-to-directory order. Rules does not automatically
load them into Ask or Ply, and Markdown instructions grant no permissions.

Use **Context** when the answer needs source material, then **Cite** to catch
invented references. With a `handbook` connector you have installed:

```sh
context query handbook 'What is the leave policy?' > evidence.jsonl &&
ask 'Explain the leave policy from these records. Cite factual claims with
exact [ref](citation.url) links from the supplied records.' \
  < evidence.jsonl > candidate.md &&
cite evidence.jsonl < candidate.md > answer.md
```

Context calls the named connector and prints JSON Lines: one JSON record per line.
The [offline example](../tools/context/README.md#first-make-a-small-evidence-file)
needs no connector or account. Retrieved content belongs in the model's input
as data; keep it out of the system instructions and skill body.

Cite requires at least one exact citation and rejects every malformed or
unknown `ctx:` reference. Rejection leaves stdout empty. It checks the identity
of links, not whether sources support the claims or every claim is cited.
Use `answer.md` only after the final command succeeds. [Cite's guide](../tools/cite/GUIDE.md)
shows how Ply can return a rejected answer to the model for correction.

## Draft and Hone: improve the design and the procedure

**[Draft](../tools/draft/README.md)** helps when you want an LLM to build a tool
but have not yet specified the contract. Begin with a design template:

```sh
draft new notes-tool
```

It creates `notes-tool/DESIGN.md` without a model call. Describe the input,
output, requirements, and executable check. `draft check notes-tool` refuses
the unfinished template; a valid design prints its check. Providing a
description to `draft new` asks a model to draft the design instead.

Review the design before `draft build notes-tool` hands it to Ply.
`draft prove -n 20 notes-tool` tests whether the check notices selected source
mutations. Run it in a quiet checkout: it temporarily changes and restores files.
Neither structural validation nor mutation testing proves every requirement is covered.

**[Hone](../tools/hone/README.md)** helps a repeated task avoid an observed
mistake. It needs a replayable Ply run with a qualifying failure and later
passing verifier result, such as the earlier `repair.jsonl` if that run recovered:

```sh
hone -why repair.jsonl
hone -into go-house -prepare lesson.json repair.jsonl
hone show lesson.json
```

`-why` inspects evidence without a model call. Preparation asks Ask to word a
lesson and writes a proposal, leaving the skill unchanged. After reviewing
the exact text, `hone admit lesson.json` writes it as a Brief skill change.
A clean first attempt, unfinished run, or missing verifier verdict can yield
“nothing to learn.” Hone does not silently learn from every conversation.

## Action, May, Cage, and Trail: permission, limits, and evidence

Suppose a worker should prepare a support ticket. Writing a proposed ticket,
approving it, sending it, and reading the receipt are separate operations:

| Question | Tool and boundary |
| --- | --- |
| May this exact connector request be sent? | **Action** checks operator policy and uses May when review is required |
| Did a person approve these exact bytes? | **May** returns a decision; it does not execute the action |
| Where may a child write, and may it use the network? | **Cage** enforces the selected OS boundary |
| What do retained model sessions contain? | **Trail** searches and inspects Ask archives without changing them |

**[Action](../tools/action/README.md)** accepts a proposal shaped like this:

```json
{"version":1,"connector":"create-ticket","input":{"title":"Fix duplicate notifications"}}
```

Supply an operator-installed connector. `action inspect ticket.json` prints the
canonical proposal without executing it. `action run -job ticket-42 -proposal ticket.json`
applies policy and returns the connector's JSON result if execution completes.
Without `-policy`, it requests May review.
Try the [harmless echo connector](../tools/action/README.md#start-with-a-harmless-connector)
before connecting a service. `-record SESSION` retains receipts in an existing Ask session.

**[May](../tools/may/README.md)** can gate a harmless terminal action directly:

```sh
printf '%s\n' 'Print a hello message' | may && printf 'Hello!\n'
```

It reads the proposal from the pipe and the person's answer from the terminal.
With a job name, May parks the request with exit 75; `may pending` lists it,
and `may decide DIGEST` asks for a decision. An identical request can spend
one matching grant. Keep May and its state outside the worker's writable area.

**[Cage](../tools/cage/README.md)** can wrap your program or an LLM-generated
one. Prove the host boundary with `cage check`, then, for an existing script:

```sh
cage -- python3 ./make-report.py
```

The current directory and process temporary directory are writable, network
access is denied, and the child's streams and status pass through. **Host
reads remain unrestricted.** `-w DIR` replaces the default workspace write
grant; `-net` deliberately permits networking. The [manual](../tools/cage/cage.1)
explains backend requirements. A Ply or Agent verifier runs outside its action
Cage, so executing worker-written code there needs its own suitable boundary.

**[Trail](../tools/trail/README.md)** answers “where did we hit that error?”:

```sh
trail find 'timeout' ~/.ply/sessions
trail show repair.jsonl
ask replay -check repair.jsonl
```

Search and show produce JSONL containing matching text or original events.
`trail check DIRECTORY` delegates replay verification to Ask. Replay checks
retained-record consistency, not factual correctness or business outcomes.
Action or MCP exit **125** means an effect may exist without a trustworthy result;
inspect receipts and the service before considering a retry.

## Tend and Weave: run a job, or decide which job is ready?

**[Tend](../tools/tend/README.md)** retains an exact command, its input, attempts,
and output on a local durable queue. Try it without a model in a fresh queue:

```sh
export TEND_ROOT="$(mktemp -d)/queue"
id=$(printf '%s\n' 'Hello from a durable job' | tend submit -- /bin/cat)
tend work
tend show "$id"
```

`submit` prints the job ID. `work` performs one transition; the child's output
is saved as an attempt artifact, while `show` prints the job record. Your shell
or OS scheduler supplies repetition. Jobs can wait for a signal or timer.
Unknown started attempts require operator inspection; Tend does not promise exactly-once external effects.

**[Weave](../tools/weave/README.md)** helps when work has dependencies: gather
evidence first, then let two independent reviewers examine it:

```sh
weave tasks.jsonl < observations.jsonl > ready.jsonl
```

Tasks name their prerequisites; observations identify each exact task and its
state. Weave prints the unchanged task records whose prerequisites are accepted.
It executes nothing. A caller verifies outcome evidence and can submit ready
work to Tend. Empty output may mean finished, blocked, or already running.
The [quickstart](../tools/weave/examples/quickstart/README.md) includes complete
offline files. The Python research/business drivers are experimental examples.

## MCP and OAuth: connect your programs to services

The **[MCP component](../tools/mcp/README.md)** supplies four public commands:

| Command | Use it when | Input → output |
| --- | --- | --- |
| `mcp` | Call a service through the implemented modern stateless protocol | Method, JSON params, explicit endpoint → exact MCP result JSON |
| `mcp-legacy` | The server uses one of the supported earlier protocol lifecycles | Same request boundary → result through the explicit compatibility client |
| `mcpbox` | Let a shell or agent call reviewed capabilities by program name | Discovery and explicit admission → a folder of executable wrappers |
| `mcpserve` | Let an MCP client use a program you built | Manifest and dispatcher program → an MCP server |

The [local hello example](../tools/mcp/README.md#make-your-first-request) needs
no service account. For a configured remote endpoint, a tool-list request is:

```sh
printf '%s\n' '{}' | mcp request tools/list -- https://YOUR_SERVICE/mcp
```

Replace the URL with a compatible service. There is no hidden protocol fallback.
`mcpbox make` discovers; `mcpbox admit` grants selected wrappers. For external
effects, admit under `actions` to produce Action connectors. A direct `tools`
wrapper permits direct invocation. Discovery and server annotations are not approval.

**[OAuth](../tools/oauth/README.md)** supplies login and refresh separately.
After configuring a resource-bound `docs` profile with the service's client ID:

```sh
oauth with docs -- mcp discover -header-fd 3 -- https://YOUR_SERVICE/mcp
```

OAuth refreshes before starting the child and gives it an Authorization header
on descriptor 3: a dedicated input channel, separate from normal stdin. Child
output and status pass through; OAuth does not retry it. A credential identifies
access to the service; Action and May separately control a proposed operation.

To build your next tool, specify its input, output, errors, and a check first.
Follow [Build with an LLM](BUILD-WITH-AN-LLM.md), try the [recipes](RECIPES.md),
or read the [comparison with current agent tooling](COMPARISONS.md).

## A2A: remote agents through the same process interfaces

**[A2A](../tools/a2a/README.md)** supplies two independent commands: `a2a`
reads one protocol request from stdin and prints a result or JSON event stream;
`a2aserve` exposes one operator-selected command over authenticated HTTPS. The
listener runs it through Tend with a separate workspace per task. Agent still
owns expert execution and Hire still owns expert construction.

```sh
a2a request -cert client.pem -key client.key -ca ca.pem \
  send https://worker.example/rpc < request.json > result.json
```

The [manual](../tools/a2a/README.md) covers request shapes, continuation, file
artifacts, mTLS, and composing the existing OAuth credential tool. Exit 75
means the task is unfinished; 125 means its outcome is uncertain. A listener
is a network service, while the client remains a Unix filter. Neither owns
a model loop, scheduler, remote agent registry, or automatic retry policy.
