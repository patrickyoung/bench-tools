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

- **done is a program's opinion.** `-check` is a verifier command. Its stdin is
  empty for the pre-check and carries the candidate final report afterwards;
  file and code checks may ignore it. Exit 0 accepts, exit 1 rejects and feeds
  its output back, and any other exit, signal, or timeout means the verifier is
  broken and stops the run. Do not add a model-based verifier: when judgment
  needs a model, `ask` is a program and goes in the check like anything else. The
  check runs before the first turn as well as after, and that is not an
  optimisation — it is what makes `ply` safe in a hook, a cron line, or a
  `Makefile`. It also runs with the *caller's* `PATH`, toolbox first: the
  toolbox exists to aim the model, and a check scoped to it would mean `go
  test ./...` needed a toolbox holding `go`, `git` and a linker;

- **a verifier run earns a durable receipt before it earns an outcome.** Every
  candidate check writes one typed `ply.verifier/v1` record through Ask,
  followed by Ask's prefix seal. It binds the normalized candidate, exact
  verifier and interpreter, status, output, and optional admitted-contract
  digest. Rejection, acceptance, and verifier breakage all count. Receipt
  failure is infrastructure failure; never print successful completion first
  and promise to record it later;

- **a turn is one action or one report; completion is still a check.** Consume
  the first complete, nonempty shell block as the turn's one action, return its
  result, and explicitly defer every later block and trailing claim. This must
  work even when a model emits an imagined multi-step workflow: formatting
  failure is not allowed to erase useful work or make unobserved work real. A
  report has no shell block and stops the model loop. Keep this grammar and the
  outcome-oriented guidance in the public `ply system` value. Do not treat
  running any command as proof of progress or completion; `pwd` is an
  interaction, not a verdict;

- **required interaction is liveness, not completion.** `-require-action`
  may refuse a final report until one command has run, but it never treats
  that command as progress or evidence. Bound persistent actionless replies
  as protocol stalls and preserve the ordinary report-without-action behavior
  when the flag is absent;

- **one run has one command interpreter.** `/bin/sh` is the stable default;
  `-shell` and `PLY_SHELL` explicitly select another executable accepting
  `-c`. Resolve it before calling the model, name the exact choice in the
  prompt, use it for both blocks and checks, and propagate it to nested Ply
  processes. Never inherit the interactive `$SHELL`, infer dialects from
  version output, or let fence labels select different interpreters;

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

- **never run a partially understood command.** An unterminated first fence is
  a note back to the model, not a best guess at what it meant. `rm -rf
  /tmp/build` cut in half is a different command. Once a complete first block
  has been consumed, later content is deferred rather than parsed as part of
  that command;

- **exact approval precedes execution, never follows it.** With `-may-job`,
  run a fresh May process on the literal action envelope, require its matching
  exit status and strict result, seal `ply.approval/v1`, and only then pass the
  unchanged script to Runner. A parked, declined, malformed, or unrecordable
  result stops before another model turn or check. May owns grants; Ply owns
  the pre-exec seam. Do not infer risk from model prose, cache May stdout, or
  let a nested Ply silently discard the inherited gate;

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

- **model policy passes through; it is not reimplemented.** `-m` and
  `-effort` are literal Ask arguments. Ply validates neither provider names nor
  effort levels, and exports explicit values to nested Ply processes so a
  delegated run does not silently change its model policy;

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

And the ones a reader arrives with after being told `ply` needs a task
runtime. Each has an answer, and the answer is in `GUIDE.md`:

- **a task record, or `ply resume`.** The state of the work is the work
  tree, and `-check` is how you read it — `make` keeps no record either, it
  stats the targets. Resuming is running it again: the pre-check makes
  re-entry free when the work is done, `-f` continues the conversation
  rather than starting over, and the log outlives the process. The
  conversation is an *optimization, not the state*; lose it and the run is
  still correct, only more expensive. Two tests pin that, and they are the
  contract;
- **recovering `-cycles` or `-turns` from the log.** It is derivable — turns
  are `assistant` events, compactions are the `parent` chain — and it is
  still wrong. Bounds are per invocation because the process is the unit;
  recovering them would poison a session that spent its budget, with no way
  to say "try again", which contradicts `-B` and contradicts `make`;
- **`ply ps`.** A session that finished wrote a `done` event. Asking which
  did not is a pipeline over a directory, and it goes in the guide as one.
  A verb here is the first step to `ply` supervising itself;
- **a `-sandbox` flag.** Shipping one means claiming one, and the claim is
  what `SECURITY.md` exists to prevent. The container recipe is a recipe;
- **a permission prompt, now with a diff.** `PLY_PROPOSE` is honoured by
  `contrib/edit` and reaches it through the inherited environment, so `ply`
  needs no code and gets none. It is documented as a convention tools may
  honour and never as a boundary — a redirect writes a file with no program
  involved, and softening that sentence is the failure this file already
  warns about two bullets up;
- **a merge program for fan-out.** It is one `ask` invocation, and a program
  whose body is one invocation of another program is an alias.
