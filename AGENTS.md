# Preserve the programs

This repository houses independent Unix tools. Read the applicable tool's
AGENTS.md and design documents before changing it. The root owns source
coordination, independent packaging, and verification only.

- Keep every tool independently buildable, testable, usable, and versioned.
- Compose public executables with literal argv, stdin, stdout, stderr, exit
  status, and documented files. Preserve each tool's own stream/exit contract.
- Do not import another tool's code, add a shared runtime, root Go module or
  workspace, local module replacement, umbrella command, or mandatory daemon.
- Do not centralize provider clients, action loops, policy, approvals, sessions,
  or state merely because source directories are adjacent.
- Source co-location grants no runtime authority. Workspaces, writable state,
  trusted controller evidence, credentials, and confinement remain separately
  selected by the caller under each program's existing contract.
- Preserve component module paths, commands, manuals, licenses, and local
  guidance. Do not turn another component's instructions into an include.
- No cross-component symlinks. Rules discovers the actual repository root;
  do not add fake nested .git markers to alter instruction discovery.
- Changes to public contracts require the affected components' own checks and
  explicit executable integration checks. A green aggregate build is not proof
  of compatibility, task completion, authority, or confinement.
- Keep live paid-model evaluations explicit and separate from offline checks.
