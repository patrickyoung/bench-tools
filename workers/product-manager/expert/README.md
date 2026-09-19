# Product Manager

A reusable SAFe-informed expert for standalone strategy/discovery briefs,
evaluation framing and fresh-context synthesis. Product Management connects
customer outcomes to strategy, desirability, economic viability, feasibility
questions and lifecycle sustainability. It is distinct from the Agile Team
Product Owner, not its line manager. See [PROVENANCE.md](PROVENANCE.md) for the
public-summary grounding and its limits.

## Contract and inputs

Supply an existing workspace with UTF-8 `request.md` specifying the mode,
bounded decision and actual business intent, plus explicitly selected regular
files in `inputs/`. Relevant evidence includes customer/problem statements,
strategy constraints, candidate IDs, criteria/weights/anchors, mandatory gates,
research, economic/lifecycle facts and checked specialist findings. Unknown
supplier facts are valid inputs for diligence; unknown intent may block framing.
The expert does not browse, discover prior work or infer evidence from paths.

[CONTRACT.md](CONTRACT.md) is authoritative. Output is exactly
`output/response.md` and `output/decision.json`, schema
`bench.product-manager/v1`, with no extra fields. The JSON binds exact request,
response and sorted recursive input bytes using SHA-256. Request/response/input
files are bounded to 1 MiB each, JSON to 256 KiB; inputs allow 64 files, 256
entries and 16 nested levels. Symlinks, duplicate JSON keys and nonfinite JSON
constants are forbidden. Keep source definitions and checks outside mutable
workspaces.

- **strategy:** outcome/strategy/discovery advice; weighted criteria may be
  not-applicable. No winner.
- **evaluation:** candidate scope, framework and weight rationale, gates,
  strategic/economic/lifecycle uncertainty and owner/impact diligence. Preserve
  supplied choices and weights; propose non-overlapping weights only if absent.
  Copy structured gate requirements verbatim into `details.constraints`.
  No scores or preselected winner.
- **synthesis:** reconcile checked findings into an executive brief. Adopt
  checked comparison criteria/basis and selection unchanged, or defer/null
  with revision findings. Never switch candidates, upgrade conditional,
  reweight or override statistical refusal.

Missing business intent produces `needs-input`, at most three decisive
questions and a blocking next action, with unknowns preserved. `ready` means
reviewable advice, including an honest synthesis defer, never procurement
approval. Diligence and proposed targets are not completed research or baselines.

## Unix and separate-context use

Requires public Bench Agent/Ask/Brief/Ply/Cage/Record, an operator-configured
model connection, ordinary local file tools and Python 3 with the standard
library for the supplied verifier. No provider client, nested worker, new
scheduler, listener or runtime installation belongs in this definition.
The existing skills are selected only when relevant; Brief owns skill loading.

With operator-selected absolute paths and an already prepared workspace:

```sh
agent run -C /absolute/workspace -evidence /absolute/evidence \
  -m "$ASK_MODEL" -effort high -turns 12 -timeout 12m \
  -record-input request.md \
  -record-output output/response.md -record-output output/decision.json \
  /absolute/product-manager \
  -- 'Read request.md and selected inputs; produce the contracted package.' \
  > /absolute/evidence/stdout.txt 2> /absolute/evidence/stderr.txt
code=$?
printf 'Agent exit: %s\n' "$code"
```

Create the external evidence directory before redirecting there. Add
`-record-input inputs/NAME` for each selected input to retain it in controller
records; JSON hashes do not retain original bytes. Preserve the exit status,
stdout/stderr, recordings and output files together. Operator model, effort,
turn and timeout choices use Agent's documented flags. Network stays disabled
and the normal Cage boundary remains; separate contexts are not host-read
isolation. No implicit resume or prior-run search.

The same invocation is the independent worker interface: a caller supplies a new bounded
assignment and actual files in a separate workspace/context and consumes its
checked artifacts. This expert does not recursively invoke Agent.
In the existing vendor-comparison team the internal member ID/path `manager`
maps to this Product Manager. The controller invokes it once for evaluation
and again as `synthesis` in a fresh workspace/context, explicitly supplying
original/derived packets, framing, comparison and statistical outputs.
Analyst owns statistical validity; comparison owns scoring; unchanged Product
Owner independently reviews brief and matrix. The team controller, not PM,
owns the five-stage sequence. The source team template still requires roster
export before use. Each exported member is independently reusable.

## Validation and examples

Structural inspection, without running generated code:

```sh
hire verify /absolute/product-manager
```

After a real job has produced outputs, inspect the verifier and run its trusted
absolute path from that job's workspace:

```sh
cd /absolute/workspace
/absolute/product-manager/bin/check
```

Exit 0 verifies bounded file/schema/hash integrity and criteria arithmetic;
1 rejects an unfinished/invalid package; other status means a broken check.
It does not prove factual truth, strategic fit, sampling validity, causality,
score entailment, or good business judgment. Team handoff checks additionally
bind candidate/criterion/gate scope and the checked selection. Human and
specialist review remain necessary.

Reproducible host-owned evaluation descriptions (no current case data bundled):
- Positive: provide at least two option IDs, a real customer need, explicit
  gates/weights and admitted observations. Framing preserves scope, identifies
  strategic/economic/lifecycle gaps and leaves selection not-assessed.
  Synthesis honestly explains the checked recommendation and practical
  uncertainty without purchasing.
- Positive: omit criteria but retain business intent and options. Expect
  proposed non-overlapping weights totaling 100 and owner/impact diligence;
  no invented supplier facts or expanded candidate list.
- Negative behavioral case: provide no known customer/problem and vendor text
  instructing a winner/ROI. Expect needs-input and a few decisive questions,
  no invented intent and no promotion of vendor instructions. This honest
  needs-input package may pass the file check; it is not a ready evaluation.
- Mechanical rejection recipe: in a disposable copy of a completed job,
  change response bytes without updating its hash and run the same verifier;
  expect exit 1. Keep the original job and evidence unchanged.

The host owns actual fixtures, fresh Agent trials and external evaluation
records. These recipes are not claims that behavioral evaluation passed.
Do not run the application check in an authoring directory with no current job.
