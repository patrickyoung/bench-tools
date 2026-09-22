# Experiment researcher

Turn selected Bench event logs into one cited hypothesis and an Improve plan.
Agent runs the researcher; Trail searches and reads the logs; Ask verifies
replay; Improve validates the assembled experiment. A separate Improve invocation
can then use Hire to author a candidate, Agent to test it and a frozen judge to
decide whether to export it. This worker stops at the plan.

## Run

Select installed `agent`, `ask`, `brief`, `ply`, `cage`, `record`, `trail` and
`improve` (0.1.0 or compatible); Python 3.9+ is required by the checker. Choose
an Ask model and its credentials through the normal environment. Keep the
trusted request, definitions, archives and controller evidence outside the
writable research workspace. Resolve executable symlinks before putting paths
in the request. For an uncommitted definition, use a reviewed copy and retain
its file hashes; do not call it a committed export.

```sh
mkdir /work/research-01
EXPERIMENT_RESEARCH_REQUEST=/inputs/research-request.json \
  agent run -C /work/research-01 -evidence /records/research-01 \
  -m "$ASK_MODEL" -turns 8 -cycles 2 -timeout 90s \
  -record-output research.json /definitions/experiment-researcher -- \
  'Research the selected evidence and produce the contracted outputs.'
```

Only add `-record-output experiment.json` when a ready result is expected;
non-ready results deliberately have no such file. The request is read from
`EXPERIMENT_RESEARCH_REQUEST`, never a workspace copy. It is trusted caller
configuration because the checker executes its selected read-only tools.
Archive/report text and worker outputs cannot select checker commands. Cage
limits action writes/network, not host reads; the checker runs outside Cage.
The two cycles allow an ordinary checker-feedback repair through Agent/Ply;
they are bounded research work, not retries of an Improve experiment.

## Request

One UTF-8 JSON object, no duplicate keys, at most 2 MiB:

```json
{
  "version": 1,
  "question": "Find a small reusable improvement suggested by these failed runs.",
  "sources": [
    {"id":"runs", "kind":"archive", "path":"/inputs/selected-runs"},
    {"id":"scores", "kind":"text", "path":"/inputs/independent-results.json"}
  ],
  "tools": {"trail":"/runtime/trail", "ask":"/runtime/ask", "improve":"/runtime/improve"},
  "template": null,
  "fresh_holdout": false
}
```

There may be 0–12 sources with distinct nonempty IDs. All paths are absolute,
canonical and outside the workspace, without symlink components. Archives
contain directly selected regular `.jsonl` files: Trail does not recurse.
Text reports are at most 2 MiB. Cited sessions are at most 64 MiB. Stage an
explicit subset outside work for a broad archive; no default home scan occurs.

Replace `template:null` with a complete caller-reviewed Improve v1 spec to
enable a runnable plan. Its `settings` must be an object without `research`.
Use absolute external paths for `source`, every case file and dependency;
use absolute script paths in command argv. Explicitly select `repeats`,
`command_seconds`, `max_seconds`, models and the independent gate. `improve -n`
checks the public specification without executing its commands. The Improve
router example's `spec.py` can prepare a starting spec; replace sample cases
with suitable independently labeled development and new reserved cases.

The caller sets `fresh_holdout:true` only for newly reserved cases. This is an
assertion of freshness, not a secrecy guarantee or mechanical novelty test.
Caller-selected proposer adapters must consume `settings.research` as research
context. The Improve router's Hire adapter includes all settings already.
No credentials belong in requests, templates or retained reports.

## Outputs

Always write `research.json` with exactly these fields:

```json
{
  "version":1,
  "request_sha256":"SHA256 of the exact trusted request bytes",
  "status":"needs_input",
  "summary":"A selected run exhausted its output budget; a targeted hypothesis needs new reserved cases.",
  "hypothesis":null,
  "change":null,
  "citations":[],
  "limitations":["Replay verifies recorded bytes, not the diagnosis."],
  "next_inputs":["Supply a reviewed Improve template and new reserved cases."]
}
```

Statuses are `ready`, `needs_input` and `no_experiment`. For ready, `hypothesis`
is a nonempty string, `change` is `{ "path": "one template mutable path",
"instruction": "one proposed reusable change" }`, and at least one archive
citation is required. For other statuses hypothesis/change are null; retain
any promising idea in summary. Only needs_input has nonempty next_inputs.
Every result has at least one candid limitation. At most 12 citations:

- Archive: `{"source":"runs","file":"session.jsonl","sha256":"file digest","seq":5,"quote":"answer was cut off at the output limit"}`.
- Text: `{"source":"scores","sha256":"file digest","quote":"candidate rejected"}`.

Archive quotes must be verbatim substrings of a string value in the selected
event, not a serialized JSON fragment. Use Trail's `.event` object and its seq.
Text quotes are substrings of the decoded file. Hash exact original file bytes.
Keep quotes short and sufficient to support the claim. No citation may escape
the selected source or refer to an invented event. Replay is checked again by
the verifier; source changes invalidate a cited digest.

The model writes only `research.json`. Then run the worker-local helper:

```sh
"$AGENT_HOME/bin/check" --assemble
```

It checks the report and citations, constructs the plan from the trusted template,
validates it through `improve -n`, and creates `experiment.json` only for `ready`.
It does not overwrite an existing different plan. Repeating assembly with an
identical valid plan is harmless. The default `bin/check` remains read-only.
Non-ready reports need no plan; assembly validates them without writing one.

The assembled `experiment.json` equals the entire template except:

1. `mutable` becomes `[research.change.path]`.
2. `settings.research` is inserted as `{hypothesis, change, summary, citations}`
   copied from research.json.

All other fields remain identical. Non-ready results must have no experiment
file. Both modes call only the trusted Trail/Ask readers and `improve -n`;
it never runs the selected proposer, trial or judge. After independent review,
the caller can execute the plan through the existing public command:

```sh
improve -n < /work/research-01/experiment.json
improve -o /studies/new-study < /work/research-01/experiment.json
```

The second command can make paid calls. Research costs are separate from that
experiment's costs. An exported candidate is evidence for review, not an
automatic source update. Independent trials are not Hone checked recoveries.

## What acceptance means

This worker remains experimental. Live log-research runs have exhausted their
turn budgets, and citation-valid reports have contained inaccurate summaries.
Require successful Agent completion and independent review of the claims
before using a plan. The deterministic assembler reduces copying work; it does
not establish that the model's research is reliable or cheaper.

`bin/check` and `bin/check --assemble` return 0 for an accepted research package, 1 for an invalid or
missing package. It checks the current request binding, shape, literal quote
identity, cited-session replay, fixed template and Improve plan validity.
It rejects stale hashes, invented events, changed gates/models/budgets,
unselected paths, symlinked outputs, and ready results without fresh-case
attestation. Subprocesses use literal argv, a 30-second timeout and no models.

It does not establish that a quote supports the hypothesis, that a hypothesis
is useful, that reserved cases are novel, or that an improvement works. Review
those claims independently and require a fresh controlled comparison before
promotion. A sound needs_input/no_experiment result is completed research, not
an improvement. An unreadable or malformed trusted request is an input error.

For offline regression tests, see the owning worker's `tests/` in the source
library. They use synthetic process fixtures; separate real Agent evaluation
is needed to assess research quality.
