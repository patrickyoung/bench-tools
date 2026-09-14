# Portable Product Owner worker

A focused filesystem expert for product outcome and backlog decisions, with
evidence-led intake and practical delivery awareness. It is current to the selected
references dated 2026-09-13, while accurately retaining older publication dates.
It does not claim employment, certification, framework compliance, or automatic
access to websites, analytics, trackers, model accounts, or other tools.

## Prerequisites

- Bench `agent` and its configured model/runtime companions
- Python 3.9+ standard library on Linux or macOS for `bin/check`
- A fresh writable workspace
- This `expert/` directory installed read-only or otherwise kept separate from
  the workspace
- Operator-selected model credentials and permissions; none are bundled

No scheduler, provider client, web application, server, or agent loop is
included.

## Input and invocation

Create a fresh workspace. Write the complete natural-language request to
`request.md`; optionally place up to 64 current regular evidence files under
`inputs/`, each no larger than 1 MiB. Do not use symlinks. A previous run is
not resumed or implicitly read.

    mkdir -p /absolute/work /absolute/control
    cat > /absolute/work/request.md
    mkdir -p /absolute/work/inputs
    agent run -C /absolute/work -evidence /absolute/control /absolute/expert -- \
      'Read request.md and selected inputs, then produce the appropriate response.'

The `cat` command reads the request from the operator's terminal until EOF.
Alternatively, write `request.md` with an ordinary file-producing command before
invoking Agent. The model writes exactly:

- `output/response.md` — human recommendation or answer
- `output/decision.json` — the bound `bench.product-owner/v2` handoff

See `CONTRACT.md` for the exact schema. Agent stdout is the concise completion
report; diagnostics go to stderr. `status: ready` means ready for human review,
not approved, released, or proven correct.

## Standalone, team, and A2A use

Standalone callers consume the two files only after Agent exits 0. A larger team
may invoke this same definition in a fresh workspace through its already
admitted Agent binding and pass the two artifacts explicitly. This worker is a
Product Owner specialist, not an automatic page-team manager or a substitute
for its controller.

A2Aserve can expose the unchanged Agent command when the operator's admitted
task binding stages the incoming natural-language request as `request.md` in
the task's fresh `A2A_WORKSPACE` and stages any named evidence beneath `inputs/`.
Configure two explicit exports:

    -artifact output/response.md -artifact output/decision.json

Then configure the fixed public program as the same invocation:

    agent run -C . -state ../state -evidence ../control /absolute/expert -- \
      'Read request.md and selected inputs, then produce the appropriate response.'

The exact A2Aserve listener, authentication/TLS, private state, credentials, and
request-to-file staging remain operator-owned. Do not point raw message stdin at
this command and assume it became `request.md`. Use A2Aserve's current manual
for listener flags and lifecycle; this definition intentionally embeds no
server or dispatcher.

## What the check proves

From the current workspace, Agent runs the executable definition check. It:

- accepts only bounded regular request, input, response, and JSON files;
- rejects source/output symlinks and unexpected output entries;
- verifies exact SHA-256 bindings for the request, every recursively supplied
  input, and `response.md`;
- checks the closed core schema, mode/status values, and compact mode-specific
  structures;
- requires a blocking question and next action for `needs-input`.

It reads no host source, network, credentials, prior run, or response code and
performs no business action. It cannot establish factual truth, evidence
quality, prioritization wisdom, interview quality, customer value, ROI,
capacity, forecast validity, legal compliance, approval, or release readiness.
Those require human review and, where appropriate, independently supplied
evidence.

## Check recipes

A positive package can be generated deterministically with Python:

    WORK=$(mktemp -d)
    EXPERT=/absolute/expert
    mkdir -p "$WORK/output"
    printf '%s\n' 'Review this evidence gap.' > "$WORK/request.md"
    printf '%s\n' '# Recommendation' 'Hold pending evidence.' > "$WORK/output/response.md"
    WORK="$WORK" python3 - <<'PY'
    import hashlib, json, os
    from pathlib import Path
    w = Path(os.environ["WORK"])
    h = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
    d = {
     "schema":"bench.product-owner/v2","mode":"review","status":"ready",
     "request_sha256":h(w/"request.md"),"inputs":[],
     "response_sha256":h(w/"output/response.md"),
     "summary":"Evidence gap identified.","recommendation":"Hold pending evidence.",
     "assumptions":[],"questions":[],
     "next_actions":[{"owner":"Product Owner","action":"Obtain evidence",
                      "acceptance":"Named source is available for review"}],
     "details":{"decision":"hold","findings":[
       {"finding":"Outcome evidence is absent","evidence":"No inputs were supplied",
        "action":"Obtain a current outcome baseline"}]}}
    (w/"output/decision.json").write_text(json.dumps(d), encoding="utf-8")
    PY
    (cd "$WORK" && "$EXPERT/bin/check")

Expected result: exit 0 and `valid bound response package`.

A negative stale-response case:

    printf '%s\n' 'unbound change' >> "$WORK/output/response.md"
    (cd "$WORK" && "$EXPERT/bin/check")

Expected result: exit 1 with a stale/invalid response hash. `tests/` beside the
definition contains broader reproducible unittest cases, but is deliberately
not part of the exported worker.
