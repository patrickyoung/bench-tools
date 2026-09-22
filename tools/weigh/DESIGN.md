# Design

Weigh reads supplied state and bounded questions, obtains typed judgments,
validates the whole response, and prints the result. It is an independent
program for any caller, not an Agent subsystem.

## One operation

One JSON request enters stdin. One JSON result leaves stdout. Invocation and
local request errors are exit 2, runtime/provider/protocol errors are exit 1,
and valid inference is exit 0 even when the judgment is negative or uncertain.
Thresholds, acceptance, retry, fallback, and actions belong to caller code.

The first backend is OpenRouter's native POST `/api/alpha/decisions`. The
`openrouter/` model prefix selects that transport; the remaining name is passed
verbatim, including an initial tilde. This is not Chat Completions. Ask's
conversation provider interface does not implement this protocol, and Weigh
does not import its code. Record can capture the invocation through ordinary
streams without a Weigh session format.

## Representation and validation

The version-1 request has exactly `version`, `state`, and `questions`. Questions
are named `choice`, `score`, or `probability` records. Questions, choice
descriptions, score levels, and probability criteria accept nonempty text,
objects, or arrays. Choice descriptions additionally accept null to use the
option name alone. Optional probability criteria contain exactly `true` and
`false` descriptions. Structured content is data, not a Weigh schema language.
The translation changes `question` to `instructions`, options/levels to
`criteria`, and probability to `noul`; explicit probability criteria pass through.
It does not invent prompts. State and descriptions use raw JSON so numeric literals remain
exact. Existing version-1 string requests retain their mapping and behavior.

Before decoding the contract, validate UTF-8, paired Unicode escapes, nesting,
the complete single document, and unique decoded keys throughout it. Unknown
contract fields fail. Neither local error messages nor provider diagnostics
echo private state, question IDs, model arguments, endpoints, or credentials.

Require precisely the requested answer IDs and types. Choice and score must
include native distributions covering precisely the requested support. Every
probability is finite and in [0,1], and total reported mass must be positive.
An exact multiple of 0.01 has an interpretation interval of ±0.005, clipped to
its permitted range; more precise numbers have zero-width intervals. The
probability intervals must permit a normalized distribution. Scores must be
within [0, number of levels minus one], and the score interval must intersect
the possible expected values of such a normalized distribution. Fill interval
capacity from lowest to highest level to find the minimum expectation, and
reverse the order for the maximum. Comparisons use 1e-6 absolute numerical
slack. Never renormalize, fill missing alternatives, round scores, or
manufacture a one-hot distribution. A supplied
score legend must equal the requested levels, ignoring object order and string
escape spelling but preserving array order and exact numeric literal spelling.
Comparison uses JSON numbers rather than float64 so adjacent large integers
cannot become equal through rounding. Choice's returned label need
only belong to its declared support; no undocumented tie or sampling rule is
imposed.

Native confidence, when present, is retained separately as provider metadata.
Jev describes it as a summary of distribution concentration, not a measured
probability of downstream success. Native usage, request identity and
reported model are retained without inventing absent values. Current native
usage fields are `input_tokens`, `output_tokens`, and optional `cost`; unknown
native response or usage fields fail explicitly rather than being interpreted.
The reported model name is not proof of immutable weights.

The hundredth intervals are an explicit empirical compatibility policy. A
synthetic native response on September 18, 2026 reported score 1.87 and level
probabilities {0:0, 1:0.12, 2:0.88}, whose displayed mean is 1.88. Those values
can arise from separate rounding of a normalized distribution and its mean.
The provider documents weighted-mean semantics but no rounding guarantee.
The raw synthetic response is an offline regression fixture. The policy does
not permit arbitrary sum/score errors, extend legal ranges, or discard native
precision; it validates possible rounding without changing any output value.

## Bounds and transport

Input and response are each bounded at 8 MiB, JSON at 64 nested containers,
questions at 1024, IDs at 256 UTF-8 bytes without control characters, choice
support at 2-255 alternatives, and score support at 2-10 levels. String
descriptions must be nonempty; object and array descriptions are passed through
after whole-document validation. Input limits fail before credentials or networking.
These are process limits, not a token estimator or promise of provider capacity.
All response checks complete before stdout. Output I/O failure can still leave
partial bytes, so downstream consumers must check status.

An explicit positive timeout (30 seconds by default) covers reading stdin,
reading a selected authorization descriptor, and network inference. SIGINT and
SIGTERM cancel this work. A canceled operation exits 1; it does not claim that
no billing occurred. A blocked generic input reader remains caller-owned; the
standalone process exits, while owned descriptor and HTTP readers are closed.

The HTTP client makes one POST on a fresh transport, with no retry, redirect,
or fallback. HTTPS is required except for literal loopback IPs used in local
fixtures. Full endpoint overrides reject URL credentials, queries and
fragments. HTTP errors report the numeric status only, never the response
body. Response headers have a 64 KiB limit. Standard HTTP proxy environment
behavior remains available through Go's transport.

`-header-fd` selects one bounded Authorization header on descriptor 3 or above
and takes precedence over `OPENROUTER_API_KEY`. The header is never stdin or
argv. It is consumed once, is at most 8192 bytes, and cannot inject additional
headers. No credential acquisition, persistence, or refresh belongs here.

## Evidence and compatibility

The September 18 implementation and September 20, 2026 capability audit were
checked against these primary protocol references. The latter used OpenRouter
SDK commit `1a09de8a9749c72450bade8a373ed2120a2865c0`:

- [OpenRouter operation](https://github.com/OpenRouterTeam/typescript-sdk/blob/main/src/funcs/alphaDecisionsCreate.ts)
- [Native request](https://github.com/OpenRouterTeam/typescript-sdk/blob/main/src/models/decisionsrequest.ts)
- [Native response](https://github.com/OpenRouterTeam/typescript-sdk/blob/main/src/models/decisionsresponse.ts)
- [Choice question](https://github.com/OpenRouterTeam/typescript-sdk/blob/1a09de8a9749c72450bade8a373ed2120a2865c0/src/models/decisionschoicequestion.ts)
- [Score question](https://github.com/OpenRouterTeam/typescript-sdk/blob/1a09de8a9749c72450bade8a373ed2120a2865c0/src/models/decisionsscorequestion.ts)
- [Noul question](https://github.com/OpenRouterTeam/typescript-sdk/blob/1a09de8a9749c72450bade8a373ed2120a2865c0/src/models/decisionsnoulquestion.ts)
- [Choice answer](https://github.com/OpenRouterTeam/typescript-sdk/blob/main/src/models/decisionschoiceanswer.ts)
- [Score answer](https://github.com/OpenRouterTeam/typescript-sdk/blob/main/src/models/decisionsscoreanswer.ts)
- [Noul answer](https://github.com/OpenRouterTeam/typescript-sdk/blob/main/src/models/decisionsnoulanswer.ts)
- [Score semantics](https://docs.typesafe.ai/primitives/score)
- [Confidence semantics](https://docs.typesafe.ai/confidence)

Offline fixtures establish the process and protocol contracts, including the
failure paths. They do not establish that a currently deployed model returns
all distributions or makes accurate judgments. OpenRouter's alpha schema
makes distributions optional; Weigh deliberately requires them. A missing
distribution is an explicit runtime capability failure. Live compatibility
and held-out task evaluation remain separate, paid, explicitly selected work.

## Model use and caller composition

TypeSafe's current guidance favors independent, atomic questions in one call,
with caller code combining the answers. Choice is relative to its menu; a
separate probability question can test absolute applicability. A model response
does not prove future improvement, and schema validation does not prove factual
correctness or resistance to instructions embedded in input data. Keep exact
arithmetic, source selection, experiments and promotion outside Weigh.

The [AutoResearch cookbook](https://docs.typesafe.ai/cookbooks/autoresearch_feature_discovery)
uses a generative model to propose questions, Jev to produce numeric features,
and external evaluation to retain useful changes. The
[skill suggestion cookbook](https://docs.typesafe.ai/cookbooks/skill_suggestion)
separates relative ranking from applicability and may reject every candidate.
Both compose through existing Weigh primitives; neither calls for adding a
research loop, skill router, training runtime or automatic promotion to Weigh.

## Exclusions

No default model, configuration file, conversation, memory, registry, daemon,
JSONL mode, concurrency controls, cache, generated explanation, threshold,
checker, worker lookup, provider plugin layer, automatic fallback, action, or
new recording format. Examples compose these concerns outside the executable.

## Checks

Run `go test ./...`, `go test -race ./...`, and `go vet ./...`. Tests use local
HTTP fixtures and private subprocess environments; no credentials or live
service are needed. Component integration tests belong to the root and use
the public executable, not imported internals.
