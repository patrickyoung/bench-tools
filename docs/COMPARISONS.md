# Where Bench fits in 2026

**Choose by the workflow you want to own.** Use an integrated assistant when
you want help doing work now. Use an agent SDK when you are building an agent
into an application. Use Bench tools when you want reusable programs whose
inputs, checks, permissions, and results you can inspect separately.

Official product documentation checked **2026-09-09**. The recommendations
below are engineering judgments based on those sources and this checkout's
documented contracts; they are not performance benchmarks.

| Your immediate need | A useful starting point | What you take responsibility for |
| --- | --- | --- |
| “Help me fix this bug and review the patch.” | Codex or Claude Code | Task instructions, permissions, and review of the result |
| “Add an agent to our customer-facing application.” | OpenAI Agents SDK or LangGraph | Application code, storage, deployment, and tool behavior |
| “Turn this recurring task into a command I can rerun.” | Ask, Ply, then Agent as needed | The command's inputs, execution boundary, and success check |
| “Keep business work progressing across failures and long waits.” | A durable workflow runtime; Tend for a local process queue | Recovery rules, operating the workers, and handling duplicate effects |

These choices can be combined. A coding assistant can build a Bench command.
An application can invoke it as a child process. An MCP server can expose it
to other assistants. The interface between the pieces still needs a test.

## Integrated coding assistants: get work done in a session

Codex CLI can inspect and edit files, run commands, review changes, use skills
and MCP connections, and run in scripts or CI. Claude Code also combines file
and command tools with an interactive coding workflow; its extensions include
skills, subagents, hooks, and MCP.
Sources: [Codex CLI](https://learn.chatgpt.com/docs/codex/cli),
[Claude Code overview](https://code.claude.com/docs/en/overview),
[Claude Code extensions](https://code.claude.com/docs/en/features-overview).

For “fix the checkout page,” an integrated assistant is a natural first choice:
the conversation, code navigation, edits, and review are already connected.
You can also automate these products; command-line composition is not unique
to Bench.

Bench exposes smaller units. [Ask](../tools/ask/README.md) handles a model
request and its conversation record. It cannot run a command or read a file
just because the prompt names it. [Ply](../tools/ply/README.md) adds the loop:
ask the model, execute a command, return its output, and check the candidate.
[Agent](../tools/agent/README.md) packages recurring work into a directory
with a goal, procedure, workspace, and executable success check.

Choose this separation when the result should become something another
program can use repeatedly: a release-note generator, a log triager, or a
worker that repairs a known class of failures. The cost is assembly. You must
choose the companion programs and understand their outputs and exit statuses;
this repository does not provide one shared runtime or conversation UI.

## SDKs and graph runtimes: put the workflow in your application

The OpenAI Agents SDK supplies an agent loop, tools, handoffs between
specialists, sessions, tracing, guardrails, and resumable approval flows in
Python or TypeScript. Your application supplies deployment, tool
implementations, storage choices, and approval decisions.
Source: [OpenAI Agents SDK](https://developers.openai.com/api/docs/guides/agents).

LangGraph lets application code combine predetermined steps with model-driven
steps in a graph. Its checkpointers save a thread's graph state, while stores
hold information across threads. This supports workflows that pause for human
input and continue later.
Sources: [LangGraph overview](https://docs.langchain.com/oss/python/langgraph/overview),
[persistence](https://docs.langchain.com/oss/python/langgraph/persistence).

Use these when an agent is part of a larger software product: a support
application with customer sessions, specialist routing, and existing Python or
TypeScript services. Keeping orchestration in that application's language can
make data structures and integration easier to manage.

Bench places composition at the process boundary. Your controller can be a
short shell script, Python program, Go service, or another agent. It starts
executables and exchanges data through their documented streams and files.
This allows reuse across languages, at the cost of writing adapters and
handling each program's contract instead of relying on one framework's types.

[Weave](../tools/weave/README.md) illustrates the distinction. It reads a
finite dependency graph and supplied observations, then prints ready tasks.
It neither runs those tasks nor verifies the underlying business evidence.
A controller verifies outcomes and arranges execution, perhaps through Tend.
Weave alone is not a replacement for LangGraph's orchestration runtime.

## MCP connects tools; skills explain procedures

MCP standardizes connections between AI applications and external tools, data,
and prompts. Agent Skills package procedural instructions in a folder with a
`SKILL.md` file and optional scripts and resources. They solve different parts
of a workflow and can be used together.
Sources: [MCP introduction](https://modelcontextprotocol.io/docs/2026-07-28/getting-started/intro),
[Agent Skills overview](https://agentskills.io/home).

For a release announcement, an MCP connection might provide access to your
issue tracker; a skill explains how your team writes release notes. Neither
the connection nor the prose, by itself, proves the announcement is correct
or that publishing it has been approved.

Bench participates in both formats. [Brief](../tools/brief/README.md) finds,
reads, and validates skills; the calling agent executes their procedures.
Bench's [MCP tools](../tools/mcp/README.md) can turn selected server capabilities
into local executable wrappers, or expose a dispatcher program as a server.
An operator reviews and admits capabilities, and wrappers check their reviewed
descriptors before calling them. Protocol versions remain explicit.

Use the host assistant's own MCP and skill support when you want the capability
in that assistant. Use Brief or MCP wrappers when your own scripts and workers
also need it. Check host-specific skill fields and server protocol support;
sharing a format does not make every extension interchangeable.

## Durability: distinguish remembering from safely repeating

A saved conversation answers “what did the model see?” A durable job record
answers “what happened to this execution?” Those are different questions.

[Tend](../tools/tend/README.md) saves local jobs, input, attempts, output, and
wait conditions in single-host state. Each `tend work` performs one transition
and exits; you arrange repeated invocations. It can run an ordinary program
without that program using an agent SDK. When an attempt may have started but
its outcome is untrustworthy, Tend records `unknown` and requires explicit
resolution. It does not silently retry a possibly completed external effect.

Temporal instead runs application workflows through its SDK, service, and
workers. It uses recorded event history to reconstruct workflow progress
after failures. It is a different fit for workflows that need a service and
worker infrastructure rather than a local command queue.
Source: [Temporal workflow execution](https://docs.temporal.io/workflow-execution).

For any design, consider “the ticket was created, but the reply was lost.”
Repeating the request could create a second ticket. A service-supported
idempotency key lets repeats identify the same operation; otherwise the
controller may need to inspect the service before deciding. LangGraph's
interrupt documentation makes the same issue concrete: resuming reruns a
node, so effects before the interrupt must tolerate repetition.
Source: [LangGraph interrupt rules](https://docs.langchain.com/oss/python/langgraph/interrupts#side-effects-called-before-interrupt-must-be-idempotent).

## Checks and records: be precise about what they prove

Tests, approvals, sandboxes, and traces also exist in modern agent products.
Bench makes their responsibilities available as separate commands:

- **Ply's check** decides whether a candidate meets the condition you wrote.
  “A file exists” is weaker evidence than “its rows match these expected rows.”
- **[Action](../tools/action/README.md) and [May](../tools/may/README.md)** gate
  an exact proposed effect. **[Cage](../tools/cage/README.md)** constrains a
  child's execution. Approval and confinement answer different questions.
- **[Ask replay](../tools/ask/README.md#inspect-the-record)** checks consistency
  of recorded events; **[Trail](../tools/trail/README.md)** helps inspect them.
  A consistent history does not prove that the model's statements are true.
- **[Cite](../tools/cite/README.md)** checks reference identity against supplied
  evidence. Assessing whether that evidence supports a claim is another check.

The practical benefit is being able to replace or test one step independently.
It only works if your controller preserves the boundary: a worker that can
rewrite its checker can weaken the meaning of “passed.”

## Choose one task to make reusable

- **“Explain this failed build.”** Start with your coding assistant, or pipe a
  saved log into Ask. A model request is enough; no worker queue is needed.
- **“Every Friday, draft release notes from our change list.”** Start with a
  Brief skill and Ask. Add Ply when the work needs tool calls and a check;
  use Agent when the recurring job needs its own home. Your scheduler supplies
  Friday's trigger. Keep publication a separately controlled step.
- **“Build a support agent inside our web app.”** Start with an SDK or graph
  runtime, especially if the application's language and storage already fit.
  Reuse a Bench executable where it gives you a useful, testable boundary.
- **“I know the task, but not shell scripting.”** Ask your coding assistant:
  “Read the relevant Bench manuals. Build a command that takes this sample
  input and produces this expected output. Explain every file and exit status.
  Include a check that rejects this deliberately wrong example.” Then run and
  inspect those examples before expanding the task.

Start at the [first-result walkthrough](GETTING-STARTED.md) or follow one
tool's linked tutorial. Add the next component when a concrete need appears.
