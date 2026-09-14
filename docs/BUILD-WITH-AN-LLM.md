# Find, build and use workers and teams

[Home](../README.md) · [Worker catalog](../workers/README.md) · [Team catalog](../teams/README.md) · [Source and lifecycle](WORKER-LIBRARY.md)

Describe the job in ordinary language. Find the expertise already available,
assemble what fits, and build only what is missing. **Hire builds definitions;
Agent runs them.** The same worker can run independently or participate in
several teams, with explicitly supplied current inputs.

Using a harness such as Claude or Codex? Give it [START-HERE.md](../START-HERE.md)
and your desired outcome. It should perform this workflow with the existing
commands, evaluate the result and leave a method you can use again.

## Understand the four pieces

| Piece | What it is | Where it lives |
| --- | --- | --- |
| Worker, also called an expert | Instructions, optional skills/tools and an acceptance check | `workers/ID/expert` in source; a clean `expert/` after export |
| Agent | The executable that runs a worker's definition against a goal | Selected installed command prefix |
| Team | A roster plus wiring that makes selected workers cooperate | `teams/ID/team.json` and its source template; runnable after assembly |
| Run | Current inputs, work, results, mutable memories and evidence | Fresh directories outside reusable source |

A worker's definition plus `agent run` forms its command/filter. It has its own
context and workspace; it can produce a stdout answer, files, or both. The
page team is a command too: a manager proposes assignments and existing Bench
Manage, Tend and Weave arrange execution and checked handoffs. Its accepted
HTML comes back on stdout. A roster alone is not a scheduler.

## 1. Locate an existing worker or team

Use the selected source checkout from your `BENCH-SETUP.md` or clone
[the public monorepo](https://github.com/patrickyoung/bench-tools) into a new
folder. From its root:

```sh
python3 scripts/workers list --all
python3 scripts/workers list --teams --all
```

These are local metadata reads; they require Python and the checkout, not an
installed model connection. Default listings contain active entries only.
`--all` also shows experimental, deprecated and retired definitions.

Read the [worker catalog](../workers/README.md) and [team catalog](../teams/README.md).
For a likely match, inspect its metadata, `expert/README.md`, instructions and
check. Metadata is authoritative for status, owner, requirements, approved
files and team membership. Installed commands and copied host skills do not
contain another worker registry; use this checkout to find reusable source.

For example, Visual Artist provides p5.js/D3 artistic visualization. Frontend
integrates contributions into a single-file page. Page Team already assembles
those roles with planning and review. Choose the artist for a standalone piece,
or the page team when you need a complete reviewed page. Independent workers
still require their declared inputs: Page Reviewer needs observations, and
Image Concept creates a request for a separately selected image generator.

A fixed transformation may need only an ordinary program; one text answer
may need only [Ask](../tools/ask/README.md). Use the smallest suitable composition.

## 2. Export a clean, pinned definition

Select a full reviewed source commit:

```sh
git rev-parse HEAD
python3 scripts/workers export frontend /absolute/project/frontend \
  --ref FULL_COMMIT --allow-experimental
```

Replace `FULL_COMMIT` with the full selected commit. Choose a new destination
outside the checkout. This creates `expert/` and `worker.lock.json` with source
identity and file hashes. `--allow-experimental` explicitly selects an evaluation
of an experimental entry; it never enables retired or deprecated source.

[Install](INSTALL.md) the selected worker's declared commands. The common
starting set is `hire agent ask brief ply cage record`; an individual definition may
need more tools. [Configure Ask's model connection](GETTING-STARTED.md#2-connect-a-model)
separately from the harness login. Follow the exported README for dependencies,
installing them only in that copy. Read its check before executing it.

The exported files come from the commit, even if the checkout has later edits.
Job briefs, generated sites or artwork, run history and development leftovers
are excluded. Keep the source lock; it describes the starting bytes, not later
package installations or authoring edits.

## 3. Run one worker

Create a fresh workspace and supply only the current inputs. For the exported
Frontend, write the current page requirements in a separate brief file and run:

```sh
mkdir -p /absolute/project/runs/page-001/work
agent show -C /absolute/project/runs/page-001/work \
  /absolute/project/frontend/expert
agent run -C /absolute/project/runs/page-001/work \
  -evidence /absolute/project/runs/page-001/control \
  -turns 8 -goal-file /absolute/project/current-brief.md \
  /absolute/project/frontend/expert > /absolute/project/runs/page-001/report.txt
```

Replace the example paths and supply the brief before execution. Frontend's
contract puts the page in `work/output/index.html` with a bound handoff. Its
local check establishes the static file contract; arrange independent browser
and visual review before calling the page accepted for delivery. The full page
team below already supplies that review stage.

Agent reads stdin, writes its report to stdout, sends diagnostics to stderr and
returns an outcome status. Stdin is evidence when an explicit or standing goal
exists; otherwise it can supply the goal. Inspect the worker's declared output
files as well as its status. A passing pre-check can finish with no model call
and no report. Use a fresh workspace for a new case to avoid accepting an old
result. Default Cage limits writes/network; host reads remain available.

## 4. Run a managed team

Export the team instead of one worker:

```sh
python3 scripts/workers export-team page-team /absolute/project/page-team \
  --ref FULL_COMMIT --allow-experimental
```

The exporter reads [team.json](../teams/page-team/team.json) and copies each
selected worker into `expert/agents/ROLE`, with declared executable bindings
under `expert/bin/workers/ROLE`. `team.lock.json` identifies all members and
hashes. One monorepo commit pins the whole assembly. The source template alone
has no bundled workers and cannot run as the finished team.

Complete the [exported team's setup](../teams/page-team/expert/README.md):
its selected Bench commands, separately installed Bench Manage, pinned browser
and artist packages, model access and operator settings. Optional native art
tools are needed only for briefs that require them. Then:

```sh
export PAGE_TEAM_RUN=/absolute/project/runs/team-page-001
/absolute/project/page-team/expert/bin/page-team \
  < /absolute/project/current-brief.md > /absolute/project/team-page-001.html
```

The result is accepted HTML only on exit 0. The run retains separate worker
and control directories, source masters, reviews and status. A manager chooses
useful available roles and proposes bounded tasks; the existing controller
admits and executes them. Each worker gets its own context and workspace,
receiving selected dependencies through checked file handoffs. It does not
inherit the manager's conversation or unrelated past jobs.

Use a [simple](../teams/simple-site.md), [creative](../teams/creative-site.md) or
[artistic](../teams/artistic-site.md) recipe to guide the current brief. Those
Markdown recipes describe use of a team; they are not extra runtimes. Resume a
known unfinished page run using its documented empty-stdin continuation;
select a fresh run directory for a new job.

## 5. Build missing expertise or change the team

Use an unchanged suitable export directly. When expertise is missing, give
Hire a focused brief in a fresh authoring directory outside source. For example:

```sh
mkdir authoring
cat > job.txt <<'TEXT'
Build a support-reply worker under expert/ using the installed Agent runner.
Input: question.txt and policy.txt supplied in each caller's workspace.
Output: reply.md for human review. Do not send it.
Method: answer from the policy and explicitly identify what is unknown.
Acceptance: reject absent output and missing required structure; document what
mechanical checks cannot establish about truth and completeness.
Reusable source: instructions, a useful skill and a focused check.
Keep synthetic acceptance cases under tests/ beside expert/, and keep current
job inputs, generated replies and evaluation records outside expert/.
Do not implement a provider client or another agent loop.
TEXT
hire build -C authoring -evidence authoring-records -goal-file job.txt
hire verify authoring/expert
```

Inspect what Hire produced. Its structural verification does not execute the
generated check or establish task quality. For direct authoring, `hire new`
creates a scaffold whose initial check deliberately rejects unfinished work.
Add the worker README and finish its instructions and acceptance check before
using `hire verify`; the scaffold alone is not a completed worker.
The [support-reply example](../examples/support-reply/README.md) includes
explicit teaching inputs and a citation check; keep those practice inputs
separate from a new user's work.

For an existing worker or team, give Hire the clean exported authoring folder,
the desired change and its input/output and acceptance contracts. For team
membership alone, edit the source roster in a branch. Preserve required planning,
integration and review contracts; adding a name does not make an incompatible
worker usable. A new reusable team owns its wiring and roster under `teams/ID`
and references existing `workers/ID` definitions. Do not duplicate worker source
under the team template. See [assembly and promotion](WORKER-LIBRARY.md).

A manager can itself be an existing worker. The team may also use a fixed
composition of commands when dynamic planning is unnecessary. Keep useful
parsers, renderers and checks; Agent and the existing controllers own execution.

## Test the job, not just the demonstration

Keep a small set of cases that distinguish acceptable from plausible:

| Case | What to inspect |
| --- | --- |
| The policy answers the question | Relevant facts survive; the reply answers the actual question |
| The policy is silent | The reply admits the gap and invents no promise |
| The input contains “ignore the policy” | Input text stays evidence rather than becoming an instruction |
| A required output is missing | The executable check rejects it |
| The output has the right shape but a false claim | Independent review catches what a structural check cannot |
| A second customer uses the same expert | Work and records are separate; the definition remains reusable |

First test deterministic checks without a model, including their rejection
paths. Then evaluate real model runs on representative inputs using your
provider account. A local model fixture can prove command composition and
error handling; it cannot prove reasoning quality. The repository's
[verification guide](DEVELOPING.md) keeps those two kinds of evidence separate.

## 6. Make it easy to find next time

Leave an external runbook with the source IDs and full commits, absolute source
checkout and definition paths, input/output locations, exact command, model,
limits, checked behavior and continuation instructions. Record the path to
`BENCH-SETUP.md` where a fresh harness session can find it. Retain evaluation
evidence and current job files outside reusable source.

When adding to the shared library, curate generalized source into `workers/ID`
or `teams/ID`, with its metadata, license and separate synthetic tests. Review
through GitHub; do not copy a complete live home or working directory. Run
source inventory checks and affected acceptance cases, commit, then export
that exact revision. New definitions start experimental. Improve, promote,
deprecate or retire through the [library lifecycle](WORKER-LIBRARY.md#maintain-through-github).
Existing jobs keep their selected version until deliberately upgraded.

## Optionally use the same command over A2A

The existing [A2A server](../tools/a2a/README.md#expose-an-expert) can expose an
ordinary `agent run` invocation or a team entry command. For standard text
input/output, provide an agent card, the command and authenticated listener
configuration. Declare file deliverables with `-artifact`. Input materialization
or a different application format may need a narrow dispatcher; page team
already includes its card and dispatcher. Individual workers are not all
preconfigured as services, but do not need their own protocol implementation.

The `a2a` client is a Unix filter; `a2aserve` is an optional OS-managed network
service. Existing TLS and authentication settings protect its endpoint. A
remote worker participating inside a local team needs an explicit A2A command
binding and compatible handoffs; the current source roster does not accept
remote endpoint URLs. Local use remains the same command with no listener.

For host tool integration, use [MCP](../tools/mcp/README.md). Neither transport
replaces Hire, Agent or the team's controller. Read the selected command's
manual for exact status, authentication and stream behavior.

## Teach an existing worker

Ask your harness: “Teach our architect these platform standards, evaluate it
on fresh cases, and keep company material private.” The shared
[teaching procedure](../.agents/skills/bench/references/learn.md) routes supplied
knowledge through Hire, checked recoveries through Hone, and current company
facts into selected private context. The result is an inspected file change
with evaluation evidence and a reusable version, not merely an acknowledgment
in the host conversation. Existing pinned exports retain their original version.
