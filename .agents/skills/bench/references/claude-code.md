# Set Claude Code up with Bench

Perform [common setup](setup.md) in Claude Code's actual shell environment.
The root `CLAUDE.md` routes a checkout session here; it does not install binaries.
Use a personal skill for availability in other projects:

```sh
skill_target="$HOME/.claude/skills/bench"
mkdir -p "$HOME/.claude/skills"
test ! -e "$skill_target" && cp -R "$BENCH_SOURCE/.agents/skills/bench" "$skill_target"
```

Compare an existing destination and preserve local changes. Invoke `/bench`
in a fresh session to verify discovery. Personal and project skill locations
are documented in [Claude skills](https://code.claude.com/docs/en/skills).

Alternatively, load this repository as a plugin for one session:

```sh
claude --plugin-dir "$BENCH_SOURCE"
```

Invoke `/bench-tools:bench`. The plugin manifest points to the same skill;
install one route to avoid duplicate menu entries. Plugin setup is documented
in [Claude plugins](https://code.claude.com/docs/en/plugins).

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
