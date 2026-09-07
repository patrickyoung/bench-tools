# Releasing Weave

The v0.1 product is the Unix filter. The Python business and research programs
ship as experimental recipes. Publishing an archive does not promote the
synthetic examples into validated business methods.

## Validate a candidate

Go 1.26 or later, Python 3.9 or later, Git, and a Unix environment are required
for all developer checks. The installed filter has no runtime dependencies.

```sh
make check
make bootstrap
make check-examples
make install DESTDIR="$PWD/var/install-check" PREFIX=/usr/local
python3 tests/smoke "$PWD/var/install-check/usr/local/bin/weave"
```

`make bootstrap` fetches the exact public Git revisions in
[dependencies.json](scripts/dependencies.json) and builds them in temporary
directories. It does not use sibling repositories, configure credentials or
run models. All inference in `make check-examples` comes from local HTTP test
fixtures. For explicit sibling-development checks, use:

```sh
./tests/check --siblings --bin "$PWD/var/sibling-tools"
```

Use separate binary and run directories when changing versions. A study binds
the exact tools and sources used; replacing those files intentionally prevents
resuming it. Archive the original tools, sources and evidence together.
The pinned public Ply is sufficient for the restricted report-only examples;
it does not include the separate local SIGTERM patch developed alongside the
prototype, and no general arbitrary-action cancellation guarantee is added here.

## Build the deliverables

```sh
make release
```

This builds four binary/source bundles and one source-only archive under
`dist/`, plus `SHA256SUMS` and `build.json`. It does not tag or publish anything.
Each bundle contains the binary, man page, license, source, tests and examples.
Install its filter with `make install PREFIX="$HOME/.local"` or copy `weave`
and `weave.1` to the desired bin/man locations. `go install .` is also supported
from a source checkout or extracted source archive.

Only paths listed in [release-files.txt](scripts/release-files.txt) are packaged.
Review changes to this list. Private run directories, credentials, local auth
wrappers, transcripts, dependency builds and git state are excluded. Binaries
are built from that exact captured source snapshot, with normalized Go settings,
CGO disabled and local build paths removed. Archive timestamps and ownership
are normalized. The same source and Go toolchain reproduce the same archives;
a different Go version can produce different binaries.

```sh
python3 scripts/release.py --output var/release-repeat
diff dist/SHA256SUMS var/release-repeat/SHA256SUMS
```

The builder refuses to overwrite different artifacts. Choose a fresh output
directory after a code, documentation or toolchain change. Checksums detect
changed downloads; they are not signatures or external provenance attestations.

## Publish after review

Check the extracted artifacts and native installed-binary smoke test. The CI
workflow runs the offline suites on Linux and macOS and builds downloadable
candidate artifacts; it has no release publishing permission. Cross-compiling
an architecture alone is not a native runtime test.

Set the module repository/remote, review the initial source commit, then tag
that reviewed commit `v0.1.0` and attach the exact checked `dist/` files to its
release. Confirm the remote module install only after the repository and tag
exist:

```sh
go install github.com/patrickyoung/weave@v0.1.0
weave version
```

Do not describe this command as available before publication. The module path
reserves the intended repository name; it is not evidence that it is published.
