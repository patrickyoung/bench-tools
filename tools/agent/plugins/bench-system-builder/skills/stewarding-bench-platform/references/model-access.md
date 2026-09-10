# Model access for local Bench Agent runs

Runtime installation, model selection, and credential readiness are different
claims. Keep them visibly separate in readiness reports.

## Setup-to-run milestones

Evaluate model access immediately after the suite is verified and before the
first Bench contract, Draft, or Agent operation that can call Ask:

1. **SUITE-READY** — suite 0.13.0, its public commands and checksums are exact,
   and `cage check` passed 13/13 on the target host. `cage status` alone earns
   only **SUITE-INSTALLED**. No provider call has been proved.
2. **MODEL-READY** — one non-secret provider/model identifier and approved
   controller-owned access complete the builder's approved, schema-constrained
   Ask probe in the current execution lane without exposing the credential
   value. This is required before model-backed Bench contract, Draft, or Agent
   work.
3. **AGENT-FIRST-RUN** — a structurally checked Agent home later completes one
   bounded fixture invocation through `bench home run`, and the literal status
   and evidence are retained.

These milestones accumulate; none is shorthand for the next. In particular,
interactive **MODEL-READY** is not fresh-scheduler authentication.

## Non-secret readiness

Record the selected public model name, such as `provider/model`, and the
account/workload identity that owns access. Capability presence is a preflight
observation, not **MODEL-READY**. With the complete plugin, inspect and use the
builder's `scripts/probe-model-access.sh`: show its read-only plan, then obtain
approval for one empty-system, no-attachment, no-tool, constant-schema Ask call
with a 60-second default timeout. Only its literal `milestone=MODEL-READY`
result proves this current process route. It retains a sanitized status and
exact retry command but no credential values or provider error bodies. Never
display a credential value or dump the process environment to diagnose it.

A Claude Code or Cowork login is not automatically an Ask provider credential.
Likewise, an interactive terminal that can call a provider does not prove that
a fresh launchd, systemd, CI, or cloud task can do so.

## Current local boundary

Ask supports provider credentials through its documented provider boundary.
Agent's public run command does not currently carry Ask's one-child OAuth
header-file-descriptor seam. API credentials inherited in an ordinary process
environment may also be readable by model-authored commands because Cage is a
write/network boundary, not a complete read or secret boundary.

Therefore:

- a bounded, low-risk interactive pilot may use an organization-approved
  controller credential setup and narrow operating identity;
- do not write a key into an Agent home, project `.env`, plist, systemd unit,
  argv, wrapper script, runbook, or log;
- do not claim that environment scrubbing or a scheduler credential file makes
  a value invisible to the model action;
- authenticated unattended operation remains **STEWARD-REQUIRED** until a
  reviewed credential mediator or stronger outer task isolation establishes
  delivery, non-disclosure, rotation, revocation, and fresh-process behavior.

This is a platform capability gap, not something a better prompt should hide.
The safe low-friction response is to install the suite and preserve every
offline business/design artifact, leave a precise provider-readiness gate, and
resume the first blocked model-backed command as soon as the named owner
supplies that boundary.

When access is unavailable, return **MODEL-ACCESS-REQUIRED** while preserving
**SUITE-READY** (or the narrower **SUITE-INSTALLED** /
**CAGE-HOST-CHECK-REQUIRED** state actually proved). Record only the
resource/account identity, non-secret model
identifier, access owner, revocation owner, earliest blocked model-backed gate,
and the literal command that should be retried after approval. Continue
business-process artifacts and offline structural work, but do not invoke or
claim Bench contract generation, Draft model work, or an Agent first run. The
next session rechecks access and resumes that exact saved gate; it does not redo
runtime installation or the owner interview.
