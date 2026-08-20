# Trail

Trail is a small, read-only browser for
[Ask](../ask) session archives. It lists sessions, searches their human text,
prints exact events and bounded windows, follows recorded lineage, and asks Ask
to verify replay. Every result on standard output is one JSON object per line.

Trail does not own the archive. It never writes a session, repairs a torn
append, invents a parent, or maintains an index. Ask remains the authority on
whether a session replays exactly.

## Install

Trail requires Go 1.26 or later.

```sh
go install github.com/patrickyoung/trail@latest
```

From this source tree:

```sh
go build .
go test ./...
```

`trail` uses `ASK_DIR` as its default archive. If that is unset it reads
`~/.ask/sessions`. `trail check` uses `ASK`, or `ask` from `PATH` when `ASK` is
unset.

## Commands

```text
trail ls [dir]
trail find query [dir]
trail show session
trail window [-before n] [-after n] session seq
trail lineage session [dir]
trail check [dir]
```

A session may be an existing path. Otherwise it is a bare session id resolved
under the default archive.

Examples:

```sh
trail ls
trail find 'connection reset'
trail show 01JY6D7RFJFE2K6EJNW4QNNZ8G
trail window -before 2 -after 1 01JY6D7RFJFE2K6EJNW4QNNZ8G 14
trail lineage 01JY6D7RFJFE2K6EJNW4QNNZ8G
trail check
```

Archive commands visit regular `.jsonl` files in lexical path order. Damage
in one file produces an `error` record and does not hide later files. An
invalid final record is reported as a `warning` because Ask treats it as a
torn append; Trail ignores it in memory and leaves the file alone.

`find` matches Unicode case-insensitively after collapsing whitespace. It
searches user and assistant text, reasoning, attachment names and media types,
notes, retry and terminal errors, and request model/effort labels. It does not
search system prompts, base64 media, provider-native state, digests, schemas,
or fields in unknown event types. The `text` in a match is the original text,
not the folded comparison value.

`lineage` emits only `parent` and `summary` relationships recorded in session
headers. A copied file with the same session id is one ambiguous node with
multiple paths, not a guessed branch.

`check` runs this command once for every archive file:

```text
ask replay -check FILE
```

Trail does not duplicate Ask's replay logic or interpret the session itself to
decide whether it is sound.

## Output

Standard output is JSONL. The `kind` field selects the record shape:

- `session`: archive summary from `ls`
- `match`: one matching event from `find`
- `event`: an exact archived event wrapped by `show` or `window`
- `node`, `edge`: a recorded lineage graph
- `check`: Ask's replay verdict for one file
- `warning`: a non-fatal condition, currently a torn final record
- `error`: damage confined to one archive file

`show` and `window` put the original event under `event`; unknown event types
and unknown fields therefore survive unchanged. Paths are always data. Fatal
usage and operating-system diagnostics go to standard error.

Exit status is 0 for a successful result, 1 for no match, archive damage, a
missing event sequence, or a failed replay check, and 2 for bad usage or an
operating error. Thus JSONL already written to standard output remains useful
when status 1 reports partial archive damage.

See [GUIDE.md](GUIDE.md) for pipelines and operational details, and
[trail.1](trail.1) for the manual page. Session contents are sensitive and
output is not redacted; [SECURITY.md](SECURITY.md) describes the boundary.

## Scope

The design is deliberately direct: list files, read each once, select records,
print JSONL. There is no daemon, watcher, database, cache, vector search, model
call, writer, repair command, replay clone, title generator, or query language.

## License

MIT. See [LICENSE](LICENSE).
