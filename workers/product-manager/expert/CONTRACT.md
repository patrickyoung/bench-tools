# Product Manager artifact contract v1

Reusable SAFe-informed product-management decision framing and synthesis.
Input: UTF-8 request.md and explicitly selected regular files under inputs/.
No prior-run search, network access or implicit data discovery. Existing source
links are provenance only. Output exactly output/response.md and
output/decision.json. The latter binds exact request, response and input bytes.

Reuse Product Owner's reviewed bounded IO/hash envelope, with a distinct schema
and product-management detail contract. Source/response/input files: 1 MiB each;
decision JSON: 256 KiB. At most 64 input files, 256 entries and 16 nested levels.
No symlinks, duplicate JSON keys, NaN/Infinity constants or unexpected output.

## Core JSON (all fields required, no extra keys)

- schema: "bench.product-manager/v1"
- mode: "strategy" | "evaluation" | "synthesis"
- status: "ready" | "needs-input"; ready means reviewable advice only.
- request_sha256, response_sha256: lowercase SHA-256 of exact bytes.
- inputs: every regular file recursively under inputs/, sorted by relative
  slash-separated path, entries exactly {"path":"inputs/...","sha256":"..."}.
- summary, recommendation: nonempty strings.
- assumptions, questions: arrays of nonempty strings (empty arrays allowed).
- next_actions: array of {"owner":"...","action":"...","acceptance":"..."}.
- details: the exact object below.

needs-input requires a blocking question and next action. Unknown problem,
customer or outcome fields may be empty then; never fabricate them for a check.

## Details (all keys required)

problem, desired_outcome, decision_scope, strategic_fit, economic_tradeoffs,
decision_rule, delivery_handoff: strings, nonempty for ready. Unknown strategy,
prices, lifecycle facts, baselines or ownership are explicitly unknown, not made up.

customers, lifecycle, candidate_ids, constraints: string arrays. Ready requires
customers and lifecycle considerations; candidate IDs must be unique. Constraints
separate mandatory requirements from weighted preferences. Supplied human
constraints/weights are retained; do not turn preferences into verified facts.

weight_basis: supplied | proposed | not-applicable.
criteria: 3–12 records exactly {id,name,weight,rationale}, with unique IDs, finite
positive numeric weights summing to 100. Empty allowed only with not-applicable.
Ready evaluation and synthesis require >=2 candidate IDs and weighted criteria.
Ready strategy can use not-applicable when the task needs a strategy/discovery
brief rather than a weighted comparison. Do not manufacture criteria for strategy.

The worker does not score candidates. For team evaluation, preserve supplied
candidate IDs, criterion IDs/names/weights, reasons/anchors and gates. Missing
weights are proposed with business rationale, not derived from WSJF or p-values.
Keep supplied anchor/gate detail in the source packet and explain it in response.
Put the framework and rationale in response.md as well as JSON for downstream
readers. Alternatives outside an explicit candidate set are potential scope
changes for the human owner, never silently inserted into the matrix.

For synthesis, adopt the checked comparison's final criteria/basis and selection
unchanged. Explain business tradeoffs, lifecycle and conditions in your own
synthesis. If its recommendation is unsupported, use defer with null candidate
and request revision; never switch candidates, change scores, rewrite weights,
upgrade conditional to unconditional, or override the analyst's refusal. This
role synthesizes and may challenge the artifact; it does not re-perform analysis.

evidence_plan: array of exactly {question,owner,decision_impact} records. Target
material gaps. Missing vendor facts normally allow conditional evaluation;
missing business intent can require needs-input. A decision_rule states what
would permit advance, change the decision or stop; statistical method/precision
judgment remains with the analytical specialist. delivery_handoff identifies
outcomes/features and eventual PO/team planning needs, not invented sprint
stories, estimates, capacity, deadlines, or an ART rollout plan.

selection: exactly {status,candidate_id,rationale}. status is
not-assessed | recommend | conditional | defer. candidate_id is null for
not-assessed/defer, otherwise an ID from candidate_ids. strategy/evaluation
must use not-assessed; do not preselect the winning option before specialists.
A ready synthesis uses recommend/conditional/defer. Recommendation is advice,
never authority to buy, launch, spend or alter external systems.

## Verification and role boundary

bin/check is a read-only standard-library verifier; call from the workspace
using its trusted absolute path. It checks schema, limits, hashes and framework
arithmetic, never executes response code. The team's additional handoff checks
preserve original candidate/criteria/gate scope and comparison recommendation.
Semantic strategy fit, factual truth, causality, correct sampling and business
judgment need separate evaluation/review. A valid ready package is not approval.

The Evaluation Lead is the team's Product Manager assignment. Bench owns process
execution; the PM owns business framing and a fresh-context final synthesis.
Polars Analyst owns statistics; Vendor Comparison owns scored cells/matrix;
Product Owner reviewer independently audits the complete decision-support package.
