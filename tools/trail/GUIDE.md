# Using Trail

Trail is most useful as a narrow adapter between Ask's append-only session
files and ordinary command-line tools. It prints records instead of a table so
scripts do not have to scrape a display format.

## Choose an archive

Set a default once:

```sh
export ASK_DIR="$HOME/.ask/sessions"
```

Every archive command also accepts a directory explicitly. An absent default
archive is an empty archive; an explicitly named absent directory is an error.
Trail ignores directories, symlinks, and files without a `.jsonl` suffix.

## Inspect and search

List summaries, then retain sessions that ended normally:

```sh
trail ls | jq -c 'select(.kind == "session" and .outcome == "end")'
```

Find a phrase and print its session and sequence:

```sh
trail find 'rate limit' |
  jq -r 'select(.kind == "match") | [.session, .seq] | @tsv'
```

Search is literal substring matching after Unicode lowercasing and whitespace
folding. There are no regular expressions, ranking, stemming, or hidden model
calls. Shell quoting determines whether a multiword query is one argument.

`show` returns the entire valid event stream. `window` returns the selected
event with three neighbors on either side by default:

```sh
trail window SESSION 27
trail window -before 8 -after 2 SESSION 27
```

The before and after counts are event counts, not sequence arithmetic. This is
important if a damaged or future archive contains gaps.

## Follow recorded ancestry

```sh
trail lineage SESSION |
  jq -c 'select(.kind == "node" or .kind == "edge")'
```

An edge has `from`, `to`, and `relation`. `relation` is either `parent` or
`summary`. Trail walks those edges in either direction to show the connected
component containing the requested session, but prints each edge in its
recorded direction. An id referenced by an edge but absent from the scanned
archive is still printed as a node without paths.

If multiple files claim the same id, the node has all lexical `paths` and
`ambiguous: true`. Conflicting lineage headers also produce an error record.

## Verify replay

```sh
ASK=/path/to/ask trail check /path/to/archive
```

Each `check` record contains the file name as `session`, its `path`, and `ok`.
On success, `detail` contains Ask's trimmed standard output. On a replay
failure, `error` contains Ask's trimmed diagnostic. Status 1 means at least one
file failed; status 2 means Trail could not run Ask or read the archive.

## Handle partial results

A robust consumer reads standard output before interpreting the exit status.
For example:

```sh
tmp=$(mktemp)
if trail ls >"$tmp"; then
  jq -c . <"$tmp"
else
  status=$?
  jq -c . <"$tmp"
  echo "trail exited $status" >&2
fi
```

Status 1 is a data result: a search found nothing, an event sequence was
absent, a replay check failed, or one archive file was damaged. Status 2 means
the invocation itself failed. Trail stops on standard-output write failure so
a broken pipeline is never reported as success.

## Archive safety

All session reads use read-only file descriptors. Trail never renames, opens
for writing, changes permissions, updates timestamps intentionally, truncates,
or removes archive data. A 64 MiB event-line limit bounds memory; exceeding it
is reported as corruption rather than silently truncating the event.
