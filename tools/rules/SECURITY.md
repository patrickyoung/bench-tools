# Security

`rules` reads repository-authored text for later, explicit prompt composition.
Treat that text as untrusted instructions unless you trust the repository.
Rules establishes file and path boundaries; it does not judge what the text
asks an agent to do.

Instruction text is guidance, not authorization. Supplying it as a system,
developer, or user prompt does not make a requested action safe and must not
grant access to secrets, the network, broader filesystem paths, approval
mechanisms, or destructive operations. Keep those capabilities in the runner,
sandbox, and operator policy, where repository text cannot change them.

The project root is the nearest regular `.git` file or `.git` directory.
Applicable files are only `AGENTS.md` and `CLAUDE.md` on the canonical path
from that root to the target directory. Instruction symlinks are fully
resolved and refused when their targets leave the project root.

Rules accepts only regular instruction files. It reads at most 32 KiB plus one
byte from a file and admits at most 32 KiB of unique fully resolved file
bodies. It validates every path before emitting stdout, so a refusal cannot
masquerade as a complete prompt by returning a truncated prefix. Canonical
aliases are reported but their bodies are admitted only once.

Rules makes no network requests, executes no repository commands, writes no
repository files, maintains no cache, and reads no configuration or secrets.

Report vulnerabilities privately to the repository owner. Include the Rules
version, operating system, a minimal directory layout, the invoked command,
and stderr. Do not include private instruction contents or credentials.
