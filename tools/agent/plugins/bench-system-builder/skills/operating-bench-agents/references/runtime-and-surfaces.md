# Runtime and surface checks

This skill release targets Bench suite 0.13.0 and Agent 0.2.1. Use public version
and help commands. A newer, older, missing, or independently mixed runtime is a
steward issue; do not apply this runbook speculatively.

## Claude Code

Claude Code can operate a local Agent home when the compatible suite is
executable on macOS or Linux and the user grants the necessary folder and tool
access. Skill presence does not pre-approve commands. Review project/plugin
skills before trusting them; `allowed-tools` would pre-approve, not restrict,
and this skill deliberately declares none.

## Claude Cowork

Cowork cloud code runs in a temporary Anthropic-managed environment. Connected
local files and a host-installed suite are separate capabilities. Operation is
supported only through an explicitly proven compatible runtime or managed Bench
integration that can reach the Agent home and controller evidence with the
documented identity and persistence.

If that bridge is absent, inspect exported artifacts only and produce a steward
handoff. Never claim to have resumed, scheduled, or retained a local Agent run
from a cloud-only conversation.

## Read-only preflight

Record through the exact suite entry point:

- `/absolute/prefix/bin/bench version`;
- `bench home check HOME` exit status and evidence;
- `bench home show HOME` composition and authority;
- `bench home history HOME check` result;
- OS/architecture and execution location;
- writable roots and controller evidence location;
- whether required sources and connectors are reachable by the operating
  identity;
- schedule/supervisor state when relevant.

Do not run `agent tick`, `agent run`, authenticate, or change anything during
preflight.

An interactive operator view is
`bench -C PARENT -m provider/model -home NAME` in a real terminal, using the
same non-secret model identifier whose readiness was proved for that lane.
Headless and scheduled operations remain the same suite Agent reached through
`bench home` or the absolute sibling `agent`; no Claude session becomes the
worker runtime.
