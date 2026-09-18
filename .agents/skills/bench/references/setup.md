# Install the knowledge and the commands

Use the user's requested machine, prefix, workspace, and existing authorization.
When none is specified, prefer the current supported execution environment and
a user-owned prefix. Identify the actual OS, shell, writable roots, command and
network access. A connected folder does not imply access to the owner's host
shell. Do not install onto a different machine by assumption.

For Codex, Claude Code and Cowork, use the host reference's GitHub marketplace
route for the skill. If it is already installed, use it directly. Install once;
do not add a personal skill copy alongside the same plugin.

## Locate one authoritative checkout

Read the user's selected `BENCH-SETUP.md` first, or check
`~/.local/share/bench/BENCH-SETUP.md`. If its source and commands still exist in
this environment, load the adjacent `env.sh` and reuse them. A record from a
different Cowork VM or host is not an installed runtime here.

Reuse an existing authoritative checkout if available: it has `components.json`,
`scripts/install`, and `tools/agent/README.md`. A host-managed plugin cache is
knowledge distribution, not the writable source checkout. Record `git rev-parse
HEAD` and any local changes.
If absent, clone the public source into a new directory:

```sh
mkdir -p "$HOME/.local/share/bench"
git clone https://github.com/patrickyoung/bench-tools.git "$HOME/.local/share/bench/source"
export BENCH_SOURCE="$HOME/.local/share/bench/source"
```

An existing destination needs inspection, not replacement. For an existing
checkout, set `BENCH_SOURCE` to its absolute path instead. Honor a requested
revision; otherwise record the selected main commit. Do not update an existing
solution's pinned runtime as a side effect of starting another job.

Read `$BENCH_SOURCE/docs/INSTALL.md` and the selected tool READMEs. The old
Bench application's pinned suite is a separate installation. Check actual
command paths and versions; do not mix its old Agent authoring commands with
the current Hire/Agent split.

## Finish setup with one command

Check `go version`, `python3 --version`, and `git --version`. Source builds need
Go 1.26+, Python 3.9+, and Git. If a prerequisite is absent, use the host's
documented package installation process within the user's authorization, or
name the exact missing prerequisite. Do not fabricate a working installation.

For “set up Bench,” run:

```sh
cd "$BENCH_SOURCE"
python3 scripts/setup
. "$HOME/.local/share/bench/env.sh"
```

This source setup helper composes the existing installer. It installs Hire,
Agent, Ask, Brief, Ply, Cage, Record, Trail and Hone into
`~/.local/share/bench/runtime`, checks command versions, verifies a sample
definition's structure and runs Cage's native proof. It writes the absolute
paths, source pin and observed checks to `~/.local/share/bench/BENCH-SETUP.md`,
plus `env.sh` and `setup.json`. It makes no model call and edits no host settings,
shell profiles or credentials. The separate prefix avoids unrelated command
collisions; loading `env.sh` keeps the selected companion programs on PATH.

For a user-selected location, pass `--prefix /absolute/runtime` and
`--state-dir /absolute/setup-records`, then load that directory's `env.sh`.
In Cowork, use an available persistent location for the setup record and verify
that paths still exist in the next task. A copied record cannot persist a VM.
Keep personal notes in a separate runbook: setup refuses to overwrite an edited
handoff or environment file. Preserve existing records and reconcile the change
or select a new state directory instead of deleting them.

If setup returns nonzero, inspect the named failing check. Installed commands
may still be usable for independent preparation, but do not report setup as
complete. Resolve missing prerequisites using the host's supported installation
process. Never disable Cage just to make setup pass.

For a particular job, inspect the selected worker or team's metadata and README.
A page team also uses Tend, Weave, separately installed Bench Manage and the
export's pinned browser/artist dependencies. Install according to those declared
requirements; optional art tools depend on the current brief. Setup for an
individual worker does not imply the entire team is ready.

For a deliberately smaller installation use `python3 scripts/install COMPONENTS
--prefix PREFIX` and record its paths/checks manually. Add `cite` for the support
starter, `mcp` for service tools, or another component from the tool map.
The installer preserves unmanaged commands and records its installed files.
On a collision inspect the existing command or choose a separate prefix;
never delete it to make an installation succeed.

Use absolute executable paths in unattended commands. Persist PATH through
the host's supported configuration only when needed and authorized; editing
a shell profile does not change every GUI app, container, or scheduled job.
The generated `BENCH-SETUP.md` captures source and command paths. Keep private
solution/run roots in the job runbook. A new session must be able to locate a
nondefault setup record; record its path in the host's project instructions or
setup handoff.
Use it to recover the source catalog after installing this skill elsewhere.
Binary installation and skill installation do not copy the worker library.

Record non-secret provider/model and approved executable-wrapper selections as
well as binary paths. Agent's `AGENT_ASK` and Hone's `ASK` are separate selectors;
use the same configured connection when teaching from its runs. A raw installed
Ask binary is not a replacement for a required operator-selected wrapper.

## Verify without a model first

The setup helper already runs these checks; use them individually to diagnose
or recheck an installation:

```sh
"$BENCH_PREFIX/bin/agent" version
"$BENCH_PREFIX/bin/hire" version
"$BENCH_PREFIX/bin/record" version
"$BENCH_PREFIX/bin/cage" check
"$BENCH_PREFIX/bin/hire" verify "$BENCH_SOURCE/examples/support-reply/expert"
```

Add Cite before running that example's checker. `hire verify` checks structure,
not task quality. Cage's native proof may be unavailable inside an outer host
sandbox. Report that fact; do not replace the default with `-no-cage` or claim
the boundary passed. An explicitly selected host boundary is a separate choice.

Verify the installed skill in a fresh session using your harness's reference.
That session must recover the setup record and command paths. Verify that it
can discover it; file creation alone is weaker evidence than discovery.

## Establish model access separately

Read `$BENCH_SOURCE/docs/GETTING-STARTED.md` and `tools/ask/README.md`. Ask uses
its own provider configuration. A Codex, Claude, or Pi subscription/login is
not proof that Ask can use that account. Reuse an explicitly configured Ask
provider, environment capability, or supported credential wrapper. Do not
extract tokens from the harness's private login files or ask for secret values
in chat. Describe the non-secret setup step when access is missing.

When the requested work needs a model and access is available, make one small
Ask request in a private workspace, retain its session, check the status, and
run `ask replay -check SESSION`. Record the model and observed outcome. This
proves access and record consistency, not the worker's business quality.

## Update and remove deliberately

For a tool update select the source revision deliberately and repeat
`scripts/setup` with the same prefix/state directory. For a smaller installation,
repeat the existing installer with the selected components.
For removal use `python3 scripts/uninstall TOOL --prefix PREFIX`; it checks
ownership. Preserve solution definitions, inputs, and evidence. Refresh or
remove the plugin through the host's own mechanism. For older copied skills,
compare local changes first. Bench's binary installer does not manage host
skill settings. Setup records and private jobs remain on disk after uninstall;
remove an obsolete generated record only after checking it is no longer used.
