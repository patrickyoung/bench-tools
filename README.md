# rules

`rules` prints the bounded repository instructions that apply at a directory.
It makes workspace context available to agents without hiding prompt input
inside an agent runner.

```sh
rules
rules -list ./internal/auth
ASK_SYSTEM="$(ask system; rules)" ply -sh -check 'go test ./...' 'fix it'
```

Instruction text is stdout. Root and file provenance are stderr. `-list`
replaces the text with one logical instruction path per line, while performing
the same validation as a normal read.

## The rule

The project root is the nearest ancestor containing either a `.git` directory
or a regular `.git` file, as used by a Git worktree. No language manifest,
configuration file, or environment variable changes that decision.

Starting at the root, `rules` walks down to the target directory. At every
level it reads, in this order:

1. `AGENTS.md`
2. `CLAUDE.md`

Broader instructions therefore appear before narrower ones. Both names apply
when both exist. If multiple applicable paths have the same fully resolved
path, `rules` emits that file body only at its first position. `-list` still
prints every logical path, and stderr identifies later paths as aliases. Two
distinct files remain distinct even when their contents match. `rules` does
not interpret, merge, or override their text.

The unique applicable file bodies may contain at most 32 KiB in total, so one
file is necessarily subject to the same bound. The byte count is compiled
into the program: there is no config file and no environment override. A
normal read copies file bodies in order and inserts one newline only when
adjacent bodies would otherwise share a line.

The whole set is resolved, read, and checked before stdout is written. An
oversized file, oversized set, special file, or unsafe symlink therefore
produces no partial prompt. Empty instruction sets are successful and produce
empty stdout.

Instruction symlinks are followed only when the fully resolved target remains
inside the canonical project root. The logical path appears under `-list`;
stderr shows both the logical and resolved paths. This lets a repository use
the common compatibility link `CLAUDE.md -> AGENTS.md` without spending prompt
space on the same instructions twice.

## Install

`rules` requires Go 1.26 or newer.

```sh
go install github.com/patrickyoung/rules@latest
```

Or build the local checkout:

```sh
go build -o rules .
```

## Exit status

| status | meaning |
| --- | --- |
| 0 | the complete set was printed, including an empty set |
| 1 | no project root was found, or an instruction was refused |
| 2 | invalid invocation, filesystem error, or output error |

`rules` is a reader only. It has no model, cache, watcher, prompt template,
configuration file, or automatic connection to `ask` or `ply`.

Repository instructions are guidance, not authorization. Putting their text
in any prompt role does not grant permission to expose secrets, widen network
access, approve actions, or escape the agent's execution boundary. Those
decisions belong to the runner and operator.

For an unattended script, check both prompt-producing commands before starting
the agent. A shell does not propagate a failed command substitution through an
environment assignment that prefixes another command:

```sh
system=$(ask system) &&
workspace=$(rules) &&
ASK_SYSTEM="$system
$workspace" ply -sh -check 'go test ./...' 'fix it'
```

See [GUIDE.md](GUIDE.md) for examples and [SECURITY.md](SECURITY.md) for the
trust boundary.
