# Set Hermes up with Bench

Perform [common setup](setup.md) where Hermes's terminal tool actually runs.
Identify the selected profile, workspace and execution backend before choosing
paths. A local chat and a messaging gateway can have different command access.

For a personal installation, copy this complete skill folder into the selected
profile's skills directory. For the default profile:

```sh
skill_target="$HOME/.hermes/skills/bench"
mkdir -p "$HOME/.hermes/skills"
test ! -e "$skill_target" && cp -R "$BENCH_SOURCE/.agents/skills/bench" "$skill_target"
```

Inspect an existing destination before refreshing it; preserve local changes.
Hermes discovers a folder containing `SKILL.md` and supporting files, lists it
with `hermes skills list`, and exposes `/bench`. Start a fresh session and invoke
the skill to verify it loads. Use the profile's own location when it differs
from the default. [Hermes skill guide](https://hermes-agent.nousresearch.com/docs/guides/work-with-skills).

Other supported routes are a trusted repository's `.hermes/skills` or
`.agents/skills`, or `skills.external_dirs` in the selected profile's config.
Repository discovery requires `hermes skills trust`; review the repository
before trusting it. Trusting a project changes host configuration and is not
part of exporting source. External directories are mutable when writable;
Hermes can update an existing skill in place. Prefer a separate installed copy
when preserving canonical Bench source.
[Hermes skills system](https://hermes-agent.nousresearch.com/docs/user-guide/features/skills).

## Verify execution separately

Run Bench version checks, Cage's native proof and the configured Ask connection
from Hermes's actual terminal context. Hermes's provider account does not
configure Ask. Keep provider credentials, current inputs, runtime memories and
evidence outside the reusable definition. Do not change the terminal backend
or permission policy merely to make a failed check disappear.

For service access use the existing [MCP procedure](mcp.md) or an explicitly
configured A2A endpoint. Record the actual host, paths, source pin, model setup
and exact repeat command in the external `BENCH-SETUP.md`.

## Use and teach workers

Follow [library discovery](library.md), then [building](build.md) for missing
expertise and [operation](operate.md) for Bench execution. A worker exported
specifically for Hermes may instead select native execution; read its generated
`PORTABILITY.md` and use that declared mode. Installing the general Bench skill
does not change existing Agent or team execution contracts.

For teaching or checked recovery, follow [teaching and learning](learn.md).
Use Hire for supplied knowledge and Hone for qualifying Bench recovery evidence.
Hermes's own memory, skill maintenance or delegation transcripts are not Hone
records. Inspect and evaluate source changes, then export the reviewed revision
again. Test the learned behavior in a fresh job on each intended execution path.
