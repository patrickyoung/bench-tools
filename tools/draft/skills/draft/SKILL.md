---
name: draft
description: Design and build a working system out of ask, brief, ply and hone — the Unix-shaped agent family where a tool is a program, the toolbox is $PATH, and done is a program's opinion. Use when planning, scoping, or building any automation, pipeline, agent, or data system with these tools, and when deciding which parts of a problem need a model at all.
---

# draft

You are designing a system that somebody has to run, fix at three in the
morning, and still trust in a year. The tools are `ask`, `brief`, `ply` and
`hone`. The bar is Rob Pike and Ken Thompson: the design is finished when
there is nothing left to take away.

Write the design before the code. `DESIGN.md` is the artifact — it is the
requirements, the procedure, and the definition of done, and a human reads
and edits it. `references/template.md` is its shape.

## The first question, and usually the last

**Is this an agent problem at all?**

Most of a working system is not. Fetching a URL, appending to a file,
running a schedule, moving bytes — these have no judgment in them, and a
model in that position is slower, costlier, and nondeterministic in exchange
for nothing. Reaching for the loop where `curl` would do is the single most
common way these systems go wrong.

Sort every stage into one of four buckets, and be hard about it:

| the stage | the tool | when |
| --- | --- | --- |
| no judgment | a script — `curl`, `jq`, a file | fetching, appending, scheduling, moving bytes |
| judgment, one shot | `ask` | classify, extract, summarize, rank, decide — anything answerable in one question |
| judgment with commands or correction turns | `ply` | an explicit check accepts the candidate or returns feedback |
| a procedure worth keeping | `brief` | the criteria are stable, versioned, and reused |

The default is the first row. Earn each move down it.

`ask` is a filter with no loop and no tools. If the work is "put this
through a model and use the answer," it is `ask`, and adding `ply` buys
nothing but latency. `ply` earns its place only when a **program** can
decide the work is done — tests, a linter, a validator, `test -f`, an exit
status. That is the whole reason it exists.

`hone` learns from runs that failed and then passed, verified by a check.
A system with no `ply -check` in it produces nothing for `hone` to learn
from, and that is fine. Do not put `hone` in a design to be thorough; it
earns its place later, or not at all.

## Data first

> Data dominates. If you've chosen the right data structures, the
> algorithms will almost always be self-evident. — Pike

Decide, before anything else, what is **raw** and what is **derived**.

- **Raw** is append-only and never rewritten. It is what you fetched, what
  was said, what happened. If you lose it you cannot get it back.
- **Derived** is everything computed from raw. It can be deleted and
  rebuilt at any time.

Get this seam right and the system becomes re-runnable: improve a
classifier, delete the derived tree, rebuild, and compare. Get it wrong —
mutate raw in place, or bake a model's answer into the only copy — and no
amount of code recovers it.

Say it plainly in the design:

    raw:     feed/YYYY-MM-DD.jsonl   fetched, never rewritten
    derived: topics/, reports/       rebuildable from feed/ + skills/

## Done is a program's opinion

Every design names a check: a shell command whose exit status is the
verdict. `draft check` refuses a design without one, and that refusal is
the most useful thing in this whole skill.

A requirement you cannot check is a wish. If you cannot name the command,
the requirement is not understood yet — that is information, and it belongs
in the design rather than being papered over.

    ply -sh -check 'go test ./...' "make the tests pass" && make release

The `&&` depends on the test command's status. Review whether those tests
cover the requirements before connecting a release operation.

A checker can call Ask when judgment needs a model, but it must distinguish
a rejected answer from a broken model call. Return 1 for task feedback and
another nonzero status for infrastructure failure. A bare `ask | grep` can
hide Ask's failure behind grep's ordinary no-match status. A model-based
checker also needs evaluation; calling it a program does not make it infallible.

The check runs **before** the first model turn. A passing pre-check avoids
that model call. The check itself may still take time or call a service.
Scheduled use needs a check that detects whether current inputs require work;
repeated invocation alone does not establish safe retries or catch-up behavior.

### The check is only as good as its coverage

"Done is a program's opinion" means the opinion is **only as good as the
program**. `ply` will drive until the check exits 0 and stop the instant it
does, so a check that tests the wrong thing produces a confident, verified,
wrong system — and everything downstream inherits that confidence.

For example, a feed importer may pass tests with a populated store but fetch
the entire remote history on first use: the empty store supplies no cursor,
and the importer treats that as "start from the beginning." A separate
empty-store case can expose the missing initial-fetch limit.

So when writing the Check section, ask three questions the compiler cannot:

- **What state does this check never see?** Empty stores, first runs, cold
  caches, and missing files expose defaults; include them explicitly.
- **Would this fail if the feature were deleted?** If not, it is testing the
  scaffolding rather than the behaviour.
- **What does it cost to be wrong here?** Bound the expensive paths first —
  anything that spends money, writes unboundedly, or touches somebody else.
  A cheap wrong answer is a bug; an unbounded one is an incident.

`draft check` refuses a check that *cannot* fail — `true`, `:`, `exit 0` —
because that is mechanical. Whether a real command covers what matters is
judgment, and it is yours. Write the check that would have caught the bug
you are about to write.

### Admit the oracle when the worker must not own it

A check inside the same writable tree as the implementation is evidence, not
an independent judge: a builder can weaken a test or replace `./bin/check`.
For consequential work, keep the semantic assertions in the Check block or an
operator-controlled program, then have the operator run `draft admit DIR`.
May binds approval to the canonical project and exact check bytes. Build with
`draft build -admitted DIR`; Cage leaves the project writable but prevents the
worker from changing the stored verifier. Follow with `draft prove DIR` to
challenge coverage. Admission freezes the verifier program, not writable
fixtures or helper programs it delegates to, so keep those outside the
candidate tree when they are part of the oracle.

## The shape of a system

```
project/
  DESIGN.md          the agreement. Requirements, the split, the check
  justfile           what a human types
  bin/               capabilities: executables, one job each
  skills/            procedures, versioned, brief-readable
  <raw>/             append-only. Never rewritten
  <derived>/         deletable, rebuildable
  .ask/              ASK_DIR — provenance lives with the data
```

Four conventions carry it:

**A capability is a file.** A `ply` invocation already carries the goal,
procedure, tool grant, model, budget, lifecycle and check. Typed at a prompt
it lives in shell history and dies there; in a file it is named, versioned,
diffable and on `$PATH`:

```sh
#!/bin/sh
# make the tests pass, and prove it
exec ${PLY:-ply} -sh -check 'go test ./...' -cycles 5 -timeout 2m \
     "make the tests pass" "$@"
```

`${PLY:-ply}` is what lets another `ply` hire it as a tool.

**A tool is a program and the toolbox is `$PATH`.** `-t dir` makes `$PATH`
that directory alone, so the model reaches what you put there and cannot
name what you did not. Adding a tool is `ln -s`. It **aims** the model; it
does not sandbox it. The boundary is the process — its user, its container.

**Choose explicit session locations.** `ASK_DIR` can keep sessions with the
project; `-f FILE` names one directly. `ask replay -check` verifies retained
conversation consistency. It does not prove why a model made a decision or
that the decision was correct. Keep trusted records outside worker write roots.

**A procedure is a `brief` skill.** Classification criteria, house rules,
report formats — versioned, lintable, and `brief cat` feeds them straight
into `ask -S`. `brief cat` takes several refs, so N criteria compose into
one prompt and one call.

## The traps

These bite everybody. Design around them from the start.

**Choose fresh or continued context deliberately.** Plain `ask` starts fresh.
`-c` continues `current`; `-f FILE` creates or continues only that named file
and leaves `current` unchanged. Use a fresh filename per independent job, or
reuse one deliberately for a conversation. There is no `ask -n` flag.

**Batch related inputs when they fit.** Batching can reduce repeated request
overhead, but check the model's input limits and measure quality. With `day.jsonl`
and installed `topic-a` and `topic-b` skills, for example:

    system=$(ask system) &&
    criteria=$(brief cat topic-a topic-b) &&
    ASK_SYSTEM="$system
    $criteria" ask -q 'Return JSON with a and b arrays of matching IDs.' < day.jsonl

That prompt requests a shape; it does not validate one. Use Ask's `-schema`
with a supported provider/model and a concrete schema when downstream code
requires validated JSON. The generated tool reference documents the interface.

**Match the model to the job.** Evaluate suitable models on representative
inputs. `-m` selects one per call; `ASK_MODEL` sets the environment default.

**Check before repeating work.** File existence alone does not prove complete,
current output. Verify the result against its inputs before skipping work.
Use the external service's idempotency mechanism when available; a checkpoint
or file cannot resolve an effect whose outcome is unknown.

**Interpret statuses per command.** Ask's 2 means a full context window; Ply's
2 means unfinished work or a limit. Other tools use 2 for operational errors.
Start fresh or compact full context deliberately. Inspect uncertain external
effects before considering a retry; do not treat every nonzero status alike.

**Choose the process environment explicitly.** An ordinary Ply shell action
can inherit secrets from its caller; a scheduled job may lack variables your
terminal supplies. Keep credentials in the appropriate controller process and
configure the scheduler's required environment deliberately.

**Never truncate silently.** Output a model cannot see it lost is the one
failure nothing downstream can detect.

## Writing the design

Follow `references/template.md`. Two sections carry more weight than the
rest:

**Not doing** is worth more than Requirements. A system is defined by what
it refuses, and writing the refusals down is what stops them being quietly
re-added later. Give each one its reason.

**The split** is where the design is won or lost. If everything is in the
`ply` row, the design is wrong and the system will be slow and expensive.

Then check it. `draft check` verifies the sections exist, the check names a
real command, and the tool reference has not drifted from the installed
binaries. `draft build` hands that same check to `ply`.

## Before you write a line of code

Ask whether it needs to exist:

> If you think you need to add a feature, first see whether you can remove
> something instead.

The best outcome of a design session is a smaller system. The second best is
a written reason why each piece survived.

`references/tools.md` is what the installed tools currently do, generated
from the binaries themselves rather than remembered. Read it rather than
trusting recall — flags change.
