# Set your harness up with Bench

Give your coding agent or assistant this request:

> Read this repository's START-HERE.md. Set yourself up to use Bench, verify
> the installation, and use its existing tools to build and evaluate my job:
> **[describe the result you want, and where the inputs live].** Leave a
> reusable expert, tested examples, and the exact command to run it again.

The public repository is [patrickyoung/bench-tools](https://github.com/patrickyoung/bench-tools).
This file is the entry point for **Codex, Claude Code, Claude Cowork, Pi,
OpenClaw, and other harnesses that can read files and run programs**.

## Instructions for the harness

Read the [Bench skill](.agents/skills/bench/SKILL.md). It contains the setup,
building, evaluation, operation, and MCP procedures. Load the references for
the current step, rather than every manual at once.

1. **Set yourself up.** Follow [setup](.agents/skills/bench/references/setup.md)
   and your own harness instructions below. Reuse installed tools when they
   fit; otherwise use this repo's source installer. Preserve existing settings.
2. **Build the requested result.** Reuse an ordinary command, Ask, or an expert
   before adding code. Hire builds expert folders; Agent runs them. Put the
   user's solution in its own directory, separate from this source checkout.
3. **Evaluate it.** Try representative inputs and deliberately bad outputs.
   Separate structural checks, runtime evidence, and judgment about quality.
4. **Make it reusable.** Leave the definition, examples, evidence, and an
   exact runbook. Test a second fresh workspace. Add a host skill or MCP
   capability when the user wants to invoke it from their harness.

If the request is only “set yourself up,” finish setup and report the verified
capabilities. Do not invent a business job. When the user supplied a job,
continue into it using the authorization already given. Ask only for missing
information or access that actually prevents the next necessary step.

## Your harness's setup instructions

| Harness | Install its Bench knowledge and verify access |
| --- | --- |
| Codex | [Repository discovery, personal skill, and MCP registration](.agents/skills/bench/references/codex.md) |
| Claude Code | [Personal skill or plugin, and MCP registration](.agents/skills/bench/references/claude-code.md) |
| Claude Cowork | [Account skill/plugin, execution environment, and remote connectors](.agents/skills/bench/references/cowork.md) |
| Pi | [Skill discovery or Git package, and ordinary command tools](.agents/skills/bench/references/pi.md) |
| OpenClaw | [Workspace skill and gateway/sandbox execution](.agents/skills/bench/references/openclaw.md) |
| Another harness | Read the skill directly; install it using that host's Agent Skills support, if available. Use its command tool for Bench processes. |

A skill installs knowledge. The source installer installs programs. The
execution environment supplies filesystem access and permissions. Ask needs
its own supported model connection. A harness login alone does not establish
that connection, and a skill cannot grant permissions its host does not have.

See [what has been verified](docs/HARNESS-VERIFICATION.md) for native host
versions, repeatable checks, and setup routes that still need a live host test.

## What is already here

- [A clean worker catalog](workers/README.md) and [team recipes](teams/README.md), with pinned source exports.
- [A complete expert](examples/support-reply/README.md), reusable across workspaces.
- [A builder walkthrough](docs/BUILD-WITH-AN-LLM.md) using Hire and Agent.
- [All 19 components and 23 commands](docs/TOOLS.md), with their own manuals.
- [Runnable examples](examples/README.md) and [verification](docs/DEVELOPING.md).
- [MCP tools](tools/mcp/README.md) for consuming services and exposing programs.
- [A2A](tools/a2a/README.md) for calling and serving remote agents.

These are independent Unix programs. This entry point adds no agent runtime,
provider adapter, scheduler, or permission bypass. The older plugin under
`tools/agent/plugins/bench-system-builder` targets a separately pinned legacy
suite; use this entry point for the current monorepo.
