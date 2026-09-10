# Source-free Bench suite installation

This skill release targets Bench suite 0.13.0, published 2026-08-30. The release
contains one tested composition, licenses, suite manifest, internal file
checksums, installer, and public commands. Source checkout is not required.

Release page:

`https://github.com/patrickyoung/bench/releases/tag/v0.13.0`

## Guided setup path

Inspect `../scripts/install-bench-suite.sh` before using it. The intended flow is:

```text
install-bench-suite.sh plan --prefix /absolute/prefix --cache /absolute/cache
# present the emitted plan and obtain approval
install-bench-suite.sh install --approve --prefix /absolute/prefix --cache /absolute/cache
install-bench-suite.sh verify --prefix /absolute/prefix
install-bench-suite.sh cage-plan --prefix /absolute/prefix \
  --evidence-out /absolute/controller/platform/CAGE-CHECK.txt
# present the Cage plan and obtain approval on the target host
install-bench-suite.sh cage-check --approve --prefix /absolute/prefix \
  --evidence-out /absolute/controller/platform/CAGE-CHECK.txt
```

`plan`, `verify`, and `cage-plan` are read-only. The download/install and
transient Cage proof will not run without `--approve`; that switch records that
the exact emitted plan was approved, not blanket authority for future actions.
Use the installed absolute
`/absolute/prefix/bin/bench` path immediately; changing a shell profile is
optional and outside the helper.

The evidence parent must already exist, be owned by the current identity, and
sit outside every Agent home. The plan prints its absolute path and the exact
absolute host command. A successful check publishes a new mode-0600 record
without replacing an existing path. When a nested interface blocks the check,
copy that printed command into a terminal on the same target host; the saved
record is the explicit input for the operator's later same-host attestation.

## Archive selection

Observe the target using `uname -s` and `uname -m`. Map only:

| OS | Architecture | Archive | SHA-256 |
| --- | --- | --- | --- |
| Darwin | x86_64 | `bench-suite-0.13.0-darwin-amd64.tar.gz` | `d9f46e17bf8ed722ef2d635332e0513f6fc158a3bcff63f20ec8555b3da86b04` |
| Darwin | arm64 | `bench-suite-0.13.0-darwin-arm64.tar.gz` | `8c5b66085523410171ea13e5f813745292f88bc7513bca8d2769b9800c6c6ba6` |
| Linux | x86_64 | `bench-suite-0.13.0-linux-amd64.tar.gz` | `0fef42246115fc84cc14cef029f46f8347fb6f1737548eadf0da15b1ce9af9d4` |
| Linux | aarch64/arm64 | `bench-suite-0.13.0-linux-arm64.tar.gz` | `6621771d68d1c52dc7939b6db1d994de0a11d186cc2757a8b0edace3a6983965` |

If the observed platform is not in the table, stop. Do not select a “close”
archive or build from source without a separate approved engineering workflow.

## Explicit source build

Source building is available for contributors or an engineering-controlled
environment, but it is not the user setup fallback:

```text
install-bench-suite.sh source-plan --source /absolute/bench-checkout --prefix /absolute/prefix
# engineering review and approval
install-bench-suite.sh source-build --approve --source /absolute/bench-checkout --prefix /absolute/prefix
```

The helper requires the checkout root to be clean, exactly tagged `v0.13.0`,
and at pinned commit `c4e02f1cce7b57265493d95964534e7ae774ad44`, plus the
target's exact release toolchain (Go 1.26.5 on macOS arm64; Go 1.26.7 on macOS
amd64 and Linux), Git, tar, and network access. Bench's own source installer
fetches only pinned revisions, builds and checks the suite archive in a staging
prefix, and the helper admits it only when its release manifest is byte-for-byte
identical to the pinned prebuilt release. Failure in prebuilt download or
verification never selects this path automatically.

## Safe sequence

1. Resolve an explicit temporary download directory and install prefix.
2. Download only the selected archive. The versioned helper already contains
   its reviewed SHA-256 pin; it does not fetch a sidecar during installation.
3. Verify the archive against that embedded SHA-256 before extraction. A
   mismatch is terminal. Independently compare the pin with published release
   checksum material when organizational supply-chain policy requires it.
4. Extract into a new directory; inspect `INSTALL.md`, `suite.json`,
   `SHA256SUMS`, and licenses.
5. Run the archive's own checksum verification and installer only after the user
   approves the prefix.
6. Ensure the selected prefix's `bin` directory is reached deliberately; do not
   overwrite unrelated commands.
7. Verify versions, internal checksums, and Cage backend discovery. Record this
   as **SUITE-INSTALLED**, not **SUITE-READY**.
8. Review `cage-plan`, then run the approved `cage-check` on the target host.
   Only its literal 13/13 pass earns **SUITE-READY**. A nested sandbox failure
   requires the same command in a host terminal and is not by itself evidence
   that Cage is broken.
9. Record archive URL, hash, suite manifest, prefix, host, install owner, date,
   and rollback directory.

Never use `curl | sh`, unverified archives, mutable “latest” URLs, or a mix of
independent component releases.

## Pinned versions

| Component | Version | Public command(s) |
| --- | ---: | --- |
| Bench | 0.7.0 | `bench` |
| Ask | 0.2.0 | `ask` |
| Brief | 0.1.1 | `brief` |
| Ply | 0.1.2 | `ply` |
| Context | 0.1.0 | `context` |
| Action | 0.1.0 | `action` |
| Cite | 0.1.0 | `cite` |
| Cage | 0.1.0 | `cage` |
| May | 0.1.0 | `may` |
| Hone | 0.2.0 | `hone` |
| Trail | 0.1.0 | `trail` |
| Agent | 0.2.1 | `agent` |
| Tend | 0.1.2 | `tend` |
| Draft | 1.0.0 | `draft` |
| MCP | 0.3.0 | `mcp`, `mcpbox`, `mcpserve` |
| OAuth | 0.1.1 | `oauth` |

The emitted suite manifest and release checksums are authoritative. A moving
README or branch is not.

## Offline verification

- Every public command returns its exact expected `version`.
- `cage status` reports a supported confinement backend; this is discovery,
  not enforcement proof.
- The separately approved target-host `cage check` reports exactly 13/13
  passed before **SUITE-READY** is claimed.
- `may check`, `tend check`, and other controller checks use isolated test state,
  not production state.
- An extracted-suite smoke test reaches adjacent companions with a minimal PATH.
- A disposable Agent scaffold can run `agent check`, an accepted pre-check with
  zero model calls, and history verification.

Model provider credentials and production connections are not an installation
test.

Checksums prove that downloaded bytes match this release lock. They do not by
themselves prove publisher identity, notarization, or suitability for a
particular organization's supply-chain policy. Never bypass host security
controls merely to make setup continue.
