# Harness setup verification

[Start here](../START-HERE.md) · [Develop and verify](DEVELOPING.md)

The canonical [Bench skill](../.agents/skills/bench/SKILL.md) supplies one shared
procedure, with a separate setup reference for each host. The Codex and Claude
plugins and Pi package point at those same files. They add discovery metadata; Bench's
existing installer still installs programs, Hire builds, and Agent runs.

## Git marketplace installation on 2026-09-17

Since **0.4.1**, the package declares a Git marketplace for Codex and a separate
Claude marketplace used by Claude Code and Cowork. Codex selects the repository
root; Claude selects the isolated `.agents` directory. Both use the single
canonical Bench skill. The portable check validates these paths,
matching release versions, absence of automatically registered runtime services,
and every skill/reference byte after ZIP extraction to a path with spaces.
Eight regression tests cover relocation and reject inconsistent releases, an
incorrect marketplace root, absolute skill paths, symbolic links to unbundled
files and nested Claude plugin manifests inside the selected Cowork source.

The installed native CLIs passed the following checks for **0.4.1** in
disposable homes:

| Host | Version | Observed result |
| --- | --- | --- |
| Claude Code | 2.1.261 | Adds a Git marketplace URL, installs and enables `bench-tools@bench-tools`, preserves every skill/reference byte, and discovers `bench-tools:bench` in a fresh initialization without `--plugin-dir` |
| Codex CLI | 0.153.4 | Adds a Git marketplace URL, installs and enables `bench-tools@bench-tools`, preserves every skill/reference byte, and discovers the enabled `bench-tools:bench` skill from the installed cache in a fresh app-server |
| Pi | 0.81.1 | Installs the local package and discovers `skill:bench` in a fresh offline RPC session |

These checks use current working-tree package files committed into a temporary
fixture repository. A process-local Git URL rewrite routes an HTTPS-shaped
fixture URL to that repository, exercising the hosts' Git marketplace path
without depending on publication or contacting a model. No user profile is
copied or changed. The existing independent MCP discovery/call and native host
registration checks also pass. Those MCP entries are test fixtures; ordinary
Bench setup does not install them.

Repeat with `python3 scripts/check-harnesses.py --host-clis`. The default
`make check-harnesses` runs portable checks without requiring any host CLI.

An **actual network installation of 0.4.2** passed for both native CLIs from
the public GitHub branch `codex/prebuilt-setup` at commit
`497fda761abe1d7c215b051f4bb8d030db62732c`. This used the complete published
repository with no Git URL rewrite. Claude Code and Codex each installed the
plugin and discovered **exactly one** enabled/namespaced Bench skill in a fresh
process; all installed skill/reference bytes matched that source commit.
The plain default-branch URL was previously verified for both native CLIs at
`fbc52833cd051ac02d37de358c907fd5b3f4a7f5` with **0.4.0**, without `--ref`.
These are distinct source revisions; the newer branch result does not establish
that 0.4.2 has reached the default branch.

The explicit network check is repeatable with:

```sh
python3 scripts/check-harnesses.py --host-clis \
  --source-url https://github.com/patrickyoung/bench-tools.git \
  --ref codex/prebuilt-setup
```

It compares the remote installed skills with the current checkout, so select
matching source before running it. Omit `--ref` to check the URL's default
branch. Network access is opt-in; the default fixture remains offline.

### Corrected Cowork marketplace verified

Cowork's account UI rejected the published **0.4.0** marketplace even though
both native CLIs installed it. Its sync diagnostic identified
`marketplace_sync_multiple_manifests`: selecting the entire repository included
the existing legacy manifest at
`tools/agent/plugins/bench-system-builder/.claude-plugin/plugin.json`.
The generic UI error did not explain that nested-manifest restriction.

A controlled upload of the minimal **0.4.0** ZIP succeeded with one skill while
retaining the custom `.agents/skills` path, confirming that path was accepted.

The **0.4.1** correction selects `./.agents` from the Claude marketplace and
adds its own `.claude-plugin/plugin.json` and license there. Its conventional
`skills/bench` path points at the existing canonical skill; no skill copy was
added and the legacy component plugin remains unchanged. The portable ZIP
uses that isolated plugin layout. The local Git fixture now includes all
competing manifests, so it cannot conceal the earlier root-selection error.
Checks reject any nested plugin manifest inside the selected Cowork source
while permitting independent component plugins elsewhere in the repository.

After PR #17 reached main at
`4d2a8346daf8fc5152400914bc93852d59068986`, Cowork successfully synced the
canonical URL `https://github.com/patrickyoung/bench-tools`. The actual account
UI showed source `bench-tools`, version **0.4.1**, and **one skill**. Adding it
completed with the plugin installed and enabled. A fresh New task draft showed
exactly one Bench tools group and one `bench` slash-menu item. The draft was
cleared without submitting a model turn.

The temporary **0.4.0** My Uploads copy was removed before this marketplace
installation; unrelated plugins were preserved. This plugin remains **0.4.1**;
refresh to **0.4.2** is not yet verified. The separate runtime setup below proves
command availability and confinement in the tested Cowork session.

### Runtime package and source setup

The new `scripts/setup` command also passed a complete source build and install
on macOS with a separate runtime prefix and setup directory. All nine version
checks, Hire's structural verification and Cage's 13 native checks passed.
An independent prebuilt-package install and repeat passed in paths containing
spaces. Eleven executable regression cases cover fresh-shell path recovery,
partial confinement failure, edited/unmanaged handoff preservation, malformed
receipts, command collisions and refusal to switch an existing runtime record.
They also preserve handoff/environment edits and receipt changes made while
setup is running, and serialize competing setups for the same state directory.
The native Cage CI jobs now exercise source setup on both Linux and macOS.
No Ask request or worker evaluation is part of these setup checks.

At source commit `d5150f4ebccdf7f2270b2c1a087c1cd7e2cc5782`, the
[Builder packages run](https://github.com/patrickyoung/bench-tools/actions/runs/35295980893)
passed all four native Linux/macOS amd64/arm64 jobs. These jobs build, package,
extract, install and verify the distributed independent packages before signing
their archives with GitHub build attestations. The downloaded archives were
also independently verified with `gh attestation verify`, enforcing the exact
repository, the `packages.yml` workflow identity at
`refs/heads/codex/prebuilt-setup`, matching source and signer commit IDs, and
GitHub-hosted runners.

These signatures establish GitHub Actions provenance. They are not Apple
Developer ID signatures or notarization. Automatic published packages are
limited to Linux; macOS defaults to source
builds even if a matching Mac pin is present. Explicit caller-selected local
packages remain supported. Twelve prebuilt acceptance tests cover that policy,
Linux selection without Go, preservation of checkout and binary source pins,
changed-source fallback and rejection of damaged or unsafe archives before
program execution. The installer verifies checked-in SHA-256/source pins;
signature verification is performed before publication, as described in
[the release procedure](RELEASES.md#published-builder-packages).

A real default macOS setup at the same `d5150f4` source passed all nine version
checks, Hire verification and native Cage checks. Its evidence is outside the
checkout under `bench-mac-source-default-fqtjgwnv`. The
[complete source CI run](https://github.com/patrickyoung/bench-tools/actions/runs/35295985305)
passed all 50 checks. The attested Linux archives were then
[published from that source](https://github.com/patrickyoung/bench-tools/releases/tag/builder-d5150f4ebccdf7f2270b2c1a087c1cd7e2cc5782).
The default downloader fetched the public Linux amd64 archive, verified its
checksum, safely extracted it and verified every independent package. Both
Linux pins also matched a fresh committed checkout. These read/verification
checks on macOS did not execute Linux binaries.

### Cowork default package setup verified

In the [observed Cowork task](https://claude.ai/cowork/cse_01UHt8EhuNhPa8WPGPARyt3G),
default `python3 scripts/setup` from clean checkout
`80da98b4c1c805c333eb348fe84320bd0f503fd8` downloaded the published Linux amd64
archive from release `builder-d5150f4ebccdf7f2270b2c1a087c1cd7e2cc5782`.
Its SHA-256 was
`c8ec1421081452e85f74dea58ace57ea0fa4a6f6629a2511349bad5178b95b3f`
and its size was **38,370,645 bytes**. A rejecting `/tmp/no-go-bin/go` was never
invoked. All nine version checks, Hire's structural verification and Cage's
13 native probes passed: all **11 setup checks exited 0**. All nine installed
package receipts matched the checked-in pins.

Cage allowed workspace and temporary writes, denied outside and read-only
writes, allowed an explicit writable path, and verified that explicit writable
paths replace the implicit workspace grant. It preserved stdin, stdout/stderr,
exit 42 and signal status 143; denied network by default; allowed deliberate
`-net`; and failed closed when its backend was unavailable.

A fresh `env -i` shell sourced `env.sh`, resolved all nine commands under
`/root/.local/share/bench/runtime/bin`, and ran their version commands with Go
absent. The runtime and `BENCH-SETUP.md`, `env.sh`, and `setup.json` under
`/root/.local/share/bench` are session-local. This proof does not establish
persistence across Cowork sessions, model access or worker execution.

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

Cowork's corrected **0.4.1** canonical GitHub marketplace installation and fresh
skill discovery and the default package runtime setup passed as described above.
Persistence across Cowork sessions, remote connector reachability, model access
and worker execution remain unverified. The
[setup reference](../.agents/skills/bench/references/cowork.md) requires checking
these properties in the selected session; the observed local checks do not
establish them for another execution environment.

OpenClaw had **not** been exercised in the 2026-09-13 setup checks. Its
[setup reference](../.agents/skills/bench/references/openclaw.md) uses the
documented local skill route and requires discovery and command checks in the
selected agent's execution environment. Primary documentation links are kept
beside the host-specific instructions.

The worker-package checks below add later OpenClaw and Hermes discovery evidence;
they do not establish live worker quality in those hosts.

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

## Worker portability on 2026-09-17

The [worker portability exporter](WORKER-PORTABILITY.md) adds a native host skill
or a Bench-invoking skill around an intact export under `references/bench/`.
The general Bench skill's Claude/Pi package version is **0.3.0**. The exporter
does not install a host or change its permissions. Teams retain their existing
Bench entry command; native team translation is explicitly refused.

The offline executable check exported Product Owner, Page Planner and Visual
Artist in both execution modes, and Page Team in Bench mode, for all seven
targets: **49 projections** at worker source commit
`9fe699632e1ac23a8542c4861098b068a0f6a8fd`. Original file bytes, executable
modes, locks, member assembly and requirements matched the committed source.
Brief lint and full-folder relocation passed. The real Page Planner checker
accepted its exact response on stdin and rejected a stale snapshot and empty
stdin after relocation. Thirteen synthetic exporter tests additionally cover
learned skill/reference preservation, standing-plan context, raw response
contracts, name limits, invalid modes, no overwrite and no source execution.

Fresh native discovery was observed independently of model quality:

| Host | Version | Observed result |
| --- | --- | --- |
| Codex CLI | 0.153.4 | Discovers all seven wrappers by exact name/path; also exposes 18 nested helper entries, including duplicate names across packages. |
| Claude Code | 2.1.261 | Strict plugin validation passes; initialization discovers all seven generated skills. |
| Pi | 0.81.1 | Offline RPC discovery lists all seven generated skills. |
| OpenClaw | 2026.9.4 (`3a9d69d`) | Native local installation and skills list/info expose the Product Owner package as eligible and invocable. |
| Hermes | 0.21.3 (`cdceca42`) | Final package exposes exactly one enabled wrapper; its original procedures remain readable by explicit bundled path. |

All discovery checks used isolated profiles or homes and made no model calls.
OpenClaw and Hermes installations preserved every package file and mode. Their
loader eligibility does not establish dependency availability, confinement or
successful task execution. The initial package layout let Hermes index six
nested procedures under bare names; a collision probe demonstrated ambiguous
lookup. The final `references/bench` layout removes those extra Hermes entries.
Codex still indexes helpers recursively, while Claude Code and Pi expose only
the wrappers in the same probes. Invoke the wrapper explicitly and read exact
bundled paths; helper names alone are not a worker output contract. OpenClaw
adds its own installation-provenance file without changing bundled source.
Cowork account upload and execution of these exported worker packages remain
untested.

A separate access audit found Claude Code signed out and the isolated
OpenClaw/Hermes profiles without a selected usable provider connection.
One bounded Hermes greeting using its documented keyless
`opencode-free/mimo-v2.5-free` route reached the provider but received HTTP 403:
the free tier refused use outside OpenCode. It produced no successful model or
tool output. This is a failed execution probe, separate from the successful
discovery checks. Other provider routes remain unevaluated; no credentials,
personal settings, gateway services or local model installations were changed.

Separately selected live evaluations used Product Owner with the same source
and identical fresh inputs for two cases: an AI release review with a misleading
aggregate score and injected vendor instruction, and planning with missing
backlog/capacity/history. They are visible regression cases, not hidden tests.
Native Codex used `gpt-6-astra`/high and Bench used the same model through the
existing Ask connection. Native Pi and its paired Bench runs used
`gpt-5.6-sol`/high; the installed Pi catalog did not include Astra.

All eight runs returned exit 0 and passed the unchanged artifact checker.
The Codex/Bench pair agreed on the independently reviewed decisions: hold
broader release, expose Spanish correctness of 20% versus English 97.8%,
treat vendor instructions as evidence, supply containment/gates, and avoid
inventing a backlog or launch date. The Pi/Bench pair agreed on the main
decisions, with evidence-precision and ownership caveats retained in the
review: one Bench response called unmeasured impact known harm, its missing
input handoff left owners unassigned, and Pi's prose omitted next actions
present in its structured handoff. These are scoped, caveated results rather
than a claim of identical quality or enforcement.

All four Bench runs retained default Cage and passed 70 Ask/Record verification
checks. Inputs and original source bytes/modes were unchanged. Native runs
loaded the original relevant skills and retained their own host traces; those
traces are not Hone-compatible recovery records. Claude Code was signed out,
so its live evaluation was not attempted. OpenClaw, Hermes and Cowork live task
quality and native team orchestration remain unverified. Later teaching and
native-tool probes are reported separately below.

After the layout correction, fresh native Codex, native Pi and direct Bench
smokes used the relocated package against another fresh missing-input case.
All three returned exit 0, passed the original check and preserved source and
inputs. The Bench run additionally passed 19 Ask/Record verifications with
default Cage. These bind the final layout to observed execution separately
from the earlier paired quality comparisons.

Hone inspected the five retained Bench sessions with `-why` and replay checking.
Each was mechanically eligible because an initial empty-workspace pre-check
failed before the completed job passed. Inspection found no substantive failed
method and repair beyond producing the already-required artifacts. No wording
call or lesson admission was made from those records. Native host transcripts
were not converted into recovery evidence.

Hire then amended only the AI-product skill in a separate Product Owner
authoring copy. The method distinguishes observed answer failures, possible
customer consequences and measured customer impact. The local evaluation-only
revision passed Hire verification, strict Brief lint and 29 original worker
tests before pinned re-export; canonical worker source was not changed.
Three fresh cases tested unmeasured harm, supplied measured harm, and ordinary
planning where the method should not apply. Bench and Codex used Astra/high;
Pi used Sol/high, so this is retention evidence rather than a matched-model Pi
quality comparison. Execution prompts did not include the taught labels or
rubric.

All nine runs completed and passed the unchanged artifact check, preserving
source and inputs. Both native hosts used all three taught labels in the AI
cases and omitted the ledger in the planning control. Bench retained the
evidence distinctions but used "Possible additional impact" in one response
instead of the required "Possible customer impact": that case failed the
strict wording criterion. Keep this variability visible; source preservation
does not guarantee exact learned wording. The three Bench runs retained
default Cage and passed 51 Ask/Record checks.
Independent review scored 9/9 for substantive retention and 8/9 for the complete
rubric; all three planning controls correctly omitted the AI evidence ledger.

A controlled Inkscape worker probe used the final package layout, paths with
spaces and real Inkscape 1.4.4. Its deterministic Cage author/finish/check
preflight passed and rejected a changed plan. Native Codex's workspace-write
execution then could not nest the worker's required Cage sandbox inside its
own macOS sandbox. Cage exited 125 and the host stopped before creating worker
artifacts. This configuration is capability-blocked, not evidence of native
graphics equivalence. The required boundary was not bypassed to obtain output.
The separately run Bench baseline completed with default Cage and all ten
declared artifacts, preserving source and inputs. Its independent original
checker and all 19 Ask/Record verifications passed. Visual inspection confirmed
the requested Observe/Shape/Share cards, focal illustrations, two connecting
arrows, palette and readable headings. This demonstrates the Bench path for
that job, while leaving the native configuration's failure visible. A separate
thumbnail conversion failed at the confinement boundary; full-size visual
inspection passed, but thumbnail review was not completed.

Repeat the offline package and optional installed-host discovery checks with:

```sh
make check-worker-portability
python3 scripts/check-worker-portability.py --bin-dir .build/bin --host-clis
```

The second command exercises Codex, Claude Code and Pi only. OpenClaw/Hermes
installation probes and live inputs, outputs, invocation records and reviews
are retained outside reusable source. No personal host settings were replaced.
