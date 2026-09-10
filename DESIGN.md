# ply — design

Written before the code, as the house style asks. It names the invariants
that are cheap now and unaffordable to retrofit, and the cuts that are
deliberate so nobody quietly re-adds them.

## The gap

`ask` is a model and no loop. `brief` is a procedure and no model. Between
them there is nothing that *acts*: no way to say "make the tests pass" and
have a program keep working until a program agrees it is done.

`mu` does that, but `mu` owns everything — providers, sessions, a verifier,
a workspace, a team, a pack format. `ask` was carved out of it as the
conversational core. `ply` is the other carving: **the loop and the tools**,
and nothing else.

    ask     the model          — no tools, no loop
    brief   the procedure      — no model, no loop
    ply     the loop           — no model, no procedure
    hone    the lesson         — no store, no retrieval, no format

Each refuses to grow the others. That is the whole architecture, and it is
why all of them stay small.

## Two decisions carry the design

### 1. A tool is a program. The toolbox is `$PATH`.

Every agent in the world defines tools as JSON schemas: `read`, `write`,
`edit`, `grep`, `list`. Those are `cat`, `tee`, `sed`, `grep`, `ls` with a
worse interface, re-implemented per agent, and each one is a place to have
a bug that `grep` does not have.

Unix already shipped the tools. What it did not ship was a way to hand a
model *some* of them. So:

    ply -t ./tools "goal"      PATH becomes ./tools, and nothing else
    ply -sh "goal"             PATH is yours: the whole machine

A toolbox is a directory of executables. You build one the way you have
built things for forty years:

    mkdir tools && ln -s $(which ls grep sed git go) tools/

The capability set is `$PATH`. That is the mechanism, and it is one
mechanism doing three jobs — it is the allowlist, it is the documentation
(`ls tools/`), and it is where your own programs go. Adding a tool is `ln
-s`. Removing one is `rm`. There is no registry, no schema, no manifest, no
plugin API, and nothing to version.

The command interpreter is mechanism, not a capability grant. It is
`/bin/sh` by default because that is the standard command-language interface,
not because every host ships the same implementation behind it. An operator
can select another executable with `-shell` or `$PLY_SHELL`; Ply resolves it
once, runs checks and (by default) model blocks through its `-c`, and tells the
model the exact path. An explicit `-action-shell` or `$PLY_ACTION_SHELL` may
instead name one executable for model blocks alone while checks retain
`-shell`. This is a process-composition seam for an operator-owned container
or remote runner, not a capability grant made by Ply. Ply deliberately ignores
`$SHELL`: a login-shell preference is not a script contract and may name Fish
or another incompatible language.

The flag names one program, not a command line. Interpreter flags belong in a
wrapper program, the Unix answer that avoids teaching Ply another quoting
language. Fence labels do not dispatch to different shells either: they are
generous spellings of the one text protocol, while the operator retains the
one execution choice.

Progressive disclosure falls out for free. `brief` needed three levels for
prose; programs already have them:

    level 1   the names, always in the prompt        ls tools/
    level 2   the documented interface, when present
    level 3   the behaviour                          run it

Nothing needed building for level 3. A toolbox author may provide level 2;
Unix does not define a universal help flag.

**The toolbox scopes; it does not sandbox.** `sh` has builtins, and `read`
plus a redirect opens a file with no program involved. The security boundary
is the process — its user, its container, its `chroot` — exactly as it is
for `mu`. `-t` is for aim, not for safety, and the manual says so in those
words.

### 2. Done is a program's opinion, not the model's.

    ply -check 'go test ./...' "make the tests pass" && git push

`&&` means something there, and it does not mean "the model felt good about
it". The loop is:

    ask the model → run what it wrote → repeat until it stops →
    run the check → if it failed, hand the failure back → repeat

`mu` answers this with a second model as a verifier. That was the right
answer when the artifact was prose. It is the wrong answer when a program
can decide, and a program can decide far more often than agent designers
admit: tests, builds, linters, `diff`, `test -f`, an exit status.

The check also runs *first*, before any model call, and `ply` exits 0 having
spent nothing if it already passes. That is `make`'s "nothing to be done",
and it is what makes `ply` safe to put in a loop, a hook, or a Makefile.
When it fails, its terminal transcript rides with the first turn instead of
being discarded; that is both the model's starting evidence and the first
stumble a later passing run can teach from.

When the judgment genuinely needs a model, the mechanism does not change —
you already have one:

    ply -check 'ask -q "does this cover install? yes/no" < README.md | grep -qi yes' ...

The model-as-judge is available *through the same hole as everything else*,
because the hole is "run a program" and `ask` is a program. No verifier
subsystem, no collusion question, no second prompt to maintain.

## Consequences that fall out

**`ply` has no provider code.** It runs `ask`, the way `brief` does, for the
same reason. Five providers, gateway auth, subscription login, reasoning
round-trips and retries arrive for free and stay somebody else's problem.

**`ply` has no log format.** The conversation *is* an `ask` session. Tool
output goes back as the next user message, so the commands and their output
are in that log verbatim — which means `ask replay -check` proves an entire
`ply` run, byte for byte, and there is no second logging pipeline to drift.

**`ply` works with models that have no tool-use API,** because the protocol
is text. A fenced shell block is a command; that is the whole wire format.
It reads as a terminal session, which is the trace format every model is
best at reading and every human already knows.

**A sub-agent is a tool.** `ply` exports `$PLY` naming its own binary, so a
program in a toolbox can start another `ply`. Fan-out, specialists, and
teams are a shell script, not a feature.

**`ask`'s exit 2 finds its purpose.** It exists "so a retry loop can stop".
`ply` is that retry loop, and it maps a full context window onto its own
exit 2 instead of retrying a permanent error until the bill arrives.

## The contract

    stdout   the final answer, alone
    stderr   the typescript: what ran, what it printed, what it exited
    exit 0   done — the check passed, or, with no check, the model stopped
    exit 1   error — usage, no ask on PATH, a provider failure
    exit 2   not done — check failing, a bound or protocol stalled, context full
    exit 3   exact action declined by May; not executed
    exit 75  exact action parked in May; not executed
    exit 130 interrupted

Exit 2 is the row that earns its keep, as it does in `ask` and `mu`: a
supervisor has to tell "did not work" from "broke", or it retries the wrong
one.

Commands run with stdin on `/dev/null` in their own process group, under a
timeout, and a timeout kills the group and reports 124 — `timeout(1)`'s
number, because it is the one already in everyone's fingers.

Output fed back is bounded and says so in the text the model reads. Input is
never silently truncated: too much on stdin spools to a file and the model
is told the path, so a large input becomes something to `grep` rather than
something to carry every turn.

One invocation is bounded to 50 model turns by default. That bound applies
even when the model continuously emits valid commands and never stops for a
check; zero removes it explicitly. A turn is either one shell program followed
by real terminal evidence, or a final report with no program. The grammar is
enforced at the text boundary by consuming the first complete command block
and explicitly deferring every later block and trailing claim. An empty or
truncated first block is corrected twice and then becomes exit 2, never guessed
execution or unchecked completion.

## Deliberately not here

A daemon. A config file. An MCP client or server. A TUI or REPL — the shell
is the REPL and `ply` is a filter. Provider code of any kind. A session
format. A risk classifier or permission database (May owns exact human
authority; the process still owns containment). Parallel tool execution (a
shell already has `&`). A plugin
API. A second way to name a tool besides putting it on `$PATH`. Retry of the
check itself with a different model. Anything that makes the sentence "ask,
run, check, repeat" longer.

`-steer FILE` is the interactive seam: a controller appends ordinary UTF-8
lines, and Ply reads them before an Ask turn, after generation before using
its response, after approval, and after a candidate check before finalization.
The file is transport, not a second log or an execution protocol; Ask records the applied guidance in the
same conversation as the tool results.

Tool results use `ask append`: an attributed, sealed user message with no
model call. That is the durable observation boundary, so exhausting a turn
budget cannot discard the result that just happened. Rejected verifier
results use the same seam before turn and cycle limits. Ask still owns every
event, lock, fold and seal. Boundary failures keep their existing terminal
receipts and approval adjacency. A partial response never becomes a command,
even with live progress enabled by `-stream`.

`-compact-at` asks Ask to measure context and conditionally compact it. Ply
holds no tokenizer or model window catalogue. After verifying the handoff,
it retains the original goal in the new session before advancing a checkpoint.
The normal provider-neutral handoff remains the default; native mechanisms
and alternate tool protocols require task-level evaluation before changing it.

`-may-job JOB` is the optional execution-authority seam. Immediately before a
model-authored block, Ply runs the public `may request JOB` filter with one
exact JSON action on stdin. Exit 75 or 3 stops without execution; only a fresh
exit 0 plus the exact `spent` object can proceed. Ply seals its own receipt
before handing the same script to Runner. Ask, Brief and the configured check
do not pass through May. There is still one model, one action loop and one Ask
log; approval and an optional action interpreter are programs in the pipeline,
not permission or remote-execution subsystems inside Ply.
