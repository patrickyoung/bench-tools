# hone

Read a session that a program judged. Write down what it teaches.

    ask     the model      — no tools, no loop
    brief   the procedure  — no model, no loop
    ply     the loop       — no model, no procedure
    hone    the lesson     — no store, no retrieval, no format

## Does this work? Yes, but only one version of it does

The question was worth asking first, because the obvious version of this
idea is known to make agents *worse*, and the evidence is not subtle.

**Unbounded memory is negative value.** An agent with an "add-all" strategy
accumulated 2,400 records and scored 13% on medical reasoning; the same
agent keeping only high-quality experiences and deleting stale ones held
248 records and scored 39%. Ten times the memory, a third of the accuracy.

**Wrong memories are worse than no memories, and they get *more*
persuasive.** Agents that retrieved notebooks from earlier incorrect runs
reused those results with more confidence than before, "because memory had
given the wrong answer the appearance of established precedent." Over 90%
of tested agents were vulnerable to memory poisoning, with a 100% relapse
rate when teams tried to fix it by correcting the agent in conversation.

**Learning from unlabeled trajectories barely works.** Skills distilled
from high-quality verified trajectories yielded +0.377 mean reward;
skills distilled from low-quality ones yielded +0.028. The label is worth
13x. It is the single largest factor in the literature.

**Rewriting a memory file wholesale destroys it.** ACE names this "context
collapse": a model asked to rewrite its accumulated context compresses away
the specifics that made it useful. Updates must be deltas.

So the version that fails is: watch everything, summarize it, embed it,
retrieve by similarity. That is a machine for manufacturing confident
falsehoods, and it is most of what ships.

The version that works is narrow:

- learn **only** from trajectories with a verified outcome;
- keep **procedural** lessons — how to do a thing here — not facts about
  the world, which go stale and which somebody else already sells;
- write **deltas**, never rewrites;
- keep the corpus **small enough to read**;
- make every lesson **traceable to the run that taught it**, so a bad one
  can be found and deleted rather than argued with.

That list is the whole design. Everything below is how to get it in four
hundred lines instead of forty thousand.

## What this family already has that nobody else does

Every memory system in the world is trying to learn from unlabeled
trajectories, because a chat transcript has no ground truth in it. That is
why the label is worth 13x and why almost nobody collects it.

`ply -check 'go test ./...'` is a ground-truth label. `make` does not ask
an oracle whether the build worked, and neither does `ply`. The outcome of
a run is a program's opinion, recorded, and `ask replay -check` proves the
run months later.

That is the input the research says you need. This family produces it as a
side effect of ordinary use.

## The gap that had to be found first

It is not produced *yet*. Two real runs, one that passed and one that
failed:

    $ ply -sh -check 'go test ./... 2>&1' -f run.jsonl  "make the test pass"
    ply: check passed: go test ./... 2>&1        # exit 0

    $ ply -sh -check 'test -f /nonexistent' -cycles 1 -f fail.jsonl ...
                                                 # exit 2

    $ jq -c '{seq,type}' run.jsonl  | tail -2
    {"seq":16,"type":"assistant"}
    {"seq":17,"type":"done"}
    $ jq -c '{seq,type}' fail.jsonl | tail -2
    {"seq":4,"type":"assistant"}
    {"seq":5,"type":"done"}

    $ grep -c 'check passed\|did not pass' run.jsonl fail.jsonl
    run.jsonl:0
    fail.jsonl:0

**The two logs are structurally identical.** A session records everything
that was tried and nothing about whether it worked. The verdict lives on
stderr and in an exit status, and both are gone the moment the shell moves
on.

This is not a missing feature. It is `ply` failing a promise it wrote down
itself, in `AGENTS.md`:

> Anything worth recording — a command, its output, its exit status, **the
> check's verdict** — goes in the text of the conversation, where it is
> already proven, rather than into a second file that can drift.

and it contradicts the principle `ask` states in `DoneData`:

> Reason is end|max_tokens|overflow|error: what stopped it, **in the log
> rather than only in the exit code**, so a session read later says how it
> finished.

So the fix is upstream, it is small, and it makes two existing documents
true:

1. **`ask note`** — append a stamped message to a session without calling a
   model. The mechanism already exists: `UserData.Source` is documented as
   existing "so that model-written text in a conversation can always be told
   apart from what was actually asked, by a reader and by a program." A
   check typescript is the *least* model-written text in the system — it is
   a command and what it printed.
2. **`ply`** writes the check's verdict through it, pass or fail.

Until then `hone` refuses to learn from the session, and says why. It does
not guess. A mislabeled lesson is the 13x failure and the poisoning failure
at the same time.

## What hone is

A filter. It reads a session that a program judged, and writes down what it
teaches.

    $ hone run.jsonl
    - Go test files in this tree declare `package main`, not `package x`;
      a mismatched package name fails as `found packages x and main`
      at setup, before any test runs.

    $ hone run.jsonl -into go-conventions
    go-conventions/SKILL.md: 1 lesson added (4 total)

    $ hone clean.jsonl
    $ echo $?
    1

## The one rule: hone learns from recoveries

A lesson is worth keeping only if it would have changed what happened.
That is checkable, mechanically, from exit codes that are already in the
log:

- a run that **never failed** teaches nothing — everything worked first
  try, and there is no counterfactual;
- a run that **never passed** proves nothing — you do not know the last
  thing tried was right;
- a run that **failed and then passed** is the only one that teaches, and
  the lesson is the difference between the two.

So `hone` finds where a command failed and a later one succeeded, and
writes down the delta. Nothing else in a session is evidence of anything.

This is the precision gate, and it is why the corpus stays small enough to
stay true. It also matches what the literature found from the other
direction: failure-derived constraints gave +14.3% on search tasks and
success-derived ones +9.0% on execution, and the contrastive pair is what
the trajectory-distillation work retrieves on.

The gate is arithmetic on exit statuses. A model is used only to *word* the
lesson, never to decide whether there is one.

## No store, no retrieval, no format

`brief` is already the store. A skill is a directory with a `SKILL.md`,
`$BRIEF_PATH` is a search path with shadowing, `brief find` ranks a task
against the catalogue and **refuses to guess**, and `brief lint` holds the
format to the specification.

So `hone` writes what `brief` reads and stops. It has no index, no
embedding model, no database, no daemon, and no format of its own. The loop
closes as a shell pipeline rather than a runtime:

    ply -s "$(brief find "$task")" -check "$check" "$task"   # act
    hone "$session" -into "$skill"                          # learn
    brief find "$next"                                       # recall

This is also the answer to retrieval quality. Dense retrieval beats lexical
on procedural memory, and `brief find -ask` is the model closing that gap
for a tenth of a cent — already built, already replayable, already
refusing to answer when nothing fits. The diagnostics on complex memory
systems land in the same place: "retrieval stability matters more than
sophistication," with graph and multi-hop schemes degrading while direct
lookup holds 85%+.

## Provenance is the feature

Every lesson carries the session it was learned from and the call that
wrote it, and both replay:

    metadata:
      learned-from: 20260801-230441-4c8100cf68ff97f5
      honed-by:   20260802-004512-9f1a2b3c4d5e6f70

    $ ask replay -check ~/.ask/sessions/20260801-230441-4c8100cf68ff97f5.jsonl
    ok: replays exactly (17 events)

This is the answer to memory poisoning that the security literature could
not give: a bad lesson is not argued with, it is traced to the run that
introduced it and deleted, along with every sibling from that run. The
100% relapse rate came from teams trying to correct a poisoned memory *in
conversation*. Here it is a file with a name on it.

## Bounded by construction

The corpus cannot grow without bound because nothing enters it except a
recovery from a verified run, and because a skill has a size budget the
specification already sets and `brief lint` already warns about. Past it,
`hone` refuses and names the limit rather than truncating — the same rule
the other three follow. Refining a full skill is a goal with a check, which
is to say it is `ply`:

    ply -check 'brief lint -strict go-conventions' \
        "merge duplicate lessons in go-conventions/SKILL.md"

There is no verb for that here, because there is already a program for it.

## The Unix contract

| stream | carries |
| --- | --- |
| stdout | the lesson, and nothing else |
| stderr | which session taught it, why a run was refused |
| exit 0 | yes: something was learned |
| exit 1 | no: nothing to learn — no failure, no recovery, or no verdict |
| exit 2 | error: bad usage, unreadable session, `ask` failed |

`brief`'s contract, not `ask`'s. Most sessions teach nothing, and that is
an ordinary answer a script branches on rather than a failure:

    for s in ~/.ask/sessions/*.jsonl; do
        hone "$s" -into "$skill" || continue
    done

## Deliberately absent

No embedding model, no vector store, no index, no database, no daemon, no
config file, no MCP, no scheduler, no retrieval, no ranking, no memory
types, no consolidation daemon, no automatic learning. Nothing writes to a
skill unless somebody typed a command that says to.

The last one is the load-bearing one. A system that silently learns is a
system that silently learns the wrong thing, and the failure mode is a
confident agent citing a precedent nobody wrote.
