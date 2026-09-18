# Set Codex up with Bench

Give Codex the repository URL and ask it to finish setup:

> Set up Bench from https://github.com/patrickyoung/bench-tools. Read
> START-HERE.md and complete the Codex setup in this environment.

The harness should complete [common setup](setup.md), which installs the
independent commands and records their paths. Use the requested project and
existing permission policy. A desktop task, worktree and cloud task can have
different files and tools.

## Install from GitHub

For Bench knowledge across projects, use Codex's native plugin installer:

```sh
codex plugin marketplace add https://github.com/patrickyoung/bench-tools
codex plugin add bench-tools@bench-tools
```

Start a fresh Codex session and ask **Use Bench to finish setup**. The installed
`bench-tools` plugin exposes `bench-tools:bench`, containing the same skill
and references as this repository. Installation requires no MCP server or
account connection. Check it with
`codex plugin list --marketplace bench-tools --json`; successful setup
also verifies that a fresh session discovers the skill.

The plugin installs the workflow knowledge. Follow [common setup](setup.md) to
install the commands in the environment where Codex's command tool runs. Keep
the source checkout separate from the host-managed plugin cache. Installing
and starting a new session are described in the official
[OpenAI plugin documentation](https://learn.chatgpt.com/docs/plugins).

To refresh the Git source and installed plugin:

```sh
codex plugin marketplace upgrade bench-tools
codex plugin add bench-tools@bench-tools
```

Then start a new session. To remove it, run
`codex plugin remove bench-tools@bench-tools`, followed by
`codex plugin marketplace remove bench-tools` if the source is no longer
needed. This leaves separately installed commands and user work intact.

## Existing checkouts and older Codex versions

Codex discovers `.agents/skills/bench` automatically inside the checkout. For
current work, read `SKILL.md` directly and continue; discovery never needs to
block setup. If this Codex version lacks `plugin marketplace`, either update
Codex or copy the complete skill to its personal skills directory:

```sh
skill_target="$HOME/.agents/skills/bench"
mkdir -p "$HOME/.agents/skills"
test ! -e "$skill_target" && cp -R "$BENCH_SOURCE/.agents/skills/bench" "$skill_target"
```

If it already exists, compare files, reuse an identical copy and preserve user
changes. Start a fresh session and invoke `$bench` to verify discovery. Use
one installation route; a separate personal copy will not update with the
plugin. These locations are documented in
[Build skills](https://learn.chatgpt.com/docs/build-skills).

## Optional MCP connection

For a tested local MCP tool, use Codex's own registration command. The included
hello example can verify the connection without a model in the server:

```sh
codex mcp add bench-hello -- "$BENCH_PREFIX/bin/mcpserve" \
  -allow-legacy \
  "$BENCH_SOURCE/tools/mcp/examples/filter-server/manifest.json" -- \
  "$BENCH_SOURCE/tools/mcp/examples/filter-server/dispatch"
codex mcp get bench-hello
```

Install `mcp` first. Inspect an existing `bench-hello` entry before changing it.
In the next session, list its tools and call `hello`; a saved configuration is
not proof of a successful call. Remove this demonstration with
`codex mcp remove bench-hello` when finished. Replace it with the user's tested
capability only when that connection is part of the requested solution.

Do not copy Codex login tokens into Ask. Complete the separate Ask model-access
step when the job needs model execution. A source/plugin installation does not
grant command, network, or account access to another Codex environment.

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
