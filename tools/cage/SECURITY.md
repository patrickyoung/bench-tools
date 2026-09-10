# Security

Cage is a write and network boundary. It is not a confidentiality boundary.

## Enforced

On a supported, available backend, the child process tree may persist host
filesystem writes only beneath the canonical directories selected by policy
plus the process temporary directory. Basic virtual devices and inherited
standard streams remain usable. Host networking is denied unless `-net` is
present. A Linux child can retain loopback inside its isolated namespace but
cannot reach host loopback. Backend setup must prove readiness from inside the
boundary; otherwise Cage returns 125 without starting the requested command.
The readiness proof has a five-second deadline.

macOS uses Seatbelt through the fixed system executable
`/usr/bin/sandbox-exec`. Linux accepts Bubblewrap only from a short list of
absolute system paths, avoiding a counterfeit backend earlier on `PATH`.

`cage check` performs real positive and negative effects. A passing check is
the minimum evidence that the current host supports the contract.

## Not enforced

Cage does not restrict reads. A child can read any file its operating-system
identity can read, including source, environment-derived paths, credentials,
and personal files. Do not place secrets in that identity if the child should
not see them.

Cage also does not provide:

- user or privilege separation;
- syscall filtering beyond the backend's filesystem and network policy;
- CPU, memory, process, time, or output limits;
- network destination filtering when `-net` is enabled;
- human approval, credential scoping, or audit logging;
- a clean image or reproducible build environment.

Use an unprivileged account. Linux Cage explicitly drops all capabilities,
but never run untrusted work as root or with elevated macOS entitlements and
assume Cage removes every ambient power. Compose a container or virtual
machine when reads must be isolated.

## Writable roots

Every `-w` directory is a complete write grant beneath that canonical path.
Cage resolves symlinks before constructing its policy, but it cannot make
unsafe contents inside a granted tree safe. Unix sockets, device nodes,
compiler hooks, package-manager configuration, and executable search paths
inside a writable root may have effects outside the intended source edit.

Hard links are inode aliases, not redirects. If one name is inside a writable
root and another is outside, writing the inside name changes the outside file.
Cage cannot recover pathname separation for a link that already exists.
Reject writable roots containing regular files whose link count exceeds the
number of names contained in the admitted roots, or use a fresh copied
workspace. The `ply -cage` integration performs this scan before starting
model actions; raw `cage` callers own the same check.

`-ro` removes the default workspace grant; it does not make the temporary
directory read-only. `-net` removes the network boundary completely for the
child process tree.

The temporary directory is always writable. Set `TMPDIR` to a private,
purpose-specific directory before invoking Cage when sharing the system temp
tree would be too broad.

Standard output, standard error, and inherited standard input remain direct
channels to the caller. They are deliberately preserved. Cage does not redact
them, and a caller should avoid passing extra privileged file descriptors.

## Backend refusal

An unavailable or rejected backend is never downgraded. Cage emits a
`confinement setup` diagnostic and exits 125. On Windows and unsupported
platforms the current release always refuses and recommends Windows Sandbox
or a container with explicit mounts and networking policy.

## Reporting

Include `cage version`, `cage status`, operating-system and kernel versions,
and the complete output of `cage check`. Use a synthetic fixture and remove
credentials or personal paths before sharing a report.
