# ply

Give a model a goal and a toolbox. Get the work done, and a program's word
that it was.

```
$ ply -sh -check 'go test ./...' "make the tests pass" && make release
```

That `&&` means something, because exit 0 is `go test`'s opinion and not the
model's.

`ply` is an agent loop built as a filter. It asks a model, runs the shell
commands the model writes, hands back what they printed, and repeats — then
runs the check, and hands its failure back if it failed. The answer is
stdout, the typescript is stderr, the exit code says what happened.

Two ideas hold it up, and both are older than any of this:

**A tool is a program, and the toolbox is `$PATH`.** Not a JSON schema. Not
a plugin. Not a registry.

**Done is a program's opinion.** `make` doesn't ask an oracle whether the
build worked; it runs the compiler and checks the status.

## Install

```
go install github.com/patrickyoung/ply@latest
```

Go 1.26 or newer, and a Unix. It needs [`ask`][ask] on `$PATH` — that is
where the model, the keys and the log live — and [`brief`][brief] only if
you use `-s`.

```
export ASK_MODEL=anthropic/claude-sonnet-5
export ANTHROPIC_API_KEY=...
```

[ask]: https://github.com/patrickyoung/ask
[brief]: https://github.com/patrickyoung/brief

**[GUIDE.md](GUIDE.md)** is the field guide: what `ply` is good at, what it
is not, the recipes, and the six things that will bite you — including the
one that bit the person who wrote it (an `ask` inside a check continues
*your* conversation unless you say `-n -f`).

## The family

    ask     the model         — no tools, no loop
    brief   the procedure     — no model, no loop
    ply     the loop          — no model, no procedure
    hone    the lesson        — no store, no retrieval, no format

`ask` is a model and no loop. `brief` is a procedure and no model. Between
them there was nothing that *acts*, and `ply` is that and nothing else: it
has no provider code, no credentials, no session format and no catalogue.
When it needs a model it runs `ask`. When it needs a procedure it runs
`brief`. Each refuses to grow the others, which is why all of them stay
small.

[`hone`][hone] closes it into a circle. A `ply` run that failed, recovered,
and was then confirmed done by the check is the one kind of run that teaches
something — so `hone` reads the log, writes the lesson down as a `brief`
skill, and the next run loads it with `-s`. It has no store of its own
either: a lesson is a skill, and `brief` was already the catalogue.

You brief it, you ask it, it plies, and it hones.

## A tool is a program

Every agent in the world defines tools as JSON schemas: `read`, `write`,
`edit`, `grep`, `list`. Those are `cat`, `tee`, `sed`, `grep` and `ls` with
a worse interface, re-implemented once per agent, and each one is a fresh
place to have a bug that `grep` does not have.

Unix already shipped the tools. What it never shipped was a way to hand a
model *some* of them.

```
$ mkdir tools && ln -s $(which git rg sed) tools/
$ ply -t tools -check 'git diff --quiet' "stage the obvious typo fixes"
```

`-t` makes `$PATH` that directory **and nothing else**. The model can reach
what you put there and cannot name what you did not. Adding a tool is `ln
-s`. Removing one is `rm`. There is no manifest to keep in sync, nothing to
version, and no second way to name a tool.

Your own programs go in the same directory, and say what they are the way
programs always have:

```sh
#!/bin/sh
# ship the current branch to staging
```

That comment is the catalogue. `ply tools` prints exactly what the model
gets:

```
$ ply tools -t tools
Your tools are the programs on PATH, and PATH holds these and
nothing else:

  deploy  ship the current branch to staging
  git
  rg
  sed

Run `name -h` to learn one you do not know. Shell builtins work as usual.
```

That is level 1 of progressive disclosure. Level 2 is `deploy -h`. Level 3
is running it. Neither needed building, because that is how programs work.

`-sh` hands over the whole machine instead, for when you mean it.

The command interpreter is a separate choice from that tool grant. Commands
and checks use `/bin/sh -c` by default. `-shell executable` selects another
interpreter that accepts `-c`; `$PLY_SHELL` provides the same Ply-specific
default. Ply resolves it before calling the model, uses it consistently for
blocks and checks, and names the resolved executable in the prompt. It never
inherits `$SHELL`, which is an interactive preference and may name a
non-POSIX shell.

```sh
ply -sh -shell /opt/homebrew/bin/bash "use modern Bash where useful"
```

Fence labels remain protocol markers: a `bash` or `zsh` fence does not switch
interpreters. If an interpreter needs fixed options, put them in a wrapper
program and give that one program to `-shell`; Ply does not parse a second
command line inside the flag.

### "But does it do MCP?"

No, and it does not need to. An MCP server is a tool cabinet and a bridge
CLI is the key, so MCP arrives the way everything else does — as a program:

```
$ mcpbox tools/ npx -y @modelcontextprotocol/server-everything
mcpbox: wrote 13 programs to tools
$ rm tools/get-env                        # bless by deleting
$ ply -t tools "add 17 and 25, put the number in answer.txt"
$ get-sum -h                              # ...which the model found itself
```

`tools/list` already returns a name, a sentence and a JSON schema — which
is exactly a synopsis, a `-h`, and a call — so
[`contrib/mcpbox`](contrib/mcpbox) turns the manifest into a directory of
programs. After that an MCP tool sits beside `git` and `sed` and is not a
special kind of thing any more, and `-t` makes the blessing real: the model
cannot name what you deleted. `GUIDE.md` has the pitfalls.

### "But how does it edit a file?"

With a program, and for most of what a model does the program is `>`. A
file it is writing whole is `cat > x <<'EOF'`, and a shell has always had
that. The awkward case is the other one — three lines in the middle of
nine hundred — where rewriting the file is not on and `sed` needs an
expression escaped out of the very code being changed.

[`contrib/edit`](contrib/edit) is that program. It replaces text by
matching it, exactly and exactly once:

```
$ edit ring.go 'r.head = r.head + 1' 'r.head = (r.head + 1) % len(r.buf)'
ring.go: 1 replacement
```

Nothing is fuzzy-matched, ever, and every edit in a call is located before
any file is written — so a call that cannot be satisfied changes nothing,
across as many files as it names. What a fuzzy matcher would have guessed
at, this reports instead:

```
$ edit ring.go '    r.n++' '    r.n += 2'
edit: ring.go: the search text was not found. It matches at line 6 once
whitespace is ignored, so the difference is tabs: the file indents with
tabs and the search text uses spaces.
```

It reads both edit dialects models actually write — `<<<<<<< SEARCH` blocks
and `*** Begin Patch` hunks — because which one a model reaches for is
trained in, and refusing the one it knows costs a turn and ends in a
hand-rolled `sed`. `GUIDE.md` has the toolbox recipe and the one trap
(`-t` means PATH is the toolbox alone, so link `python3` in beside it).

> **The toolbox aims the model; it does not sandbox it.** `sh` has builtins,
> and a redirect opens a file with no program involved. The security
> boundary is the process — its user, its container, its `chroot` — as it
> always was. Run `ply` somewhere you would be willing to hand a shell,
> because that is what you are doing.

## Done is a program's opinion

```
$ ply -sh -check 'go test ./...' "make the tests pass"
```

The loop is: ask → run what it wrote → repeat until it stops → run the
check → if it failed, hand the failure back → go again. `-cycles` bounds
that; exit 2 means it ran out.

The check runs **before** the first turn too, so a goal already met costs
nothing and leaves no session behind:

```
$ ply -sh -check 'go test ./...' "make the tests pass"
ply: check passed: go test ./...
ply: nothing to do
$ echo $?
0
```

That is `make`'s oldest manner, and it is what makes `ply` safe in a git
hook, a `Makefile`, or a loop.

When that first check fails, its terminal output is not thrown away. It
rides with the goal in the first model turn and therefore lands in the Ask
session as the run's first evidence. It does not count against `-cycles`:
that bound still counts only checks after the model has had a chance to work.

The check is **yours**, not the model's, so it runs with your `$PATH` — with
the toolbox merely first on it. Scoping it the way the model is scoped would
mean `go test ./...` needed a toolbox holding `go`, `git` and a linker.

When the judgment genuinely needs a model, nothing about the mechanism
changes — because `ask` is a program:

```
ply -sh -check 'ask -n -q -f /tmp/judge.jsonl "Does this cover install?
    Answer only yes or no." < README.md | grep -qi "^yes"' "document the installer"
```

(`-n -f` gives the judge its own thread — `ask` continues *your* conversation
otherwise, which is the sharpest edge in the whole system and has its own
section in the guide.)

Without `-check`, exit 0 means only that the model stopped. That is a real
answer for a goal no program can judge, and it is a weaker one than it
looks. The default prompt says so to the model, too: *nothing checks this
work but you; run the command that would show somebody else you are right.*

## The protocol is a terminal session

The model runs a command by writing a fenced shell block:

````
```ply
go test ./... 2>&1 | tail -20
```
````

An assistant turn is one action or one report. An action is optional leading
prose followed by exactly one nonempty fenced shell block as the final content
of the turn. Ply executes that one shell program and returns its result before
the model can continue. A report contains no shell block and ends the loop.

This is enforced rather than suggested. Ply consumes only the first complete
command block. Later blocks and trailing prose are explicitly deferred and do
not run; the first result comes back before the model chooses again. An empty
or unfinished first block runs nothing and receives a correction. Commands
that do not require observation can still be lines in one shell script;
dependent work must wait for the next turn. The transcript therefore never
pretends the model observed output that did not exist yet, without making
useful work depend on perfect response formatting.

There is no way to write a fenced shell block that is merely quoted. Indent it
to quote it; that is the whole escape hatch. Blocks and checks run under the
resolved `-shell` interpreter, `/bin/sh` by default. The prompt names that
executable; fence labels do not select a different one. What comes back is
what a terminal would have shown:

```
$ go test ./... 2>&1 | tail -20
--- FAIL: TestWrap (0.00s)
    ring_test.go:41: want 3, got 2
FAIL
exit 1
```

A report with no block ends the run, and that report is the answer.

The default prompt reads a goal by its outcome. An answer, review or diagnosis
means inspecting relevant evidence and reporting it without unrequested
changes. A requested artifact or system change means using programs to make
the effect real, then inspecting the resulting state. Text in the final reply
is only a report; it is not a substitute for writing the file or doing the
work.

That is guidance for the loop, not a second verdict. Ply does not mistake
"some command ran" for completion: without `-check`, exit 0 still means only
that the model stopped. When completion has an executable meaning, give it to
`-check`.

Two things fall out of choosing text over a tool-use API. It works with
**any model `ask` can reach**, including ones whose tool support is an
afterthought. And the trace is a typescript — the one format every model
has read a million of, and every human already knows.

## The Unix contract

| stream | carries |
| --- | --- |
| stdout | the answer, and nothing else |
| stderr | the typescript: what ran, what it printed, what it exited |
| exit 0 | done — the check passed, or, with no check, the model stopped |
| exit 1 | error — usage, no `ask`, a provider failure |
| exit 2 | not done — check failing, a bound tripped, protocol stalled, context full |
| exit 130 | interrupted |

Exit 2 earns its row, as it does in `ask`: a supervisor has to tell "did not
work" from "broke", or it retries the wrong one.

Commands run with stdin on `/dev/null`, in their own process group, under
`-timeout` — and a kill reports **124**, `timeout(1)`'s number. Output is
capped per command, keeping both ends, and the elision is announced *in the
text the model reads*, because truncation it cannot see is the one failure
nothing downstream can detect.

An invocation gives up after 50 model turns by default, including a model
that keeps emitting commands and never stops for the check. `-turns 0`
deliberately removes that bound. An empty or unfinished first command block is
returned for correction twice; a third malformed reply stops at exit 2 instead
of being guessed at or mistaken for unchecked completion.

## The log is somebody else's problem

`ply` keeps no log. The conversation is an `ask` session: each consumed action
is in an assistant turn, its output and any explicit deferral are in the next
user turn, so the file is the entire run — and `ask` already proves those.

```
$ ask replay -check ~/.ply/sessions/20260801-142233-a3f9c1e0.jsonl
ok: 20260801-142233-a3f9c1e0.jsonl replays exactly (24 events)
```

One log format, one replay invariant, no second pipeline to drift.

The verdict goes in it too, as an `ask note` — stamped, not folded, so it
records how the run ended without changing the conversation:

```
$ jq -r 'select(.type=="note") | "[" + .data.source + "] " + .data.text' run.jsonl
[ply] the check passed:

$ go test ./... 2>&1
ok  	x	0.253s
```

That line is worth more than it looks. Without it a session holds every
command that ran and nothing about whether the work was *done* — a run that
passed and a run that gave up are the same shape on disk. It is the only
thing in the file that a program decided, and it is what lets
[`hone`][hone] learn from the run without guessing. A run with no `-check`
writes none: done is a program's opinion, and with no program there is no
opinion to record.

What `ply` loaded goes in beside it, because `brief cat` prints a body
without its frontmatter — a skill arrives as anonymous prose, and its name
would otherwise die with the terminal:

```
[ply] loaded skill house (named)
```

That is what lets `hone -into -` put a lesson back on the procedure the run
was following. A run that loaded a procedure and stumbled *anyway* is
evidence that procedure is incomplete.

[hone]: https://github.com/patrickyoung/hone

## Brief it first

```
$ ply -sh -s web-perf -check './budget.sh' "get LCP under 2.5s"
$ ply -sh -s -       "make my page load faster"     # brief picks the skill
```

`-s` appends a [`brief`][brief] skill to the system prompt; `-s -` lets the
catalogue choose. `brief` refuses to guess, so nothing matching says so on
stderr and the run continues without one — a confidently wrong procedure is
worse than none.

## A sub-agent is a program

Every command `ply` runs gets `$PLY` naming the running binary and
`$PLY_DEPTH` counting the nesting. So a tool that starts another `ply` is
just a tool:

```sh
#!/bin/sh
# review one file and print the findings
exec $PLY -t "$(dirname "$0")" -q "review $1 for concurrency bugs"
```

When the goal explicitly asks for subagents or parallel agent work, the root
prompt teaches the same mechanism directly. It recommends no more than three
independent, read-heavy children, 12 turns per child, indexed
sessions/output/status files, and a root synthesis that preserves failed
children. Nested prompts do not advertise delegation again. A toolbox-scoped
run sees this guidance only when its toolbox contains the bookkeeping programs
the recipe needs; Ply never widens a grant.

Fan-out, specialists and teams are still background jobs, `wait`, `xargs -P`,
and shell scripts. There is no team format, provider-specific orchestration, or
daemon. `PLY_DEPTH` stops at 8, because a loop that spawns itself is otherwise
a fork bomb with a credit card. Concurrent writers should use disjoint
worktrees; one shared tree has one writer.

## Think in shell

```bash
# a goal per package, four at a time, each in its own tree
ls -d ./pkg/*/ | xargs -P4 -I{} ply -sh -C {} -check 'go vet ./...' "fix the vet warnings"

# in a Makefile, where the check is already written
release: ; ply -sh -check 'go test ./...' "make the tests pass" && goreleaser

# as a filter, mid-pipe
kubectl logs deploy/api | ply -t tools "find the cause of the 500s" | tee triage.md

# until it holds, on a schedule; the pre-check makes the quiet runs free
*/30 * * * * ply -sh -q -check '/usr/local/bin/slo-ok' "bring the SLO back"
```

## Commands

```
ply [flags] <goal>       work the goal; stdin rides with it, or is it
ply tools [flags]        print the toolbox exactly as the model sees it
ply system [flags] [goal] print the system prompt that would be sent
ply version              print the version
ply help                 print the flag summary
```

Anything that is not a command is the goal; `--` sends a word that is one.
Full reference in `ply.1`, and tests keep it true: every flag and verb
appears in the man page and in `ply help`, help fits eighty columns and goes
to stdout while misuse goes to stderr, and the man page is lint-clean pure
ASCII carrying the version the binary reports.

## What it deliberately is not

No daemon, no config file, no MCP, no REPL or TUI — the shell is the REPL
and `ply` is a filter. No provider code, no session format, no permission
prompt (the toolbox is the policy and the process is the boundary), no
plugin API, and no second way to name a tool besides putting it on `$PATH`.

The list is in [AGENTS.md](AGENTS.md), which is short and is the whole set
of rules a change is held to. [DESIGN.md](DESIGN.md) is why it is shaped
this way. Adding one of the missing things back is a conversation before it
is a patch.

## Contributing

```
go test ./...
```

The two rules that matter most: the Unix contract (stdout is the answer
alone, exit 2 means not-done and never broke) and the sentence the whole
program has to keep fitting inside — *ask, run, check, repeat*.

Security issues go through [SECURITY.md](SECURITY.md), not the issue
tracker.

## License

[MIT](LICENSE).
