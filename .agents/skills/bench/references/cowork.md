# Set Claude Cowork up with Bench

## Install from GitHub

1. Open the **Cowork** tab, then **Customize → Plugins**.
2. Choose **Add → Add marketplace → Add from a repository** (some versions show **Personal plugins +**).
3. Paste `https://github.com/patrickyoung/bench-tools` and choose **Sync**.
4. Browse the synced marketplace, find **bench-tools**, and select **Install**.

Start a fresh Cowork task, select the Bench skill from the `/` or `+` menu,
and ask: **“Set up Bench for this environment.”** Then give it your job.
No ZIP creation, copied skill directory or terminal installation is required
to add the plugin. Cowork stores personally installed plugins locally; use
its supported UI rather than copying files into `~/.claude/skills`.
[Claude's plugin and marketplace instructions](https://support.claude.com/en/articles/13837440-use-plugins-in-claude).

The repository includes both the marketplace and its plugin. The installed
skill follows [common setup](setup.md), locates a stable source checkout, runs
`python3 scripts/setup` in its available execution environment, and records the
result in `BENCH-SETUP.md`. Setup makes no paid model call. Installing the plugin
adds knowledge; it does not establish an execution boundary or a model account.

This is the personal marketplace route. Organization administrators use a
different distribution flow with their own repository and policy requirements.
If that UI is unavailable, check the account's plugin policy. If this skill is
already readable, continue authorized setup and the current job while clearly
reporting that future-session plugin discovery is not yet verified.
[Organization plugin management](https://support.claude.com/en/articles/13837433-manage-plugins-for-your-organization).

## Establish where commands can actually run

Inspect OS, available commands, writable roots, outbound access, and persistence.
Do not assume a mounted folder provides a host shell, host Go installation,
credentials, or the same sandbox backend. If source builds and native Cage work
there, complete common setup in that environment and run the requested tests.
Record whether the installation survives a fresh task; repeat setup when it does
not. Keep the setup record in a user-selected persistent folder accessible to
the next task; record the execution environment as well as absolute paths.
Do not claim a one-task installation is permanent or use the laptop's paths
as proof that programs exist inside Cowork.

If that environment cannot execute Bench, use an explicitly available remote
execution connection to a selected Linux/macOS host. Run the same source
installer and verification there. When no such connection exists, prepare the
expert definition, cases, and exact host setup/run commands; report the specific
unexecuted step. Do not silently use the user's laptop or disable Cage.

## Connect a reusable capability

Plugins can include local MCP servers where Cowork and account policy support
them. Bench does not register one by default; command setup is enough for
ordinary local work. A remote connector requires a reachable authenticated
service; laptop localhost alone is insufficient.
[Claude plugin capabilities](https://support.claude.com/en/articles/13837440-use-plugins-in-claude).

Use the [MCP procedure](mcp.md) on the selected host. Test the program first,
then serve MCP over an operator-managed authenticated HTTPS endpoint. Use the
account's custom connector interface and verify a harmless call from Cowork.
Do not publish a development listener without the required hosting and access
decisions. Keep remote Agent workspaces and evidence on the execution host and
return explicit artifact references through the agreed tool contract.

## Update or remove

Use Cowork's marketplace/plugin controls to refresh or remove Bench. Start a
fresh task and verify the skill menu after updating. Runtime updates follow
common setup inside the selected execution environment; refreshing the plugin
does not update installed binaries or a solution's pinned source. Preserve
private setup records, definitions, current inputs and results on removal.

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
