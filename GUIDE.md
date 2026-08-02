# The ply field guide

What it is good at, what it is not, the recipes, and the four things that
will bite you. `README.md` is the pitch and `ply.1` is the reference; this
is what you learn in the first week.

## What it is good at

**Anything a program can check.** This is the whole sweet spot. If you can
write the command that decides, `ply` will work until that command is happy,
and `&&` afterwards means what it says.

```
ply -sh -check 'go test ./...'          "make the tests pass"
ply -sh -check 'go vet ./... && gofmt -l . | grep -q ""' "quiet the vet warnings"
ply -sh -check 'terraform validate'     "fix the module after the provider bump"
ply -sh -check 'curl -fsS localhost:8080/health' "get the dev server up"
ply -sh -check 'test -f dist/app'       "get this thing to build"
```

**Narrow, repeatable jobs with a small toolbox.** A directory of six
programs is a better agent than a shell, because the model cannot wander
into `curl` and it cannot mistake your build for somebody's.

**Work you want a transcript of.** stderr is a terminal session; the log is
an `ask` session that replays exactly. Nothing about a run is unavailable
afterwards.

## What it is not

**It is not a chat.** There is no REPL and no back-and-forth. `ply` runs
until it is done, fails, or hits a cap. If you want to talk about the
result, `ask` is right there — and the run is an `ask` session, so it
already remembers:

```
$ ply -sh -check 'go test ./...' "make the tests pass"
$ ask -f ~/.ply/sessions/20260801-142233-a3f9c1e0.jsonl "why was that failing?"
```

**It is not a sandbox.** See SECURITY.md; the short version is that `-t`
aims the model and the process is the boundary.

**It is not for goals no program can judge.** You can run them — `ply -sh
"tidy up the docs"` works — but exit 0 then means "the model stopped", and
you are back to reading the output yourself. That is a real mode. It is
just a weaker one than it looks, and the default prompt tells the model as
much.

## Six things that will bite you

### 1. `ask` inside a check continues *your* conversation

This is the sharpest edge in the whole system, and it bit the person who
wrote `ply`. `ask` continues the current conversation by default. A check
like this:

```
-check 'ask -q "is this README any good? yes/no" < README.md | grep -qi yes'
```

runs inside whatever you were last asking `ask` about, appends the README to
it, and does that once per cycle. The judgment is polluted and so is your
conversation. Always give a model-judge its own thread:

```
-check 'ask -n -q -f /tmp/judge.jsonl "...yes or no..." < README.md | grep -qi "^yes"'
```

`-n -f` is the same trick `brief find -ask` uses, for the same reason. And
`grep -qi "^yes"` rather than `grep -qi yes`, or "no, because yes would
be..." passes.

### 2. Without `-check`, exit 0 is an opinion

Models announce success. In the run that shipped this program, the model
wrote *"Fizzcheck passes successfully with no reported errors"* in the same
breath as the command that would have told it otherwise — the command had
not run yet. `fizzcheck` then exited 1 and it went back to work.

That is the feature working. Without `-check`, nothing catches it.

### 3. The model cannot see output it has not waited for

A reply can hold several blocks; they all run, in order, before the model
sees any of them. The prompt says so, plainly, and models still sometimes
write four blocks and then reason about the second one's output. Two
consequences worth knowing:

- Cheap, obvious sequences in one block are fine and fast: `cd x && make`.
- Anything where step two depends on reading step one belongs in the *next*
  turn. You cannot force this, but `-check` makes guessing expensive rather
  than final.

### 4. One writing worker per tree

Sessions are safe under concurrency; your working directory is not. Fan out
with `-C`, one directory per worker:

```
ls -d ./services/*/ | xargs -P4 -I{} ply -sh -C {} -check 'make test' "fix the build"
```

Four `ply`s in one tree will interleave their edits and each will be
surprised.

### 5. It reads stdin, because it is a filter

`ply "goal"` with something holding a pipe open on its stdin waits for that
pipe to close, exactly as `cat` and `grep` do — the goal in argv does not
excuse it, because `git diff | ply "fix what this shows"` has to work. If a
run seems to be thinking for a very long time before it says anything,
that is what is happening, and after a second it says so:

```
ply: waiting for stdin to end (^D closes it, ^C gives up)
```

Supervisors, CI runners and cron sometimes hand a program a socket that
nobody ever writes to or closes. `< /dev/null` settles it for good, and
belongs in any `ply` line that is not meant to read anything.

### 6. Keep the session out of the work tree

The default puts it in `~/.ply/sessions`, which is fine and needs no
thought. `-f` is where this goes wrong:

```
$ ply -sh -C ./src -f ./src/run.jsonl "fix the build"     # don't
```

The session is now a file in the directory the model is working in, so an
ordinary `grep -rn foo .` or `find . -type f` reads the transcript back
into the conversation it is a record of. You pay for several turns of the
model's own reasoning to tell it what it already knew, and on a provider
that returns encrypted reasoning most of those bytes are not even legible
— they are just bulk, and they push out the output cap for everything
else in that command.

Nothing breaks. It is a waste, and it compounds: the log grows every turn,
so the same `grep` costs more each time round the loop. `ply` says so once
on stderr when it notices:

```
ply: the session is inside the work tree, so a grep or a find will
     read it back into the conversation it is a record of; -f a path
     outside the tree keeps the record out of the work
```

`$PLY_DIR` set to somewhere inside a repository does the same thing more
quietly, because then it is every run and not just the one you typed `-f`
on. The same goes for large piped input, which spools next to the session
as `<id>.stdin`.

## Recipes

### Build a toolbox

The toolbox is a directory. That is the entire format.

```
mkdir -p tools
ln -s $(which git rg sed jq curl) tools/
ply -t tools "find every TODO older than a year and list them by author"
```

Your own programs go in the same place, and introduce themselves in the
first comment after the shebang:

```sh
#!/bin/sh
# stage - deploy the current branch to staging and print the URL
```

Check what the model will actually see before you spend a call on it:

```
ply tools -t tools
```

Keep a toolbox per job and point `$PLY_TOOLS` at the one you use most.

A script in a toolbox brings its interpreter's name with it, and `-t` means
PATH is the toolbox and nothing else — so the interpreter has to be in
there too, or the tool fails with `env: python3: No such file or
directory`, which reads like a broken tool and is not:

```
ln -s "$(command -v python3)" tools/
```

`ply tools -t tools` will not catch that one, because the program is there
and it is executable. Run it once yourself.

### Change a file without rewriting it

Most of the time a model writing a file should just write it: `cat > x
<<'EOF'` is a shell builtin and a redirect, and for a file it is producing
whole there is nothing better. The case that needs a program is three
lines in the middle of nine hundred, where `>` is not an option and `sed`
means escaping regex metacharacters out of the code being edited — which
models get wrong, and get wrong silently.

[`contrib/edit`](contrib/edit) is that program:

```
ln -s "$PWD/contrib/edit" tools/
ln -s "$(command -v python3)" tools/
ply -t tools -check 'go test ./...' "fix the percentile bug"
```

Its one rule is that the search text must appear exactly once, matched
byte for byte. It is never fuzzy, it never takes the first of several
matches, and it locates every edit in a call before writing any file — so
a call that cannot be satisfied leaves every file it named alone. When it
cannot place an edit it does the analysis a fuzzy matcher would have done
and reports it rather than acting on it: which line nearly matched, and
whether the difference was tabs, spacing, CRLF, an ambiguous match wanting
more context, or a replacement already in place because the edit already
ran.

Three ways to call it, all held to the same rule:

```sh
edit ring.go 'r.head + 1' '(r.head + 1) % len(r.buf)'   # one edit, no format

edit ring.go <<'EOF'                                     # many, atomically
<<<<<<< SEARCH
	r.head = r.head + 1 // BUG: never wraps
=======
	r.head = (r.head + 1) % len(r.buf)
>>>>>>> REPLACE
EOF

edit <<'EOF'                                             # or as a patch
*** Begin Patch
*** Update File: ring.go
@@
 func (r *Ring) Push(v int) {
-	r.head = r.head + 1
+	r.head = (r.head + 1) % len(r.buf)
 }
*** End Patch
EOF
```

Two dialects is not indulgence, it is measurement. Which edit format a
model reaches for is trained in and a system prompt does not talk it out
of one: on the run that shipped this, the model wrote `*** Begin Patch`,
was refused, and spent the next turn building a `sed` pipeline with a
temporary file — the exact thing `edit` exists to prevent. Both dialects
say the same thing anyway.

`edit -n` prints the diff and writes nothing. Exit 0 applied, 1 refused,
2 broken — so `edit ... && go test ./...` means what it says.

`sh contrib/edit_test.sh` is its test suite.

### Put it in a Makefile

The pre-check is what makes this safe: a target that is already satisfied
costs nothing.

```make
lint:
	ply -sh -q -check 'golangci-lint run' "fix what the linter is complaining about"

release: lint
	ply -sh -q -check 'go test ./...' "make the tests pass"
	goreleaser release
```

### Put it in a git hook

```sh
#!/bin/sh
# .git/hooks/pre-commit
ply -sh -q -check 'gofmt -l . | grep -q .; test $? -eq 1' "gofmt everything" || exit 1
```

### Judge with a model, when no program can

Through the same hole as everything else, because `ask` is a program. Mind
bite #1.

```sh
judge() {
  printf '%s' "ask -n -q -f $(mktemp -t judge).jsonl '$1 Answer only yes or no.'"
}
ply -sh -check "$(judge 'Does this CHANGELOG entry explain the user-visible change?') \
     < CHANGELOG.md | grep -qi '^yes'" "write the changelog entry for HEAD"
```

### Brief it, then set it to work

```
ply -sh -s web-perf -check './budget.sh' "get LCP under 2.5s"
ply -sh -s -        -check 'wrangler deploy --dry-run' "ship this worker"
```

`-s -` asks `brief` to choose. `brief` refuses to guess, so a goal made of
common words gets no skill and says so — which is the right answer, because
a confidently wrong procedure is worse than none.

### Sub-agents, without a sub-agent feature

Every command gets `$PLY`. So a specialist is a file:

```sh
#!/bin/sh
# review - review one file for concurrency bugs and print findings
exec $PLY -t "$(dirname "$0")" -q "review $1 for data races and lock-order inversions"
```

Drop that in the toolbox and the outer model can hire it, or you can:

```
git diff --name-only main | xargs -P4 -n1 tools/review
```

There is no team format and no orchestrator. `xargs` was the orchestrator.

### MCP

`ply` does not speak MCP and is not going to. An MCP server is a tool
cabinet and a bridge CLI is the key — the same answer `mu` gives, for the
same reason: the protocol lives at the edge, as an adapter, not in the loop.

Install a bridge. [`mcptools`][mcptools] is one binary, no config file, all
three transports:

```
go install github.com/f/mcptools/cmd/mcptools@latest
```

Then the whole integration is one symlink, and the cabinet is open:

```
ln -s $(which mcptools) tools/
ply -t tools "list what the jira server offers, then file the bug I described"
```

[mcptools]: https://github.com/f/mcptools
[mcpc]: https://github.com/apify/mcpc

That is the `mu` form, and it works. But `ply` can do better, because in
`ply` a blessing is *enforced* rather than advised — the model can only
name what is in the directory. So make each MCP tool its own program:

```sh
#!/bin/sh
# file a bug on the corp jira -- {"title": string, "body": string}
exec /usr/local/bin/mcptools call create_issue --params "$1" \
     /usr/local/bin/mcpc @jira
```

Now the model gets `create_issue` and nothing else from that server: not
`delete_project`, not `list_users`. `ply tools` shows it beside `git` and
`sed`, because at that point it *is* beside `git` and `sed` — once an MCP
tool is a program, it is not a special kind of thing any more.

Writing those by hand is tedium, and it is unnecessary: `tools/list` already
returns a name, a sentence and a JSON schema, which is exactly a synopsis, a
`-h`, and a call. [`contrib/mcpbox`](contrib/mcpbox) turns one into the
other:

```
$ mcpbox tools/ npx -y @modelcontextprotocol/server-everything
mcpbox: wrote 13 programs to tools
$ rm tools/get-env                 # bless by deleting
$ ply tools -t tools
  echo     Echoes back the input string -- {"message": string}
  get-sum  Returns the sum of two numbers -- {"a": number, "b": number}
  ...
$ tools/get-sum 17 25
The sum of 17 and 25 is 42.
```

The arguments are positional, in schema order, and typed from the schema —
`get-sum 17 banana` says `b must be a number, not 'banana'` and exits 2. The
JSON form still works for anything awkward:
`get-sum '{"a":17,"b":25}'`.

That is not a convenience. The first cut of `mcpbox` took JSON only, and a
model handed `resolve-library-id` read its `-h`, saw `usage: <json>`, and
typed `resolve-library-id zod` anyway — then did it twice more. It was
right and the wrapper was wrong: a Unix program that takes a library name
takes a library name. Half the run went on arguing about it.

The directory is the allowlist. `rm` is how you revoke.

And the catalogue earns its keep hardest here, because an MCP description is
written to be injected into a model's context whole, so it is routinely a
page long. [Context7][context7] spends **2,435 bytes** describing two tools
— one of them a 2 kB essay on how to choose a library. As a `ply` toolbox
that is **347 bytes**:

```
$ mcpbox tools/ npx -y @upstash/context7-mcp
$ ply tools -t tools
  query-docs          Retrieves and queries up-to-date documentation and code exam...
  resolve-library-id  Resolves a package/product name to a Context7-compatible lib...
```

Seven times smaller, and nothing is lost: the essay is still there, one
`query-docs -h` away, for the model that has already decided to call it.
This is `brief`'s argument about skills, arriving unchanged for tools —
which is not a coincidence, because it was never an argument about prose.

[context7]: https://github.com/upstash/context7

Four things that bite, three of them found the hard way:

- **A server needs a `PATH` of its own.** `npx` is `#!/usr/bin/env node` and
  `uvx` is much the same, so under `-t` — where the toolbox is the entire
  `PATH` — the server never starts and you get `initialization timed out`.
  `mcpbox` bakes a `PATH` into each wrapper for this. It does not leak: the
  model still cannot name `node`, because it cannot name a program's
  insides. This is the same rule as the check — a blessed program is the
  caller's, not the model's.
- **Absolute paths, always**, for the bridge and the server both. Same
  reason.
- **A tool error is not always a non-zero exit.** `mcptools` prints
  `MCP error -32602: ...` and exits 0. The model reads the text and copes;
  a `-check` written against the exit status will not. Check the artifact,
  not the call.
- **stdio servers spawn per call** with `mcptools`, so nothing persists
  between them. When the integration needs a session, OAuth in the
  keychain, or exit codes you can branch on, [`mcpc`][mcpc] is the fuller
  key and the operator connects it once — the wrapper names only `@jira`
  and never holds the token.

### As a filter, mid-pipe

```
kubectl logs deploy/api --since=1h | ply -t tools "what is causing the 500s?" | tee triage.md
```

Big input spools to a file next to the session and the model is told the
path, so a 40 MB log becomes something to `grep` rather than a tax on every
request.

### On a schedule

```
*/30 * * * * ply -sh -q -check '/usr/local/bin/slo-ok' "bring the error budget back"
```

The quiet runs are free: the check passes, `ply` prints nothing, exits 0,
and never calls a model.

## Reading a run afterwards

```
$ ls -t ~/.ply/sessions | head -1
20260801-142233-a3f9c1e0.jsonl

$ ask replay ~/.ply/sessions/20260801-142233-a3f9c1e0.jsonl   # for a human
$ ask replay -json ~/.ply/sessions/...                        # for a program
$ ask replay -check ~/.ply/sessions/...                       # prove it
ok: 20260801-142233-a3f9c1e0.jsonl replays exactly (24 events)
```

The commands are in the assistant turns, their output is in the user turns.
To see every command a run ran:

```
ask replay -json "$s" | jq -r 'select(.type=="assistant")
  | .data.blocks[]? | select(.type=="text") | .text' | grep -A100 '^```ply'
```

## Tuning

| symptom | flag |
| --- | --- |
| a command hangs on a prompt | it already gets `/dev/null`; raise `-timeout` only if it is genuinely slow |
| the model keeps re-reading a huge file | raise `-cap`, or give it `head`/`grep` and let it narrow |
| it churns without converging | lower `-cycles`, and make the check's *output* more specific — that text is what it reads |
| it costs more than it should | `-turns`, and a smaller toolbox: fewer wrong turns are available |
| you want a cheaper model | `-m anthropic/claude-haiku-4-5-20251001`, or `$ASK_MODEL` |

## The check is also a tool

The system prompt tells the model, verbatim, what command decides. With
`-sh` — or with the check's program in the toolbox — the model can therefore
run it itself, and it does: in a six-turn run that documented a small shell
tool, the model called the same `ask` judge three times of its own accord
before it was willing to stop, and `ply` ran it twice more — once before the
first turn, and once at the end.

That is worth knowing for two reasons. It converges faster than the cycle
loop alone, so a good check pays for itself twice. And your check will run
more often than `-cycles` suggests, so if it is slow, expensive, or has side
effects, either make it cheap or keep its program out of the toolbox.

## Tuning, continued

The single highest-leverage tuning knob is the check's failure output. It is
the only feedback the loop has that nobody wrote by hand, and a check that
prints `FAIL` teaches the model nothing that a check printing
`want: 3, got: 2 (ring_test.go:41)` does not teach it in one turn.

## The cheapest possible sanity check

Before a real run, on a real toolbox:

```
ply tools  -t tools      # what can it reach?
ply system -t tools -check 'make test'   # what is it being told?
```

Both print exactly what the run would use. Neither calls a model.
