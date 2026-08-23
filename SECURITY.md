# Security

Report a vulnerability by email rather than through the issue tracker.

## What ply is

`ply` runs shell commands that a language model wrote. That is not a bug in
it; it is the program. Everything below is about being precise about what
that does and does not mean, because a vague answer here is worse than none.

## The toolbox is scope, not a sandbox

`-t dir` sets `$PATH` to that directory alone. It is a strong tool for
*aiming* a model: it cannot name a program you did not put there, so it
cannot reach for `curl` or `ssh` by accident, and the catalogue it is shown
is exactly what it can run.

It is not containment, and it must never be described as containment:

- `sh` builtins do not live on `$PATH`. `read` with a redirect opens a file
  with no program involved; `echo >` writes one.
- A program in the toolbox may itself run anything. A `git` there brings
  `git config core.pager`, hooks, and aliases with it.
- Nothing limits network, memory, signals, or what the process may write.

**The security boundary is the process** — the user it runs as, the
container or VM around it, the `chroot` or jail it sits in. Run `ply` where
you would be willing to hand somebody a shell, because that is what you are
doing. This is the same doctrine as the agent `ply` was carved from, stated
the same way on purpose.

`-sh` is that with the aiming removed.

## What ply holds

Nothing. It has no credentials, no config file, and no state beyond the
session it writes.

- **Model credentials belong to `ask`.** Keys come from the environment and
  subscription logins live in `~/.ask/auth.json`. `ply` never reads, stores,
  or forwards them; it runs `ask` and `ask` does what it does.
- **The system prompt travels in `ASK_SYSTEM`, not argv**, because argv is
  world-readable in `ps(1)` on every machine this runs on and a `brief`
  skill can be a private procedure.
- **Commands inherit the environment.** They see whatever `ply` saw,
  including API keys. `$PATH` is the only variable `ply` sets, plus `PLY`
  and `PLY_DEPTH`. If a secret must not be reachable by a model-authored
  command, it must not be in the environment `ply` was started with.

## Where the run is written

Sessions land in `$PLY_DIR` (default `~/.ply/sessions`), created mode 0700,
files mode 0600. They are `ask` session logs and they hold the whole run:
the goal, the commands, and **everything those commands printed**. If a
command prints a secret, the secret is in the log. Piped input past 64 KB is
spooled next to the session as a `.stdin` file, with the same modes and the
same caveat.

## Prompt injection is not solved here, and cannot be

The output of every command goes back to the model as text. A file, a log
line, an HTTP response, or a dependency's README can therefore contain
instructions, and the model may follow them. `ply` does not attempt to
detect this, because reliable detection does not exist and a filter that
claimed otherwise would be lying to the person deciding whether to trust it.

What is offered instead is real but limited:

- `-t` bounds what an injected instruction can reach for.
- `-check` bounds what a run can *conclude*: a program, not the model,
  decides whether exit 0 happens.
- The typescript on stderr and the session log make what happened readable
  after the fact, in full, in order.
- `-timeout`, `-cycles`, and the finite default `-turns` bound how long a
  hijacked run can continue.

Treat a `ply` run over untrusted input as untrusted execution of that input,
and put the boundary in the operating system.

## Bounds ply does enforce

These exist to stop accidents, not attackers:

- A command gets `/dev/null` on stdin, its own process group, and a
  `-timeout`; the timeout kills the group and reports 124.
- Command output is capped per command (`-cap`), keeping both ends, with the
  elision stated in the text the model reads.
- Piped input past 16 MB is an error naming the limit, never a truncation.
- A run stops after 50 model turns by default. `-turns 0` explicitly removes
  that invocation-wide bound.
- `PLY_DEPTH` refuses to start a ply more than eight deep inside another, so
  a toolbox program that runs `ply` cannot become an unbounded fan-out.
- An unterminated fenced block runs nothing. After two corrective replies,
  another malformed fence stops the run at exit 2; a truncated command is
  never an unchecked success.
- `-steer` is cooperative guidance, not an authority boundary. Ply rejects
  malformed, truncated, or oversized steering input, but a process able to
  write that file can influence future model turns. It cannot directly replace
  Ply's fixed verifier or toolbox arguments.
