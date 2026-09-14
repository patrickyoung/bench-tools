# Create and use workers and teams with Bench

Give your harness this request:

> Read START-HERE.md. Use Bench to create and evaluate a worker or team for
> [the outcome I want, with the current inputs at these locations]. Find
> reusable definitions first, assemble or adapt what fits, and build only the
> missing expertise. Leave clean reusable source, separate run results, and
> the exact command to use it again.

This is the entry point for **Codex, Claude Code, Claude Cowork, Pi, OpenClaw,
and other harnesses that can read files and run commands**. The public source
is [patrickyoung/bench-tools](https://github.com/patrickyoung/bench-tools).
For human readers, follow the [practical walkthrough](docs/BUILD-WITH-AN-LLM.md).

## The harness's primary job

Help the user find, create, assemble, evaluate and use workers and teams.
Reuse the existing commands to deliver the requested result. **Hire builds;
Agent runs; Unix connects them.** Toolkit development is a separate task when
the user actually needs a new capability in the tools themselves.

Read the [Bench skill](.agents/skills/bench/SKILL.md), then load only the
references needed for the current request:

1. **Locate the source and current setup.** Read the existing `BENCH-SETUP.md`
   if available. Follow [setup](.agents/skills/bench/references/setup.md) and
   the host reference below when commands or skill discovery need setup.
   Listing available workers needs only the source checkout, not a model.
2. **Find before building.** Follow [library discovery and assembly](.agents/skills/bench/references/library.md).
   Inspect both catalogs, metadata, requirements and checks. Use `--all` to
   see experimental entries. Select one worker or an appropriate team.
3. **Export or build.** Export suitable committed source into a new directory.
   A team export assembles its roster and clean member copies. For missing
   expertise or changed wiring, follow [building](.agents/skills/bench/references/build.md)
   with Hire. Keep source, installed dependencies, current work and evidence
   identifiable and separate.
4. **Run and evaluate.** Use Agent for an individual definition or the team's
   documented entry command. Each assigned worker has its own context and
   workspace. Follow [evaluation](.agents/skills/bench/references/evaluate.md);
   distinguish structural checks, process fixtures and actual job quality.
5. **Make it discoverable next time.** Record source IDs, pins, absolute paths,
   output locations and the repeat command in the external runbook. Promote
   useful generalized changes to `workers/` or `teams/` through GitHub when
   that is in scope. Follow [operation](.agents/skills/bench/references/operate.md)
   for reuse, continuation, optional MCP or A2A.

A worker/expert is a definition; Agent is its runner. A team is a roster plus
wiring; it may put a planning worker in front of specialists. The same worker
can serve several teams or run alone with explicitly supplied inputs. A run
contains the job's private inputs, output, runtime memory and records. Those
files are never default inputs to another job or part of a reusable export.

## Your harness's setup instructions

| Harness | Install its Bench knowledge and verify access |
| --- | --- |
| Codex | [Repository discovery, personal skill, and MCP registration](.agents/skills/bench/references/codex.md) |
| Claude Code | [Personal skill or plugin, and MCP registration](.agents/skills/bench/references/claude-code.md) |
| Claude Cowork | [Account skill/plugin, execution environment, and remote connectors](.agents/skills/bench/references/cowork.md) |
| Pi | [Skill discovery or Git package, and ordinary command tools](.agents/skills/bench/references/pi.md) |
| OpenClaw | [Workspace skill and gateway/sandbox execution](.agents/skills/bench/references/openclaw.md) |
| Another harness | Read the skill directly; use the host's supported skill discovery and command tool. |

If Bench is already installed, refresh the copied skill from the selected
checkout after comparing local changes, then verify discovery in a fresh
session. Updating binaries or pulling the source does not refresh a separate
personal/account skill copy. The host references explain each installation route.

A skill installs knowledge; the source installer installs programs. The host
supplies execution permissions. Ask uses its own model connection; a harness
login alone does not configure it. See [observed setup evidence](docs/HARNESS-VERIFICATION.md).

If the request is only setup, finish setup without inventing a business job or
paid evaluation. If the user supplied a job, continue using the authorization
already given. Resolve routine choices yourself; ask only for missing facts
or access that prevent necessary work. Preserve existing settings.

## Where to look

| Need | Authoritative location |
| --- | --- |
| Available specialties | [workers/README.md](workers/README.md), then the selected `worker.json` and `expert/README.md` |
| Runnable teams and member selection | [teams/README.md](teams/README.md), then the selected `team.json` and wiring README |
| Source exports, versioning and retirement | [Library guide](docs/WORKER-LIBRARY.md) |
| Teach a worker, learn from a recovery or retain company context | [Teaching and learning](.agents/skills/bench/references/learn.md), using Hire, Hone and fresh Agent cases |
| A new worker or changed team | [Builder walkthrough](docs/BUILD-WITH-AN-LLM.md) and Hire |
| Practice cases and historical showcases | [examples/](examples/README.md); supply them explicitly for a chosen evaluation |
| Installed programs and their boundaries | [Tool guide](docs/TOOLS.md) and each component's manual |
| Remote workers or services | Existing [A2A](tools/a2a/README.md) or [MCP](tools/mcp/README.md) |

The older plugin under `tools/agent/plugins/bench-system-builder` targets a
separately pinned legacy suite. Use this entry point and its shared skill for
the current monorepo. The large Architect, Writer and Present applications
remain outside the worker library.
