# Cite

Cite verifies Markdown citations against Context evidence and then gets out of
the way.

```sh
q='How does the Unix philosophy relate to Rob Pike?'
context query wikipedia "$q" > evidence.jsonl

ask -q "Question: $q

Answer from the supplied records. After every factual claim, add a Markdown
link whose label is the exact ref and whose URL is citation.url from the same
record." < evidence.jsonl |
  cite evidence.jsonl
```

On success, stdout is the Ask answer byte for byte. On rejection, stdout is
empty and stderr says what is wrong:

```text
cite: unknown ref "ctx:wikipedia:made-up"
cite: ref "ctx:wikipedia:..." must appear exactly as
      [ctx:wikipedia:...](https://en.wikipedia.org/w/index.php?oldid=...)
```

Cite proves a deliberately small fact: every `ctx:` occurrence in the answer
is a literal Markdown link whose ref and URL are paired in the supplied
Context records. It also requires at least one valid citation. It does not
claim that evidence supports the prose, that every factual claim is cited, or
that the source is authoritative. Those are semantic and policy questions for
a task-specific check.

## Install

Cite requires Go 1.26 or later and a Unix.

```sh
go install github.com/patrickyoung/cite@latest
```

From this source tree:

```sh
go build .
go test ./...
```

## Interface

```text
cite evidence.jsonl
cite version
cite help
```

The evidence path is an argument because stdin belongs to the candidate. The
file must contain normalized `context/v1` JSONL records. Records without
`citation.url` may be present but cannot be cited as links. Conflicting URLs
for one ref make the evidence broken rather than letting file order decide.

The required citation is exact:

```markdown
[ctx:wikipedia:c15c600dd289fe907323ab4150222f29](https://en.wikipedia.org/w/index.php?oldid=1365516372)
```

Whitespace changes, a bare ref, an invented ref, or the right ref with the
wrong URL are rejections. The exact match means URLs containing parentheses
work without Cite inventing a partial Markdown parser.

## With Ply

Ply is useful when a rejected answer should go back to the model instead of
ending the pipeline:

```sh
q='How does the Unix philosophy relate to Rob Pike?'
context query wikipedia "$q" > evidence.jsonl

ply -sh \
  -check 'cite evidence.jsonl >/dev/null' \
  "Question: $q

Answer from the supplied records. Cite every factual claim as an exact
[ref](citation.url) Markdown link." \
  < evidence.jsonl
```

Ply's pre-check gives Cite an empty candidate, so Cite exits 1 and the work
starts. When the model stops, Ply sends the proposed report to Cite on stdin.
An unknown ref or mismatched URL becomes specific verifier feedback for the
next turn. Acceptance is recorded in Ply's Ask session like every other
verifier result.

See [GUIDE.md](GUIDE.md) for composition patterns and limits.

## Unix contract

| channel | meaning |
| --- | --- |
| stdout | the candidate unchanged, only after complete validation |
| stderr | rejection reasons and fatal diagnostics |
| exit 0 | at least one exact citation and no invalid `ctx:` occurrence |
| exit 1 | rejected candidate, empty candidate, or no citeable evidence |
| exit 2 | bad usage, unreadable or invalid evidence, or oversized input |

Both inputs are bounded and the candidate is buffered before any output, so a
bad final citation cannot leave a plausible partial answer in a pipeline.
Evidence is limited to 32 MiB, each evidence record to 8 MiB, and the candidate
to 4 MiB. Excess input is an error rather than a truncated answer.

## Scope

Cite has no model, provider, network, source selection, retrieval, Markdown
renderer, semantic entailment judge, policy engine, cache, database, config,
daemon, or session format. Context retrieves; Ask writes; Cite checks literal
references; Ply retries; Ask and Trail retain history.

## License

MIT. See [LICENSE](LICENSE).
