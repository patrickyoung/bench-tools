# agent

Point at a directory-shaped digital worker, inspect its exact composition,
and run its durable goal until an executable check accepts.

## What it is

`agent` is the link step for a filesystem agent. It validates a small set of
well-known files, composes them through Brief and Ply, and leaves the model,
loop, skills, verifier receipts, and confinement to the programs that already
own them.

The complete research and product rationale is in
`../bench/FILESYSTEM_AGENTS.md`.

## Requirements

- [x] An agent is an ordinary directory with human-readable definition,
      mutable work/state, and controller evidence in distinct locations.
- [x] `agent show` reveals the exact context order, hashes, paths, authority,
      skill catalogue, and specialist catalogue without calling a model.
- [x] `agent check` is offline and refuses missing, oversized, symlinked, or
      unwritable core structure and delegates skill validation to Brief.
- [x] `agent run` loads private home context through Brief rather than argv,
      works in `work/`, stores Ask sessions under `.agent/runs`, and lets the
      home-owned executable `bin/check` decide completion through Ply.
- [x] A named checkpoint maps to one controller-owned Ply session pointer,
      follows compaction, excludes concurrent use, and resumes conversation
      context without making Agent a scheduler or claiming effect safety.
- [x] The default model-action boundary lets the worker write only `work/`,
      `state/`, and its temporary directory, with network denied by Cage.
- [x] A home cannot grant itself broader authority. Network and unconstrained
      execution are explicit invocation flags.
- [x] A heartbeat pre-check can make an idle scheduled tick cost no model call.
- [x] Nested homes are exposed as bounded, separately replayable specialists.
- [x] Verified recoveries can be offered explicitly to Hone without creating
      an automatic memory path.
- [x] `learn -why` exposes replay-verified recovery evidence without a model
      call or skill write; generated dry-run wording is not misrepresented as
      an exact later admission.
- [x] Exact learning proposals are user-named controller artifacts; prepare
      never overwrites or writes a skill, show calls nothing, and admission
      replays both provenance sessions, rejects stale paths/bytes, and writes
      the reviewed Hone delta without another model call.
- [x] Prior run evidence can be listed, searched, shown, windowed, lineage-
      inspected, and replay-checked through Trail without another session
      parser or writer.
- [x] A frozen representative-home corpus measures deterministic validation,
      compiled bytes, zero-model gates, recursive homes, clean re-entry, and
      Ask-owned replay without network access.
- [x] Definition changes use one reviewed unified diff, an exact one-shot May
      grant, stale-input rechecks, complete home validation, rollback, and a
      controller-owned receipt.
- [x] Proposal review is a bounded, side-effect-free public command that shows
      literal patch bytes and the exact action before May is invoked.
- [x] External effects are strict model-writable proposals; bounded review is
      read-only, while an explicit controller command invokes Action outside
      Cage and records proposal/decision/attempt/sent/result in the selected
      Ask session.

## Not doing

- No `agent.yaml`, registry, database, daemon, scheduler, provider code,
  prompt cache, tool schema, session schema, or embedded runtime.
- No automatic memory writes. `MEMORY.md` is curated input; verified procedure
  changes belong in Brief skills through Hone.
- No completion inferred from model prose. `bin/check` is required.
- No authority read from Markdown. The caller chooses confinement and network.
- No writable definition during a confined run. Proposed definition changes
  go under `work/proposals/`; only the controller's explicit `amend` command
  can apply one after exact human approval.
- No effect authority in a proposal. `work/actions/` names a connector and
  input only; operator policy, connector paths, May, and execution remain in
  the controller.

## The split

| stage | tool | why |
| --- | --- | --- |
| scaffold and validate a home | none -- this script | bounded filesystem mechanics |
| print exact composition | none -- this script + public catalogues | inspection, not judgment |
| validate/select procedures | brief | it owns Agent Skills |
| work until the check accepts | ply | it owns the action/check loop |
| call the model and record turns | ask, through ply | it owns providers and sessions |
| lock and advance a conversation checkpoint | ply | it owns the current Ask session |
| confine model-authored actions | cage | it owns the OS boundary |
| admit learned procedures | hone | it owns verified learning |
| browse prior runs | trail | it owns read-only Ask archive inspection |
| authorize exact definition bytes | may | it binds a human decision to one proposal and current definition |
| parse, apply, validate, and receipt a patch | this script + git | bounded controller mechanics with rollback |
| authorize and receipt external effects | action + may + ask | Action owns exact policy, release, and typed replay events |

## Data

      raw:
      AGENTS.md GOAL.md SOUL.md PLAN.md HEARTBEAT.md MEMORY.md
      skills/ agents/ tools/ bin/check bin/wake
    mutable:
      work/ state/
      work/proposals/   proposed one-file unified diffs
      work/actions/     strict external-effect proposals
    evidence:
      .agent/runs/       Ask sessions and Ply verifier receipts
      .agent/checkpoints/ locked current-session pointers
      .agent/learning/   Hone wording sessions
        proposals/       exact reviewed-learning artifacts
      .agent/amendments/ accepted definition-change receipts
    derived:
      private temporary Agent Skill used only to load bounded home context

The temporary skill is deleted after Ply has loaded it. The resulting exact
system prompt remains in the Ask session, so deleting the transport does not
delete the evidence.

## Check

```sh
sh -n bin/agent bin/agent-action-shell && sh bin/agent_test.sh
sh eval/run.sh  # integration corpus; requires the installed public suite
```

## Layout

    agent/
      AGENTS.md
      DESIGN.md
      README.md
      bin/
        agent
        agent-action-shell
        agent_test.sh

## Traps

- `ply -S` would put the private assembled context in `agent`'s argv. The
  implementation instead creates a private temporary Agent Skill and passes
  only its path through Ply's existing `-s` seam.
- A toolbox aims a model but is not confinement. Cage supplies the default
  write and network boundary independently of `tools/`.
- The check runs outside the action boundary. That keeps it operator-owned,
  but a check that executes worker-written code still executes that code with
  controller authority.
- Sessions must remain outside `work/` and `state/`, or the worker can rewrite
  the evidence that later readers trust.
- A model-launched nested agent inside a network-denied Cage cannot reach its
  provider. Parallel specialists therefore require an explicitly networked or
  externally orchestrated slice, not a silent widening.
- Cage does not become the effect policy. The worker can prepare an Action
  proposal, but `agent act` is a separate controller invocation and scrubs its
  connector and approval capabilities from model runs.
