# Source and releases

[Home](../README.md) · [Install](INSTALL.md) · [Develop and verify](DEVELOPING.md)

**Use [patrickyoung/bench-tools](https://github.com/patrickyoung/bench-tools)
for the shared toolkit source and installer.** This repository maintains the
20 components together, so documentation, command compatibility fixes, and
integration checks can change in one commit.

## Choose an installation

| What you want | Installation route |
| --- | --- |
| Set up a harness to build workers and teams | Clone this repository and use `python3 scripts/setup`; it prefers source-matched published builder packages |
| Build scripts or applications with selected tools | Clone this repository and use `python3 scripts/install`, selecting the components you need |
| Work on a single independent program | Build its `tools/NAME` directory, or use the standalone repository instructions in its README |
| Use the Bench interactive application | Follow the [Bench application installer](https://github.com/patrickyoung/bench#install), which selects its own pinned suite |
| Build expert folders with headless Hire | Install `hire agent ask brief ply cage record` from this monorepo; see [Hire](../tools/hire/README.md) |
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

## Published tool packages

`releases/builder.json` pins GitHub release assets by SHA-256, size, source
revision, platform and the independent packages' source and receipt digests.
The archive includes every component declared in `components.json`: currently
20 independent packages and 24 public commands. The existing `builder.json`
and `bench-builder-PLATFORM.tar.gz` names are retained for compatibility;
their inventory is no longer limited to the nine default setup tools.
Automatic downloads are limited to Linux. Macs build from source until
Developer ID signing and notarization are configured; Mac CI packages remain
verification artifacts. Setup uses published packages only when the current
selected component sources match. Root-only
documentation or skill changes may therefore reuse a tested package without
pretending its build came from the later commit. Changed component source uses
the ordinary source build. `scripts/install` builds from source by default;
`--from-release` installs selected pinned Linux packages without compilation,
and `--from-build` installs caller-selected local packages. Individual tools
keep their own versions. For example, `python3 scripts/install oauth --from-release`
installs only OAuth; omitting component names installs all published components.

The [Builder packages workflow](../.github/workflows/packages.yml) builds and
tests native amd64/arm64 packages on Linux and macOS. It derives the complete
inventory from `components.json`, verifies every installed command, relocation,
Draft assets, repeat installation and removal, then checks the default setup,
Hire's structural check and Cage before retaining an artifact. The
Linux packages use `scripts/build --no-cgo` to avoid a distribution-specific
glibc dependency; that compiler selection is retained in the package receipt.
The archive contains each selected tool's unchanged package and receipt, plus
`runtime.json` describing the transport bundle. It is not a shared executable,
daemon, state directory or new component version.

The workflow also creates a signed [GitHub build attestation](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations/use-artifact-attestations)
for the archive and retains its Sigstore bundle as `FILE.tar.gz.sigstore.json`.
Before uploading artifacts, it verifies the archive digest, this repository,
the exact package workflow, source ref and source/workflow commit, and GitHub's
hosted runner identity. GitHub issues the signing identity from the workflow's
short-lived OIDC token; this needs no maintainer signing key. This establishes
build provenance. It does not provide Apple Developer ID signing or notarization.

To refresh the published packages:

1. Select a clean, checked source commit and run the Builder packages workflow
   for that exact ref. Require its four native jobs and the ordinary component
   and integration checks to pass.
2. Download its four `builder-PLATFORM` artifacts. Verify each adjacent JSON
   file's source revision, archive size and SHA-256, and verify each archive's
   signed provenance as shown below. Publish the Linux archives and their
   JSON/checksum/Sigstore files as a new GitHub release tagged `builder-COMMIT`,
   targeting that full commit. Do not replace assets of an existing pin.
3. Update `releases/builder.json` by copying each Linux archive's complete
   adjacent JSON record into `artifacts[PLATFORM]`, adding its exact release
   asset `url`. Retain all identity, source and checksum fields. Commit the pin
   so it is reviewed alongside installer source.
4. Exercise `scripts/setup` from the published URL on the target environments
   before claiming those routes verified.

`python3 scripts/package-runtime --from-build DIR --output FILE.tar.gz` is the
source packaging helper used by the workflow. It checks package source and
native executable identity and writes deterministic archives and metadata.
Checksums pinned in Git detect changed download bytes; these are not separate
release signatures. The publication gate verifies the signed provenance;
installation verifies the reviewed archive pin and component receipts.
System Bubblewrap remains a Linux host prerequisite.

To verify a downloaded release before publication, use a current GitHub CLI
with `COMMIT` set to the reviewed full build commit, `REF` to the exact source
ref used by that run (such as `refs/heads/main`), and `ARCHIVE` to its local path:

```sh
gh attestation verify "$ARCHIVE" \
  --bundle "$ARCHIVE.sigstore.json" \
  --repo patrickyoung/bench-tools \
  --cert-identity "https://github.com/patrickyoung/bench-tools/.github/workflows/packages.yml@$REF" \
  --source-digest "$COMMIT" \
  --source-ref "$REF" \
  --signer-digest "$COMMIT" \
  --predicate-type https://slsa.dev/provenance/v1 \
  --deny-self-hosted-runners
```

The local bundle avoids fetching the attestation from the repository API.
Verification still needs current trusted roots; do not describe this command
as fully offline. See the [GitHub CLI verification policy](https://cli.github.com/manual/gh_attestation_verify).

## Publication boundaries

This public repository provides source installation and pinned builder packages.
Publishing it does not
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
