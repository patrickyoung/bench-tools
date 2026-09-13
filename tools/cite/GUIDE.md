# Build a cited-answer pipeline

Cite is an identity filter with a condition: a candidate reaches stdout only
when its Context references are exact.

Start with the [offline citation example](README.md#check-your-first-citation)
to see an accepted and a rejected answer. The recipes below assume Ask, Ply,
Context, and Cite are installed, Ask has a model, and the named source connectors
are configured. `glean` is an example connector you supply, not a built-in.

## Ask once

Use a file because the evidence has two consumers: Ask reads its contents and
Cite reads its ref-to-URL index.

```sh
q='What did Pike contribute to Unix practice?'
context query wikipedia "$q" > evidence.jsonl || exit

ask -q "Question: $q

Use only the supplied records. Cite factual claims with exact Markdown
[ref](citation.url) links copied from those records." < evidence.jsonl > candidate.md &&
  cite evidence.jsonl < candidate.md
```

Ask records the complete evidence and candidate in its append-only session.
Cite writes no history of its own. If Cite rejects the output, Ask's session is
still available for review with `ask replay`; the answer simply never reaches
normal stdout through this sequence. Saving the candidate and checking Ask's
status prevents an upstream failure from being hidden by the last command.

## Let Ply correct a rejection

```sh
q='What did Pike contribute to Unix practice?'
context query wikipedia "$q" > evidence.jsonl || exit

ply -sh \
  -check 'cite evidence.jsonl >/dev/null' \
  "Question: $q

Answer from the evidence and cite factual claims with exact
[ref](citation.url) links." < evidence.jsonl
```

The check receives empty stdin before the first model turn and the final report
after each proposed stop. Cite's exit 1 is therefore an ordinary failed check.
Its diagnostic names the unknown ref, missing URL, or exact link required, so
Ply has concrete feedback instead of `FAIL`.

The check runs on the caller's PATH. Cite need not be in Ply's model toolbox
unless the model itself has a reason to invoke it. Context need not be in the
toolbox when retrieval happened before the run.

## Aggregate first

```sh
context query glean "$q" > glean.jsonl &&
context query wikipedia "$q" > wikipedia.jsonl &&
cat glean.jsonl wikipedia.jsonl | context merge > evidence.jsonl &&
ask "Question: $q; answer with exact cited links." < evidence.jsonl > candidate.md &&
  cite evidence.jsonl < candidate.md
```

Cite treats all sources alike. The Context ref is the identity and
`citation.url` is the destination. A duplicated ref with the same URL is one
entry; one ref with two URLs is broken evidence and exits 2.

## What a passing check means

A pass means:

- at least one literal `[ctx:...](url)` citation appears;
- each cited ref exists in the evidence;
- each cited URL is exactly the URL paired with that ref;
- no bare, malformed, unknown, or mismatched `ctx:` occurrence appears; and
- the candidate printed by Cite is byte-for-byte the candidate it checked.

A pass does not mean:

- every factual claim has a citation;
- the cited record supports or entails the nearby sentence;
- the evidence is true, fresh, sufficient, or authoritative; or
- Markdown is safe to render as HTML.

Those statements require task knowledge. A Brief skill can define which
sources and citation coverage are required. A separate semantic check may use
Ask in a fresh named session when entailment genuinely needs model judgment.
Keeping that judgment outside Cite preserves the value of its deterministic
answer.

## Limits

For a complete reusable worker, the
[support-reply starter](https://github.com/patrickyoung/bench-tools/tree/main/examples/support-reply)
puts Cite in an expert's `bin/check` and leaves execution to Agent. That check
reads policy records from the definition outside the default action write
roots. The examples above use ordinary shell authority; an instruction not
to edit evidence is not filesystem protection.

Evidence is limited to 32 MiB, one evidence record to 8 MiB, and the candidate
to 4 MiB. Excess is an error naming the limit. Cite never truncates either
input and never prints a candidate before validation completes.

Evidence and answers can be sensitive. Cite makes no network calls and writes
no files, but the caller-created evidence file remains the caller's
responsibility. Store it with appropriate permissions and retention.
