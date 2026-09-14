# Development guidance

Record observes one explicitly selected process through ordinary Unix pipes.
Ask owns the session, event format, locking, and seals. Invoke its public
commands; never import another Bench tool or write Ask events directly.

- Record a sealed intent before starting the command. Missing terminal
  evidence is unknown, even when all surviving seals verify. Never retry.
- Preserve literal argv, separate binary streams, and the observed status.
  Do not infer business success, permission, or absence of external effects.
- Keep one invocation in a newly named recording session. Child conversations
  have their own files. Snapshot selected artifacts and sessions explicitly.
- Retain evidence before publishing output. Storage failures stop capture and
  return 125; they must not appear as successful execution.
- Replay verifies the complete snapshot before emitting any captured bytes.
  It never starts the recorded executable or opens the original artifacts.
- Do not capture environment values or private descriptor contents. Record
  explicitly selected metadata only. Never add credential discovery.
- Keep streaming stdin: a protocol server may answer before input reaches EOF.
- Run ordinary tests, race tests, vet, and public Ask integration checks when
  changing the capture or replay boundary.
