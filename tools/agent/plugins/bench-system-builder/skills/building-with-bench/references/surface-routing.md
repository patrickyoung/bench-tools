# Claude surface and runtime routing

Use this reference for the first preflight. The skill installation location and
the Bench execution location are separate questions.

## Claude Code

### Skill

- Plugin install: add the `patrickyoung/agent` marketplace and install
  `bench-system-builder@bench-agent-tools`.
- One-session plugin trial on Claude Code 2.1.128 or newer: inspect the
  downloaded plugin ZIP, then load it directly with
  `claude --plugin-dir ./bench-system-builder-0.2.0.zip`. On older releases,
  extract it and pass the `./bench-system-builder` directory.
- Standalone personal skill: extract the skill folder beneath
  `~/.claude/skills/`.
- Standalone project skill: extract it beneath `.claude/skills/` in the project.

Confirm that `SKILL.md` is exactly one directory below the selected skills
directory. Review every downloaded skill before trusting it.

### Runtime

On macOS or Linux, Claude Code can invoke a locally installed compatible suite.
The source-free path is the platform-specific Bench release archive verified
against the versioned steward helper's embedded SHA-256 pin. Never use
`curl | sh`, never choose an architecture by guess, and do not install until the
user approves the target prefix.

A present skill does not grant shell tools. A present binary does not grant the
user's approval to run it.

## Claude Cowork

### Skill

Upload the plugin ZIP through **Customize → Plugins → Upload**, or upload the
standalone skill ZIP through **Customize → Skills → Create skill → Upload a
skill**. Code execution and Skills must be enabled. Keep the skill disabled when
it is not needed.

### Runtime

Cowork cloud sessions run code in an Anthropic-hosted temporary environment.
Connected local files are reached through the Desktop app; that connection does
not make a host-installed Bench suite executable in the cloud. Local MCP servers
also are not a cloud runtime bridge.

A Cowork build is **BUILD-READY** only when a named managed Bench integration or
an explicitly supported compatible runtime is visible to the session and the
preflight proves its version. Otherwise Cowork remains **DESIGN-ONLY** and can
produce a complete system package and steward handoff.

Do not download a Linux suite into a Cowork sandbox and call the system durable
without confirming persistence, workspace mapping, evidence export, identities,
network policy, and how the next session will find the same state.

## Read-only preflight

Report:

1. Claude surface: Code, Cowork cloud, Cowork local, or unknown.
2. Execution location: local host, temporary cloud sandbox, managed runtime, or
   unknown.
3. Dedicated project folder and the exact writable scope granted to the session.
4. Compatible Bench version evidence, if any.
5. Available file, shell, network, connector, and browser capability by name;
   do not exercise or widen them.
6. Whether controller evidence and withheld labels can live outside the worker's
   writable/readable roots.

Return **BUILD-READY**, **DESIGN-ONLY**, or **STEWARD-REQUIRED**. When evidence is
ambiguous, choose the narrower state.

## Security boundary

- Interface permission prompts are not Bench Action or May receipts.
- Cowork isolation limits where code runs; it does not reduce the reach of files,
  apps, browsers, or connectors the user granted.
- Claude Code `allowed-tools` pre-approves; it does not remove unlisted tools.
- Cage currently restricts writes and network for model actions but can still
  expose host-readable files available to the operating-system identity.
- Computer use is an effectful outer capability and is never a substitute for a
  connector contract, an executable check, or controller evidence.
