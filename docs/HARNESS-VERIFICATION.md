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
| “Convert this fixed data format” | Use ordinary deterministic tools when sufficient; do not add an agent loop merely because Bench is installed |
| Existing skill or command collides | Inspect and preserve local changes; select an explicit nonconflicting installation instead of erasing files |
| Generated checker accepts an invalid answer | Reject that evaluation, fix the checker or acceptance design, and retain the failed case before accepting the worker |
| Cowork can read files but cannot run Bench | Complete useful definition/evaluation preparation and identify the exact execution step and host needed; do not claim a successful run |
| “Make this capability available through MCP” | Test the ordinary program, expose it with MCPserve, use the host's setup route, and verify a real tool call from that host |

Retain the prompt, host/model versions, selected source, actions, output, and
independent verdict for each model evaluation. Report observed failures as well
as successes; do not treat these expected behaviors as measured outcomes.

## Automatic recording in the current source

The shared Bench skill now installs Record with Agent and tells harnesses to
use Agent's default action/check recording. It teaches explicit file selection,
retention and offline replay, and standalone Record wrappers for commands
executed directly by a host. This updates source instructions; it does not
claim existing copied skills or installed binaries have been upgraded.

`python3 scripts/check-record-agent.py --bin-dir BIN` verifies the actual
Agent/Ply/Ask/Record composition, including full streams beyond presentation
caps, zero-model pre-checks, artifact snapshots, checkpoint continuation,
compaction summaries and recording failures. The required root gate runs it.
Native host discovery results above remain the dated observations listed.
