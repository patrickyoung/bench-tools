# Cage contributor contract

Cage establishes one kernel boundary around one child process. Keep that
sentence true.

## Invariant

If confinement is not established, the requested child does not start.
Standard streams and the child's result otherwise pass through.

## Rules

- Supported backends enforce filesystem writes and default network denial in
  the kernel. A warning, environment convention, or toolbox is not a backend.
- Never fall back from confinement to an ordinary process.
- Reads remain unrestricted and the documentation must say so plainly.
- `-net` grants networking; it never means "probably enough networking."
- `-w` grants canonical existing directories. Do not add a policy language.
- Backend executables come from trusted absolute paths, not the caller's
  `PATH`.
- Exit 125 is setup failure. Exit 2 is usage. Preserve child results.
- `status` reports availability; `check` proves allowed and denied effects.
- Platform refusal is a valid backend. Best effort presented as containment
  is not.
- No daemon, image, download, updater, config file, registry, telemetry,
  approval system, credential handling, or model call.

## Before shipping

Run on a supported native host:

```sh
go test ./...
go test -race ./...
go vet ./...
go build .
./cage check
```

Compile the other platform paths even when their kernels are unavailable:

```sh
GOOS=linux GOARCH=amd64 go build -o /tmp/cage-linux .
GOOS=windows GOARCH=amd64 go build -o /tmp/cage-windows.exe .
```

Render `cage.1`, keep built-in help within 80 columns, and do not weaken a
negative acceptance probe merely because an enclosing sandbox blocks its
positive counterpart.
