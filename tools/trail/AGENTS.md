# Development guidance

Rob Pike is the bar, and `ask` is the file format. `trail` reads what `ask`
wrote and never becomes another owner of it.

The whole program is four steps: list session files, read each once, select
records, print JSONL. `check` replaces the selection with one invocation of
`ask replay -check` per file. If a change makes either sentence longer, it
needs a very good reason.

When changing `trail`:

- **read only means read only.** No command creates, appends, renames, locks,
  repairs, indexes, caches, touches, or changes the mode of a session. A torn
  final line is reported and ignored in memory, exactly as `ask` reads it; the
  file remains untouched. `check` runs `ask replay -check`, which is also a
  reader. A future speed problem may earn a separate, disposable index
  builder; it does not earn a write in this program;

- **the log belongs to `ask`.** Keep a small wire view of the public JSON,
  not an imported copy of `ask` internals and not a second session package.
  Unknown event types pass through `show` and `window` untouched. Only `ask`
  can prove replay, so `check` delegates rather than reproducing its fold;

- **stdout is JSONL and nothing else.** One complete object per line.
  Progress and fatal diagnostics go to stderr. Archive-wide commands turn a
  bad file into an `error` record and keep walking, so one broken session
  cannot hide the healthy ones after it;

- **preserve the exit contract:** 0 is a successful listing/result or an
  all-good check; 1 is no `find` match, damaged input, a missing window seq,
  or a replay check that failed; 2 is bad usage or an operational failure
  that prevented the requested scan. Exit 1 is an answer a pipeline can use;
  exit 2 means the answer could not be obtained;

- **never infer lineage.** `parent` and `summary` in the session header are
  the entire graph. `cp` creates no new fact, so copied files with one header
  id are shown as one recorded identity with several paths, never as a fork;

- **semantic text has one documented definition.** User and assistant text,
  reasoning, attachment names and media types, notes, retry errors, terminal
  errors, and a few request labels are searchable. Base64 media, opaque
  provider state, digests, schemas, and system prompts are not. Search folds
  whitespace and Unicode case for matching but prints the extracted text
  unchanged;

- **never truncate.** A session event over the named 64 MiB line limit is an
  error. Extracted text and raw events are printed whole. Attachment bytes are
  deliberately not semantic text, but the raw event remains available from
  `show` and `window`;

- **paths are data.** JSON encodes them. Never print a shell command that
  quotes one, never follow a `current` symlink as an archive member, and scan
  regular `.jsonl` entries only;

- run `go test ./...`, `go test -race ./...`, `go vet ./...`, and the built
  binary smoke before reporting success;

- keep the docs true: every command belongs in `trail help`, `README.md`, and
  `trail.1`; every environment variable belongs in all three. Tests enforce
  the whole command set, help width, a pure-ASCII man page, and the version.

Things left out on purpose: a writer, repair, an index, SQLite, embeddings, a
model, a daemon, a watcher, a TUI, session titles, ranking, a query language,
configuration, and a second replay implementation. `jq`, `sort`, `join`, and
`rg` are already programs.
