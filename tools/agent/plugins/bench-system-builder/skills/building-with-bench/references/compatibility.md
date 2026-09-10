# Compatibility snapshot

This reference is intentionally versioned. Read it before invoking Bench
commands. Do not silently combine it with a different suite.

## Skill release

- Skill: `building-with-bench` 0.2.0
- Reviewed: 2026-09-01
- Bench suite: 0.13.0
- Agent: 0.2.1
- Supported suite hosts: macOS and Linux, `amd64` and `arm64`

The compatible suite is a checksummed binary distribution. A user does not need
to clone or read any source repository. The official release is:

`https://github.com/patrickyoung/bench/releases/tag/v0.13.0`

The steward helper contains an immutable archive name and reviewed SHA-256 for
each supported platform. It downloads only the selected archive, then verifies
those bytes against the embedded pin before extraction. It does not fetch a
checksum sidecar during installation. An organization may independently compare
the embedded pin with the release's published checksum material under its own
supply-chain policy.

## Read-only suite doctor

Inspect `../scripts/doctor.sh`, then use it with an explicit install prefix,
run-in-place suite directory, Bench executable, or PATH selection. It resolves
the physical suite root, anchors its installed checksum manifest to one of the
four reviewed release manifests, verifies every covered file, and calls all
eighteen commands from that one root. With `--prefix`, it also proves all
eighteen public entries resolve back to that exact suite. Matching version text
from independently installed binaries is not a compatible suite.

Missing commands are observations, not permission to install:

```text
bench version
draft version
agent version
brief version
ply version
ask version
```

Also report the execution operating system and architecture. Do not print the
environment, tokens, cookies, headers, or credential values.

Classify:

- Exact suite and Agent versions: compatible.
- Missing runtime on a supported local Claude Code host: invoke the steward's
  read-only install plan and ask for the exact setup decision; remain
  design-only until that plan is approved and verification passes. Verification
  earns **SUITE-INSTALLED**. The separately approved target-host `cage check`
  must pass 13/13 to earn **SUITE-READY**; then test provider readiness
  separately before model-backed Bench or Draft work.
- Newer or older runtime: do not use the versioned command runbook. Read the
  installed public `help` output, ask the user whether to update this skill or
  use an explicitly compatible suite, and stop before build work.
- Independently installed commands with mismatched versions: incompatible until
  a steward replaces them with one pinned suite or proves the composition.

## Release boundary

The suite contains the fourteen-component core and four edge commands:

`bench`, `ask`, `brief`, `ply`, `context`, `action`, `cite`, `hone`, `trail`,
`agent`, `tend`, `draft`, `may`, `cage`, `mcp`, `mcpbox`, `mcpserve`, and
`oauth`.

The release manifest and checksums are authoritative. Prose from a moving branch
is not a compatibility contract.
