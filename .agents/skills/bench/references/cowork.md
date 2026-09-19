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
skill follows [common setup](setup.md), locates or creates a source checkout, runs
`python3 scripts/setup` in its available execution environment, and records the
result in `BENCH-SETUP.md`. Setup makes no paid model call. Installing the plugin
adds knowledge; it does not establish an execution boundary or a model account.

Setup prefers source-matched packages from this repository's pinned GitHub
release. It does not need a Go compiler or access to Go module servers on that
route. Install the system Bubblewrap package if the Linux environment lacks it,
using the environment's normal package manager. Complete the actual Cage check;
namespace availability alone does not prove that it works inside Cowork.

This is the personal marketplace route. Organization administrators use a
different distribution flow with their own repository and policy requirements.
If that UI is unavailable, check the account's plugin policy. If this skill is
already readable, continue authorized setup and the current job while clearly
reporting that future-session plugin discovery is not yet verified.
[Organization plugin management](https://support.claude.com/en/articles/13837433-manage-plugins-for-your-organization).

## Establish where commands can actually run

Inspect OS, available commands, writable roots, outbound access, and persistence.
Do not assume a mounted folder provides a host shell, host Go installation,
credentials, or the same sandbox backend. If package installation and native Cage work
there, complete common setup in that environment and run the requested tests.
Cloud Cowork gives each session a temporary sandbox that does not share its
runtime filesystem with another session. Its home directory, source checkout
and installed executables are session-local. Repeat setup and Cage verification
in a new sandbox; the installed desktop plugin remains available as knowledge.
[Cowork architecture](https://support.claude.com/en/articles/14479288-claude-cowork-architecture-overview).

Keep a copy of the setup record in a user-selected connected folder or project
files when continuity matters. Record the session environment, source/release
pins and checks as well as absolute paths. A fresh task can use that record to
select the same source, but must verify paths and install/check its own runtime.
Do not claim a one-task installation is permanent or use the laptop's paths
as proof that programs exist inside Cowork.

If that environment cannot execute Bench, use an explicitly available remote
execution connection to a selected Linux/macOS host. Run the same source
setup and verification there. When no such connection exists, prepare the
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
