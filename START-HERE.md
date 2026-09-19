# Create and use workers and teams with Bench

For installation, give your harness this request:

> Install and set up Bench from https://github.com/patrickyoung/bench-tools.
> Read START-HERE.md and complete setup for this environment.

Install the shared knowledge through the [host's plugin route](#your-harnesss-setup-instructions),
then follow [common setup](.agents/skills/bench/references/setup.md). Complete
both within the user's existing authorization. The default setup command is
`python3 scripts/setup`: it installs the builder tools to a dedicated user
prefix, checks them, and writes `~/.local/share/bench/BENCH-SETUP.md` with an
environment file the next session can load. Supported Linux hosts use
source-matched GitHub packages without a Go compiler; Macs build locally with
Go 1.26+. No manual PATH edit or demo MCP
server is needed. A plugin cache is not the stable writable source checkout.

Give your harness this request:

> Read START-HERE.md. Use Bench to create and evaluate a worker or team for
> [the outcome I want, with the current inputs at these locations]. Find
> reusable definitions first, assemble or adapt what fits, and build only the
> missing expertise. Leave clean reusable source, separate run results, and
> the exact command to use it again.

This is the entry point for **Codex, Claude Code, Claude Cowork, Pi, OpenClaw, Hermes,
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
| Codex | [GitHub plugin installation and command setup](.agents/skills/bench/references/codex.md) |
| Claude Code | [GitHub plugin installation and command setup](.agents/skills/bench/references/claude-code.md) |
| Claude Cowork | [GitHub marketplace installation and execution environment](.agents/skills/bench/references/cowork.md) |
| Pi | [Skill discovery or Git package, and ordinary command tools](.agents/skills/bench/references/pi.md) |
| OpenClaw | [Workspace skill and gateway/sandbox execution](.agents/skills/bench/references/openclaw.md) |
| Hermes | [Profile skills, trusted projects and terminal backend](.agents/skills/bench/references/hermes.md) |
| Another harness | Read the skill directly; use the host's supported skill discovery and command tool. |

If Bench is already installed, reuse its setup record and installed plugin.
Update knowledge through the host's plugin controls when requested, then verify
discovery in a fresh session. For an older copied skill, compare local changes
before refreshing it. Updating binaries or pulling source does not refresh a
separate skill copy. The host references explain each installation route.

A skill installs knowledge; the source installer installs programs. The host
supplies execution permissions. Ask uses its own model connection; a harness
login alone does not configure it. See [observed setup evidence](docs/HARNESS-VERIFICATION.md).

To take one worker into another host's native model/tool loop, use a
[targeted worker export](docs/WORKER-PORTABILITY.md). Keep the full definition
and select `native` or `bench` execution explicitly. Teams retain their existing
Bench entry command; native team translation is not yet supported.

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
| Worker/team evaluation suites and repeat commands | [Evaluation runbook](docs/WORKER-EVALUATIONS.md) |
| Teach a worker, learn from a recovery or retain company context | [Teaching and learning](.agents/skills/bench/references/learn.md), using Hire, Hone and fresh Agent cases |
| A new worker or changed team | [Builder walkthrough](docs/BUILD-WITH-AN-LLM.md) and Hire |
| Practice cases and historical showcases | [examples/](examples/README.md); supply them explicitly for a chosen evaluation |
| Installed programs and their boundaries | [Tool guide](docs/TOOLS.md) and each component's manual |
| Remote workers or services | Existing [A2A](tools/a2a/README.md) or [MCP](tools/mcp/README.md) |
| Omnigent chat with local or remote job sandboxes | [Portable deployment](examples/omnigent/README.md) |
| Persistent builder and deployment chats through Matterbridge | [Matterbridge application](examples/matterbridge/README.md) |

The older plugin under `tools/agent/plugins/bench-system-builder` targets a
separately pinned legacy suite. Use this entry point and its shared skill for
the current monorepo. The large Architect, Writer and Present applications
remain outside the worker library.
