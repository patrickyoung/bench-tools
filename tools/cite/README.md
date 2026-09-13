# Cite

**Catch invented or mismatched citation links before an answer leaves your pipeline.**

An answer can look well sourced while linking to a record that never existed.
Cite catches that specific mistake. It compares a Markdown answer with the Context evidence that was supplied
to its author. Every `ctx:` reference must be an exact `[ref](citation.url)`
link from that evidence. On success, the answer passes through unchanged.
On rejection, stdout stays empty and stderr explains what to fix.

Cite checks **citation identity**. It does not judge whether a source supports
a claim, whether every claim is cited, or whether a source is trustworthy.
That small, precise check makes it useful as a building block.

## Install

From the [Bench tools monorepo](https://github.com/patrickyoung/bench-tools),
run `python3 scripts/install cite` at the repository root. See
[installation and updates](https://github.com/patrickyoung/bench-tools/blob/main/docs/INSTALL.md)
for prerequisites and PATH setup, or
[getting started](https://github.com/patrickyoung/bench-tools/blob/main/docs/GETTING-STARTED.md)
for a guided first result.

**Standalone install.** Requires **Go 1.26+** and **Unix or WSL**. Install
current `main`:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/patrickyoung/cite@main
export PATH="$HOME/.local/bin:$PATH"
cite version
```

Keep the PATH setting in your shell startup file. Cite itself needs no model,
API key, or network connection.

## Check your first citation

Create a tiny, fictional evidence fixture:

```sh
cat > evidence.jsonl <<'JSON'
{"kind":"context","version":1,"source":"demo","type":"document","id":"hours","title":"Demo opening hours","retrieved_at":"2026-01-01T00:00:00Z","content":{"text":"The demo library opens at 09:00."},"citation":{"locator":"hours","url":"https://example.com/hours"},"ref":"ctx:demo:a785c43849ba0d88312f120af2825527"}
JSON

printf '%s\n' 'Opens at 09:00. [ctx:demo:a785c43849ba0d88312f120af2825527](https://example.com/hours)' |
  cite evidence.jsonl
```

You should get exactly the input line back, with exit 0. Now try an invented
reference:

```sh
printf '%s\n' 'Opens at 09:00. [ctx:demo:made-up](https://example.com/hours)' |
  cite evidence.jsonl
echo "$?"
```

Cite explains the unknown ref, prints no candidate, and exits **1**. A bare
ref, a mismatched URL, or no citation at all also fails.

The evidence is an argument because stdin belongs to the answer being checked:

```text
evidence.jsonl ──┐
                ├── Cite ── valid: exact candidate bytes
candidate.md ───┘           invalid: empty stdout + diagnostics
```

## Put it after a model

With [Context](https://github.com/patrickyoung/bench-tools/tree/main/tools/context) and its Wikipedia
connector installed, and [Ask](https://github.com/patrickyoung/bench-tools/tree/main/tools/ask) configured:

```sh
q='How does a ring buffer work?'
context query wikipedia "$q" > evidence.jsonl &&
ask "Question: $q
Answer from the supplied records. Cite factual claims with exact
[ref](citation.url) links from the same records." \
  < evidence.jsonl > candidate.md &&
cite evidence.jsonl < candidate.md > answer.md
```

The `&&` sequence stops if retrieval or writing fails. Use `answer.md` only
when the final check succeeds. The `example.com` fixture above makes no live
source claim; this workflow replaces it with retrieved evidence.

## Let a rejected answer get another turn

[Ply](https://github.com/patrickyoung/bench-tools/tree/main/tools/ply) can give Cite's feedback to the
model and ask it to correct the candidate:

```sh
ply -sh -f cited-run.jsonl -turns 12 \
  -check 'cite evidence.jsonl >/dev/null' \
  "Question: $q
Answer using the supplied evidence. Use exact
[ref](citation.url) links and do not modify evidence.jsonl." < evidence.jsonl
```

This reuses the question and evidence from the previous step. The initial
empty candidate is rejected, starting the work. Later candidates arrive on Cite's stdin. A
rejection becomes feedback; an accepted result receives a sealed Ply verifier
receipt in the Ask session.

This example grants full shell access. For a check the worker cannot change,
keep the evidence and checker outside its write boundary. Cite itself grants
no execution or filesystem isolation.

A direct `ask | cite` pipeline is also possible, but Cite does not write the
Ask log. Use Ply when you need the check result recorded with the run.

For a reusable worker, the
[support-reply expert](https://github.com/patrickyoung/bench-tools/tree/main/examples/support-reply)
puts Cite in `bin/check`. Agent runs that check through Ply and returns a
rejection to the model. The policy records live in the expert definition,
outside the default action write boundary; the draft lives in the workspace.

## Understand the boundary

- Input evidence is a regular file containing normalized Context JSONL.
- Records without `citation.url` may remain in the evidence but cannot supply
  a clickable citation.
- Two URLs for one ref make the evidence invalid; file order cannot choose one.
- Matching is literal, including URL bytes and link formatting. Cite does not
  render Markdown or sanitize HTML.
- At least one valid citation is required, and every `ctx:` occurrence must
  be valid. Unreferenced factual claims are outside this check.

Limits are **32 MiB** of evidence, **8 MiB** per evidence record, and **4 MiB**
for the candidate. Cite buffers the candidate before printing, so a bad final
reference cannot leak an accepted-looking prefix. Excess input is an error.

## Outcomes and next steps

| Exit | Meaning |
| --- | --- |
| 0 | At least one exact citation; no invalid `ctx:` occurrence |
| 1 | Candidate rejected, empty candidate, or no citeable evidence |
| 2 | Usage, I/O, invalid evidence, or a size limit exceeded |

If a candidate fails, use the exact ref and URL from its source record. If
support or factual coverage matters, add an appropriate task-specific review
or check rather than treating citation identity as that judgment.

```text
cite evidence.jsonl
cite version
cite help
```

[GUIDE.md](GUIDE.md) has compositions and [cite.1](cite.1) is the manual.
Contributors: read [AGENTS.md](AGENTS.md), then run `go test ./...`,
`go test -race ./...`, and `go vet ./...`.
[Security](SECURITY.md) · [MIT license](LICENSE).
