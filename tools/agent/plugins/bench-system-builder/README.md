# Bench System Builder

Three inspectable Agent Skills for building, operating, and stewarding governed
Bench systems from Claude Code or Claude Cowork. The package contains Markdown
instructions, references, templates, evaluation cases, and explicit helpers for
runtime checks, platform-aware installation, local operation, and host scheduling.
It contains no hooks, connectors, credentials, or automatic permission grants.

Claude Code or Cowork is the authoring and control surface. The built worker
always runs as an Agent home through the Bench suite's shipped `agent` program
or its transparent `bench home` entry point. These skills do not create a
Claude-native agent, a second loop, a parallel scheduler, or another state
format.

## Skills

- `building-with-bench` turns a business process into a reviewed system design,
  executable proof plan, Agent home, and controlled-pilot handoff.
- `operating-bench-agents` runs, inspects, changes, learns from, and retires a
  built Agent home from controller evidence.
- `stewarding-bench-platform` installs and governs the runtime, checks,
  identities, connections, effects, budgets, and operating boundary.

In Claude Code, the plugin exposes
`/bench-system-builder:building-with-bench` and
`/bench-system-builder:operating-bench-agents`, and
`/bench-system-builder:stewarding-bench-platform`.

## Install

Claude Code users can add the Agent repository as a marketplace:

```text
/plugin marketplace add patrickyoung/agent
/plugin install bench-system-builder@bench-agent-tools
```

For a one-session trial with Claude Code 2.1.128 or newer, download and inspect
the plugin ZIP from the guide, then run it directly:

```text
claude --plugin-dir ./bench-system-builder-0.2.0.zip
```

On an older Claude Code release, extract the ZIP and pass the extracted
`./bench-system-builder` directory instead.

Cowork users can download the same plugin ZIP, then open
**Customize → Plugins → Upload**. Standalone ZIPs for each skill are also
available from the guide under **Customize → Skills → Create skill → Upload a
skill**.

## Runtime boundary

The skills are a complete, versioned operating manual; users do not need to
clone or read the Bench repositories. On supported macOS and Linux hosts, the
steward skill resolves the platform, presents the exact Bench suite 0.13.0
archive and SHA-256, and obtains approval before downloading or installing it.
The checksummed prebuilt suite is the normal path. Building from an exact source
checkout is a separate explicit engineering path, never silent recovery or a
prerequisite.
During a new local build, the builder invokes that reviewed setup procedure and
then resumes the saved design/build gate automatically; the user does not have
to restart the tutorial or translate a platform handoff.

Setup reports separate evidence states. **SUITE-INSTALLED** means the exact
runtime bytes and public commands are verified and a Cage backend is discovered;
it is not yet an enforcement claim. **SUITE-READY** additionally requires the
separately disclosed `cage check` to pass 13/13 on the target host. When a
nested Claude/Cowork sandbox blocks that kernel-level test, the procedure emits
**CAGE-HOST-CHECK-REQUIRED** with one exact host-terminal command and keeps the
saved resume gate. **MODEL-READY** means a non-secret model identifier
and approved controller-owned provider access completed one bounded,
schema-constrained Ask probe in the current lane; a Claude login or credential
presence alone does not establish this. **AGENT-FIRST-RUN** means a checked
home completed one bounded `bench home run` and retained its literal result. If
provider access is absent, the journey stops at the resumable
**MODEL-ACCESS-REQUIRED** gate with an owner and exact next command; it does not
discard the suite or process design.

After a checked home exists beneath `/absolute/agents/NAME`, use the same Bench
Agent system in every mode:

```text
bench -C /absolute/agents -m provider/model -home NAME
bench home -C /absolute/agents run -m provider/model NAME
bench home -C /absolute/agents run -m provider/model -checkpoint monthly-close NAME
bench home -C /absolute/agents tick -m provider/model NAME
```

The first command opens Bench's interactive Agent-home view. `run` pursues the
standing goal on demand; repeating the same named checkpoint resumes its Ask
session; `tick` runs the home's cheap wake gate before any model work. For
recurring work, the operator skill renders a launchd or systemd user schedule
around the physical suite Bench executable's transparent `home ... tick`
boundary for review before activation. Scheduled ticks are distinct by default;
a checkpoint is added only for explicitly intended same-conversation
continuity. Fresh-process provider readiness must be proved first, and generated
scheduler files never embed secrets. Use Tend only when that exact Agent process
needs crash-durable supervision; the host scheduler still owns business cadence.
The generated local schedule is a reviewed pilot scaffold: production still
needs outer duration/cost enforcement, log retention, failure notification, and
the controller-owned provider credential boundary described by the steward
skill.

Cowork's cloud task and a connected local folder do not by themselves prove
that a Mac-installed Bench binary is executable there. Without an approved
managed Bench bridge, use Cowork for design and produce a steward handoff; run
the resulting Agent home from a local Claude Code/terminal surface. If no
compatible suite or managed bridge is reachable, the build skill stays in
design-only mode instead of claiming runtime proof.

Release `0.2.0` describes Bench suite `0.13.0` and Agent `0.2.1`. The skills check
the installed runtime before using versioned commands and stop on an unknown or
incompatible version.
