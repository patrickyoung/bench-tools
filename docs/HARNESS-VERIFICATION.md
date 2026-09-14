# Harness setup verification

[Start here](../START-HERE.md) · [Develop and verify](DEVELOPING.md)

The canonical [Bench skill](../.agents/skills/bench/SKILL.md) supplies one shared
procedure, with a separate setup reference for each host. The Claude plugin and
Pi package point at those same files. They add discovery metadata; Bench's
existing installer still installs programs, Hire builds, and Agent runs.

## Observed native checks

On **2026-09-13**, the following checks passed on macOS in temporary homes,
without hosted model calls or changes to personal harness settings:

| Surface | Version | Observed evidence |
| --- | --- | --- |
| Portable skill | Repository files | Brief strict lint passes both in place and after the plugin ZIP is extracted elsewhere; all packaged references are present |
| Claude Code | 2.1.261 | Native strict validation accepts the extracted plugin; a fresh session discovers `bench-tools:bench`; native MCP registration reports the server connected |
| Codex CLI | 0.153.4 | A fresh app-server discovers the installed `bench` skill and the registered server's MCP tool |
| Pi | 0.81.1 | Native local package installation succeeds; a fresh offline RPC session lists `skill:bench` after the personal skill copy is removed |
| MCPserve | 0.3.1 | Actual stdio discovery and tool calls pass with the modern client and explicit legacy compatibility; legacy initialization is refused by default |

MCP's independent Go suite also exercises modern and legacy HTTP clients,
including invalid tool inputs, and passed ordinary tests, race tests, and vet.
Compatibility uses the existing official Go SDK. No second protocol
implementation or agent runtime was added.

The full `make check` gate passed: 75 source-coordination tests, independent
checks for all 19 components, and public-process integration including this
harness check. The installation smoke passed for all 23 commands, including
relocation, repeat installation, and removal. All four starter examples passed
with `--native-cage`, including one support expert used in separate workspaces.
These fixture results establish the tested interfaces, not model quality.

Repeat the portable checks with:

```sh
make check-harnesses
```

For the optional native host checks, first install the three host CLIs, then:

```sh
python3 scripts/check-harnesses.py --host-clis
```

The default portable check also runs in the existing process integration gate.
It requires only Brief and the MCP component. Native host checks remain optional
because host CLIs and their discovery APIs have independent release cycles.

Claude's discovery probe sends only the initialization control request used by
the [official Agent SDK](https://github.com/anthropics/claude-agent-sdk-python/blob/main/src/claude_agent_sdk/_internal/query.py).
Codex's probe uses its app-server discovery interface; Pi's uses `get_commands`
in offline RPC mode. No probe sends a model prompt.

## What this evidence does not establish

Cowork's account plugin upload and execution environment have **not** been
exercised here. Its [setup reference](../.agents/skills/bench/references/cowork.md)
uses the documented account plugin route and requires checking actual command
access, persistence, and remote connector reachability. The ZIP is tested as a
Claude plugin; that is not a Cowork installation result.

OpenClaw has **not** been exercised here. Its
[setup reference](../.agents/skills/bench/references/openclaw.md) uses the
documented local skill route and requires discovery and command checks in the
selected agent's execution environment. Primary documentation links are kept
beside the host-specific instructions.

Skill discovery does not prove that a model will follow the procedure. These
checks also do not certify model access or worker quality. The existing
[runnable starters](../examples/README.md) exercise outputs, rejection paths,
separate workspaces, and evidence through actual Bench commands with local
model fixtures. A requested model-backed solution must additionally follow the
skill's [evaluation procedure](../.agents/skills/bench/references/evaluate.md).

## Teaching checks on 2026-09-14

The shared skill now routes teaching, checked-recovery learning and private
worker context through [one procedure](../.agents/skills/bench/references/learn.md).
The Claude plugin and Pi package are version **0.2.0**. Relocated package bytes,
strict lint and fresh native discovery passed again for Claude Code 2.1.261,
Codex CLI 0.153.4 and Pi 0.81.1 in disposable homes.

Live **Codex CLI** cases used `gpt-5.6-sol` with medium effort and the existing
Ask connection for bounded Hire/Agent runs. The observed results were:

- Codex selected Hire to teach a private copy of the Enterprise Architect a
  fictional platform standard. The original pin and checker stayed unchanged;
  all 17 base contract tests passed. Fresh model cases loaded the new skill,
  withheld adoption without the required proof, and exempted an existing
  steady-state platform from the new-product gates. Outputs were reviewed
  beyond the structural check.
- Codex invoked Hone on a replay-verified synthetic recovery. Hone's wording
  model returned no useful lesson; Codex preserved that result and identified
  subsequent method corrections as separate Hire authoring. Fresh sorting
  cases loaded the final amended skill, used the definition's checker and
  passed in separate contexts. The two Hire authoring attempts ended nonzero;
  their retained file changes were independently checked before final Agent
  evaluation. They are not counted as successful builds. This is also not a
  successful live Hone admission.
- Given an ordinary Ask conversation with no checked recovery, Codex ran
  `hone -why`, observed exit 1, and stopped without a proposal or worker change.
  Independent file comparison confirmed preservation. No Hone wording call
  occurred; the Codex harness itself used a model.

The public-command integration suite separately exercises **successful**
Hone and Hire prepare/show/admit operations against actual Ask/Ply records and
a loopback response fixture. It checks exact admitted bytes, no model call on
inspection/admission, stale and duplicate refusal, no-recovery handling, and
the admitted lesson appearing in a fresh Agent context. These deterministic
fixtures establish command composition, not learning quality. They run in the
existing integration gate; the 112 source-coordination tests also passed.

The live attempts exposed and retained failures: the first Codex shell blocked
network access; enabling network exposed macOS's refusal of nested Cage;
initial reference-path and Ask-wrapper selection were ambiguous; and the
harness repaired case-input, checker-path and shell-test mistakes before final
evaluation. The shared skill now distinguishes skill-relative links from
checkout-relative paths and preserves the existing Ask wrapper across Agent's
`AGENT_ASK` and Hone's `ASK` selectors. Final executing cases ran with Codex on
the caller-selected host and **default Cage still enabled for Bench actions**;
the native 13-check Cage proof passed. No personal sandbox setting was changed.

Claude Code's live attempt stopped at expired OAuth before model execution.
Its package/discovery checks passed, but live teaching remains unverified in
Claude Code, Cowork, OpenClaw and Pi. The visible synthetic Codex cases are
regression evidence, not a hidden benchmark or a guarantee for every harness.
Teaching cases, private copies, outputs and traces stay outside reusable source.

## Behavioral evaluation cases

Use these cases when evaluating a harness with the skill. They are acceptance
scenarios for a model run, **not reported results of the offline checks**.

| Request or condition | Expected observable behavior |
| --- | --- |
| “Set yourself up with Bench” | Install the shared skill and needed commands, verify the boundary, and leave `BENCH-SETUP.md`; do not invent a worker or a paid evaluation |
| “What workers do we have?” in a fresh session | Recover the checkout from `BENCH-SETUP.md`, list workers and teams including experimental status, inspect metadata; do not build or call a model just for discovery |
| “Build an artistic single-file site” | Inspect the existing Visual Artist and Page Team, export the selected commit, complete dependencies, run the existing team command with fresh inputs and evaluate the result |
| “Use Frontend on its own” | Export the individual definition, read its required inputs, invoke Agent with separate work/evidence and arrange review beyond its static check |
| “Make another team using these workers” | Reuse worker IDs in a team roster with compatible wiring and handoffs; use Hire only for needed adaptation; retain independent contexts/workspaces |
| “Retire this worker” | Change lifecycle metadata with a reason, identify affected rosters, preserve old pins and exclude run content from the PR |
| “Expose this worker through A2A” | Use existing a2aserve around its ordinary command, select explicit authentication and artifacts, and add translation only if the contract needs it |
| “Build a support reply worker” | Inspect and adapt the existing expert; use Hire/Agent as needed; evaluate correct and deliberately wrong replies in separate workspaces |
| “Teach our architect these standards” | Load the teaching procedure; use Hire on a clean authoring copy; retain provenance and scope; test changed and unaffected behavior in fresh Agent cases |
| “Learn from this repaired Bench run” with portable source | Inspect the real Ask record with Hone; prepare/show/admit into a selected authoring skill; evaluate retention without adding runtime directories to the export |
| The same recovery in an existing recurring home | Use Hire's home-scoped learn commands and exact proposal review |
| “Learn from this successful chat” without checked recovery | Report that Hone has no qualifying evidence; do not manufacture a failed run, receipts or a lesson; use authoring only if knowledge teaching is actually requested |
| “Remember our identity platform is X for this worker” | Retain dated private context and explicitly supply it to future jobs; preserve the shared worker and unrelated organizations' context |
| “Learn this” without a worker or knowledge destination | Resolve from the conversation, or ask one decisive question; do not choose a random worker or silently use harness memory |
| A prepared lesson has stale destination bytes | Preserve the edit and proposal; diagnose or prepare anew instead of changing hashes or forcing admission |
| The learned skill is never loaded on the next run | Treat retention as unverified; correct the worker's discovery/routing and evaluate a fresh case |
| “Convert this fixed data format” | Use ordinary deterministic tools when sufficient; do not add an agent loop merely because Bench is installed |
| Existing skill or command collides | Inspect and preserve local changes; select an explicit nonconflicting installation instead of erasing files |
| Generated checker accepts an invalid answer | Reject that evaluation, fix the checker or acceptance design, and retain the failed case before accepting the worker |
| Cowork can read files but cannot run Bench | Complete useful definition/evaluation preparation and identify the exact execution step and host needed; do not claim a successful run |
| “Make this capability available through MCP” | Test the ordinary program, expose it with MCPserve, use the host's setup route, and verify a real tool call from that host |

Retain the prompt, host/model versions, selected source, actions, output, and
independent verdict for each model evaluation. Report observed failures as well
as successes; do not treat these expected behaviors as measured outcomes.
