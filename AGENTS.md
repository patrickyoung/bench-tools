# Development guidance

Rob Pike is the bar, and `ask`, `brief` and `hone` are the siblings. `ply` is
the third of four carvings from one agent: `ask` took the model, `brief` took
the procedure, `ply` took the loop, and `hone` took the lesson. It stays
small the same way they do — by refusing to grow the other three.

The whole program is four steps: ask the model, run what it wrote, repeat
until it stops, then run the check and hand back the failure if it failed.
If a change makes that sentence longer, it needs a very good reason.

When changing `ply`:

- **preserve the Unix contract**: stdout is the answer alone, stderr is the
  typescript, the exit code is the outcome. Exit 2 means *not done* — the
  check still failing, a cap reached, or a context window that is full — and
  never means broken. It exists so a supervisor can tell the two apart
  instead of retrying the wrong one;

- **a tool is a program and the toolbox is `$PATH`.** Do not add a tool to
  `ply`. If a capability is missing, it is a program somebody puts in a
  directory, and that is the entire extension mechanism. The moment `ply`
  ships a `read` or an `edit`, it has begun re-implementing `cat` and `sed`
  in JSON, badly, and there is no natural place to stop;

- **the toolbox is scope, not a sandbox, and the docs say so in those
  words.** `sh` has builtins and a redirect opens a file with no program
  involved. Do not describe `-t` as a security feature, do not add a
  permission prompt, and do not build a policy language: the process — its
  user, its container, its `chroot` — is the boundary, exactly as it is for
  `mu`. Softening that sentence is worse than the missing feature, because
  somebody will trust it;

- **done is a program's opinion.** `-check` is a command and its exit status
  is the verdict. Do not add a model-based verifier: when judgment needs a
  model, `ask` is a program and goes in the check like anything else. The
  check runs before the first turn as well as after, and that is not an
  optimisation — it is what makes `ply` safe in a hook, a cron line, or a
  `Makefile`. It also runs with the *caller's* `PATH`, toolbox first: the
  toolbox exists to aim the model, and a check scoped to it would mean `go
  test ./...` needed a toolbox holding `go`, `git` and a linker;

- **never truncate silently.** A command's output is capped, and the
  elision is announced *in the text the model reads*, with both ends kept.
  Too much on stdin spools to a file the model is told about; past the hard
  limit it is an error naming the limit. Output that was cut and does not
  say so is the one failure nothing downstream can detect;

- **a command runs with stdin on `/dev/null`, in its own process group,
  under a timeout, and the timeout kills the group.** Each of those is one
  hang that would otherwise repeat every turn forever: nothing is typing, and
  a background child holding the pipe open is the difference between a
  timeout and a hang;

- **never run a truncated command.** An unterminated fence is a note back to
  the model, not a best guess at what it meant. `rm -rf /tmp/build` cut in
  half is a different command;

- **keep the log somebody else's.** The conversation is an `ask` session and
  `ply` writes no log of its own, so `ask replay -check` proves an entire
  run. Anything worth recording — a command, its output, its exit status,
  the check's verdict — goes in the text of the conversation, where it is
  already proven, rather than into a second file that can drift. The verdict
  is an `ask note`, written at exactly the two points the check reaches a
  terminal answer and nowhere else: a failing check the loop carries on from
  is already in the conversation as the rejection the model was handed, and
  recording it twice would say it happened twice. A run with **no** check
  writes none — done is a program's opinion, and with no program there is no
  opinion to record. Do not weaken that into a guess: the absence is what
  stops `hone` mistaking the model's word for a check. `ply` records what
  it *loaded* the same way and for the same reason: `brief cat` prints a
  body without its frontmatter, so a skill reaches the prompt as anonymous
  prose and its name would otherwise live only on stderr. A run that loaded
  no skill claims none;

- **`ply` holds no provider code, no credentials, and no catalogue.** If it
  needs a model it runs `ask`; if it needs a procedure it runs `brief`.
  Neither dependency is imported, both are named by an environment variable,
  and the failure to find one is an error that says what to install. The
  system prompt travels in `ASK_SYSTEM`, not argv, because argv is
  world-readable in `ps` and a skill can be private;

- run `go test ./...` — and `go test -race ./...` when touching the runner or
  the loop — before reporting success;

- **keep the docs true**: a new flag belongs in `ply help` and `ply.1`; a new
  command belongs in both plus `README.md`; a new environment variable
  belongs in `ply help` and `ply.1`. Tests enforce each, plus a lint-clean
  pure-ASCII man page carrying the version the binary reports, help inside
  eighty columns, and help on stdout with usage errors on stderr. Guard
  wholes as well as parts: a flag-level check stays green while an entire
  verb goes undocumented.

Things left out on purpose. Do not add them back without asking: a daemon, a
config file, MCP, a REPL or TUI, provider code, a session format, built-in
tools, a permission prompt, a plugin API, a model-based verifier, parallel
tool execution (a shell already has `&`), a team or workspace format, and any
second way to name a tool besides putting it on `$PATH`.
