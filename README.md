# hone

Read a run that a program judged. Write down what it teaches.

```
$ ply -sh -check 'go test ./...' "make the tests pass"
$ hone
- Go files in this module root declare `package main`, matching
  `add_test.go`; `package x` fails at setup with "found packages x (add.go)
  and main (add_test.go)" before any test runs.
```

That is a real lesson from a real run, and the thing worth noticing is what
it is *not*. The same run also failed a `git status` in a directory that was
not a repository. `hone` threw that away, because a stumble that teaches
nothing is most of them.

It is a filter: the lesson is stdout, progress is stderr, and the exit code
says whether anything was learned.

```
ask     the model      — no tools, no loop
brief   the procedure  — no model, no loop
ply     the loop       — no model, no procedure
hone    the lesson     — no store, no retrieval, no format
```

## The idea that does not work

Watch everything an agent does, summarize it, embed it, retrieve by
similarity. That is most of what ships, and the evidence against it is not
subtle.

An agent adding every experience to memory accumulated 2,400 records and
scored 13% on medical reasoning. The same agent keeping only high-quality
experiences and deleting stale ones held 248 records and scored **39%**. Ten
times the memory, a third of the accuracy.

Worse, wrong memories do not sit quietly. Agents that retrieved notes from
earlier incorrect runs reused them *with more confidence than before* —
memory had given the wrong answer the appearance of established precedent.
Over 90% of tested agents were vulnerable to memory poisoning, with a 100%
relapse rate when teams tried to fix it by correcting the agent in
conversation.

And the input matters more than the algorithm: skills distilled from
verified trajectories were worth roughly **13x** what the same method
produced from unverified ones. Almost nobody collects the verification,
because a chat transcript has no ground truth in it.

So a memory system that writes down a lot is not a better memory system. It
is a machine for manufacturing confident falsehoods.

## The one rule

**`hone` learns from recoveries.**

- a run that **never failed** teaches nothing — everything worked first
  try, so there is no counterfactual;
- a run that **never passed** proves nothing — the last thing tried might be
  wrong, and a wrong lesson is worse than a missing one, because the next
  agent reads it as established practice and cannot tell;
- a run that **failed and then passed** is the only one that teaches, and
  the lesson is the difference between the two.

The gate is arithmetic on exit statuses already in the log. A model is used
to *word* a lesson, never to decide there is one — a model asked whether a
transcript contains a lesson will always find one.

Three runs in four teach nothing:

```
$ hone
hone: 20260801-2304: the check passed and nothing ever failed: the run had
       nothing to teach because it needed nothing
$ echo $?
1
```

That is the design working. It is also why the corpus stays small enough to
stay true.

## Why this family can do it

`ply -check 'go test ./...'` is a ground-truth label. `make` does not ask an
oracle whether the build worked, and neither does `ply` — the outcome is a
program's opinion, and `ask replay -check` proves the run months later.

That is exactly the input the research says you need, produced as a side
effect of ordinary use. It was not quite recorded, though, and finding that
out was the first thing that happened here. Two real runs, one that passed
and one that gave up:

```
$ jq -c '{seq,type}' pass.jsonl | tail -2
{"seq":16,"type":"assistant"}
{"seq":17,"type":"done"}
$ jq -c '{seq,type}' fail.jsonl | tail -2
{"seq":4,"type":"assistant"}
{"seq":5,"type":"done"}
```

Structurally identical. A session recorded everything that was *tried* and
nothing about whether it *worked*; the verdict lived on stderr and in an
exit status, and both are gone the moment the shell moves on. `ply` now
writes it into the log as an `ask note` — stamped, not folded — which is
what its own documentation had always said it did.

```
$ jq -r 'select(.type=="note") | "[" + .data.source + "] " + .data.text' pass.jsonl
[ply] the check passed:

$ go test ./... 2>&1
ok  	x	0.253s
```

A run with no `-check` writes no verdict, and `hone` refuses it rather than
taking the model's word:

```
$ hone chat.jsonl
hone: chat: no check ran, so nothing judged it but the model. Run it again
       with ply -check, and the verdict lands in the log
```

## Install

```
go install github.com/patrickyoung/hone@latest
```

Go 1.26 or newer, and a Unix. It needs [`ask`][ask] on `$PATH` — that is
where the model, the keys and the log live — and [`brief`][brief] only to
lint what it writes.

[ask]: https://github.com/patrickyoung/ask
[brief]: https://github.com/patrickyoung/brief
[ply]: https://github.com/patrickyoung/ply

## No store, no retrieval, no format

A lesson is a skill. `brief` is already the catalogue — `$BRIEF_PATH` is a
search path with shadowing, `brief find` ranks a task against it and
**refuses to guess**, and `brief lint` holds the format to the
specification. So `hone` writes what `brief` reads, and stops.

```
$ hone -into go-house
hone: go-house is new; writing the line brief will find it by
~/.claude/skills/go-house/SKILL.md: 1 lesson(s) added (1 total)

$ brief find "go test package mismatch"
go-house

$ ply -sh -s go-house -check 'go test ./...' "add a ring buffer"
```

There is no index, no database, no embedding model, no vector store and no
daemon. The loop closes as a shell pipeline rather than a runtime:

```sh
ply -sh -s "$(brief find "$task")" -check "$check" "$task"   # act
hone -into house                                            # learn
brief find "$next"                                           # recall
```

Every write is a delta. Lessons are appended under a `## Lessons` heading
and what is already there is never regenerated — a model handed its own
accumulated notes and asked for the next version rewrites them, and
rewriting compresses away the specifics that made them worth keeping.

## The skill that was in play

`ply -s house` loads a procedure, and `brief cat` prints a body *without its
frontmatter* — so a skill reaches the system prompt as anonymous prose. What
shaped a run was provable byte for byte; which of those bytes were a skill
called `house` lived only on stderr.

`ply` records it now, and `-into -` reads it back:

```
$ hone -into -
hone: wire: the run was following house -- the lesson belongs to it
~/.claude/skills/house/SKILL.md: 1 lesson(s) added (3 total)
```

This is the joint that makes the family a loop rather than a line. A run
that loaded a procedure and stumbled **anyway** is not merely a run that
stumbled — it is evidence that procedure is incomplete, and the lesson
belongs to it rather than to wherever you happened to point `-into`. The
model is told so, and words the lesson as an amendment:

```markdown
## Lessons

- Run `make check`, not `go test`, to verify work here.

- Every source file must begin with the project's SPDX header on line 1;
  `make check` rejects files with `add.go: line 1 must be the SPDX header`.
<!-- hone wire find-20260802-104011-290336 -->
```

A run that loaded no skill, or more than one, is refused rather than guessed
at. And there is a second reading worth watching for: a skill accumulating
lessons faster than it is used is a **pressure gauge**, not a growing asset.
The procedure itself is missing a step, and the fix is to rewrite it — which
is a goal with a check, which is `ply`.

## Provenance is the feature

```markdown
## Lessons

- Go files in this module root declare `package main`, matching
  `add_test.go`; `package x` fails at setup with "found packages x".
<!-- hone 20260801-230441-4c8100cf68ff97f5 20260802-024255-9f1a2b3c -->
```

The first id is the run it was learned from, the second the call that worded
it. Both replay:

```
$ ask replay -check ~/.ask/sessions/20260801-230441-4c8100cf68ff97f5.jsonl
ok: replays exactly (18 events)
```

Which is what makes a bad lesson something to **delete** rather than argue
with. That 100% relapse rate came from teams trying to correct a poisoned
memory *in conversation*. Here it is a file with a name on it:

```
$ hone forget 20260801-230441-4c8100cf68ff97f5 go-house
hone: ~/.claude/skills/go-house/SKILL.md: forgot 2 lesson(s)
```

The unit is a run, because a session that taught one wrong thing usually
taught its neighbours too.

## Bounded by construction

A run teaches once. The mark records it, so a second pass over an archive
costs nothing — not even a model call:

```
$ hone -into house run.jsonl
hone: run: already learned from, in house -- hone forget run house to
       learn it again
```

Nothing else bounds it, because nothing else needs to: the only thing that
enters the corpus is a recovery from a verified run, `-n` caps one run at
three lessons, and a skill has the size budget the specification already
sets and `brief lint` already warns about.

Refining a skill that has grown is a goal with a check, which is to say it
is `ply`:

```
$ ply -check 'brief lint -strict go-house' "merge duplicate lessons in go-house/SKILL.md"
```

There is no verb for that here, because there is already a program for it.

## Read the evidence first

A lesson is a claim. `-why` prints what it would be drawn from and calls no
model:

```
$ hone -why
GOAL
make the test pass

CHECK (this passed, so the work was done)
go test ./... 2>&1

STUMBLE 1
this failed:
$ cat > add.go <<'EOF'
package x
...
it printed:
found packages x (add.go) and main (add_test.go)
FAIL	x [setup failed]
exit 1
then this was done, and worked:
$ sed -i.bak 's/^package x$/package main/' add.go
```

`-N` goes one step further: it words the lesson and writes nothing.

## The Unix contract

| stream | carries |
| --- | --- |
| stdout | the lesson, or the path written — the answer, and nothing else |
| stderr | which session taught it, why a run was refused |
| exit 0 | yes: something was learned |
| exit 1 | no: nothing to learn |
| exit 2 | error: bad usage, unreadable session, `ask` failed |

`grep`'s contract, not `ask`'s, because `hone` asks a question where no is
a real answer — and it is the common one:

```sh
for s in ~/.ask/sessions/*.jsonl; do hone -into house "$s" || continue; done
```

## Commands

```
hone [flags] [session ...]       distil what a run teaches
hone forget <id> <skill>...      remove what a run taught
hone prompt                      print the system prompt that words a lesson
hone version                     print the version
hone help                        print the summary
```

Flags come before sessions: after one, a word is a filename. Full reference
in `hone.1`, and `hone help` fits eighty columns.

## The prompt is a value

`hone prompt` prints exactly what is sent, so you can read the bar a lesson
has to clear before it is written down — four tests, and a paragraph
explaining that `none` is the expected answer.

## Deliberately absent

No embedding model, no vector store, no index, no database, no daemon, no
config file, no MCP, no scheduler, no memory types, no consolidation pass,
and no automatic learning. Nothing writes to a skill unless somebody typed a
command that says to.

That last one is load-bearing. A system that learns silently is a system
that silently learns the wrong thing, and the failure mode is a confident
agent citing a precedent nobody wrote.
