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

Each of the three refuses to grow the other two. That is the whole
architecture, and it is why all three stay small.

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

Progressive disclosure falls out for free. `brief` needed three levels for
prose; programs already have them:

    level 1   the names, always in the prompt        ls tools/
    level 2   the interface, when the model wants it cmd -h
    level 3   the behaviour                          run it

Nothing needed building for levels 2 and 3. They are how programs work.

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
    exit 2   not done — check still failing, a cap tripped, context full
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

## Deliberately not here

A daemon. A config file. An MCP client or server. A TUI or REPL — the shell
is the REPL and `ply` is a filter. Provider code of any kind. A session
format. A permission prompt (the toolbox is the policy, and the process is
the boundary). Parallel tool execution (a shell already has `&`). A plugin
API. A second way to name a tool besides putting it on `$PATH`. Retry of the
check itself with a different model. Anything that makes the sentence "ask,
run, check, repeat" longer.
