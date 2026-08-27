# Development guidance

Rob Pike is the bar, and `ask`, `brief` and `ply` are the siblings. `hone`
is the fourth carving from one agent: `ask` took the model, `brief` took the
procedure, `ply` took the loop, and `hone` took the lesson. It stays small
the same way they do — by refusing to grow the other three.

The whole program is four steps: read the log, check it earned a lesson,
have one worded, write it where `brief` will find it. If a change makes that
sentence longer, it needs a very good reason.

When changing `hone`:

- **learn from recoveries, and nothing else.** A run that never failed
  teaches nothing and a run that never passed proves nothing. The gate is
  arithmetic on exit statuses already in the log, and it must stay
  arithmetic: a model is used to *word* a lesson and never to decide there
  is one, because a model asked whether a transcript contains a lesson will
  always find one. If a change makes `Teaches` return true for most
  sessions, it is a regression however good the lessons look;

- **the verdict is a program's opinion or it is not a verdict.** Only a typed
  `ply.verifier/v1` receipt, or a legacy prose note `ply` signed, counts. Do
  not infer a verdict from a session's shape, from
  the model's final reply, or from the absence of a rejection — a run that
  passed and a run that gave up were once the same shape on disk, and that
  ambiguity is the thing this program was arranged around. A session with no
  verdict is refused, and the message says how to get one;

- **a lesson belongs to the procedure the run was following.** `ply` records
  what it loaded because `brief cat` prints a body without its frontmatter,
  so a skill reaches the prompt as anonymous prose. `-into -` reads that
  back, and refuses a run that loaded none or more than one rather than
  guessing which. A run that stumbled *despite* a procedure is evidence that
  procedure is incomplete, and the evidence sent says so — only the skill's
  name, never its body, which is somebody else's text and none of hone's
  business to quote back;

- **watch the gauge.** A skill accumulating lessons faster than it is used
  is not a richer skill; it is a procedure missing a step. Do not add a verb
  that "consolidates" it — rewriting is a goal with a check, which is `ply`
  — but do not let a change hide the signal either;

- **never guess, and never guess louder.** A wrong lesson is worse than a
  missing one: the next agent reads it as established practice, follows it,
  and has no way to tell it was invented from a single typo. That is
  `brief`'s rule about skills, and it is the same rule — the literature is
  unanimous that memory makes agents *worse* when it is unearned, and that
  wrong entries gain authority rather than losing it. `none` is a real
  answer from the model and the expected one, and it must stay cheap to
  give;

- **`hone` has no store, no retrieval and no format.** A lesson is a skill,
  `brief` is the catalogue, `$BRIEF_PATH` is where it lives. Do not add an
  index, an embedding, a database, a cache or a daemon. `$BRIEF_PATH` is
  looked up here rather than by running `brief path`, and that is not a
  copy of the format: it is an environment contract like `$PATH`, and it is
  what keeps `brief` optional. The *format* — frontmatter, the
  specification's limits, the lint — stays entirely `brief`'s, and `hone`
  checks its own work by running it;

- **exact review is a user-owned delta, not a second store.** `-prepare`
  requires one operator-named file and binds the replayed source, the Ask
  call that worded it, the resolved destination-before bytes, and the exact
  final skill document. `show` calls nothing; `admit` calls no model,
  replays both provenance sessions, rejects every stale hash or path, proves
  the document is only Hone's append/scaffold delta, and writes atomically.
  Never add a default proposal directory, list, index, implicit admission,
  overwrite-on-prepare, or regeneration during admission;

- **every write is a delta.** Append under the heading; never regenerate
  what is there. A model handed its own accumulated notes and asked for the
  next version rewrites them, and rewriting compresses away the specifics
  that made them worth keeping. Refining a full skill is a goal with a
  check, which is to say it is `ply`, and there is already a program for it;

- **a run teaches once, and the mark is how that is known.** Check it
  *before* the model is called, so a second pass over an archive costs
  nothing. Match the session field exactly and never as a substring: `ask -f
  pass.jsonl` makes a session called `pass`;

- **provenance is not decoration.** Every lesson names the run it came from
  and the call that worded it, and both replay. It is the only reason a bad
  lesson can be deleted rather than argued with, and the security literature
  is clear that arguing does not work — a poisoned memory survives being
  told it is wrong. Do not write a lesson without a mark, and do not invent
  a second way to record one;

- **`hone` holds no provider code, no credentials, and no catalogue.** If
  it needs a model it runs `ask`; if it needs the format checked it runs
  `brief`. Neither is imported, both are named by an environment variable,
  and a missing `brief` is a narrower `hone` rather than a failure. The
  system prompt travels in `ASK_SYSTEM`, not argv, because argv is
  world-readable in `ps` and evidence can be private;

- **send only what proved something.** The evidence is the goal, the check
  and the stumbles. Sending the whole transcript would cost the window and
  invite a lesson grounded in the parts of a run that proved nothing;

- **preserve the exit contract**: 0 yes, 1 no, 2 error. It is `grep`'s, not
  `ask`'s, because `hone` asks a question where no is a real answer and the
  common one. A "no" must leave stdout empty, or a loop over an archive
  appends a diagnostic to somebody's skill;

- run `go test ./...` — and `go test -race ./...` — before reporting
  success;

- **keep the docs true**: a new flag belongs in `hone help` and `hone.1`;
  a new command belongs in both plus `README.md`; a new environment variable
  belongs in both. Tests enforce each of those, plus a lint-clean pure-ASCII
  man page carrying the version the binary reports and help inside eighty
  columns. Guard wholes as well as parts: a flag-level check stays green
  while an entire verb goes undocumented.

Things left out on purpose. Do not add them back without asking: an
embedding model, a vector store, an index, a database, a cache, a daemon, a
config file, MCP, a scheduler, memory types, a consolidation pass, a
retrieval verb, a ranking function, and any provider code at all.

Two more that arrive together, from anyone who reads the gauge and wants it
to do more:

- **counting uses, not just lessons.** Lessons-per-use is the truer signal
  and it needs `brief` to keep a counter, which is a cache, which is on
  `brief`'s own refused list. The total is printed because `add` is already
  holding the document it just wrote and the number is therefore free. Count
  what costs nothing; let a human read the gauge;
- **detecting contradiction.** Near-duplicates are already suppressed by
  `have`, which is string comparison and stays that way. Opposite claims are
  not, and the only way to find them is to ask a model whether two lessons
  disagree — which is a model *deciding* something, and the first rule in
  this file says it never does. The gauge is what points a human at a skill
  worth rereading, and rewriting one is a goal with a check, which is `ply`.

And the one that matters most: **nothing writes to a skill unless somebody
typed a command that says to.** There is no hook, no watcher and no
automatic learning, and adding one would not be a feature. A system that
learns silently is a system that silently learns the wrong thing, and what
it produces is a confident agent citing a precedent nobody wrote.
