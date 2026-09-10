# Install Bench tools

[Start here](GETTING-STARTED.md) · [Tool guide](TOOLS.md) · [Home](../README.md)

Use the shared source installer for the Bench tools toolkit. You need Go 1.26+,
Python 3.9+, Git, and a Unix shell on macOS, Linux, or WSL. Go may download
pinned dependencies on the first build. Make is optional.

```sh
git clone https://github.com/patrickyoung/bench-tools.git
cd bench-tools
```

Run the remaining installation commands from this directory, where
`components.json` and `scripts/` live. They install the source in your checkout.
For a repeatable deployment, [pin a checked commit](RELEASES.md#pin-the-source-you-use).

## Install a few tools or the whole toolkit

For the first walkthrough:

```sh
python3 scripts/install ask
export PATH="$HOME/.local/bin:$PATH"
ask version
```

Add tools when you reach the corresponding workflow: `python3 scripts/install brief`
for reusable instructions, or `python3 scripts/install ask ply` for checked actions.

Without names, the installer builds and installs all 20 commands:

```sh
python3 scripts/install
```

The default location is `~/.local`; no sudo is needed. Add the PATH line once
to your shell's startup file (`~/.zshrc` for interactive zsh or `~/.bashrc` for
interactive Bash). If a new login Bash terminal still cannot find a command,
check that `~/.bash_profile` loads `~/.bashrc`.

An existing Bench suite or standalone installation may already expose commands
with these names. The installer refuses to overwrite unrelated files. Choose
another prefix and put its `bin` first on PATH:

```sh
python3 scripts/install --prefix "$HOME/bench-local"
export PATH="$HOME/bench-local/bin:$PATH"
command -v ask
```

`command -v ask` shows which executable your shell will use. Keep companion
tools from the same intended installation together on PATH.

The equivalent Make commands are `make install`, or
`make install TOOLS="ask brief ply" PREFIX="$HOME/bench-local"`.

## Try without installing

```sh
python3 scripts/build ask brief ply
export PATH="$PWD/.build/bin:$PATH"
ask help
```

This PATH value contains the absolute checkout location when you run it, so
the commands still work after `cd`. Reapply it if you move the checkout.
Use `python3 scripts/build` for all tools. `make build` is equivalent.

## Know what a component includes

Selection is by component name. Selecting `mcp` builds `mcp`, `mcp-legacy`,
`mcpbox`, and `mcpserve`; selecting `ply` builds only `ply`, so also select
`ask` for its normal model path. `python3 scripts/build --list` shows the full
mapping. The [tool guide](TOOLS.md) explains runtime companions.

Programs, private assets, top-level docs, manuals, licenses, and source receipts
are copied under `PREFIX/lib/bench-tools/TOOL`. Relative links in `PREFIX/bin`
expose the commands. Installed copies work without the checkout and can move
with their prefix. Build output in `.build/` is disposable.

Source connectors and provider accounts require their own setup; installing
Context, for example, does not install a Wikipedia connector. Follow the
component README for those steps. [Ask's provider setup](../tools/ask/README.md#install)
covers model IDs and credentials.

## Draft and Cage setup

Draft needs its bundled skill on Brief's search path. For the default prefix,
the following preserves an existing custom path, or includes Brief's normal
search locations when no custom path is set:

```sh
export BRIEF_PATH="$HOME/.local/lib/bench-tools/draft/skills:${BRIEF_PATH:-.claude/skills:$HOME/.claude/skills:$HOME/.brief/skills}"
draft sync
```

Use your chosen prefix in place of `$HOME/.local`. Keep the export in your
shell startup file once. Run `draft sync` again after updating companions;
it refreshes Draft's generated command reference. `draft prove` also needs Perl.

Cage uses macOS Seatbelt or Linux Bubblewrap. Linux needs a trusted system
Bubblewrap installation and usable user namespaces. Run `cage check` on the
actual host before relying on confinement; an unsupported host refuses
execution. Cage limits writes and network access; host reads remain available.
On Ubuntu with restricted unprivileged user namespaces, an administrator may
need to allow the system Bubblewrap executable through an AppArmor profile;
see [Ubuntu's user-namespace guidance](https://documentation.ubuntu.com/release-notes/24.04/).
The installer does not change this host policy. Run `cage check` again after
the administrator configures it.

## Update or remove

With a clean checkout on `main`, pull the latest source and repeat the install
command with the same prefix and desired components:

```sh
git pull --ff-only
python3 scripts/install ask brief ply
```

For a pinned checkout, select the next reviewed commit instead of pulling.
Stop active uses of those programs before replacing them. The installer updates
its own unmodified files and refuses conflicts or unexpected edits; Draft's
generated reference is a declared exception. Install interacting companions
from the same selected checkout; selection does not automatically add dependencies.

You can also copy completed build packages without a compiler, then remove only
the tools owned by this installer:

```sh
python3 scripts/install ask brief ply --from-build .build --prefix "$HOME/bench-preview"
python3 scripts/uninstall --prefix "$HOME/bench-preview"
```

The first command uses the three-tool build above. Omit component names only
after building the whole toolkit. Add names to select a subset for removal.
Uninstall removes managed installation
files; separately created sessions and worker data remain yours to manage.

Component READMEs also document independent repository installation. Stay with
this shared installer when following the toolkit guides. The Bench application
and Hire maintain their own pinned suites; [source and release guidance](RELEASES.md)
explains how those distributions relate to this repository.
For package verification and installer details, see the [runner reference](../scripts/README.md).
