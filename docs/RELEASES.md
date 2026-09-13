# Source and releases

[Home](../README.md) · [Install](INSTALL.md) · [Develop and verify](DEVELOPING.md)

**Use [patrickyoung/bench-tools](https://github.com/patrickyoung/bench-tools)
for the shared toolkit source and installer.** This repository maintains the
19 components together, so documentation, command compatibility fixes, and
integration checks can change in one commit.

## Choose an installation

| What you want | Installation route |
| --- | --- |
| Build scripts, workers, or applications with these tools | Clone this repository and use `python3 scripts/install`, selecting the components you need |
| Work on a single independent program | Build its `tools/NAME` directory, or use the standalone repository instructions in its README |
| Use the Bench interactive application | Follow the [Bench application installer](https://github.com/patrickyoung/bench#install), which selects its own pinned suite |
| Build expert folders with headless Hire | Install `hire agent ask brief ply cage` from this monorepo; see [Hire](../tools/hire/README.md) |
| Maintain an existing Hire web installation | Keep its separately pinned suite until its call sites are migrated; see [the split boundary](../tools/agent/RUNNER.md#adoption-and-release-boundary) |

The shared installer builds all selected tools from one checkout. Each keeps
its public command name, module path, version, license, and runtime contract.
There is no common runtime, automatic companion installation, or shared state
directory. For example, select both `ask` and `ply` for Ply's normal model path.

Keep companions from the intended installation together on PATH. A tool from
another installation can have a different protocol even if its name matches.
Use `command -v ask` and `command -v ply` to see what your shell selects.

## Pin the source you use

`main` is the development branch. Review the [CI result for the exact commit](https://github.com/patrickyoung/bench-tools/actions/workflows/check.yml)
you intend to use. Record its full identity with:

```sh
git rev-parse HEAD
```

To reproduce that source, clone the repository into a fresh directory, then
replace `COMMIT` with the recorded full commit ID:

```sh
git checkout --detach COMMIT
python3 scripts/install ask brief ply --prefix "$HOME/bench-pinned"
export PATH="$HOME/bench-pinned/bin:$PATH"
```

Build from a clean checkout when the commit is your reproducibility reference.
The builder deliberately includes current working-tree edits; each package's
`package.json` records the actual source inventory, source digest, compiler,
and build flags. Installed receipts live under `PREFIX/lib/bench-tools/NAME`.
They establish local build provenance; they are not release signatures.

The source commit identifies the toolkit snapshot. Component `version` output
identifies that program's own version, which may remain unchanged across
documentation or packaging commits. `components.json` retains the original
import commits and trees for historical provenance; it is not a current
dependency lock or a toolkit version number.

## Publication boundaries

This public repository provides source installation. Publishing it does not
replace the Bench or legacy Hire application releases, change their pins, or redirect
existing `go install github.com/patrickyoung/TOOL@...` module paths. Those routes
continue to use their independent repositories. Changes here are not
automatically mirrored back to them.

A future application release can select this source by repository, commit,
component subdirectory, and tree digest after its packager supports those
identities and tests the resulting suite. A future standalone release can use
an explicit component export while retaining its module path and version.
Neither publication route should infer compatibility from directory proximity.

For changes in this toolkit, run the [full checks](DEVELOPING.md), installation
smoke, and runnable starter checks before pushing. CI repeats those checks on
Linux and macOS. Paid model evaluations remain separate: fixture success
establishes command compatibility, not model quality.
