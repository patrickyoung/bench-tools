# Trail

**Find the useful part of a past model run without digging through log files.**

Trail lists, searches, and inspects [Ask](https://github.com/patrickyoung/ask)
session archives. You can find an old error, read the events around it, follow
a recorded compaction, or ask Ask to verify the history. Trail is read-only:
it never modifies a session or builds a second database.

```sh
trail ls
trail find 'connection reset'
trail check
```

Every result is JSON Lines: one complete JSON object per line. Use the output
as-is or pipe it through `jq`.

## Install

Requires **Go 1.26+**. Install current `main`:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/patrickyoung/trail@main
export PATH="$HOME/.local/bin:$PATH"
trail version
```

Keep the PATH setting in your shell startup file. Searching and reading need
only session files. `trail check` also needs Ask on PATH; no provider key or
model call is needed for replay verification.

## Explore your first archive

If you already use Ask, start here:

```sh
trail ls
trail find 'error'
```

Trail reads `$ASK_DIR`, or `~/.ask/sessions` by default. If your sessions came
from another tool, supply its archive directory explicitly:

```sh
trail ls ~/.ply/sessions
trail find 'error' ~/.ply/sessions
```

A listing returns `session` records; a successful search returns `match`
records containing the original text and its event sequence. No match is
exit 1 with no match records—it does not mean the search is broken.

For a fully named example, create a tiny archive with configured Ask first:

```sh
mkdir trail-demo
ask -f trail-demo/example.jsonl 'Explain what a connection reset means.'
trail ls trail-demo
trail find 'connection reset' trail-demo
trail show trail-demo/example.jsonl
trail check trail-demo
```

Only the `ask` question calls a model. The Trail commands read existing files.

## Zoom in instead of reading everything

Use the `seq` value from a search result:

```sh
trail window -before 2 -after 1 trail-demo/example.jsonl SEQUENCE
```

Replace `SEQUENCE` with that numeric event sequence. You get the matching
event and its neighboring events, which is often enough to understand what
was tried and what happened next. `show` prints every event instead.

With `jq` installed, turn a raw archive view back into event JSONL:

```sh
trail show trail-demo/example.jsonl |
  jq -c 'select(.kind == "event") | .event'
```

`show` and `window` preserve unknown event types and fields under `event`.
They do not rewrite Ask's format.

## Follow a longer run

```sh
trail lineage trail-demo/example.jsonl trail-demo
trail check trail-demo
```

Lineage follows only `parent` and `summary` fields recorded in session headers.
Copying a session file does not invent a new branch relationship. A copied ID
is shown as one identity with multiple paths.

Verification runs `ask replay -check FILE` once per archive member. Ask owns
that verdict. Replay consistency does not prove an answer is correct or a
job passed its business check.

[Agent](https://github.com/patrickyoung/agent) exposes the same functionality
as `agent history HOME`. [Ply](https://github.com/patrickyoung/ply) records
its work in Ask sessions, so actions and verifier evidence can be inspected
without learning another log format. [Hone](https://github.com/patrickyoung/hone)
uses qualifying checked recoveries from that history to propose lessons.

## Know what a search covers

Search ignores case and folds whitespace for matching, but prints original
text. It covers user/assistant text, reasoning, attachment names and media
types, notes, retry/terminal errors, and request model/effort labels.

It does not search system prompts, attachment bytes, provider-native opaque
state, digests, schemas, or unknown event fields. Use `show` when you need
those raw records. Output is not redacted; archives can contain private data.

Archive scans visit regular `.jsonl` files in lexical path order. A damaged
file produces an `error` record while the scan continues. A torn final line
is reported as a `warning` and ignored in memory; the file stays unchanged.
Events larger than 64 MiB are refused, never truncated.

## Output and outcomes

| `kind` | What it carries |
| --- | --- |
| `session` | One archive entry |
| `match` | Matching event text |
| `event` | An original event from `show` or `window` |
| `node`, `edge` | Recorded lineage |
| `check` | Ask's replay result |
| `warning`, `error` | A condition encountered while reading |

Exit 0 means a successful result or an all-good check. Exit 1 means no match,
damage, a missing sequence, or failed replay; already printed records may
still be useful. Exit 2 means usage or an operational failure prevented the
requested scan. Fatal diagnostics go to stderr.

`ASK_DIR` selects the default archive; `ASK` selects the Ask executable used
for checks. Existing file paths work as session arguments; bare IDs resolve
under the default archive.

## Reference and development

```text
trail ls [dir]
trail find query [dir]
trail show session
trail window [-before n] [-after n] session seq
trail lineage session [dir]
trail check [dir]
trail help
trail version
```

See [GUIDE.md](GUIDE.md), [trail.1](trail.1), and [SECURITY.md](SECURITY.md).
Contributors should read [AGENTS.md](AGENTS.md), run `go test ./...`,
`go test -race ./...`, and `go vet ./...`, then build and smoke-test the binary.
[MIT license](LICENSE).
