# Set Claude Code up with Bench

## Install from GitHub

```sh
claude plugin marketplace add https://github.com/patrickyoung/bench-tools
claude plugin install bench-tools@bench-tools
```

These use Claude Code's default user scope, so Bench is available across
projects. Inside an interactive Claude session, the same commands start with
`/plugin` instead of `claude plugin`. Add the repository URL above, not a raw
URL to its `marketplace.json`: the skill and references must be fetched together.
[Claude marketplace installation](https://code.claude.com/docs/en/plugin-marketplaces).

Start a fresh Claude Code session and invoke:

```text
/bench-tools:bench Set up Bench for this environment.
```

Then give Bench the job you want done. The plugin supplies the shared skill;
the setup request authorizes the skill to follow [common setup](setup.md),
locate a stable source checkout, run `python3 scripts/setup`, and verify the
installed commands. It leaves `BENCH-SETUP.md` for the next session. Setup makes
no paid model call. Plugin installation itself does not run a shell hook or
install programs. Missing prerequisites or unavailable execution permissions
are reported specifically, with independent setup completed first.

Do runtime setup in Claude Code's actual shell environment. A plugin cache is
managed by Claude and can change during updates: keep the source checkout,
runtime prefix and setup record in the locations selected by common setup.
An existing personal `/bench` skill is a separate copy; compare it before
removing or refreshing it, and use the namespaced plugin skill above to avoid
ambiguity. [Claude skills](https://code.claude.com/docs/en/skills).

## Update, remove or develop

To refresh the installed knowledge:

```sh
claude plugin marketplace update bench-tools
claude plugin update bench-tools@bench-tools
```

Start a fresh session and ask Bench to update the runtime if that is also
wanted. A plugin update does not change a solution's pinned source or binaries.
Remove the plugin with `claude plugin uninstall bench-tools@bench-tools`;
remove the catalog with `claude plugin marketplace remove bench-tools`.
Runtime removal follows [common setup](setup.md).

For checkout development, `claude --plugin-dir "$BENCH_SOURCE"` exposes the
same `/bench-tools:bench` skill for one session. The root `CLAUDE.md` routes a
checkout session to Bench; Claude does not load that file from an installed
plugin, so the plugin carries all required entry instructions in its skill.

## Optional MCP connection

After installing `mcp` and testing its local hello server, register it using
Claude Code's command, preserving any existing entry of that name:

```sh
claude mcp add --transport stdio --scope local bench-hello -- \
  "$BENCH_PREFIX/bin/mcpserve" \
  -allow-legacy \
  "$BENCH_SOURCE/tools/mcp/examples/filter-server/manifest.json" -- \
  "$BENCH_SOURCE/tools/mcp/examples/filter-server/dispatch"
```

Use `/mcp` to inspect the connection and call `hello` from Claude. Remove the
demo with `claude mcp remove --scope local bench-hello` when finished. Use
absolute paths and the desired project scope for a real generated capability.

Claude Code's login and tools are the authoring environment. A Bench worker
uses Agent and Ask, with its own configured model access. Do not extract Claude
credentials or replace the worker with a Claude-specific loop to bypass setup.

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
