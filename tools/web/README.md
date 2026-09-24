# web

A Go command that renders a page or executes an explicit browser plan. Page
content goes to stdout; diagnostics go to stderr. Web contains no model,
thresholds, task loop or shared Bench runtime.

```sh
go build -o /tmp/web .
/tmp/web check                     # offline, no browser needed
/tmp/web setup                     # print installed browser path
/tmp/web get https://example.com
/tmp/web snapshot https://example.com --records-selector 'article'
```

Requires Go 1.26+ to build and installed Chrome/Chromium to browse. `WEB_BROWSER`
selects an executable; otherwise Web finds installed Chrome/Chromium. Install
and update the browser through your OS package manager. `web setup` checks it;
Web never downloads a browser or selects an ambient login profile. The runtime
needs no Python, Node, Playwright, other Bench command, or daemon for reads.

## Read a page

`get` (also `read`) prints readable Markdown; `text` removes link markup;
`html` prints rendered HTML; `links` prints deduplicated `label<TAB>URL` lines;
`shot URL [out.png]` saves a full-page PNG (default `shot.png`) and prints
its filename. The text reducer
drops script/style, navigation, sidebars and footers. Snapshot preserves visible
navigation links so a caller can decide where to go next.

```sh
web get https://example.com | ask 'Summarize this page.'
web snapshot https://example.com/catalog --records-selector '.product' > page.json
```

Snapshot is one JSON object with `version:1`, `requested_url`, final `url`,
`title`, body `text`, `links:[{label,url}]` and
`records:[{text,links:[{label,url}]}]`. It collects all fields in one evaluation,
resolves links against the live document base, preserves row/link ownership,
and omits hidden elements. An empty selection is `[]`. It does not traverse
iframes or shadow roots. Output over 8 MiB fails without partial stdout.
An operator-selected record selector is data, never JavaScript source.

Render options (before or after URL): `--wait domcontentloaded|load|networkidle`
(default domcontentloaded), `--timeout MS` (default 30000), and either
`--profile FILE` or `--attach ENDPOINT`. A ready event does not prove an app has
finished rendering; use a plan with an explicit selector wait when necessary.
A page can execute JavaScript or cause server effects even on a read. Web is
not a sandbox, and its action gate is not a proof that other steps are harmless.

## Execute a supplied plan

```sh
web run <<'PLAN'
[
  {"//":"Read the live result after the app renders"},
  {"goto":"https://example.com"},
  {"wait":"h1","timeout":5000},
  {"read":"h1"},
  {"html":"body","out":"page.html"}
]
PLAN
```

`web run [FILE|-] [--may-job JOB]` reads a finite JSON array or `{"steps":[...]}` from the named
file or stdin. Steps: `goto:URL`, `type:[CSS,text]` (replace), `select:[CSS,value]`,
`wait:CSS` or milliseconds, `read:CSS`, `html:CSS` (optional `out:FILE`),
`shot:FILE`, `click:CSS`, `submit:DESCRIPTION` (press Enter). Select supports a
string, a list, or `{value:...}`, `{label:...}`, `{index:...}` descriptors. CSS is
the selector language. Password fields are refused; use manual auth.

`timeout` is milliseconds per operation, default 30000 for goto and 10000 for
other browser steps. A goto's `wait` chooses its lifecycle event. A `profile`
field on a step selects storage state for the entire run. Comment-only objects
with `//` keys are skipped. A step with multiple operation keys uses the first
in this order: goto, click, type, select, wait, read, html, shot, submit. Prefer
one operation per step. `submit` with `click` therefore executes as a click.

Stdout is `{ok,ms,steps:[...]}`. Each step has `i` (zero based), `ok`, the
operation and any returned `text`, `markup`, file path or `error`. Input text
is not echoed. The first failure stops the plan. Inline HTML above 200,000
characters fails; `out` saves the complete value. Plans/profiles over 8 MiB fail.

Every click, submit and step marked `irreversible:true` asks first. A false
value cannot remove a gate. Set `may:DESCRIPTION` for exact approval words;
otherwise Web uses the operation, selector and host from the plan's preceding
goto. Page content never supplies the words.

With `--may-job JOB`, Web executes literal argv `may request JOB` from PATH,
writing the exact description to stdin. It validates May's JSON verdict against
its exit status and the supplied job/action. Legacy Clerk variables
(`CLERK_WORKER`, `JOB_ID`, `CLERK_WROOT` all set) select `WORKER/JOB_ID` when
no explicit job is given. This uses Bench May, not the old Clerk May protocol.
With no selected job, Web asks on `/dev/tty`.
Only an explicit grant proceeds. Refusal still produces a JSON transcript:
exit 75 parked, 3 declined, 77 no approver; ordinary operation failure is 1.
A caller retrying a parked plan restarts from the beginning. Web does not retry
or infer that earlier actions are safe to repeat.

## Compose with Weigh

Keep the observation, question and judgment as ordinary files/streams:

```sh
web snapshot https://example.com/catalog --records-selector '.product' > page.json
jq '{version:1, state:., questions:{listed:{type:"probability",
  question:"Does the observed page explicitly list a product available to buy?"}}}' \
  page.json > question.json
# Explicit paid inference: choose a model and configure Weigh credentials first.
weigh -m "$WEIGH_MODEL" < question.json > judgment.json
```

This is one observation and one judgment. The caller chooses any acceptance
threshold and subsequent action. Neither Web nor Weigh follows a model-chosen
URL or runs an implicit browser loop. `jq` is only needed for this example.

## Identity and ownership

Fresh invocations use disposable browser profiles, a 1280×720 viewport, and
close their browser.
`--profile FILE` replays selected cookies/localStorage. `--attach ENDPOINT`
uses the default context in the caller's running browser. Endpoints are a port,
HTTP(S) base URL or browser WebSocket URL. `WEB_ATTACH_TIMEOUT` bounds discovery
and connection (default 10000 ms). Connection failure never launches a fallback.

For an operator-owned browser, start Chrome yourself with a dedicated profile:

```sh
# Substitute the installed Chrome executable on your platform.
google-chrome --remote-debugging-port=9222 --user-data-dir="$HOME/.local/share/web-browser"
web get https://example.com --attach 9222
```

Sign in by hand. Attachment carries that browser's existing identity. Web opens
one tab, closes its tab, and disconnects; it never closes the attached browser.
An explicit `web run --attach 9222 --keep` retains its tab only on successful
completion and prints the target ID on stderr. A later
`web run --attach 9222 --tab ID` uses exactly that tab and leaves it open even
on failure. `--keep` and `--tab` conflict and both require attachment. SIGINT
and SIGTERM clean up owned tabs/processes; SIGKILL cannot run cleanup.

## Capture a session

`web auth URL FILE [--channel chrome]` opens a headed browser for manual login;
press Enter on stdin after logging in. EOF cancels saving.
`web auth URL FILE --attach ENDPOINT [--may-job JOB]` asks permission **before connecting** to
export the attached browser's session. The export includes all cookies in the
default context and localStorage from its open HTTP(S) origins. It omits closed
origins, IndexedDB, sessionStorage and passwords. Files use the Playwright
cookies/origins JSON shape and are atomically written with mode 0600. Treat
these session files as credentials. Partitioned cookies are refused on export
and replay so their scope cannot silently widen; use `--attach` for those sessions. `--profile` and `--attach` conflict.

## Audit, checks and migration

`WEB_STATE` selects the best-effort append-only audit JSONL. Default:
`$XDG_STATE_HOME/web/web.jsonl`, or `~/.local/state/web/web.jsonl`. Records contain
`t,cmd,url,bytes,ms,outcome,mode` and gate `grant,what` when relevant. The audit
is optional diagnostic evidence; it never grants authority.

```sh
go test ./...
go test -race ./...
go vet ./...
go run . check
WEB_TEST_BROWSER=/path/to/chromium go test -run TestBrowser -v -timeout 180s
```

The browser suite builds and exercises the public executable against a local
HTTP fixture and an isolated browser. Set `WEB_TEST_WEIGH` to an independently
built Weigh executable to also check snapshot-to-judgment composition against
a local provider fixture (CI does this). It checks rendering, profile replay,
approval refusals, kept/named tabs, signal cleanup and an unrelated tab's
survival. Ordinary tests need no browser, network or paid model.

Version 2.0.0 ports standalone Web to Go. [DESIGN.md](DESIGN.md) lists intentional
migration changes: external browser setup, user audit path, CSS selectors,
input bounds, complete-or-error inline HTML, final-base URL reduction and
explicit password/EOF refusal. [ORIGIN.md](ORIGIN.md) records source provenance.
See [web(1)](web.1) for the command and exit reference.
