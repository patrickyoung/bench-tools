# A2A is a protocol boundary

Read DESIGN.md before changing this component. Keep its Go module independently
buildable. Use the official A2A SDK for the wire protocol. Never import a sibling
Bench tool or add model clients, goal loops, a scheduler, or an agent registry.

- `a2a` makes one explicit request or follows one explicitly selected stream.
- `a2aserve` exposes a configured executable through authenticated A2A requests.
- Use literal argv and bounded streams. Credentials use protected files or a
  descriptor, never arguments, task payloads, worker environments, or logs.
- Preserve failure, unfinished work, and uncertainty. Never retry a transmitted
  mutation, restart a worker, or claim cancellation stopped an external effect.
- Authenticate every operation and authorize task/context access. A caller may
  select an admitted endpoint, not arbitrary local programs or filesystem roots.
- Persist protocol facts in an explicit private directory; scheduling remains
  with the operating system and durable local execution can use Tend.
- Run Go tests, race tests, vet, and executable integration checks. Include
  upstream interoperability, TLS/authentication, task isolation, and interrupted
  execution. Keep paid model calls separate from offline verification.
