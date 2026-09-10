# The hone field guide

What it is good at, what it is not, and the things that will bite you.

## What you actually get

Not much, and that is the product. Over a working week of `ply` runs, expect
most of them to teach nothing:

```
$ for s in ~/.ask/sessions/*.jsonl; do hone -q -into house "$s"; done 2>&1 | tail
hone: 20260801-1712: no check ran, so nothing judged it but the model
hone: 20260801-1715: the check passed and nothing ever failed
hone: 20260801-2147: nothing worth keeping
~/.claude/skills/house/SKILL.md: 1 lesson(s) added (12 total)
```

That trailing count is the one number worth watching, and it is why it is
printed. A skill with fifteen lessons in it after a month is a good outcome.
A skill with three hundred is a broken tool, and the numbers behind that are
in the README.

It reads two ways, and the second is the useful one. Lessons accumulating
slowly is a procedure being refined. Lessons accumulating *faster than the
skill is used* is a procedure missing a step — the run keeps stumbling in
the same place, and each stumble is being written down instead of fixed.
The fix is to rewrite the procedure, which is a goal with a check, which is
`ply`:

```
ply -check 'brief lint -strict house' "merge the duplicate lessons in house/SKILL.md"
```

There is no verb here for that, and there should not be one.

## The three refusals, and what to do about each

```
hone: no check ran, so nothing judged it but the model
```

The run had no `-check`, so nothing but the model believes it worked. This
is the common one when you start, because `ply` without a check is easier to
type. There is nothing to fix after the fact — the evidence was never
created. Run with a check next time.

```
hone: the check never passed
```

The run gave up. There genuinely is no lesson here: you know what did not
work, but not what does, and "X did not work" written down as guidance is
how an agent learns to avoid the right answer. If you fixed it yourself
afterwards, the run that *proves* the fix is the one worth honing — and
`ply -B -check '...' "confirm"` is a cheap way to make one.

```
hone: the check passed and nothing ever failed
```

The model knew what to do. Nothing to learn, and that is the healthy case.

## Read before you believe

`-why` prints the evidence and calls no model, so it costs nothing:

```
$ hone -why | less
```

Do this the first few times. What you are checking is whether the stumbles
are *real* — a genuine wrong turn — or exploration noise like a `sed` on a
file that did not exist yet. Noise is fine; the model is asked to discard
it, and mostly does. But if every stumble in a session is noise, a lesson
drawn from it will be noise dressed as guidance.

`-N` goes one step further: it words the lesson and writes nothing.

## The three things that will bite you

**1. Flags come before sessions.**

```
$ hone run.jsonl -into house
hone: -into came after a session, where it is a filename, not a flag: put
flags first
```

Go's flag package stops at the first non-flag argument, the same as every
other program in this family. It says so rather than failing to open a file
called `-into`, but it is still the mistake you will make twice.

**2. `hone` with no arguments reads your *current* conversation.**

That is `ask`'s convention and it is usually what you want right after a
`ply` run. It is not what you want if you have since asked `ask` a question,
because that moved `current`. Name the session, or `-d` its directory:

```
$ ply -sh -check 'make test' -f ./run.jsonl "fix it" && hone -into house ./run.jsonl
```

Keeping the session in a file of your own is the habit worth forming.

**3. A run teaches once, and the mark is why.**

Re-running `hone` over the same archive is free and does nothing. If you
*want* it to learn again — you changed the prompt, or you switched models —
forget first:

```
$ hone forget 20260801-230441-4c8100cf68ff97f5 house
$ hone -into house ~/.ask/sessions/20260801-230441-4c8100cf68ff97f5.jsonl
```

## Recipes

**The loop, as a shell function.** Work a goal, then learn from it:

```sh
work() {
  local task=$1 check=$2 skill=${3:-house}
  ply -sh -s "$skill" -check "$check" -f ./run.jsonl "$task"
  local r=$?
  hone -q -into "$skill" ./run.jsonl
  return $r
}

work "make the flaky test deterministic" 'go test -count=5 ./...'
```

`hone`'s exit status is deliberately ignored: it is 1 most of the time, and
that must not fail the function.

**Learn from a whole archive, once.** Safe to re-run; marks make the second
pass free:

```sh
hone -q -into house ~/.ask/sessions
```

**Only from today.** A directory is every session in it, so narrow with the
shell rather than a flag:

```sh
hone -into house ~/.ask/sessions/$(date +%Y%m%d)-*.jsonl
```

**A cheaper model for the wording.** Distilling a stumble is a small job:

```sh
hone -m anthropic/claude-haiku-4-5 -into house
```

**Keep the corpus honest in CI.** A skill that stops linting is a skill that
will stop loading:

```sh
brief lint -strict .claude/skills/house || exit 1
```

**Refine one that has grown.** This is a goal with a check, so it is `ply`:

```sh
ply -check 'brief lint -strict .claude/skills/house' \
    "merge duplicate lessons in .claude/skills/house/SKILL.md; keep every
     <!-- hone --> comment with the lesson it belongs to"
```

Keep the marks. They are the only reason a bad lesson can be deleted rather
than argued with.

## Reading a skill you did not write

Every lesson names its run, and every run replays:

```
$ grep -A1 'package main' .claude/skills/house/SKILL.md
- Files added beside `add_test.go` must declare `package main`; …
<!-- hone 20260801-230441-4c8100cf68ff97f5 20260802-024133-9f1a2b3c -->

$ ask replay ~/.ask/sessions/20260801-230441-4c8100cf68ff97f5.jsonl | less
$ ask replay -check ~/.hone/lessons/20260802-024133-9f1a2b3c.jsonl
```

The first id shows you the work the lesson came from. The second shows you
the exact call that worded it, including the evidence it was given. If a
lesson looks wrong, one of those two will show you why in about a minute.

## What it is not

It is not a memory of your conversations. It will not remember your name,
your preferences, or what you said last Tuesday — that is a different
product with a different failure mode (staleness), and there are vendors
selling it.

It is not automatic. There is no hook and no watcher, and there will not be
one. What a system learns silently, it learns wrongly in exactly the cases
you would most want to catch.

It is not a summarizer. `ask compact` writes a handoff note for a
conversation that filled its window; `hone` writes a claim about how work
is done in a place. If you want the former, the verb already exists.
