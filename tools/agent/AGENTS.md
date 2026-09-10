# Development guidance

`agent` compiles an agent home into the existing Bench-family process
boundaries. It is glue, never a second implementation of Ask, Brief, Ply,
Hone, Trail, May, Action, or Cage.

When changing `agent`:

- keep definition, mutable work/state, and controller evidence as separate
  write domains;
- Markdown shapes behavior but never grants authority, network, approval,
  model choice, or completion;
- completion remains `bin/check`'s exit status through Ply;
- load Agent Skills only through Brief and keep private context out of argv;
- keep Ask sessions authoritative; do not add another transcript or parse
  Ask's event format;
- keep external effects proposal-shaped inside `work/actions/`; only the
  controller may invoke Action, and Action owns policy, May, release, and
  effect receipts;
- keep checkpoints as controller-owned pointers to Ask sessions, never as a
  second log, filesystem snapshot, retry policy, or completion claim;
- call public binaries with literal arguments and never run user text through
  an extra shell;
- resolve every home and control path before work, refuse unsafe symlinked
  definition or controller paths, and leave the home untouched on validation
  failure;
- tests use fake public binaries and never need a model, network, credentials,
  or the user's state;
- stdout is the answer or requested artifact, stderr is progress, and exit
  status is the outcome;
- run `sh -n bin/agent bin/agent-action-shell` and
  `sh bin/agent_test.sh` before reporting success.

Do not add a provider adapter, agent loop, skill parser, scheduler, daemon,
registry, memory index, task database, sandbox implementation, or approval UI.
