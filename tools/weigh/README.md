# Weigh

**Give software typed judgments and their native probabilities.**

Weigh evaluates explicitly supplied state against named choice, score, or
probability questions. It returns one validated JSON result. Any program can
use it; a worker, agent loop, or Bench installation is not required.

```sh
weigh -m 'openrouter/~typesafe/jev-latest' < request.json > result.json
```

The first backend uses OpenRouter's native Decisions API. An explicit model
and `OPENROUTER_API_KEY` are required, unless a private authorization header is
supplied with `-header-fd`. Requests use your provider account. The mutable
alias above is passed verbatim; select a provider version for comparisons.

## Install

From the Bench tools checkout:

```sh
python3 scripts/install weigh
```

Or build this independent module with Go 1.26 or later:

```sh
go build -o weigh .
./weigh version
```

The runtime uses the Go standard library and does not require Ask, Agent, or
another Bench command. Installation and offline checks make no model calls.

Bench's Hire builder defaults new Weigh use to off and preserves existing
dependencies during unrelated revisions. The supplied check/improvement
adapters also default Weigh execution to off. Set `BENCH_WEIGH=1` to permit it;
unset, empty or `0` means off. Enabling it selects no model or provider access.
Only a selected live Weigh path needs this external service; deterministic rules
and explicit Ask routes remain usable without it. An unavailable required
judgment fails without accepting the candidate or switching models.

Single-call adapters select their backend and model explicitly; the semantic
batch evaluator additionally requires `--live`. This is a caller convention:
the independent `weigh` command above executes an explicit request regardless
of `BENCH_WEIGH`. Hire requires no universal Weigh decision or manifest.

## One explicit request

Save this as `request.json`:

```json
{
  "version": 1,
  "state": {
    "claim": "Customers can export CSV files.",
    "passage": "Download your records as a CSV file."
  },
  "questions": {
    "support": {
      "type": "choice",
      "question": "Does the passage support the claim?",
      "options": {
        "supported": "The passage directly supports the claim.",
        "contradicted": "The passage directly contradicts the claim.",
        "insufficient": "The passage establishes neither."
      }
    }
  }
}
```

State can be a JSON string, object, or array. Questions share the state; they
do not consume one another's answers. Put the meaning in question text and
descriptions, not merely in IDs. Weigh supplies no domain policy or sources.

| Type | Required fields beyond `type` | Result |
| --- | --- | --- |
| `choice` | `question`, `options`: map of 2-255 IDs to descriptions | Option ID and complete probability map |
| `score` | `question`, `levels`: ordered array of 2-10 descriptions | Fractional zero-based position and complete probability map |
| `probability` | `question`: proposition/question text | Value between zero and one |

Descriptions are nonempty strings. Unknown fields, duplicate keys, missing
questions, trailing documents, invalid Unicode, and unsupported shapes fail
before inference. Nested state numbers retain their original precision.

## Read the result

The following is illustrative, not an observed model result:

```json
{
  "version": 1,
  "model": {
    "requested": "openrouter/~typesafe/jev-latest",
    "reported": "typesafe/jev-1.13"
  },
  "answers": {
    "support": {
      "type": "choice",
      "value": "supported",
      "probabilities": {
        "supported": 0.96,
        "contradicted": 0.01,
        "insufficient": 0.03
      }
    }
  }
}
```

Score maps use `"0"`, `"1"`, and so on for the requested level positions.
The fractional value must match the distribution's expectation. Probability
answers have a `value` and no fabricated distribution. Choice labels must
belong to the declared options. Distributions must have full support, values
in [0,1], and nonzero total mass. Weigh never fills, renormalizes, or rounds
provider answers.

Native scores and probabilities have been observed with separate rounding to
hundredths. For compatibility, a value that is an exact multiple of 0.01 gets
a bounded ±0.005 interpretation interval, clipped to its allowed range; more
precise values get no rounding allowance. A distribution must permit total
mass one within those intervals. A score must be in range, and its interval
must intersect the possible weighted means of that normalized distribution.
These checks use 1e-6 absolute numerical slack. This is Weigh's empirical
compatibility policy, not a provider precision guarantee. Output retains every
native number, so callers must apply their policies to the reported values.

Optional `metadata` contains service-reported `request_id`, `provider`, `usage`
(`input_tokens`, `output_tokens`, optional `cost`), and `confidence` keyed by
question ID. Missing fields stay missing. Provider confidence describes the
provider's quantity; it is not a universal probability of correctness. A
reported model name is not proof of immutable weights.

**Exit 0 means valid inference, not that a candidate passed a check.** Your
program applies thresholds and decides whether to accept, reject, or escalate.
If composing a Ply checker, translate any Weigh failure into the checker's
broken-check status rather than passing exit 1 through as candidate rejection.

## Keep evidence outside the filter

With Record installed:

```sh
record run -f judgment.jsonl -- \
  weigh -m 'openrouter/~typesafe/jev-latest' < request.json > result.json
record replay -f judgment.jsonl > replayed-result.json
```

Record retains the request, result, diagnostics, and process outcome; replay
does not contact the model. Weigh creates no session or background service.
See the [semantic checker](examples/semantic-check/README.md) and
[visual checker](examples/visual-check/README.md) for caller-owned compositions.
The [improvement workflow](examples/improve-checks/README.md) adds optional
diagnosis, next-fix selection, offline calibration and exact proposal review
around existing tools. Its `select-fix.py` helper lets trusted rules settle
obvious repairs and optional Weigh select a remaining semantic action from a
caller-supplied menu. It returns an ID, never a command or acceptance decision;
the caller executes the bounded action and retains its independent final check.

## Credentials and transport

`OPENROUTER_API_KEY` supplies a Bearer credential. Alternatively, provide a
single `Authorization: ...` header through descriptor 3 or higher:

```sh
weigh -m 'openrouter/~typesafe/jev-latest' -header-fd 3 \
  < request.json 3< /private/authorization-header
```

The descriptor takes precedence, is consumed once through EOF, and is limited
to 8192 bytes. A final LF or CRLF is allowed. Credentials never enter normal
input, argv, or diagnostics. Credential creation and refresh remain external.
If Record wraps this invocation, explicitly use its `-pass-fd` mechanism.

`-timeout` is a positive duration, default `30s`, covering input, header reads,
and inference. SIGINT/SIGTERM cancel the operation. `-endpoint` overrides the
full URL, normally `https://openrouter.ai/api/alpha/decisions`. HTTPS is required;
HTTP permits only literal loopback IPs for local fixtures. URL credentials,
queries, and fragments are refused. Redirects are never followed. Standard
HTTP proxy environment behavior follows Go's transport.

There is at most one inference request per invocation, with no retry or
fallback. A timeout does not establish that the service performed no work or
charged nothing. Error bodies and private request data are not printed.

## Bounds and outcomes

Requests and responses are each limited to 8 MiB, JSON nesting to 64
containers, questions to 1024, and question/option IDs to 256 UTF-8 bytes
without control characters. Excess data is refused, never truncated.

| Exit | Meaning |
| --- | --- |
| 0 | Valid inference, including negative or uncertain judgments |
| 1 | Runtime, credential, provider, protocol, cancellation, or output failure |
| 2 | Invalid invocation or request; no inference attempted |

Response validation finishes before stdout. Output-write failure can leave
partial bytes; check the process status before consuming the result. A shell
pipeline must preserve upstream failures.

## Validation status

The offline tests exercise the documented OpenRouter alpha protocol and Unix
boundaries. Live compatibility smoke checks on September 18, 2026 used alias
and versioned selectors and reported `typesafe/jev-1.13-20260917`. Limited
synthetic text and image checks completed; they do not establish held-out
calibration or improved final-output quality. OpenRouter makes distributions
optional; Weigh requires them, so a missing distribution fails explicitly.
Evaluate the selected model and rubric on held-out cases before using scores
for acceptance.

See the [design](DESIGN.md) for primary protocol references, [manual](weigh.1)
for the full interface, and [development guidance](AGENTS.md) before changes.
Run `go test ./...`, `go test -race ./...`, and `go vet ./...` for offline checks.
