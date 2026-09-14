# Install the knowledge and the commands

Use the user's requested machine, prefix, workspace, and existing authorization.
When none is specified, prefer the current supported execution environment and
a user-owned prefix. Identify the actual OS, shell, writable roots, command and
network access. A connected folder does not imply access to the owner's host
shell. Do not install onto a different machine by assumption.

## Locate one authoritative checkout

Reuse this repository if available: it has `components.json`, `scripts/install`,
and `tools/agent/README.md`. Record `git rev-parse HEAD` and any local changes.
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

## Use the existing installer

Check `go version`, `python3 --version`, and `git --version`. Source builds need
Go 1.26+, Python 3.9+, and Git. If a prerequisite is absent, use the host's
documented package installation process within the user's authorization, or
name the exact missing prerequisite. Do not fabricate a working installation.

For “set up Bench to build workers,” the useful starting set is:

```sh
cd "$BENCH_SOURCE"
export BENCH_PREFIX="$HOME/.local"
python3 scripts/install hire agent ask brief ply cage record trail --prefix "$BENCH_PREFIX"
export PATH="$BENCH_PREFIX/bin:$PATH"
```

Before installing, inspect the selected worker or team's metadata and README.
A page team also uses Tend, Weave, separately installed Bench Manage and the
export's pinned browser/artist dependencies. Install according to those declared
requirements; optional art tools depend on the current brief. Setup for an
individual worker does not imply the entire team is ready.

For a smaller job install only its components. Add `cite` for the support
starter, `mcp` for service tools, or another component from the tool map.
The installer preserves unmanaged commands and records its installed files.
On a collision inspect the existing command or choose a separate prefix;
never delete it to make an installation succeed.

Use absolute executable paths in unattended commands. Persist PATH through
the host's supported configuration only when needed and authorized; editing
a shell profile does not change every GUI app, container, or scheduled job.
Keep the absolute `BENCH_SOURCE`, full selected source revision, chosen prefix,
actual command paths and private solution/run root in `BENCH-SETUP.md` in the
user's setup directory. A new session must be able to locate this record;
record its path in the host's existing project instructions or setup handoff.
Use it to recover the source catalog after installing this skill elsewhere.
Binary installation and skill installation do not copy the worker library.

## Verify without a model first

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

Install this skill using your harness's reference. Verify that a fresh session
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

For a tool update use the existing installer from the chosen source revision.
For removal use `python3 scripts/uninstall TOOL --prefix PREFIX`; it checks
ownership. Preserve solution definitions, inputs, and evidence. Refresh or
remove the copied skill through the host's own mechanism, comparing local
changes first. Bench's binary installer does not manage host skill settings.
