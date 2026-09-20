# Product Manager

A reusable expert governed by SAFe for shaping valuable product and platform
features, standalone strategy/discovery advice, evaluation framing and
fresh-context synthesis. It challenges solution-first requests constructively,
defines useful vertical slices and separates verifiable feature acceptance
from measurable benefit, then integrates a leadership recommendation and ask.
SAFe Product Management owns customer/market needs, vision/roadmap and ART
backlog/features with System Architect, RTE and Business Owners. Product
Owner/team owns team backlog and actual stories/planning; PM is not its line
manager. Customer centricity and design thinking guide desirability, viability,
feasibility and sustainability. See [PROVENANCE.md](PROVENANCE.md) for the
public-summary grounding and its limits.

## Contract and inputs

Supply an existing workspace with UTF-8 `request.md` specifying the mode,
bounded decision and actual business intent, plus explicitly selected regular
files in `inputs/`. Relevant evidence includes customer/problem statements,
strategy constraints, candidate IDs, criteria/weights/anchors, mandatory gates,
research, economic/lifecycle facts and checked specialist findings. Unknown
supplier facts are valid inputs for diligence; unknown intent may block framing.
The expert does not browse, discover prior work or infer evidence from paths.

[CONTRACT.md](CONTRACT.md) is authoritative for the unchanged IO/schema.
Its retained "SAFe-informed" label does not weaken SAFe governance in the
operating instructions. The two-file JSON/prose packaging is Bench storage,
not an assertion that SAFe specifies this schema. Output is exactly
`output/response.md` and `output/decision.json`, schema
`bench.product-manager/v1`, with no extra fields. The JSON binds exact request,
response and sorted recursive input bytes using SHA-256. Request/response/input
files are bounded to 1 MiB each, JSON to 256 KiB; inputs allow 64 files, 256
entries and 16 nested levels. Symlinks, duplicate JSON keys and nonfinite JSON
constants are forbidden. Keep source definitions and checks outside mutable
workspaces.

- **strategy:** outcome/strategy/discovery and feature shaping/sequencing advice;
  weighted criteria may be not-applicable. Selection stays not-assessed/null:
  recommending a first feature is not selecting a vendor. Narrative and useful
  feature detail go in response.md; existing JSON captures decision, outcomes
  and handoff, with no new fields.
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

## Feature and platform use

AGENTS.md and product-management explicitly route feature strategy to
`feature-shaping`, internal/developer platform features additionally to
`platform-product`, and the integrated recommendation to `leadership-narrative`.
Existing discovery, outcomes, capabilities and finops skills remain available
only as relevant. No feature template is imposed on vendor evaluation/synthesis.

Minimal `request.md` pattern (replace placeholders with actual intent and
admitted evidence, not invented case facts):

```text
Mode: strategy
Shape the smallest useful feature for <consumer> doing <job/situation>.
Problem and desired outcome: <observed friction and intended behavior/value>.
Read only request.md and inputs/<selected-evidence-file>.
Evidence locators: <source sections>; baseline/target: <supplied or unknown>.
Constraints/non-goals: <known boundaries>; decision owner: <role or unknown>.
Recommend the next commitment and leadership ask, with useful feature detail.
```

Prepare actual evidence bytes under `inputs/`, listing each selected file in
the request; omit the input line if none are available. Supply known journey,
failure, dependency, economic and strategic facts where consequential; mark
unknowns honestly. Do not assume a cited pathname provides its contents.

Expect a main leadership narrative of about 250–450 words by default, with
feature detail only as useful. Explicit total length limits take precedence.
For a single feature, expect ID/name, context, benefit hypothesis and acceptance
criteria plus a concise business narrative. Broader breakdowns produce cohesive
features and justified sequence tied to vision/customer need/business outcome
and product/solution roadmap, with readiness and unresolved decisions.
Include dependencies, enablers/NFRs, WSJF preparation/prioritization and relevant
PI Planning input as useful. Scope/non-goals, minimum useful slices, failure
paths, measures and validation/handoffs remain useful supporting detail, not
a custom mandatory artifact stack or external product framework.
Readiness is recommended validation/build/refinement in prose, not approval or
a new field. Platform work starts with one evidenced consumer journey and
relevant contracts, self-service, migration/rollback and operating guardrails.

Leadership communication traces vision -> customer need -> feature benefit ->
sequence/dependencies -> business decision and proposes review of actual
outcomes after release. Benefit hypotheses are not realized value; acceptance
verifies feature behavior including relevant NFRs. Roadmaps are forecasts when
appropriate; distinguish supplied current commitments from later forecasts.
Teams create PI Objectives during PI Planning. PM provides product context or
illustrative proposals only, never commits objectives, team capacity,
business-value scores or dates.

Limits: no invented WSJF scores, ROI, funding, dates, agreement or PI fit;
without comparable supplied/agreed relative inputs, sequencing is qualitative
and team estimates remain a next action. Illustrative story-sized slices are
PO/team refinement input, never a committed sprint backlog. Adoption is not
proof of customer/financial value; a plausible hypothesis is not validated
discovery. The expert cannot settle architecture, delivery capacity or funding
authority on another role's behalf. The manual [QUALITY.md](QUALITY.md) rubric
supports review; neither it nor the file checker certifies business quality.

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
- Positive feature strategy: supply a consumer journey, evidenced friction and
  known constraints. Expect a stable-ID useful slice, separate works/matters
  tests, honest baseline/target status, leadership ask and review rule; JSON
  selection remains not-assessed/null.
- Negative feature strategy: request a broad platform and demand WSJF/ROI with
  no size/cost inputs. Expect constructive problem framing, qualitative
  sequencing, validation of unknowns and no fabricated scores or benefit.
  If business intent is absent, expect needs-input, not a fictional journey.
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
