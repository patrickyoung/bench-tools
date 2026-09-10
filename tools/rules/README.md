# Rules

**Read the repository instructions that apply exactly where you are working.**

Rules gathers `AGENTS.md` and `CLAUDE.md` from a project's root down to a target
directory. It prints the complete instructions in a predictable order, so you
can inspect them yourself or explicitly give them to an agent.

```sh
rules                     # instructions for the current directory
rules -list ./internal    # applicable file paths for a subdirectory
```

No model, account, configuration file, or background service is needed.

## Install

From the [Bench tools monorepo](https://github.com/patrickyoung/bench-tools),
run `python3 scripts/install rules` at the repository root. See
[installation and updates](https://github.com/patrickyoung/bench-tools/blob/main/docs/INSTALL.md)
for prerequisites and PATH setup, or
[getting started](https://github.com/patrickyoung/bench-tools/blob/main/docs/GETTING-STARTED.md)
for a guided first result.

**Standalone install.** Requires **Go 1.26+**. Install current `main`:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/patrickyoung/rules@main
export PATH="$HOME/.local/bin:$PATH"
rules help
```

Keep the PATH setting in your shell startup file. Examples below use a Unix
shell and Git.

## See it work in a minute

Make a disposable repository with broad and local instructions:

```sh
mkdir rules-demo
cd rules-demo
git init -q
mkdir -p internal
printf '%s\n' 'Use clear names and explain behavior changes.' > AGENTS.md
printf '%s\n' 'Preserve the public API in this directory.' > internal/AGENTS.md

rules -list internal
rules internal
```

`-list` prints the two applicable instruction paths. The second command prints
both bodies, with the root instruction first. File provenance goes to stderr;
only the instructions go to stdout.

Now try `rules .`: only the root instruction applies. A directory with no
applicable instructions produces empty stdout and exits successfully.

## Give the same context to an agent

With [Ask](https://github.com/patrickyoung/bench-tools/tree/main/tools/ask) installed and configured:

```sh
system=$(ask system) &&
workspace=$(rules internal) &&
ASK_SYSTEM="$system
$workspace" ask 'Summarize the conventions I should follow here.'
```

For executable work, install [Ply](https://github.com/patrickyoung/bench-tools/tree/main/tools/ply) and run
this from the repository you want it to change:

```sh
system=$(ask system) &&
workspace=$(rules) &&
ASK_SYSTEM="$system
$workspace" ply -sh -check 'go test ./...' 'Fix the failing tests.'
```

Both reads are checked before the model starts. Use your project's actual
verifier instead of `go test ./...` when appropriate. Ply's `-sh` grants shell
execution; repository instructions are **guidance, not authorization**.

## What gets included

1. The root is the nearest ancestor containing a `.git` directory or regular
   `.git` file, including a Git worktree marker.
2. Rules walks from that root to the target directory.
3. At each level, it reads `AGENTS.md` before `CLAUDE.md`.
4. It validates the whole set before printing any instruction text.

Rules does not interpret or reconcile conflicting prose. Broader instructions
appear first, and narrower instructions follow them.

Symlinks may resolve only inside the canonical project root. If two paths
resolve to the same canonical file, its body appears once; `-list` still
shows both logical paths. Separate files with identical text remain separate.
The complete set of unique bodies is limited to **32 KiB**. Oversized files,
unsafe symlinks, and special files are refused without a partial prompt.

## Combine the right kinds of context

| Tool | Supplies |
| --- | --- |
| Rules | Repository conventions that apply at a directory |
| [Brief](https://github.com/patrickyoung/bench-tools/tree/main/tools/brief) | A reusable procedure selected for a task |
| [Context](https://github.com/patrickyoung/bench-tools/tree/main/tools/context) | Retrieved evidence, supplied as message data |
| [Ask](https://github.com/patrickyoung/bench-tools/tree/main/tools/ask) / [Ply](https://github.com/patrickyoung/bench-tools/tree/main/tools/ply) | Reasoning, or actions judged by an explicit check |

Rules connects to nothing automatically. You choose when its output is used,
and it never grants file access, network access, or approval.

## Troubleshooting and reference

| Status | Meaning and next step |
| --- | --- |
| 0 | Complete output, including a legitimately empty instruction set |
| 1 | No project root or refused instruction; check location, size, and symlinks |
| 2 | Invalid invocation, filesystem error, or output error |

```text
rules [-list] [directory]
rules help
rules version
```

See [GUIDE.md](GUIDE.md), [rules.1](rules.1), and [SECURITY.md](SECURITY.md).
For changes, read [AGENTS.md](AGENTS.md) and run `go test ./...`,
`go test -race ./...`, and `go vet ./...`. [MIT license](LICENSE).
