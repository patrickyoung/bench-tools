---
name: assembling-experts
description: Assemble a reusable manager and focused specialists when a requested worker needs complementary expertise, artifact production, or independently checked contributions; expose the resulting workers through existing Unix, MCP, and A2A interfaces.
---

# Assemble a worker from expertise and checked capabilities

The user supplies an outcome. Hire builds a runnable definition and its
evaluation cases. Agent runs each expert; existing process tools arrange work.
Do not perform the requested business job in place of building its worker.

## Choose the team from the deliverable

Inspect available experts, skills, programs, and examples before adding code.
Keep useful deterministic renderers, parsers, and checks. A specialty should
usually add instructions, selected tools, and a deliverable check. Do not
duplicate a model connection, context loader, action loop, or scheduler.
Prefer an existing worker unchanged when it already meets the job. State the
gap that each added specialist or executable fills; assembling a larger team
is not itself an improvement.

Identify which decisions need distinct expertise and which are ordinary
transformations. Give a specialist a coherent responsibility that can be used
independently. A manager chooses useful work, resolves shared design decisions,
integrates contributions, and responds to rejected results; it need not summon
every available specialist for every request.

## Make every assignment self-contained

For each specialist state its purpose, actual input, expected files or stdout,
required tools, allowed effects, limits, and an independently executable check.
Keep definitions and checks outside mutable workspaces. Supply actual input
bytes or accessible files; a pathname alone is not input to Ask.

Place reusable child definitions in `agents/NAME/`. Each is a valid expert in
its own right, with `AGENTS.md` and executable `bin/check`. Its memory and skills
belong to that definition. Agent gives children separate model contexts; they
receive an explicit assignment and dependencies, not the parent's conversation.
Different writers get different workspaces. Context separation is not host-read
isolation.

Match the stream to the work. A planner or judge can return a checked JSON
candidate on stdout, with `bin/check` judging that candidate on stdin. Do not
force a file-writing action merely to return text to another program. Asset
production needs files and explicit manifests. Exercise the chosen form within
the actual admitted turn limit; an action followed by a final answer costs more
than one model turn.

Define artifact handoffs before parallel work: formats, filenames, dimensions,
units, shared design tokens or API contracts, and which contributor owns the
final integration. Preserve source masters, production inputs, and provenance
when a renderer or image tool produces a creative asset. Confirm a capability
actually ran before describing its contribution.

## Give the manager a checked backlog

The manager proposes a finite backlog of task IDs, goals, selected specialists,
dependencies, outputs, acceptance criteria, and bounds. Weave already validates
a dependency graph and selects ready tasks. Tend already owns durable execution
and attempts. Inspect an available manager/controller before writing a new one.
The manager's proposal is not an authoritative execution result.

Use verified child outcomes to release dependent work. Keep rejected and unknown
attempts visible; do not blindly retry nonzero statuses. Integration is explicit,
followed by the original root check. Accepted contributions can still combine
into a broken result. An empty ready list does not establish completion.

Admit a known dependency graph together when useful; the controller can release
its stages without asking the manager to rediscover each next step. Budget
artifact creation and final submission. A new assignment has a new workspace;
failed-task files are not magically carried over. A retry needs explicit inputs
or enough allowance to reproduce the work. Repeated turn-limit failures call
for a changed allowance or plan, not smaller limits and more task IDs.

Give a manager the decision-relevant progress, not repeated successful command
transcripts. Preserve task inputs, dependency edges, states, candidates,
receipts and allowances; retain full diagnostics separately. Label any display
projection and keep its authoritative snapshot handle unchanged. Exercise a
realistically large packet: a small inline fixture does not test a file-spooled
input or the turns required to read it.

Choose the execution boundary explicitly. Agent's default action Cage does not
support ordinary recursive model calls or broader controller writes. Use an
existing external controller for confined child execution, or document an
operator-selected host boundary. Never silently disable confinement to make a
team run. Scheduling and network listeners remain operator-managed processes.

## Offer the requested interfaces over the same definition

- **Unix:** use `agent run -C WORKSPACE EXPERT` with a supplied goal/input and
  explicit evidence root. Preserve stdout, stderr, artifacts, and exit status.
- **Subagent:** use that same definition in a separate Agent invocation, with
  a bounded assignment and explicit input/output handoff.
- **MCP:** use the existing MCPserve manifest and dispatcher contract. A thin
  adapter translates application input/output and calls fixed public programs.
  Test the host's lifecycle, including explicit legacy compatibility if needed.
- **API/A2A:** use A2Aserve's existing REST/JSON-RPC task interface, artifact
  export, and authentication support. Keep task workspaces and credential
  selection under the operator's control; do not build another network server.

Expose only requested interfaces. Adapters do not implement intelligence or a
second loop. Keep local use possible without a listener. Read the selected
commands' current manuals instead of inventing flags or protocol envelopes.

## Prove the assembled worker

Before expanding the team, make one small contribution travel through the real
public commands and its acceptance check, including a rejected case. This
catches invented flags, packet shapes, and workspace boundaries early. Extend
that working path while retaining its checks; a partial path is not the whole
requested result.

Build positive and negative cases for each contribution and the combined
deliverable. Inspect generated checks before executing them. Verify that a
specialist works independently, that required dependencies are actually passed,
and that a failed contribution cannot yield a successful manager result.
Use the `checking-deliverables` skill when those checks need semantic or visual
judgment. Team integration needs its own evidence; accepted parts alone do not
prove coherent writing, faithful claims or a usable final layout.

Run representative model-backed cases when the user requests a working solution
and its configured capabilities are available. Keep offline fixture results
separate from live quality evidence. A generated plan, valid folder, or successful
tool installation is not a completed worker evaluation.

Leave the reusable definitions, capability prerequisites, cases, observed
evaluation results, exact rerun commands, and a documented way to add one more
specialist without changing the runner. State any unverified capability or
remaining setup precisely. Do not label a worker perfect or autonomous on the
strength of one successful demonstration.
