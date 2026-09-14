# Set Claude Cowork up with Bench

Read this skill directly to begin the current job. For future tasks, install it
through the account's supported skill/plugin interface. Cowork does not load
the laptop's `~/.claude/skills` directory; account-enabled skills are synced at
session start. [Claude's Cowork skill guidance](https://code.claude.com/docs/en/skills#use-skills-in-cowork-and-cloud-sessions).

This repo can be packaged as a small skills-only plugin. From the checkout:

```sh
plugin_file="$(mktemp -d)/bench-tools.zip"
zip -r "$plugin_file" .claude-plugin/plugin.json .agents/skills LICENSE
```

Use **Cowork → Customize → Plugins** to upload the resulting custom plugin.
If the harness has a supported way to perform that account action, use it
within the user's authorization; otherwise hand over the exact ZIP for that
one UI step and continue the current job from the already readable skill.
Start a fresh task and verify Bench is available in the skill menu.
[Claude plugin installation](https://support.claude.com/en/articles/13837440-use-plugins-in-claude).

## Establish where commands can actually run

Perform [common setup](setup.md) inside Cowork's execution environment.
Inspect OS, available commands, writable roots, outbound access, and persistence.
Do not assume a mounted folder provides a host shell, host Go installation,
credentials, or the same sandbox backend. If source builds and native Cage work
there, install into that environment's user prefix and run the requested tests.
Record whether the installation survives a fresh task; repeat setup when it does
not. Do not claim a one-task installation is permanent.

If that environment cannot execute Bench, use an explicitly available remote
execution connection to a selected Linux/macOS host. Run the same source
installer and verification there. When no such connection exists, prepare the
expert definition, cases, and exact host setup/run commands; report the specific
unexecuted step. Do not silently use the user's laptop or disable Cage.

## Connect a reusable capability

Cowork connectors reach services through Anthropic's cloud. A custom connector
needs a reachable server; laptop localhost is insufficient.
[Connector network boundary](https://support.claude.com/en/articles/13837440-use-plugins-in-claude).

Use the [MCP procedure](mcp.md) on the selected host. Test the program first,
then serve MCP over an operator-managed authenticated HTTPS endpoint. Use the
account's custom connector interface and verify a harmless call from Cowork.
Do not publish a development listener without the required hosting and access
decisions. Keep remote Agent workspaces and evidence on the execution host and
return explicit artifact references through the agreed tool contract.

## Use Bench for the user's job

Once the needed setup works, follow [library discovery and assembly](library.md)
first. Find an existing worker or team, export its reviewed source, then use
[building](build.md) for missing expertise and [operation](operate.md) to run it.
Leave the `BENCH-SETUP.md` location, source pin, definition path and repeat command
where this host's next session can find them. Keep current job content separate.
The host authors and invokes Bench definitions; Agent and the existing team
commands retain their execution and context boundaries.

## Teach and improve a worker

For “learn this,” “teach this worker,” corrections or worker-specific memory,
follow [teaching and learning](learn.md). Use the same Hire/Hone workflow in
this host; distinguish supplied knowledge, a checked recovery and private
company context. Report the changed files and fresh-run evidence. Refresh an
older copied or installed skill through this host’s existing installation route,
preserving local edits; updating Bench binaries alone does not update the skill.
