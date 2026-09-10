# The hone field guide

What it is good at, what it is not, and the things that will bite you.

## What you actually get

Hone keeps lessons from checked recoveries. Runs without that evidence are
refused before a model words a lesson. For example, an archive may contain:

```
$ hone -into house "${PLY_DIR:-$HOME/.ply/sessions}"
hone: 20260801-1712: no check ran, so nothing judged it but the model
hone: 20260801-1715: the check passed and nothing ever failed
hone: 20260801-2147: nothing worth keeping
~/.claude/skills/house/SKILL.md: 1 lesson(s) added (12 total)
```

This is illustrative output, not a prediction of how many lessons your work
will produce. Watch for repeated or contradictory lessons. Repeated advice
may belong in the procedure itself; the count alone does not measure quality.
To revise an existing, already valid skill in your project:

```
ply -sh -B -check 'brief lint -strict .claude/skills/house' \
  'Merge duplicate lessons in .claude/skills/house/SKILL.md; preserve provenance marks.'
```

`-sh` grants shell execution. `-B` requests work even if the initial lint check
would pass. Lint checks the format; review the resulting diff to judge whether
the meaning and useful details were preserved.

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

The retained evidence has no final passing check, so Hone has no verified
resolution to learn from. If you fixed the work yourself, rerun the original
check against the same recorded task. For example, if `run.jsonl` already
contains the failed attempt:

```sh
ply -sh -f run.jsonl -check 'go test ./...' 'Confirm the repair.'
hone -why run.jsonl
```

A passing pre-check records its verdict in that existing session without a
worker model call. Hone still requires qualifying failure evidence; a fresh
session containing only a pass is not a recovery.

```
hone: the check passed and nothing ever failed
```

No failure/recovery pair is recorded, so Hone has no lesson to extract.

## Read before you believe

`-why` prints the evidence and calls no model, so it costs nothing:

```
$ hone -why run.jsonl
```

Do this the first few times. What you are checking is whether the stumbles
are *real* — a genuine wrong turn — or exploration noise like a `sed` on a
file that did not exist yet. Noise is fine; the model is asked to discard
it, and mostly does. But if every stumble in a session is noise, a lesson
drawn from it will be noise dressed as guidance.

`-N` goes one step further: it calls a model to preview the lesson without
changing a skill. That wording call still creates an Ask session under
`$HONE_DIR` (default `~/.hone/lessons`). To review exact bytes for later
admission, use [prepare, show, and admit](README.md#review-the-exact-lesson-before-saving).

## The three things that will bite you

**1. Flags come before sessions.**

```
$ hone run.jsonl -into house
hone: -into came after a session, where it is a filename, not a flag: put
flags first
```

This is an intentionally incorrect command. Hone stops parsing flags at the
first session path and reports misplaced flags. The correct order is:

```sh
hone -into house run.jsonl
```

**2. `hone` with no arguments reads your *current* conversation.**

Hone uses Ask's `current` pointer in `$ASK_DIR`, defaulting to
`~/.ask/sessions`, or the newest session there if no usable pointer exists.
Ply normally writes separate sessions under `$PLY_DIR`, defaulting to
`~/.ply/sessions`, and does not move Ask's pointer. Plain Ask starts a new
conversation and moves that pointer. Ask's `-f` creates or continues its named
file without moving the pointer.
Name the Ply session explicitly:

```
$ ply -sh -check 'make test' -f ./run.jsonl "fix it" && hone -into house ./run.jsonl
```

Keeping the session in a file of your own is the habit worth forming.

**3. A run teaches once, and the mark is why.**

When `-into` has already recorded a source run in that destination skill,
Hone skips it before another wording call. Runs that produced no saved lesson,
previews with `-N`, and a different destination do not have that mark and may
call the model again. To remove the saved lessons from one source before
reconsidering it:

```
$ hone forget 20260801-230441-4c8100cf68ff97f5 house
$ hone -into house ~/.ply/sessions/20260801-230441-4c8100cf68ff97f5.jsonl
```

## Recipes

**The loop, as a shell function.** Work a goal, then learn from it:

```sh
work() (
  task=$1 check=$2 skill=${3:-house}
  run_dir=$(mktemp -d ./work-run.XXXXXX) || exit 2
  printf 'Run directory: %s\n' "$run_dir" >&2
  if ply -sh -s "$skill" -check "$check" -f "$run_dir/run.jsonl" "$task"; then
    work_status=0
  else
    work_status=$?
  fi
  if [ -f "$run_dir/run.jsonl" ]; then
    if hone -into "$skill" "$run_dir/run.jsonl"; then
      :
    else
      lesson_status=$?
      if [ "$lesson_status" -ne 1 ]; then
        printf 'Hone failed with status %s; inspect the run.\n' "$lesson_status" >&2
      fi
    fi
  fi
  exit "$work_status"
)

work "make the flaky test deterministic" 'go test -count=5 ./...'
```

The function returns Ply's outcome. Hone's ordinary “nothing to learn” status
does not fail the task; a learning error is reported separately. Each call
uses a fresh record directory in the project so unrelated tasks do not share
a conversation or an already-learned source ID. The named skill must exist.

**Learn from a whole archive.** Already marked runs are skipped; other
qualifying runs may still need a model call:

```sh
hone -into house "${PLY_DIR:-$HOME/.ply/sessions}"
```

**Only from today.** A directory is every session in it, so narrow with the
shell rather than a flag:

```sh
hone -into house "${PLY_DIR:-$HOME/.ply/sessions}/$(date +%Y%m%d)"-*.jsonl
```

This filename pattern matches Ply's default local-date naming. If no files
match, choose a real session path instead of passing the unmatched pattern.

**Choose a model for the wording.** Replace the placeholder with a model your
provider account supports, then evaluate its lessons before changing defaults:

```sh
hone -m anthropic/YOUR_MODEL_ID -into house run.jsonl
```

**Check the format in CI.** Strict lint reports format errors and warnings;
it does not establish that a lesson is correct or useful:

```sh
brief lint -strict .claude/skills/house || exit 1
```

**Refine one that has grown.** This is a goal with a check, so it is `ply`:

```sh
ply -sh -B -check 'brief lint -strict .claude/skills/house' \
    "merge duplicate lessons in .claude/skills/house/SKILL.md; keep every
     <!-- hone --> comment with the lesson it belongs to"
```

Keep the marks so `hone forget` can find the lessons from one source run.
`-B` deliberately bypasses an already passing initial check. Review the diff;
the final lint check only validates format.

## Reading a skill you did not write

Every lesson names its run, and every run replays:

```
$ grep -A1 'package main' .claude/skills/house/SKILL.md
- Files added beside `add_test.go` must declare `package main`; …
<!-- hone 20260801-230441-4c8100cf68ff97f5 20260802-024133-9f1a2b3c -->

$ ask replay ~/.ply/sessions/20260801-230441-4c8100cf68ff97f5.jsonl | less
$ ask replay -check ~/.hone/lessons/20260802-024133-9f1a2b3c.jsonl
```

The first id shows you the work the lesson came from. The second shows you
the exact call that worded it, including the evidence it was given. If a
lesson looks wrong, inspect both records and the underlying work. Replay
checks retained-record consistency; it does not prove that the lesson is true.

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
