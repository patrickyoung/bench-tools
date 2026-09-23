# Browser tasks with Web and Weigh

Two runnable applications of the September 2026 browser builds reviewed in
[RESEARCH.md](RESEARCH.md): follow observed links to a requested page, and
shortlist rendered records against several requirements in one judgment.

`web` owns the browser and provides observations. `weigh` makes a single typed
inference per invocation. This example owns the navigation budget and decision
policy. Record captures every Web and Weigh invocation for offline replay.
No tool imports another tool's code, and no browser/model daemon is introduced.

These are working CLI compositions, not a general form-filling or shopping
agent. Navigation follows HTTP(S) links on the initial origin, using the URLs
the browser observed. It cannot click a JavaScript-only control, submit a form,
book or buy. Web's existing action gates remain its own. Normal page scripts
and GET requests can have effects; same-origin selection is not a network
sandbox. Redirects are checked after observation, not prevented by this caller.

## Requirements

- Python 3.9+, Weigh 0.2.1+, Record 0.1+ and Ask 0.3+.
- The separate [Web](https://github.com/patrickyoung/web) command with the new
  `snapshot` interface (Web 1.1.0 source), set up using `web setup`.
- Explicit `--live`, `BENCH_WEIGH=1`, an OpenRouter model, provider access and
  application thresholds for inference. No model is selected automatically.

`web snapshot` is an additive change in the separate Web repository. An older
installed Web that lacks it will fail visibly; no alternate browser is used.
The task's external runbook identifies the exact tested source and binaries.
To use an uninstalled source checkout, pass its executable with `--web`.

## Find a page by what it contains

From this directory, with commands on PATH:

```sh
BENCH_WEIGH=1 python3 browser_tasks.py navigate \
  --url https://docs.typesafe.ai/cookbooks \
  --goal 'Find how to check whether a cited source supports a claim.' \
  --model 'openrouter/~typesafe/jev-latest' \
  --accept-at 0.90 --min-confidence 0.80 --max-pages 6 \
  --live --output /absolute/new-results/navigation
```

Those thresholds are illustrative, not calibrated recommendations. Pin the
model and evaluate thresholds on your own goals. If the requested information
is absent, absence must not be reported as successful navigation.
Start from a relevant section or catalog when one is known. A generic landing
page with many loosely related links can produce low-confidence routing and
an explicit `review` result.

The caller observes a page once. It asks whether the page itself contains the
requested information and, speculatively in the same inference, which observed
link to follow. A navigation label alone is insufficient for a page match.
Candidate URLs are deduplicated, fragments removed and already visited URLs
excluded. The model returns an ID; code looks up its actual observed URL.
There is an explicit `stop` option. No model-generated URL, selector, command
or JavaScript is executed.

`--expect-text 'exact source phrase'` adds an independent substring check to a
found page. `found` without this flag means a semantic match, not independently
verified correctness. Results say `semantic_only` or `observed_text`; the latter
establishes that substring, not the entire goal. Low or missing navigation
confidence produces `review`. A dead end produces `not_found`; exhausted page
budget or a redirect cycle produces `stopped`. These are successful application
results (exit 0); provider/process/input failures exit 2 without a result.

## Shortlist actual rows or cards

Write a caller-owned `criteria.json`:

```json
{
  "workspace": "There is a quiet dedicated place to work.",
  "refundable": "Cancellation with a full refund is explicitly offered."
}
```

Select actual records with an operator-written CSS selector:

```sh
BENCH_WEIGH=1 python3 browser_tasks.py shortlist \
  --url https://YOUR-SITE/stays --selector 'article' \
  --criteria /absolute/criteria.json \
  --model 'openrouter/~typesafe/jev-latest' --accept-at 0.90 \
  --live --output /absolute/new-results/shortlist
```

One Choice per record/condition distinguishes `yes`, `no` and `unknown`.
All conditions are sent together, and each refers only to its own record.
Selection needs every condition's yes probability above the chosen threshold;
a sufficiently supported no rejects; everything else stays in review. Missing
cancellation terms cannot be borrowed from the neighboring hotel. Arithmetic,
prices, counts and date comparisons should be handled in code, not added as
JEV reasoning questions. This example preserves exact observed text and links
in every group; it does not generate product facts or a summary.
The distinction between contradictory and missing evidence is itself a model
judgment. Audit negative decisions when discarding a record matters; a valid
typed `no` does not prove that the record contains conflicting evidence.

An empty selector match returns `empty` without inference. More than 1024
record/condition questions or more than 254 eligible navigation links is an
explicit failure. Input is never silently truncated. Narrow the starting page
or record selector. There is no batching that quietly changes question context.

## Evidence and repeatability

Every invocation requires a new output directory. It contains Web snapshots,
Weigh results, ordinary Record archives (including actual request/response
streams), and `result.json`. Use an external directory for real data. A profile
is selected only by `--profile FILE`; otherwise Web uses a fresh anonymous
browser context. Selected page data is sent to the chosen model provider.

`OPENROUTER_API_KEY` is read by Weigh, not this adapter. For the single-call
shortlist path, `--header-fd N` passes a private header through Record's
`-pass-fd` to Weigh. A descriptor is consumed once, so navigation explicitly
rejects that option instead of trying to reuse it across page judgments.

```sh
record check -f /results/navigation/002-weigh.record.jsonl
record replay -f /results/navigation/002-weigh.record.jsonl
record replay -f /results/navigation/002-weigh.record.jsonl -stream stdin
```

Model errors stop the application; they do not become negative labels and do
not trigger a retry or fallback. The raw provider distributions and confidence
metadata remain in the recorded Weigh output. Provider confidence is a property
of its distribution, not a calibrated probability that this task is correct.

## Checks and a local demo

```sh
python3 -m unittest discover -s . -p 'test_*.py' -v
python3 integration.py --web /path/to/web --weigh /path/to/weigh \
  --record /path/to/record --ask /path/to/ask \
  --output /absolute/new-results/offline-browser-check
```

The integration test runs **real Chromium, Web, Weigh, Record and Ask** against
a local synthetic site and a local Decisions fixture with a fake credential.
It traverses four pages, including a redirect and a relative link, and finds
the exact backup instructions. It classifies JavaScript-rendered accommodation
cards into selected, rejected and missing-evidence review groups. It also
checks hidden records, empty selections, default-off behavior, provider failure,
invalid distributions, no retries and replay after the fixture service stops.
It makes no paid model calls and proves no JEV accuracy, latency or cost claim.

To view the synthetic site separately, run `python3 fixture_site.py`; its
printed loopback URL serves `/` for navigation and `/stays` for shortlisting.
The integration outputs retain the exact source pages and process evidence.
A real-model evaluation is separate: freeze independent expected destinations
and record labels, run those same tasks with explicit provider access, and
report success/abstention/error counts, all attempts, end-to-end latency and
reported cost. Do not count model-only `found` as independent success.
