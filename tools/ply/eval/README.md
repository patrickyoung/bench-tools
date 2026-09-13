# Paired agent-loop evaluations

This directory runs external driver executables against identical fixtures and
outside-worker checks. It adds no provider integration, evaluator, or task
runtime to Ply itself. All model calls are made by an explicitly selected
driver. No paid calls run during the self-test.

The [2026-09-09 verbosity comparison](reports/2026-09-09-verbosity.md) records
the completed Ply/Pi task comparison, source findings, and its limitations.

Start with the deterministic harness checks. From the Bench monorepo root:

```sh
cd tools/ply
python3 -m unittest discover -s eval -p 'test_*.py' -v
```

In a standalone Ply checkout, omit `cd tools/ply`. Python 3 is required; this
first command needs neither provider credentials nor a hosted model. The live
task trials below answer a separate question: how well the chosen models and
drivers handle the supplied cases under the recorded limits.

The checks exercise repeated pairing, equal initial file digests, isolated
state, correct and false success claims, unknown cost, invalid driver output,
timeouts, unsupported suites, oracle tampering, and refusal to reuse a run
directory. `drivers/fake.py` is a test fixture, not an agent or a baseline for
model quality.

## Two separate suites

| Suite | Case | External acceptance check |
| --- | --- | --- |
| task | `multi-file-repair` | Hidden invoice tests span two modules, quantity, discounts, and invalid arguments. |
| task | `ambiguous-request` | Original records and missing values survive; the driver asks for clarification without selecting a metric or claiming the redesign is complete. |
| task | `large-output` | Exact count and first/last error IDs from a log larger than an ordinary command-output cap. |
| lifecycle | `transient-provider-failure` | The controller observes a 429 before a later response and the requested artifact. |
| lifecycle | `repeated-compaction` | At least two summarization requests retain a continuity token, followed by all three stages exactly once. |
| lifecycle | `cancellation-resume` | A controlled interruption stops the action process; the next invocation completes in the same work and session directories. |
| lifecycle | `uncertain-external-effect` | An effect commits while its response is withheld. After cancellation and resume, status is inspected and exactly one effect exists. |

The **task** suite measures a live driver's handling of these small tasks.
The **lifecycle** suite uses a local scripted Messages API and a local effect
service. Those responses test execution and session mechanics; they provide
no evidence of a model's judgment or task quality. A driver must explicitly
declare lifecycle support and route its calls to the supplied fixture URL.
Unsupported cases are recorded and excluded from paired outcome counts.
The included Ply adapter supports both suites. The included Codex, Claude
Code, and Pi adapters support fresh task trials only; they reject lifecycle
fixtures and resume requests.

## Driver interface

A driver is an ordinary executable. The harness gives it one JSON request on
stdin, with `schema: "ply.eval/request/v1"`, the case and suite, prompt, work
and state paths, configured model, budget, experiment options, a `resume`
boolean, and a lifecycle `fixture_url` (otherwise null). It must run inside
the supplied workspace and keep trial evidence under the supplied state path.
Native login state is the explicit exception: it lives in a separate private
auth root, described below, and never belongs in the result artifacts.
It prints one JSON result on stdout; diagnostics go to stderr:

```json
{
  "answer": "The driver’s final answer",
  "claimed_success": true,
  "exit_code": 0,
  "usage": {
    "input_tokens": 1200,
    "output_tokens": 300,
    "cached_input_tokens": null,
    "cost_usd": null,
    "source": "the driver’s actual usage receipt"
  },
  "interventions": [
    {"kind": "user_steering", "detail": "clarified the requested metric"}
  ]
}
```

Use `usage: null` when measurements are unavailable. Missing cost or usage is
never converted to zero. A claim can also be null when the adapter cannot
determine it. Driver interventions are labeled as driver observations;
harness cancellations/resumes and fixture faults have separate origins.
The Ply adapter verifies every discovered Ask session with public
`ask replay -check -json`, including source, successor, and summary sessions.
It sums each `(session ID, assistant sequence)` once, including partial
responses. On resume the final observation is cumulative for the trial;
the harness uses that final total rather than summing the attempts again.
Verified replay snapshots are retained under the trial's state directory.
Reported cache reads, cache writes, and reasoning counts remain separate.
Omitted optional cache/reasoning fields remain unknown rather than zero.
If a turn has no reported token counts, or a request has no assistant usage
event, token totals are null and `unreported_turns` records that gap.
`unreported_requests` separately counts requests without a response event.
Ask retry events identify additional failed attempts without usage records;
`unreported_retries` counts those gaps too. `observed_*` token/cost fields
retain the measurements that do exist as lower bounds, while complete
totals stay null whenever any attempt's usage is unavailable.
Costs are summed only when every turn
reports a positive billed cost; absent or zero cost means unknown, not free.
Failed requests without a usage event cannot be assigned invented usage.
Lifecycle fixture counts are synthetic provider observations and say
nothing about real model cost or efficiency.

The task prompt requests a final `EVAL_SUCCESS` line only for a claim that
the full work is complete. All included adapters use this exact line. Exit
zero alone is never treated as a success claim or an external verdict. Custom
drivers can report their own explicit native completion claim instead.

Configure drivers in a JSON array, using absolute script paths:

```json
[
  {
    "name": "ply-text",
    "argv": ["python3", "/absolute/ply/eval/drivers/ply.py"],
    "model": "provider/exact-model-id",
    "budget": {"wall_seconds": 180, "turns": 20, "cycles": 3, "action_seconds": 60},
    "options": {"tool_protocol": "text", "compaction": "ask-local", "compact_at": 24000},
    "suites": ["task", "lifecycle"],
    "pass_env": ["PLY_EVAL_PLY", "PLY_EVAL_ASK", "ANTHROPIC_API_KEY"],
    "files": ["/absolute/bin/ply", "/absolute/bin/ask"]
  },
  {
    "name": "codex-native",
    "argv": ["python3", "/absolute/ply/eval/drivers/codex.py"],
    "model": "the-exact-matched-model-id",
    "budget": {"wall_seconds": 180},
    "options": {"tool_protocol": "native", "compaction": "native", "effort": "medium"},
    "suites": ["task"],
    "pass_env": ["PLY_EVAL_CODEX", "PLY_EVAL_AUTH_ROOT"],
    "files": ["/absolute/bin/codex"]
  }
]
```

The harness enforces the driver's wall-clock budget across interruption and resume,
with up to three additional seconds for process-group teardown after a stop.
It checks the deadline before each launch, including resume, and stops any
remaining owned process group before the oracle runs. A driver that returns
while descendants are still running is cleaned up and recorded as invalid.
The external oracle has a separate 15-second limit and the same bounded
group teardown. Trial elapsed time includes that independent verification;
each attempt's execution time is also retained separately.
Other budget fields and experiment options are passed literally to the
driver and recorded. The adapter must enforce or report them accurately;
an option label by itself does not prove a native protocol was used.
The Ply adapter forwards its turn/cycle/action limits, optional `compact_at`,
and an optional `extra_args` array as literal CLI arguments. It uses
`-checkpoint` to retain Ask's current session through resume and compaction.

## Included native adapters

The adapters use CLI behavior checked against these installed versions.
Recheck invocation and event schemas when upgrading; record the actual binary
in `files` so the manifest hashes it.

| Adapter | Verified CLI version | Executable environment name | Private login file |
| --- | --- | --- | --- |
| `drivers/codex.py` | Codex 0.153.4 | `PLY_EVAL_CODEX` | `codex/auth.json` |
| `drivers/claude.py` | Claude Code 2.1.261 | `PLY_EVAL_CLAUDE` | `claude/.credentials.json` |
| `drivers/pi.py` | Pi 0.81.1 | `PLY_EVAL_PI` | `pi/auth.json` |

To configure Claude or Pi, use the corresponding script, executable variable,
and binary path in the native entry above. Include `PLY_EVAL_AUTH_ROOT` in
each native driver's `pass_env`. Select an exact model ID supported by that
product and account, including its provider prefix where required, and set
`options.effort` explicitly. The adapters pass these values literally. Native
events and metadata retain model evidence when the CLI emits it; a configured
model label alone is not proof of the server-resolved model.

Native invocations exclude personal instructions, extensions, plugins, and
connected tools through their supported CLI controls. Codex uses ephemeral
`exec --json`, a private `HOME`/`CODEX_HOME`, workspace-write command sandboxing,
and disabled web search and command networking. Claude restricts its tools to
local Bash/file operations and disables persistent sessions, user settings,
slash commands, and MCP. Pi disables extensions, skills, prompt templates,
and context files, while retaining native session events in trial state.
These are different native execution policies, not equivalent sandboxes.

Use a common `budget.wall_seconds: 180` for the initial comparison. Ply also
enforces its turn, cycle, and action timeout limits. Claude forwards
`budget.turns` as its native maximum turns (default 20). Codex and Pi have no
adapter-enforced turn or action timeout cap beyond the parent wall deadline.
Record these differences instead of describing every budget as identical.
Native versus text tools and native versus local compaction remain explicit
driver configurations; the harness selects no default protocol.

Provision `PLY_EVAL_AUTH_ROOT` as an operator-owned directory with mode 0700,
outside both trial workspaces and result artifacts. Begin with only the
required native login files above, with mode 0600, and explicitly required
nonsecret model catalogs or caches (`codex/models_cache.json` or
`pi/models.json`). Do not copy a personal configuration or session directory.
Never put credential values in configuration JSON, argv, metadata, or result
artifacts; the manifest records environment names only.

Run trials sharing this auth root serially because OAuth refresh may rotate
credentials. Claude and Pi refresh their private native login files directly.
Codex creates an invocation-specific private home, atomically publishes any
refreshed auth back to the private base after its child exits, and removes the
temporary copy. These adapters do not rewrite the user's original login.
The operator owns final reconciliation of refreshed credentials and cleanup
of the private base, including any invocation directory left by a hard kill.
Only native event/session evidence, answers, and nonsecret metadata belong in
the trial state directories.

Codex usage describes native user-turn totals rather than individual model
calls. Claude's `total_cost_usd` and Pi's catalog cost are client estimates:
their metadata sidecars label them `estimated_cost_usd`, while harness
`cost_usd` remains null. Never present these estimates as billed costs or
unknown cost as zero. Provider token counters also differ in whether cached
input is included; compare their stated counter definitions before totaling
or ranking token consumption.

Only the environment names listed in `pass_env` are inherited, together
with PATH and basic locale settings. HOME, temporary files, and XDG state
directories are isolated per trial. Export credentials yourself before a
live run; do not put secret values in argv or configuration files. The
manifest records environment names, not their values.

```sh
export PLY_EVAL_PLY=/absolute/bin/ply
export PLY_EVAL_ASK=/absolute/bin/ask
export PLY_EVAL_CODEX=/absolute/bin/codex
export PLY_EVAL_CLAUDE=/absolute/bin/claude
export PLY_EVAL_PI=/absolute/bin/pi
export PLY_EVAL_AUTH_ROOT=/private/operator-owned-live-auth
# Export the provider credentials required by the chosen live drivers.
python3 eval/run.py --drivers /absolute/drivers.json \
  --out /absolute/results/task-001 --suite task --repetitions 3
```

For the offline lifecycle suite, use an explicit configuration containing
only the Ply adapter, a model label such as `scripted-fixture`, lifecycle
support, a generous wall limit (for example 45 seconds), and the two binary
environment names. No provider credentials are required. The adapter uses
the local fixture endpoint and a dummy key. For the repeated-compaction
case, omit `compact_at` so the adapter uses threshold 1 and forces multiple
handoffs; a live-sized threshold may never trigger on this small fixture.

```sh
python3 eval/run.py --drivers /absolute/lifecycle-drivers.json \
  --out /absolute/results/lifecycle-001 --suite lifecycle --repetitions 3
```

## Evidence and interpretation

Each run snapshots the corpus and oracle outside all worker directories.
Every trial receives a fresh writable copy and isolated state. Driver
executables, explicit dependency files, script argv files, initial fixture
trees, oracle code, and harness code are hashed. Oracle files are read-only,
and their hashes and driver dependencies are checked before and after the
oracle runs. A changed controller or driver produces an invalid result.
This detects edits; it is not a sandbox against a malicious same-user
process. Use an independently confined driver when that is the threat model.

`manifest.json` records provenance. `results.jsonl` is flushed after every
trial. Each trial retains requests, raw stdout/stderr, attempts, observed
events, and its external oracle verdict. `summary.json` reports per-case
outcomes and paired counts: both pass, left only, right only, neither, and
unpaired. Per-case summaries include usage/cost observations and sums (null
when any included value is unknown), latency observations, and observed
intervention counts. Lifecycle events are grouped by origin and kind so a
fixture fault cannot be mistaken for a human intervention.
Repetition order rotates between drivers; fixture digests must
match within every pair. An invalid or unsupported result is unpaired,
not silently treated as a loss. A valid false success is an explicit
completion claim with a failing oracle.

For three repetitions of the three task cases, report outcome counts against
all nine scheduled trials per driver, with failed, invalid, timed-out, and
unsupported trials shown separately. The paired summary excludes invalid
and unsupported results; its measured-only denominator must not hide those
scheduled trials or inflate a success rate.

Existing output directories are refused. There is no automatic campaign
resume that might repeat an unknown external effect. The controlled
cancel/resume cases are explicit within-trial experiments.

Small fixtures, scripted lifecycle responses, unequal models, different
budgets, unmeasured cost, and missing native adapters limit conclusions.
Report every failed, unsupported, and invalid trial alongside successes.
Do not aggregate the two suites into a single score, infer statistical
significance from a few repetitions, or claim competitive parity from this
corpus alone. Expand held-out tasks and use matched models and budgets
before making a broader comparison.
