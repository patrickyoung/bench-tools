# Set Pi up with Bench

Perform [common setup](setup.md) where Pi's bash tool executes. Pi discovers
`.agents/skills` in trusted projects and `~/.agents/skills` for personal use;
it also accepts an explicit `--skill` path.
[Pi skill discovery](https://pi.dev/docs/latest/skills).

For one session:

```sh
pi --skill "$BENCH_SOURCE/.agents/skills/bench"
```

For use across projects, copy the complete folder without replacing an existing
skill's local changes:

```sh
skill_target="$HOME/.agents/skills/bench"
mkdir -p "$HOME/.agents/skills"
test ! -e "$skill_target" && cp -R "$BENCH_SOURCE/.agents/skills/bench" "$skill_target"
```

Invoke `/skill:bench` in a fresh session and verify it loads. Codex and Pi can
share this personal folder; do not install a second conflicting copy.

The repository also declares its skill in `package.json`, so Pi's native Git
package route can install it with `pi install git:github.com/patrickyoung/bench-tools`.
Use one discovery route. Package installation installs the skill; the skill
then uses Bench's installer for the native commands.
[Pi packages](https://pi.dev/docs/latest/packages).

Pi can call Bench executables through bash immediately. For service tools,
use `mcpbox` to admit reviewed executable wrappers and document them in a skill.
MCP integration in Pi can also be supplied by extensions; inspect an existing
extension and follow its own configuration instead of inventing a built-in
`pi mcp add` command. [Pi's extension model](https://pi.dev/).

Keep Pi's conversation and model setup separate from Agent/Ask evidence. A
new worker still runs through Agent. Use Pi's normal permission and project
trust controls; a package should not add startup hooks or grant itself access.

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
