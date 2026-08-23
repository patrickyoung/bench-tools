# Using Cage

Cage is the boundary around one process tree. Put it around the ordinary
program whose effects you want to constrain:

```sh
cage -- sh -c 'make test && make package'
```

Everything the child starts inherits the same kernel policy. A toolbox limits
which program names are convenient; Cage limits filesystem writes and network
operations even when a shell builtin or redirection performs them.

For Ply, choose the process tree deliberately. `cage -- ply ...` confines the
whole model client, including Ask, and normally denies the network connection
the model needs. `ply -cage` instead leaves Ply, Ask, May, Brief, and the
verifier outside while wrapping each exact model-authored action after May
approval. That action-only composition is usually the useful agent boundary.

## Choose writes explicitly

The common case needs no write flags:

```sh
cd project
cage -- go test ./...
```

The current directory and temporary directory are writable. Other paths are
readable but not writable.

For an inspection job, make the workspace read-only:

```sh
cage -ro -- sh -c 'find . -type f -maxdepth 2 -print'
```

For a build with distinct source and output roots:

```sh
cage -w ./build -w ./generated -- generator ./schema
```

Once any `-w` is present, the current directory is not implicitly writable.
Every named directory must already exist. Cage resolves symlinks before
building the policy, so the kernel receives canonical roots.

Canonical path policy cannot distinguish two hard-link names for one inode. A
pre-existing hard link inside a writable root can therefore modify the same
file through an external name. Reject such roots or verify that every link to
each writable regular inode is contained inside the admitted roots. Ply's
`-cage` integration performs that conservative scan before work.

The temporary directory always remains writable. Set `TMPDIR` before Cage if
a job needs a specific scratch root:

```sh
mkdir -p .scratch
TMPDIR="$PWD/.scratch" cage -ro -- compiler source
```

## Grant the network deliberately

The default is no access to the host or public network:

```sh
cage -- curl https://example.com
```

Use `-net` when the command genuinely needs all host networking:

```sh
cage -net -- curl https://example.com
```

On Linux the child receives a separate network namespace, which can have its
own isolated loopback; it cannot reach a listener on the host's loopback.

`-net` is not a host allowlist. If a job needs narrower routing, compose Cage
with a network namespace, proxy, firewall, or container that owns that job.

## Distinguish failures

```sh
if cage -- make check; then
    echo passed
else
    status=$?
    case $status in
    125) echo 'Cage could not establish confinement' >&2 ;;
    *)   echo "the child returned $status" >&2 ;;
    esac
fi
```

Cage waits for a launcher already inside the kernel boundary before treating
any result as the child's. If the backend executable is missing, rejects its
policy, or exits before that proof, Cage reports setup failure on standard
error and returns 125. It never retries without confinement.

The readiness proof has a five-second setup deadline. Cage does not impose a
runtime deadline after the child starts; compose a timeout tool separately.

A child is technically free to return 125 itself. In that case its own output
appears without Cage's `confinement setup` diagnostic. Unix has only 256 exit
statuses; preserving every child status and reserving a mathematically unique
one are incompatible.

## Inspect and check

```sh
cage status | jq .
cage check
```

`status` is discovery, not proof: it says which backend file is installed and
which classes it enforces. `check` performs real allowed and denied effects.
Run `cage check` on every target operating system and after security or kernel
updates.

An enclosing sandbox may legitimately prevent Cage from creating a nested
boundary. In that case Cage must fail closed with 125. Run the acceptance
check directly on the host when the outer sandbox does not support nesting.

## What Cage does not contain

Cage intentionally permits reads. It does not hide SSH keys, environment
variables, browser profiles, source code, or other readable secrets. It also
does not limit CPU, memory, process count, elapsed time, or disk consumption
inside an allowed write root.

Compose separate locks for separate questions:

```text
toolbox     which executable names are available
Cage       where the process can write and whether it can network
approval    whether exact consequential words may proceed
identity    which credential or account the operation receives
bound       how long and how much output the operation may consume
```

No one of those honestly substitutes for another.
