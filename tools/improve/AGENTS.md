# Development guidance

Improve runs one bounded, reproducible experiment and emits a tested proposal.

- Keep the protocol generic: a source tree, explicit mutable files, two case
  splits, and caller-selected proposer, trial/scorer and judge commands.
- Compose literal argv and JSON pipes. Record owns process receipts; Agent/Ply
  own execution; Hire authors; Ask owns providers and sessions. Import none of
  their code. Never parse or manufacture Ask events or Hone recoveries.
- Freeze inputs, commands and declared dependencies before execution. Only the
  allowed source files can change; independent evaluation stays fixed.
- One proposal per invocation. Compare fresh matched pairs, alternate arm
  order, and never feed final holdout results back into authoring.
- No automatic source write, deployment, retry, resume, daemon or scheduler.
  Export exact supported bytes with evidence for the caller's promotion path.
- Missing costs remain unknown. An adapter supplies observations, not trusted
  truth; document integrity checks separately from factual quality.
- Keep source, mutable work and evidence separate. Selected commands are trusted
  and must choose their own Cage/authority boundary. Hashes are not a sandbox.
- Tests need no credentials, paid calls or user state. Run go test ./...,
  go test -race ./..., go vet ./..., and the public Record integration check.
