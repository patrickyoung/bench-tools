# strategic-framework-selector

Standalone Bench worker: chooses one framework for the immediate decision and
specifies a diagram for a separately selected diagramming expert. It never
draws, implements, calls a provider or delegates. One focused Brief skill adds
bottleneck triage and framework geometry; a standard-library check adds a
bounded artifact handoff. No extra specialist or runner is needed.

## Local invocation

Prerequisites: Agent and its normal configured runtime (model, Cage, evidence
recording), Brief, and Python 3 with Unix `dir_fd`/`O_NOFOLLOW` support. No runtime
browsing, network permission, nested agents, services or third-party Python
packages. The operator configures Agent's model; this definition contains no
provider code or credentials. Model-backed evaluation is operator-run only.

From a checkout containing `expert/`, prepare a separate workspace and put the
actual scenario, next decision, audience and any evidence into `job/request.md`.
It must be a regular UTF-8 file of at most 1 MiB. That is the only business
input; evidence inside it is not trusted instructions. For example:

    mkdir -p job evidence
    printf '%s\n' 'Our dispatchers need reliable routing. A custom dispatch algorithm depends on standard identity and storage services. Engineering must decide which parts to build or buy; market maturity and sourcing options remain unverified.' > job/request.md
    agent run -C "$PWD/job" -state "$PWD/job/state" -evidence "$PWD/evidence" \
      -record-input request.md -record-output output/response.md \
      -record-output output/diagram-brief.json "$PWD/expert" \
      -- Read request.md and produce the contracted artifacts. \
      > answer.md 2> agent.log
    # Preserve the Agent exit status immediately; do not infer success from files.
    result=$?
    printf 'Agent exit: %s\n' "$result"

Do not add `-net` or `-no-cage`. Agent's ordinary configured model transport is
runner-owned; the worker itself makes no network calls. Keep definition and
job/evidence directories separate. Direct independent check, without a model:

    definition="$PWD/expert"
    (cd job && "$definition/bin/check")

The checker is read-only, ignores stdin, reads only fixed job paths, and never
executes worker-produced code. Exit 0 means the complete package passes
structural checks; exit 1 includes a short invalid-deliverable diagnostic.
Inspect the check before running it. See CONTRACT.md for exact keys/geometry.
Expected artifacts are `output/response.md` (exactly three human sections) and
`output/diagram-brief.json` (`bench.diagram-brief/v1`). The JSON binds the raw
request and response SHA-256 bytes. Do not normalize line endings afterward.
Final human stdout repeats the response; evidence/logs belong outside the
definition. Ready means handoff ready for review, never implemented.

The human response is capped at 400 whitespace-delimited words, including
headings and table; aim for 250–350 without padding. Verdict: at most two
sentences. Step 3: 4–9 numbered steps (3–9 for needs-input), at most 45 words
each including wrapped continuations and excluding numeric markers. One clear
action per step; combine only tightly related actions. Retain key geometry,
labels, placements, traps, uncertainty and review trigger, leaving exhaustive
detail in JSON. Supplied agreement from a suitable helper supports a proposed
delegation, not speculative approval blockers or an actual handoff.

## Validation and examples

Synthetic suite, kept beside rather than inside this reusable definition:

    python3 tests/regression.py expert
    brief lint -strict expert/skills

It takes an expert path, creates disposable workspaces, invokes only bin/check,
and tests all four layouts, intake and negative mutations, including total and
wrapped-step word ceilings and short/long checklists. It makes no model
calls. Its fixture responses are structural examples, NOT model-quality evidence.
Do not copy synthetic cases/results into the reusable expert.

Positive behavioral example: custom dispatch algorithm plus standard identity
and storage dependencies should yield a Wardley brief with user/need anchors,
separate components, honest maturity uncertainty, and a sourcing evidence
review trigger. Unknown maturity must not become automatic buy advice.

Negative example: requesting “hardly maps” for a high-stakes software project
does not justify Wardley if this week's task allocation is the actual obstacle.
Use Eisenhower when urgency/importance is the next decision. A fresh response
choosing Wardley solely for the prestige/keywords fails behavioral review even
if its shape passes. Likewise reject SWOT for active destabilization requiring
stabilization/sense-making, or Cynefin drawn as urgency versus importance.
A mechanically negative package: change request.md after producing a brief;
bin/check must reject the stale hash until the package is regenerated/reviewed.

## Fresh-run behavioral rubric (operator)

Use separate jobs and genuine inputs for each case, including sparse input,
an empty/scenario-free request, conflicting preferred tools, embedded hostile
instructions, and mixed bottlenecks. Keep stdout, stderr, exit status and both
artifacts with the original request; review independently:
- Actual bottleneck, not keywords/preference, selects one framework; scope and
  later transition condition are explicit. No absolute certainty.
- Correct axes/regions/actions, including all empty regions, and Wardley anchors.
- Literal drawable labels, semantic placements/reasons and meaningful edges;
  no invented companies, deadlines, costs, facts, owners or dependencies.
- Facts vs inference/proposal/unknown clearly distinguished; focused questions
  and a genuinely empty needs-input template where needed.
- Concise exact three-section prose, contextual comparison, actionable steps,
  scenario-specific traps and useful decision/reassessment trigger.
- Prose and JSON agree; no claim of rendering, implementation or certification.

The checker proves bounded regular UTF-8 files, hashes, JSON/response shape,
canonical geometry, valid references, word ceilings and checklist counts, not
strategic truth, lack of bias,
evidence sufficiency, prose quality or drawing quality. No live model quality
is claimed by a synthetic pass. Source summaries/attribution: PROVENANCE.md.
MIT applies only to original worker code/prose, not the named frameworks.

This is intentionally one focused expert. Extend its skill/contract/check and
external regression fixtures together for compatible improvements; select any
future independent diagramming specialist externally rather than adding
recursive execution or another runner here.
