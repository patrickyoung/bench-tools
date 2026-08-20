# Cage

Cage runs one command inside the host operating system's filesystem and
network boundary:

```sh
cage -- go test ./...
cage -net -- web get https://example.com
cage -ro -- untrusted-inspector source.tar
cage -w . -w ../fixtures -- make check
```

The child may read the host filesystem. By default it may write the current
directory and the process temporary directory, and it cannot reach host or
public networks.
`-ro` removes the current-directory write. Repeated `-w DIR` flags replace the
default current directory with the named writable roots. The temporary
directory remains writable because ordinary compilers and Unix programs need
it.

Cage is containment, not a container runtime. It has no daemon, image,
configuration file, policy language, or hidden state.

## Install

Cage requires Go 1.26 or later to build and has no external Go dependencies.

```sh
go install github.com/patrickyoung/cage@latest
```

From this tree:

```sh
go build .
./cage check
```

## Interface

```text
cage [-net] [-ro | -w DIR ...] -- command [args...]
cage check
cage status
```

`--` is recommended and is required when the command begins with `-`. For
compatibility, the first non-option also starts the command.

Standard input, standard output, and standard error pass through unchanged.
An ordinary child exit status passes through unchanged. If a backend reports
signal termination as `128+signal`, Cage preserves that convention; on macOS,
for example, SIGTERM is status 143.

Exit status 125 means Cage could not establish confinement and did not start
the command. Exit status 2 is Cage usage. Other statuses belong to the child,
except status 1 from a failed `cage check`. Like every finite exit-status
space, a child can itself choose 125; the setup diagnostic on standard error
identifies Cage's own failure.

The backend has five seconds to prove that confinement is active. This is a
setup deadline, not a child timeout; after readiness the command may run as
long as it normally would.

`cage status` writes one JSON record describing the selected backend,
availability, and enforcement completeness. It exits successfully even when
the backend is unavailable, so scripts can inspect the record. Running a
command still fails closed with status 125.

## Backends

### macOS

Cage uses the system Seatbelt boundary through
`/usr/bin/sandbox-exec`. It generates one temporary profile per invocation,
waits for a launcher inside that profile to prove setup completed, and then
reports the child's result. A missing or rejected backend never falls through
to an unconstrained command.

### Linux

Cage uses Bubblewrap from a trusted system path. It mounts `/` read-only,
binds the selected write roots back read-write, uses a private `/dev`, drops
capabilities, disables further user namespaces, and creates a network
namespace unless `-net` was deliberate. Systems that disable the user or mount
namespaces Bubblewrap needs will reject setup with status 125.

Install Bubblewrap through the operating system package manager. Cage does
not download a backend or silently substitute a weaker mechanism.

### Windows and other platforms

Cage reports that no complete backend is available and refuses to run the
child with status 125. Use Windows Sandbox or a container with networking
disabled and explicit read-only and writable mounts. Cage will not label a
job-object-only or best-effort wrapper as filesystem confinement.

## Prove the boundary

Run:

```sh
cage check
```

The check creates an isolated temporary fixture and tests the installed
binary from the outside. It proves:

- default workspace writes work;
- the temporary directory remains writable under `-ro`;
- writes outside the workspace fail;
- `-ro` denies workspace writes;
- `-w DIR` grants an explicit root and replaces the workspace default;
- standard input, output, and error survive unchanged;
- child exit and signal results propagate;
- networking fails by default and succeeds with `-net`; and
- an unavailable backend fails closed without starting the child.

The network test uses a loopback listener, not the public internet. The check
does not require credentials, a model, a daemon, or network access.

See [GUIDE.md](GUIDE.md) for operational examples, [cage.1](cage.1) for the
manual, and [SECURITY.md](SECURITY.md) before treating Cage as a security
boundary.

## License

MIT. See [LICENSE](LICENSE).
