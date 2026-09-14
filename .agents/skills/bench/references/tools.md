# Choose components by responsibility

Find reusable roles and assemblies through [library](library.md) before
selecting lower-level components. `scripts/workers list` lists worker metadata;
`list --teams` lists team metadata. This source utility lives in the checkout,
not the installed command prefix, and never executes jobs.

Read `docs/TOOLS.md` in the selected Bench checkout for full examples, then only
the chosen tools' READMEs and manuals. `components.json` is the command inventory;
`python3 scripts/build --list` prints the available build selections.

| Responsibility | Component / public commands | Read next in the checkout |
| --- | --- | --- |
| One model request and its replayable conversation | ask | tools/ask/README.md |
| Execute actions, return feedback, check candidates | ply | tools/ply/README.md |
| Run a portable expert or standing home | agent | tools/agent/README.md |
| Build, inspect, or maintain its definition | hire | tools/hire/README.md |
| Find, load, and validate Agent Skills | brief | tools/brief/README.md |
| Repository instructions for a directory | rules | tools/rules/README.md |
| Retrieve and normalize source records | context | tools/context/README.md |
| Validate citation identities against records | cite | tools/cite/README.md |
| Design and build a genuinely new program | draft | tools/draft/README.md |
| Propose a lesson from a checked recovery | hone | tools/hone/README.md |
| Gate an exact connector operation | action | tools/action/README.md |
| Record a person's decision on exact bytes | may | tools/may/README.md |
| Limit a child's writes and networking | cage | tools/cage/README.md |
| Capture full process streams and selected files for offline replay | record | tools/record/README.md |
| Inspect retained Ask archives | trail | tools/trail/README.md |
| Durable local jobs, waits, attempts, and output | tend | tools/tend/README.md |
| Select ready tasks from a finite dependency graph | weave | tools/weave/README.md |
| Call services; admit wrappers; expose a program | mcp, mcp-legacy, mcpbox, mcpserve | tools/mcp/README.md |
| Resource-bound login, refresh, and credential handoff | oauth | tools/oauth/README.md |
| Call or expose independently operated agents | a2a, a2aserve | tools/a2a/README.md |

Common compositions: Brief → Ask for a repeatable text job; Context → Ask →
Cite for a cited draft; Hire → expert folder → Agent for reusable work; Tend →
Agent for retained attempts; mcpserve → dispatcher → ordinary program for a
host tool; a2aserve → Tend → Agent for a remote expert.

For worker teaching, use [the learning procedure](learn.md): Hire amends supplied
knowledge; Hone prepares a lesson from a checked recovery; Brief and a fresh
Agent run verify that the changed knowledge is available and useful.

Install companion commands explicitly. Dependency adjacency in the monorepo
does not install credentials, grant access, or require loading every tool.
