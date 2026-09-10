# Rules guide

## See what applies

From anywhere inside a Git repository:

```sh
rules -list
```

The nearest regular `.git` file or `.git` directory fixes the project root.
Rules then considers `AGENTS.md` followed by `CLAUDE.md` at the root, and does
the same in every directory on the path to the current directory. Files in
sibling directories do not apply.

Pass a directory to answer the same question elsewhere without changing the
shell's working directory:

```sh
rules -list ./cmd/server
rules ./cmd/server
```

Relative paths are interpreted from the current directory. The target is
canonicalized before root discovery, so entering through a directory symlink
does not create an alternate instruction hierarchy.

## Compose a prompt explicitly

`rules` does not install its output anywhere. Use the shell at the point where
the composition should be visible:

```sh
ASK_SYSTEM="$(ask system; rules)" \
  ply -sh -check 'go test ./...' 'fix it'
```

The command substitution captures stdout only. The root, each source path,
each byte count, and the final total stay visible on stderr. This separation
also makes ordinary inspection straightforward:

```sh
rules | less
rules > /tmp/project-instructions.md
```

The compact form is useful at an interactive prompt, where stderr is visible.
For a script, make failure propagation explicit:

```sh
system=$(ask system) &&
workspace=$(rules) &&
ASK_SYSTEM="$system
$workspace" ply -sh -check 'go test ./...' 'fix it'
```

This matters because the shell expands a leading environment assignment and
then runs the following command even when a command substitution in that
assignment returned nonzero. Separate assignments joined with `&&` ensure
that exit 1 or 2 from Rules prevents the agent from starting without the
repository instructions.

If two adjacent files would share a line because the earlier file lacks a
final newline, Rules inserts one newline between them. Otherwise file bytes are
left unchanged. Provenance never enters the instruction stream.

If two applicable logical paths have the same fully resolved path, both paths
appear under `-list`, but the body appears only at the first path's position.
Stderr marks each later path as an alias. This is identity deduplication, not
content deduplication: two separate files containing the same bytes are both
printed.

## Bounds and refusals

The unique applicable file bodies are limited to 32 KiB in total. Rules reads
at most one byte beyond that limit from any one file to detect a file that
changed while being read. It never truncates.

Validation completes before stdout begins. That includes `-list`, so a list
with exit 0 is also proof that a normal invocation can safely read the current
set. These fail with exit 1 and empty stdout:

- there is no regular `.git` file or `.git` directory above the target;
- an instruction is a directory, device, socket, FIFO, or other special file;
- an instruction exceeds 32 KiB;
- the unique body set exceeds 32 KiB;
- an instruction symlink resolves outside the project root.

A missing target, unreadable file, dangling symlink, or output failure is an
operational error and exits 2. A repository with no applicable instruction
files is valid: it exits 0 with empty stdout and reports zero files on stderr.

## Symlinks

An instruction symlink can share policy within one repository:

```text
repo/
  .git/
  policy/agent.md
  cmd/server/AGENTS.md -> ../../policy/agent.md
```

The logical `cmd/server/AGENTS.md` is the applicable path and is what `-list`
prints. Rules fully resolves the target before opening it and verifies that
the result remains under the canonical repository root. A link to
`/etc/policy.md` or `../../another-repo/policy.md` is refused before stdout.

Aliases also support tool-specific names without duplicating prompt text:

```text
repo/
  .git/
  AGENTS.md
  CLAUDE.md -> AGENTS.md
```

Both paths are listed. The `AGENTS.md` body is emitted once.

A `.git` symlink is not a root marker. Git repositories and worktrees use a
directory or a regular file there; declining other file types keeps the root
rule short and prevents the marker itself from redirecting scope.

## What Rules deliberately does not do

Rules does not parse Markdown, choose between `AGENTS.md` and `CLAUDE.md`,
search sibling trees, infer roots from `go.mod` or `package.json`, watch for
changes, cache text, run a model, or modify another program's prompt. Its fixed
32 KiB bound has no flag, environment variable, or config.

That narrow contract keeps the important question inspectable:

```sh
rules -list
```
