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
| judgment, until a program agrees | `ply` | the answer must satisfy a check the model cannot fake |
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

The `&&` means something there, because exit 0 is `go test`'s opinion and
not the model's. When judgment genuinely needs a model, nothing about the
mechanism changes, because `ask` is a program:

    ply -check 'ask -n -q -f /tmp/j.jsonl "Does this cover install? yes/no" \
        < README.md | grep -qi "^yes"' "document the installer"

The check runs **before** the first turn too, so work already done costs
nothing. That is what makes a system safe in a hook, a `Makefile`, or a
schedule — and it is why a missed run heals itself instead of needing
catch-up logic.

### The check is only as good as its coverage

"Done is a program's opinion" means the opinion is **only as good as the
program**. `ply` will drive until the check exits 0 and stop the instant it
does, so a check that tests the wrong thing produces a confident, verified,
wrong system — and everything downstream inherits that confidence.

This is not hypothetical. A design here specified an append-only feed and a
fixture-based check. The check was genuinely good: syntax, compile, unit
tests, lint, plist validation. It passed. The first real run then walked the
entire history of the upstream API, 190 MB, because the fetcher's lower
bound was `None` on an empty store and the cursor loop read `None` as zero —
the one path the fixtures never exercised, since a fixture store is never
empty.

So when writing the Check section, ask three questions the compiler cannot:

- **What state does this check never see?** Empty stores, first runs, cold
  caches and missing files are where defaults are decided, and fixtures are
  never empty. Most escaped defects live in the state you did not fixture.
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

**`ASK_DIR` points into the project.** Then `ask replay -check` proves,
months later, exactly why a thing was classified the way it was. For
analytics that is the difference between a number and a citation.

**A procedure is a `brief` skill.** Classification criteria, house rules,
report formats — versioned, lintable, and `brief cat` feeds them straight
into `ask -S`. `brief cat` takes several refs, so N criteria compose into
one prompt and one call.

## The traps

These bite everybody. Design around them from the start.

**A scripted `ask` must pass `-n` or `-f`.** By default `ask` continues the
current conversation. In a scheduled job that means every run appends to one
ever-growing session until it hits exit 2 — and silently inherits whatever
you last asked at your terminal. Every call in `bin/` gets `-n`, or its own
`-f` thread. This is the sharpest edge in the family.

**Batch, do not iterate.** One call classifying 1,000 items against three
criteria beats 3,000 calls, and usually beats 3. Ask for a shape:

    ask -n -q -S "$(ask system; brief cat topic-a topic-b)" \
        'Return ONLY minified JSON: {"a":[ids],"b":[ids]}' < day.jsonl

**Match the model to the job.** A cheap model for classification, a strong
one for synthesis. `-m` per call; it is not a global setting.

**Idempotency is a feature.** Skip work already on disk. Then a re-run is
free, a missed schedule self-heals, and a crash costs one tick.

**Exit 2 means *not done*, never *broken*.** A supervisor must tell a
failing check from a provider outage, or it retries the wrong one. `ask`
returns 2 for a full context window, which is permanent — retrying it until
the bill arrives is the failure this row prevents.

**Secrets are inherited, and schedulers inherit nothing.** A model-authored
command sees every variable in the environment `ply` started with. `launchd`
and `cron` see almost none. Both facts break systems, in opposite directions.

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
