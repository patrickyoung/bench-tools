# Cage

**Run one command with explicit write access and networking off by default.**

Cage uses the host operating system to constrain a child process. Give it a
workspace, run the command, and keep the same stdin, stdout, stderr, and child
exit status. There is no image to build or daemon to start.

```sh
cage -- your-command
```

By default, the current directory and process temporary directory are writable.
Host and public networks are denied. **Host filesystem reads remain
unrestricted**: Cage is a write/network boundary, not a place to hide secrets.

## Install and check your host

From the [Bench tools monorepo](https://github.com/patrickyoung/bench-tools),
run `python3 scripts/install cage` at the repository root. See
[installation and updates](https://github.com/patrickyoung/bench-tools/blob/main/docs/INSTALL.md)
for prerequisites and PATH setup, or
[getting started](https://github.com/patrickyoung/bench-tools/blob/main/docs/GETTING-STARTED.md)
for a guided first result.

**Standalone install.** Requires **Go 1.26+** to build. Install current
`main`:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/patrickyoung/cage@main
export PATH="$HOME/.local/bin:$PATH"
cage status
cage check
```

Keep the PATH setting in your shell startup file.

| Host | Backend |
| --- | --- |
| macOS | System Seatbelt via `/usr/bin/sandbox-exec` |
| Linux | Bubblewrap at a trusted system path; install it with your OS package manager |
| Other platforms | Refuses execution when no complete backend is available |

`status` prints JSON even if the backend is unavailable. **Use `cage check` to
prove the boundary works on this machine.** Linux configurations that prohibit
the namespaces Bubblewrap needs will refuse setup. Cage never falls back to
an ordinary unconstrained process.

## See what is writable

Start in a fresh directory:

```sh
mkdir cage-demo
cd cage-demo
cage -- /bin/sh -c 'printf "hello\n" > hello.txt'
cat hello.txt
```

You should see `hello`. Now remove workspace write permission:

```sh
cage -ro -- /bin/sh -c 'printf "blocked\n" > blocked.txt'
```

That write should fail. `-ro` still allows the process temporary directory,
because ordinary programs often need temporary files.

Grant just a particular output directory:

```sh
mkdir output
cage -w output -- /bin/sh -c 'printf "allowed\n" > output/result.txt'
cat output/result.txt
```

An explicit `-w` replaces the default workspace grant; repeat it for multiple
existing directories. Writable roots are resolved to canonical paths.

## Choose a boundary for the task

| Invocation | Workspace writes | Temporary writes | Network |
| --- | --- | --- | --- |
| `cage -- COMMAND` | Current directory | Allowed | Denied |
| `cage -ro -- COMMAND` | Denied | Allowed | Denied |
| `cage -w DIR -- COMMAND` | Named roots only | Allowed | Denied |
| `cage -net -- COMMAND` | Current directory | Allowed | Allowed |

For example, `cage -net -- curl https://example.com` deliberately permits
networking. `-net` is a full network grant, not a list of allowed services.
Use `--` to separate Cage's options from the command's arguments.

Standard streams pass through unchanged, so Cage can sit inside a pipeline or
in front of a test command. The backend must establish confinement within five
seconds; that is a setup deadline, not a time limit on the child.

## Use it in an agent system

[Ply](https://github.com/patrickyoung/bench-tools/tree/main/tools/ply) can combine `-cage` with
[May](https://github.com/patrickyoung/bench-tools/tree/main/tools/may) approval around each model-authored
action. Ask and the verifier remain outside the Cage action child, allowing
the model connection to work while action networking is denied.

[Agent](https://github.com/patrickyoung/bench-tools/tree/main/tools/agent) uses Cage by default for its
worker actions, granting its work and state directories. [Draft](https://github.com/patrickyoung/bench-tools/tree/main/tools/draft)
can freeze an admitted check outside the builder's write roots.

Keep controller records, approval state, and trusted check definitions outside
writable roots. If a verifier executes worker-modified code outside Cage, that
code has the verifier's authority. Reads, resource consumption, and other host
capabilities also require separate consideration; see [SECURITY.md](SECURITY.md).

## What the check proves

`cage check` uses temporary fixtures and a loopback listener to exercise
allowed writes, denied writes, `-ro`, explicit roots, standard streams, child
exit/signal propagation, default network denial, deliberate `-net`, and refusal
when a backend is unavailable. It makes no model call and uses no public API.

## Outcomes and reference

| Status | Meaning |
| --- | --- |
| Child's status | The child ran; its result passes through |
| 125 | Cage setup failed; the requested child was not started |
| 2 | Cage usage error |
| 1 from `cage check` | An acceptance probe failed |

A child can choose 125 itself; stderr identifies Cage's own setup failure.
Wrappers may conservatively reserve this status for an uncertain boundary
outcome. Check their contract before retrying.

```text
cage [-net] [-ro | -w DIR ...] -- command [args...]
cage status
cage check
cage help
cage version
```

See [GUIDE.md](GUIDE.md) and [cage.1](cage.1). Contributors: read
[AGENTS.md](AGENTS.md), run the Go tests, race tests, and vet, build, and run
`cage check` on a supported host. Cross-build the other platform paths as
listed there. [MIT license](LICENSE).
