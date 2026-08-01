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
is not, the recipes, and the five things that will bite you — including the
one that bit the person who wrote it (an `ask` inside a check continues
*your* conversation unless you say `-n -f`).

## The family

    ask     the model         — no tools, no loop
    brief   the procedure     — no model, no loop
    ply     the loop          — no model, no procedure

`ask` is a model and no loop. `brief` is a procedure and no model. Between
them there was nothing that *acts*, and `ply` is that and nothing else: it
has no provider code, no credentials, no session format and no catalogue.
When it needs a model it runs `ask`. When it needs a procedure it runs
`brief`. Each of the three refuses to grow the other two, which is why all
three stay small.

You brief it, you ask it, and it plies.

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

Every fenced shell block it writes is executed — there is no other way to
act, and no way to write shell that is not run. (To quote a command without
running it, indent it. That is the whole escape hatch.) What comes back is
what a terminal would have shown:

```
$ go test ./... 2>&1 | tail -20
--- FAIL: TestWrap (0.00s)
    ring_test.go:41: want 3, got 2
FAIL
exit 1
```

A reply with no block ends the run, and that reply is the answer.

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
| exit 2 | not done — check still failing, a cap tripped, context full |
| exit 130 | interrupted |

Exit 2 earns its row, as it does in `ask`: a supervisor has to tell "did not
work" from "broke", or it retries the wrong one.

Commands run with stdin on `/dev/null`, in their own process group, under
`-timeout` — and a kill reports **124**, `timeout(1)`'s number. Output is
capped per command, keeping both ends, and the elision is announced *in the
text the model reads*, because truncation it cannot see is the one failure
nothing downstream can detect.

## The log is somebody else's problem

`ply` keeps no log. The conversation is an `ask` session: the commands are
in the assistant turns and their output is in the user turns, so the file is
the entire run — and `ask` already proves those.

```
$ ask replay -check ~/.ply/sessions/20260801-142233-a3f9c1e0.jsonl
ok: 20260801-142233-a3f9c1e0.jsonl replays exactly (24 events)
```

One log format, one replay invariant, no second pipeline to drift.

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

Fan-out, specialists and teams are `xargs -P` and a shell script. There is
no team format, no orchestrator, and nothing to configure. `PLY_DEPTH` stops
at 8, because a loop that spawns itself is otherwise a fork bomb with a
credit card.

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
