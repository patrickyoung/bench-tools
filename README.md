# draft

Design a system built from `ask`, `brief`, `ply` and `hone`. Get a document
you can argue with, and a program's word on whether it is buildable.

```
$ draft new hn "pull the YC feed, keep a history, report on AI topics"
draft: drafting hn/DESIGN.md
draft: ask replay -check hn/.draft/design.jsonl
draft: read it, argue with it, then draft check hn
hn/DESIGN.md

$ draft check hn
draft: hn/DESIGN.md is buildable
./bin/check

$ draft build hn
```

It is a shell script, and that is the design rather than an economy. The
other four are Go programs because each had something genuinely hard in it:
provider adapters and their reasoning round-trips, a search path with
shadowing, a loop that runs what a model wrote, arithmetic over exit
statuses. `draft` has nothing hard in it. It composes, and composition in
this family is a file.

```
draft   the design      — no model, no loop, no catalogue
ask     the model       — no tools, no loop
brief   the procedure   — no model, no loop
ply     the loop        — no model, no procedure
hone    the lesson      — no store, no retrieval, no format
```

You draft it, you brief it, you ask it, it plies, and it hones.

## The one idea

**A design that cannot say how you would know it worked is not a design.**

So `DESIGN.md` carries a `## Check` section naming a shell command whose
exit status is the verdict, and everything here is arranged around that:

```
$ draft check t1
draft: t1/DESIGN.md: the Check section is still the template's `false`
draft: t1/DESIGN.md: an empty requirement checkbox is left in the template
$ echo $?
1
```

That refusal is the most useful thing in the program. A requirement you
cannot check is a wish, and if you cannot name the command then the
requirement is not understood yet — which is information, and it belongs in
the document rather than being papered over.

`draft build` hands that same command to `ply` as `-check`, so the thing
that decided the design was finished is the thing that decides the build is.
One sentence, one place, no second opinion.

For work where the builder must not be able to rewrite that opinion, admit it
before building:

```sh
draft admit project
draft build -admitted project
```

`admit` sends the canonical project path, the normalized Check bytes, and
their SHA-256 digest to `may`. Only after the operator approves those exact
bytes does Draft store a content-addressed verifier under the operator's state
directory (`$XDG_STATE_HOME/draft/verifiers`, or
`~/.local/state/draft/verifiers`). State inside the project or system temporary
directory is refused because Cage deliberately grants both write access.
`build -admitted` refuses a missing or changed receipt and runs Ply
through `cage -net -w PROJECT`; the project and temporary directory stay
writable, while the admitted verifier is read-only to the worker. Network is
deliberately enabled because Ask needs its provider connection—Cage is the
write boundary here.

Admission freezes the shell program in the Check block. It cannot freeze the
meaning of writable programs or fixtures that program calls. Put the semantic
assertions directly in the block or in operator-controlled programs, and use
`draft prove` to measure whether the resulting oracle notices broken code.

## But is the check any good?

`draft check` says a design names a check. `ply` says the check passes.
Neither says the check is worth anything — and it is written by the same
model that wrote the code, so it only ever tests the states that model
imagined.

`draft prove` breaks the code on purpose and reports what the check did not
notice:

```
$ draft prove -n 46 hn
draft: check passes clean in 2s; mutating with a 15s bound
draft: 46 sites; measuring all of them
lib/hn_topics/algolia.py:59: <= -> < survived
    if oldest<=lower: break
lib/hn_topics/cli.py:83: >= -> > survived
    if number>=max_batches:break
draft: killed 12 of 46 (26%) (1 by hanging), 34 survived
```

A change the check survives is a hole in the check. **No model is called** —
this is a program's opinion, which is the standard everything else here is
held to.

The mutations are few and semantic: boundary and equality flips, where
off-by-one and empty-state bugs live. They almost never break syntax, and
that matters — a mutant that fails to parse is killed by any check that runs
a compiler, so counting those would flatter a check that does nothing else.
A survivor here is unambiguous.

Three things it is careful about, each learned by getting them wrong:

- **Your source is never at risk.** Writes go through a sibling temporary
  and a rename, because `sed … > file` truncates the target as the shell
  opens it — a kill in that window once left a source file at zero bytes.
  The original is also journalled inside the project, so even an
  uncatchable kill is recovered on the next run.
- **A hanging mutant is killed, not waited on.** Removing a loop's
  termination condition makes a check run forever rather than fail. Each
  run is bounded by five times the clean run, in its own process group so
  the kill reaches the interpreter and not just the shell. A hang is a
  difference the check detected.
- **The sample spans the codebase.** `-n` collects every site first and
  takes them evenly, because walking files in order until the cap trips
  measures your alphabetically-first file and calls it a project.

The score is a signal, not a target. Some mutants are equivalent and nothing
can kill them. Read the distribution instead: a file with a cluster of
survivors is a file whose tests are decorative.

## Install

```
go install github.com/patrickyoung/ask@latest
go install github.com/patrickyoung/brief@latest
go install github.com/patrickyoung/ply@latest
go install github.com/patrickyoung/hone@latest

ln -s "$PWD/bin/draft" ~/bin/draft
ln -s "$PWD/skills/draft" ~/.claude/skills/draft
draft sync
```

`draft` holds no credentials and no provider code. Models and keys belong to
`ask`. A missing tool is an error that says what to install, all four at
once, because being told about one at a time is four trips through a shell.

## Perfect knowledge, without remembering anything

A hand-written reference to the four tools is a promise to keep it true by
hand, and nobody does. So it is generated:

```
$ draft sync
draft: wrote skills/draft/references/tools.md from the installed binaries
```

That file is `ask help`, `brief help`, `ply help`, `hone help`, `ask system`
and every version string, collected from the binaries actually on this
machine. `draft check` regenerates it into a temporary file and compares — so
**staleness is a program's opinion**, like everything else here, and a design
is never planned against a tool that has moved on.

It is the same code path `sync` uses. Two ways to decide whether something
had drifted would eventually disagree.

## The document

`draft new` writes a template with the questions in it. Two sections carry
more weight than the rest.

**The split** is where a design is won or lost:

| stage | tool | why |
| --- | --- | --- |
| fetch, append, schedule, move bytes | none — a script | no judgment in it |
| classify, extract, rank, summarize | `ask` | judgment, answerable in one question |
| until a program agrees it is done | `ply` | the answer must satisfy a check |

The default is the first row, and each move down it has to be earned. Most
of a working system is not an agent problem, and reaching for the loop where
`curl` would do is slow, costly and nondeterministic in exchange for
nothing. A design whose rows are all `ply` is wrong.

**Not doing** is worth more than Requirements. A system is defined by what it
refuses, and writing the refusals down with their reasons is what stops them
being quietly re-added six months later.

## With a description, a model fills it in first

```
$ draft new hn "pull the YC feed, keep a history, report on AI topics"
```

`ask` drafts the document with the `draft` skill in its system prompt, in a
session of its own under `hn/.draft` — never the conversation you are having.
It is `-n`, always, because a scheduled or scripted `ask` that continues your
terminal's thread is the sharpest edge in the family.

The session replays, because a design a model drafted is a decision it made
on your behalf:

```
$ ask replay -check hn/.draft/design.jsonl
```

Then you read it and argue with it. That is the interview: there is no
wizard, no chat, no questionnaire. The artifact is a document, and a document
is a thing two people can disagree about precisely.

## The Unix contract

| stream | carries |
| --- | --- |
| stdout | the path written, or the check extracted — the answer, and nothing else |
| stderr | what was refused and why, which session recorded a decision |
| exit 0 | yes: written, buildable, built |
| exit 1 | no: not buildable, nothing to do |
| exit 2 | error: bad usage, missing tools, unreadable design |
| exit 3 / 75 | May declined / parked an admission for a human |
| exit 125 | Cage could not establish the admitted-build boundary |

`grep`'s contract, not `ask`'s, because `draft check` asks a question where
*no* is a real answer and often the right one.

## Commands

```
draft new <dir> [description ...]  scaffold a project and its DESIGN.md
draft check [dir]                  is this design buildable? (exit 1 = no)
draft admit [dir]                  approve and freeze its verifier
draft prove [-n N] [dir]           break the code; does the check notice?
draft build [-admitted] [dir]      build it; optionally require admission
draft sync                         regenerate the tool reference
draft tools                        print what the family can currently do
draft version                      print the version
draft help                         print the summary
```

## Deliberately absent

No binary, daemon, config file, cache, registry, templates for the system
being built, provider code, or capability of its own. Ordinary design and
build remain stateless. The one deliberate state is an operator-approved,
content-addressed verifier used only by `admit` and `build -admitted`.

The interview is the one worth explaining. `ask` cannot ask a clarifying
question — nothing is listening on stdin — and `ply` gives every command
`/dev/null`. That looked like a limitation until it was the answer: the
human in the middle *is* the interview, the document is what they argue with,
and `draft check` is what settles it.

## Contributing

```
sh bin/draft_test.sh
```

No test calls a model or reaches the network. The suite needs the four tools
on PATH and says so once, at the top, rather than surfacing as eleven
unrelated failures — a partial run reporting "21 passed" reads like success,
and that is worse than a refusal.

[DESIGN.md](DESIGN.md) is `draft`'s own design, written in `draft`'s own
template, and `draft check .` passes on it. If that ever stops being true,
one of the two is wrong.

## License

Draft is available under the [MIT License](LICENSE).
