# Install Bench tools

[Start here](GETTING-STARTED.md) · [Tool guide](TOOLS.md) · [Home](../README.md)

For a harness to set itself up, use [the harness entry point](../START-HERE.md).
It includes each host's skill installation and discovery instructions, followed
by this same installer and verification.

For the standard worker-building setup, once the checkout below is available:

```sh
python3 scripts/setup
. "$HOME/.local/share/bench/env.sh"
```

This installs the builder commands into `~/.local/share/bench/runtime`, verifies
versions, structure and native confinement, and writes
`~/.local/share/bench/BENCH-SETUP.md` for future sessions. It calls no model and
changes no shell or host settings. Use `--prefix` and `--state-dir` for selected
locations. A nonzero result means setup is incomplete; inspect the recorded
failure before running workers. Keep personal notes outside generated files.
The component installer below remains available for a smaller selection.

Setup needs Python 3.9+ and Git. Mac installs build from source with Go 1.26+;
automatic Mac binary distribution is deferred until Developer ID signing is
available. Linux uses prebuilt packages on amd64 or arm64, pinned by
`releases/builder.json`. The download checksum and
every component's current source digest must match before installation. These
packages retain the independent programs, licenses and original build receipts;
the normal installer verifies and owns the installed files. Linux still needs
system Bubblewrap for Cage. Use `--from-source` to compile locally, or
`--from-build DIR` for your own previously built packages. A host without a
matching published package falls back to a source build; a failed download or
integrity check stops with the exact error.

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

For building and running reusable experts:

```sh
python3 scripts/install hire agent ask brief ply cage record
cage check
```

Hire builds the definition; Agent runs it with those companions. Add `cite`
for the [support-reply starter](../examples/support-reply/README.md). Python
runs the source installer, not Hire or Agent: both commands are native Go.

For browser work, install `web` and separately install Chrome/Chromium.
`web setup` checks browser discovery; `WEB_BROWSER` selects its executable.
Web is native Go and needs no Python/Node browser runtime. See
[Web setup and commands](../tools/web/README.md).

Without names, the source installer builds and installs all 28 commands:

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

### Install published Linux packages without Go

The pinned release includes 20 components and their 24 public commands,
including OAuth, MCP, A2A and Draft. Newer tools such as Moniker require a source
build (`python3 scripts/install moniker`). For release installation, select
components whose source still matches that release:

```sh
python3 scripts/install oauth --from-release --prefix "$HOME/.local/share/bench/runtime"
python3 scripts/install oauth mcp a2a --from-release --prefix "$HOME/bench-edges"
```

`--from-release` downloads the pinned archive, verifies its contents and source
identity, and installs only the selected components. It needs Python and Git,
but no Go compiler. It fails explicitly if the selected source has changed,
no matching Linux package exists, or verification fails; it never switches to
compilation. Omit the flag for a source build. Macs retain source builds.
The default `scripts/setup` still selects its nine builder tools from this
complete release; package availability does not require installing every tool.

### Optional typed judgments

Weigh is a new optional component and is not part of the pinned 20-component
Linux release above or the default nine-tool setup. Build it from the selected
source checkout when a checker explicitly needs it:

```sh
python3 scripts/install weigh
```

Installing it makes no model call. Using its OpenRouter Decisions backend needs
separately supplied OpenRouter access and an explicit model. Existing checks can
continue to use Ask or deterministic programs. The [Weigh guide](../tools/weigh/README.md)
and [checker example](../tools/weigh/examples/semantic-check/README.md) document
backend selection; missing access never means that a check passed.

## Deploy only the runtime

An already built worker needs no Hire, Draft or Hone to run. Install Agent's
runtime companions into a fresh prefix:

```sh
python3 scripts/install agent ask brief ply cage record --prefix /absolute/runtime
```

Export the worker or assembled team on the authoring machine with
`scripts/workers`, then transfer that definition and the installed prefix to
a compatible host (same OS and architecture). The installed programs run
without the source checkout, Go compiler, exporter or authoring tools.
Alternatively transfer selected build packages and use the installer's
`--from-build` option on the target; that installation step needs Python.

Agent loads skills through Brief, runs the action/check loop through Ply,
calls models through Ask, and retains execution evidence through Record.
Cage supplies the default action boundary; Linux also needs Bubblewrap.
Ply remains necessary when a worker produces documents or edits job files.
Keep definitions separate from writable work, state and evidence, and
configure the model on the host.

Teams retain their existing entry commands and dependencies. Add Tend and
Weave where used; the page team also needs its separately pinned Bench Manage
and browser setup. Specialty tools, Python/Node packages, connectors and
credentials still come from the selected worker/team README. Catalog
`requires` lists currently include authoring/setup tools as well as runtime
requirements; they are not an automatic minimal dependency resolver.
External effect controllers need their own Action/May setup where applicable.

The [Omnigent deployment](../examples/omnigent/README.md) follows the same split:
worker and team images omit Hire; only an explicitly packaged builder includes
it. Its enclosing Docker boundary replaces Cage for those jobs.

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
mapping. Selecting `a2a` builds `a2a` and `a2aserve`; serving also requires
Tend. Hire builds expert definitions and Agent runs them. The
[tool guide](TOOLS.md) explains runtime companions.

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
and the legacy Hire web application maintain their own pinned suites; [source and release guidance](RELEASES.md)
explains how those distributions relate to this repository.
For package verification and installer details, see the [runner reference](../scripts/README.md).
