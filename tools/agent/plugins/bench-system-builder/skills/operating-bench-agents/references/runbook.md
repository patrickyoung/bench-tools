# Operator runbook

Use only with one verified suite and an inspected home. In examples, `BENCH`
means the absolute path to that suite's `bench`; `PARENT` contains the home,
`NAME` is its basename, and `MODEL` is the non-secret provider/model whose
readiness probe passed in this lane. `bench home` passes literally to the shipped
Agent and supplies its exact suite companions.

## Inspect

```text
BENCH home -C PARENT check NAME
BENCH home -C PARENT show NAME
BENCH home -C PARENT history NAME check
```

`check` status 0 means structurally valid, 1 invalid, and 2 a usage or
controller error. It does not prove business quality, model access, or that the
goal is complete.

## Operate interactively

```text
BENCH -C PARENT -m MODEL -home NAME
```

Run this in a real terminal. The view uses public Agent show/check/run/tick,
history, specialist, learning, proposal, and amendment operations. If the
current Claude surface has no interactive terminal, return this exact command;
do not claim the TUI was opened and do not emulate it with another agent.

## Run now

```text
BENCH home -C PARENT run -m MODEL NAME -- INPUT
```

Without a checkpoint, start a distinct run. Keep network off unless a
separately reviewed run envelope requires it. Do not use `-no-cage` as
troubleshooting convenience.

## Resume one conversation

```text
BENCH home -C PARENT run -m MODEL -checkpoint CASE-NAME NAME -- INPUT
```

Repeat the same named checkpoint only when the later invocation should continue
that exact Ask conversation. A checkpoint is context continuity, not work
rollback, a task database, overlap policy, or effect safety. Use a fresh name
for distinct work and never edit private checkpoint files.

## Wake only when work exists

```text
BENCH home -C PARENT tick -m MODEL NAME -- INPUT
```

`bin/wake` is a cheap controller probe: 0 quiet, 1 wake, another status broken.
Tick spends a model call only on work. It is not a scheduler; an outer system
still decides when to invoke it. The default scheduled tick is a distinct
invocation. Add `-checkpoint CASE-NAME` only when the owner explicitly intends
the next wake to continue that exact Ask conversation; never use it as an
overlap lock or effect-safety mechanism.

## Specialist

```text
BENCH home -C PARENT specialist NAME CHILD -m MODEL -- INPUT
```

Here `NAME` is the existing parent home and `CHILD` is one direct child home.
A specialist has its own goal, check, writable state, and evidence. Do not use
a specialist merely to hide a large context or bypass the parent's authority.

## Inspect history

```text
BENCH home -C PARENT history NAME ls
BENCH home -C PARENT history NAME show SESSION
BENCH home -C PARENT history NAME window -before 4 -after 4 SESSION SEQ
BENCH home -C PARENT history NAME lineage SESSION
BENCH home -C PARENT history NAME check
```

Use bounded windows for diagnosis. History integrity is not business truth.

## Review and execute an effect proposal

```text
BENCH home -C PARENT actions NAME
BENCH home -C PARENT actions NAME PROPOSAL
BENCH home -C PARENT act NAME PROPOSAL SESSION
```

These commands require Agent 0.2.1 from the pinned suite. Inspect the proposal
before the controller invokes Action. A policy decision of review parks through
May. Status 75 is unfinished; status 125 means a possibly-started effect lacks a
trustworthy terminal result. Neither should be retried automatically.

## Learn from a verified recovery

```text
BENCH home -C PARENT learn -why -into SKILL NAME SESSION
BENCH home -C PARENT learn -into SKILL -m MODEL -prepare PROPOSAL NAME SESSION
BENCH home -C PARENT learn -show PROPOSAL NAME
BENCH home -C PARENT learn -admit PROPOSAL NAME
```

The first step is read-only evidence. Prepare exact proposed skill bytes, inspect
them, then admit without another model call. A run that never failed needed no
recovery; a run that never passed proves no lesson.

## Amend governed definition

```text
BENCH home -C PARENT proposals NAME PATCH
BENCH home -C PARENT amend NAME PATCH
```

Inspect the bounded patch and exact May action. Amendment dry-runs and binds the
current definition and patch hashes. Exit 75 is awaiting approval, 3 declined;
only an identical retry after the exact May grant may apply it.

## Terminal handling

Do not use one generic status table for every subcommand. For Agent `run`,
`tick`, and `specialist`, the composed Agent/Ply contract is: 0 accepted; 1
runtime, provider, or broken-verifier error; 2 not done or a bound reached; 3
approval declined; 75 parked; 125 uncertain confinement; 130 interrupted.
`agent check` instead uses 0 valid, 1 invalid, and 2 usage/controller error.
Action commands can use 125 for a possibly-started external effect with no
trustworthy terminal result. Always read the exact command help and retained
evidence. Never automatically retry 125 or any interrupted operation that may
have released an effect.
