# Set OpenClaw up with Bench

Perform [common setup](setup.md) where the selected OpenClaw agent's command
tool executes. Confirm whether that is the gateway host, a container, or a
remote machine before selecting paths or installing programs.

OpenClaw's local skill install accepts a folder whose root contains `SKILL.md`:

```sh
openclaw skills install "$BENCH_SOURCE/.agents/skills/bench" --as bench
```

This installs into the active workspace; use its documented global option
only when setup for all local agents was requested. Inspect an existing skill
before replacement. Verify discovery in a fresh session for the selected agent.
[OpenClaw skills](https://docs.openclaw.ai/tools/skills).

If that command is unavailable in the installed version, copy the same skill
folder to the selected workspace's `skills/bench`, preserving existing files,
and follow that version's discovery controls. The repository root is not itself
a single skill directory; pass `.agents/skills/bench` to local skill installers.

A visible skill does not make gateway-host binaries visible inside a sandbox.
Install or mount the commands using the operator's existing execution boundary,
then repeat version, Cage, and model-access verification from the agent's actual
command context. Do not change global sandbox policy as a setup shortcut.

Run local workers with Agent through the command tool. Use admitted `mcpbox`
wrappers for MCP services, or the existing host integration if it has one. For
cross-machine agent tasks use A2A with explicit endpoint and authentication.
Do not introduce another gateway scheduler or copy private login material into
the expert folder.

## Use Bench for the user's job

Once the needed setup works, follow [library discovery and assembly](library.md)
first. Find an existing worker or team, export its reviewed source, then use
[building](build.md) for missing expertise and [operation](operate.md) to run it.
Leave the `BENCH-SETUP.md` location, source pin, definition path and repeat command
where this host's next session can find them. Keep current job content separate.
The host authors and invokes Bench definitions; Agent and the existing team
commands retain their execution and context boundaries.
