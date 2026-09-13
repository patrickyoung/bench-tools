# Set Codex up with Bench

Perform [common setup](setup.md) in the environment where Codex's command
tool runs. Use the requested project and existing permission policy. A local
desktop task, worktree, and cloud task may have different files and tools.

Codex discovers this repo's `.agents/skills/bench` when working in the checkout.
For use across projects, copy the same complete skill into its personal skills
directory. These locations are documented in
[Build skills](https://learn.chatgpt.com/docs/build-skills).

```sh
skill_target="$HOME/.agents/skills/bench"
mkdir -p "$HOME/.agents/skills"
test ! -e "$skill_target" && cp -R "$BENCH_SOURCE/.agents/skills/bench" "$skill_target"
```

If it already exists, compare the files; reuse an identical installation and
preserve user changes. Start a fresh Codex session and invoke `$bench` to
verify discovery. For the current session, read `SKILL.md` directly and continue
the requested setup/build; a restart need not block useful work.

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
