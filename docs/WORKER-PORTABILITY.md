# Use a Bench worker in another harness

[Library](WORKER-LIBRARY.md) · [Harness setup evidence](HARNESS-VERIFICATION.md)

Export the same reviewed worker definition as an installable skill, selecting
whether the host performs the work or calls Bench. The canonical `expert/`
files and source lock remain intact under `references/bench/`. Packaging adds host instructions; it does
not replace Agent, weaken checks, install dependencies or grant permissions.

## Choose execution explicitly

| Execution | Worker | Team | What runs the job |
| --- | --- | --- | --- |
| `bench` | Supported | Supported | Agent or the team's existing documented entry command, invoked through the host's command tool. |
| `native` | Supported as an adaptation requiring evaluation | Rejected | The host's model and tools use the worker's instructions, skills, deterministic helpers and output checker. |

Bench execution preserves its existing context, loop, Cage, Record and team
handoff mechanisms when their declared dependencies and execution environment
are available. Source portability does not make an incompatible operating
system, sandbox or unavailable credential work.

Native execution retains the expertise and acceptance files. The host supplies
its own tool loop, permissions, context management, model and history. Native
instructions to run a checker do not enforce the same retry or admission
behavior as Agent. Cage confinement, automatic Record evidence, continuation
and Hone-compatible recovery records are not supplied by a skill. A host's
tool named "shell" is not proof that every required command is available.

A team is more than a set of role descriptions. Native team export is rejected
until its controller, selected inputs, separate contexts, handoffs, admission
checks and failure behavior have an implemented and verified equivalent.
Use `--execution bench` for a team, or export members individually for separately
evaluated native work. Host subagent support alone does not establish team
equivalence.

## Export a pinned package

Inspect both catalogs and the selected metadata and README first:

```sh
python3 scripts/workers list --all
python3 scripts/workers list --teams --all
git rev-parse HEAD
python3 scripts/workers export frontend \
  /absolute/exports/bench-worker-frontend-native \
  --ref FULL_COMMIT --allow-experimental --target pi --execution native
python3 scripts/workers export-team vendor-comparison-team \
  /absolute/exports/bench-team-vendor-comparison-team-bench \
  --ref FULL_COMMIT --allow-experimental --target claude-code --execution bench
```

Replace `FULL_COMMIT` with the reviewed full commit and choose new destinations
outside the source checkout. Experimental status still requires explicit
selection. Uncommitted authoring changes are not exported.

`--target` accepts `skill`, `codex`, `claude-code`, `cowork`, `pi`, `openclaw`
or `hermes`. A target requires `--execution native|bench`. Ordinary exports
without these options retain the existing layout and behavior. `skill` supplies
generic installation guidance for another host implementing Agent Skills;
it does not certify that host's execution support. `pi` refers to the harness
at [pi.dev](https://pi.dev/).

A targeted export contains:

```text
bench-worker-frontend-native/
  SKILL.md                 host entry and selected execution procedure
  PORTABILITY.md           installation, requirements and fidelity limits
  portability.json         generated package metadata
  references/bench/
    worker.lock.json        original committed source identity and hashes
    expert/                original worker files, helpers and checks
```

A team has `references/bench/team.lock.json` and its assembled roster in
`references/bench/expert/`. This nested directory is an intact ordinary Bench
export: the lock retains its original relative paths and bytes, and the expert
runs through the same Bench command at its new absolute location. The source
lock describes the original definition; it does not certify later edits,
installed dependencies or generated host instructions. Keep portability
metadata with the export so the selected execution mode remains explicit.

The standard references directory also prevents Hermes from registering each
bundled helper skill as another bare top-level name. Installing the original
expert tree directly under the skill root caused real helper-name collisions
in discovery probes. Keep the generated layout and invoke its wrapper skill;
load selected procedures by their exact bundled paths. Codex still recursively
indexes nested helpers beneath this directory, including duplicate names when
multiple packages contain them. Explicit wrapper invocation and exact-path
procedure loading are the tested route; automatic selection of a helper alone
does not establish the full worker contract. Do not interpret catalog discovery
as a namespace or context-isolation guarantee.

Install the **whole export folder**. Moving only `SKILL.md` breaks its references
to expertise, checks and provenance. Use the generated name as the installed
directory name: `bench-worker-ID-native`, `bench-worker-ID-bench`, or
`bench-team-ID-bench`. Read the actual frontmatter rather than guessing after a
rename. Agent Skills requires matching directory and skill names, a name of
1–64 lowercase alphanumeric/hyphen characters without consecutive, leading
or trailing hyphens, and a nonempty description of at most 1024 characters.
[Agent Skills specification](https://agentskills.io/specification).

## Install using the host's route

Export only writes the selected destination. It does not alter personal skills,
project trust, plugins, provider settings or tool permissions. Inspect collisions
and preserve local changes before a separately selected installation.

| Target | Install the complete package | Verify in a fresh session |
| --- | --- | --- |
| `codex` | Use the personal or project `.agents/skills/NAME` route in the [Codex reference](../.agents/skills/bench/references/codex.md). | Discover and invoke `$NAME`. |
| `claude-code` | Copy to `~/.claude/skills/NAME` or the chosen project's `.claude/skills/NAME`. | Invoke `/NAME`. |
| `cowork` | Upload/enable the complete skill through account customization, or put it in a supported Claude plugin's `skills/NAME` directory and upload that plugin. | Start a Cowork task and find/invoke the enabled skill. |
| `pi` | Use `pi --skill /absolute/path/NAME`, or copy to `~/.agents/skills/NAME` or `~/.pi/agent/skills/NAME`. | Invoke `/skill:NAME`. |
| `openclaw` | Use `openclaw skills install /absolute/path/NAME --as NAME` for the selected workspace. | Verify it is eligible and invoke its skill command in the selected agent. |
| `hermes` | Copy to the selected profile's skills directory; default `~/.hermes/skills/NAME`. | Run `hermes skills list`, then invoke `/NAME`. |
| `skill` | Use the chosen host's supported skill directory, preserving `NAME` and every support file. | Verify actual discovery and loading with that host. |

Claude's local skills use filesystem discovery; Cowork uses account-enabled
skills and does not read the laptop's `~/.claude/skills`. Follow
[Claude skill locations and Cowork guidance](https://code.claude.com/docs/en/skills).
Cowork can also accept a custom plugin; its execution environment and connector
network boundary still need separate verification.
[Claude plugins](https://support.claude.com/en/articles/13837440-use-plugins-in-claude).
The targeted export itself is a skill folder, not a prebuilt plugin archive.

Pi supports explicit skill paths and shared skill directories. A Pi package can
instead declare skill directories under `pi.skills` in `package.json`; packaging
does not create a new agent runtime.
[Pi skills](https://pi.dev/docs/latest/skills),
[Pi packages](https://pi.dev/docs/latest/packages).

OpenClaw local installation expects `SKILL.md` at the source root and installs
into the selected workspace by default. A gateway's binaries and injected
environment are not automatically available inside an execution sandbox.
[OpenClaw skills](https://docs.openclaw.ai/tools/skills).
Hermes supports local skill folders; preserve the full directory and inspect
any existing installed copy before replacing it.
[Hermes skill guide](https://hermes-agent.nousresearch.com/docs/guides/work-with-skills).

Host conventions above were reviewed against primary documentation on
2026-09-17. They are installation guidance, not evidence that these worker
packages have run in every host. Consult the generated `PORTABILITY.md` for
the selected target and the [setup references](../START-HERE.md#your-harnesss-setup-instructions)
for execution setup.

## Run and compare actual results

Keep the installed definition separate from current inputs, workspaces,
private memories and evaluation evidence. For native work, read
`references/bench/expert/AGENTS.md`, its README, relevant skills and contracts.
Resolve their relative
paths against the definition. Use the documented deterministic helpers and
the unchanged checker with the current workspace; inspect the check result,
not just the model's claim. If required tools or an output contract cannot be
satisfied, report that limitation. Do not replace a required artifact with a
chat summary or disable a gate to make the run appear successful.

Evaluate comparable results with paired fresh jobs:

1. Pin the same worker source and record host, version, model, tool availability,
   permissions, execution mode, input hashes and resource limits.
2. Run Bench and the selected native host in separate fresh contexts and
   workspaces with identical selected inputs. Prevent earlier answers from
   becoming implicit inputs.
3. Apply the same unchanged output checks and an independent task rubric to
   both results. Include refusal, missing-input and adversarial cases where
   they matter to the worker's contract.
4. Compare required artifacts, evidence and citations, substantive correctness,
   retained taught behavior, tool use, failures, cost and time. For visual work,
   inspect the rendered artifacts. Define acceptable differences for the job
   before declaring the native result comparable.
5. Retain outputs and failures outside source. Record the scope and limits of
   the observed result; a single successful case is not a universal guarantee.

Evidence has distinct scopes:

| Evidence | What it establishes |
| --- | --- |
| Export hashes, structure, relocation and lint | Source and required package files survive export. |
| Fresh native discovery | The selected host finds and loads the package. |
| Executable process fixtures | Exercised commands, streams, exit codes and checker paths compose as tested. |
| Live worker case with independent verdict | That host/model completed the particular job at the recorded quality. |
| Paired representative evaluations | Measured comparability for those jobs, versions and resource limits. |

The [harness verification record](HARNESS-VERIFICATION.md) distinguishes observed
results from untested scenarios. General Bench-skill discovery is not proof of
native worker quality, and preserving source bytes is not proof of identical
runtime behavior.

## Teach once and export again

Keep the worker definition as the authoritative source of reusable expertise.
Follow the [teaching procedure](../.agents/skills/bench/references/learn.md): use
Hire for supplied knowledge or an adaptation, Hone for qualifying checked
Bench recovery, and explicitly selected private context for company facts.
Verify changed and unaffected behavior through fresh Bench cases before
promoting a reviewed source revision. Then re-export that revision for each
selected target and evaluate retention natively too.

Host transcripts are not automatically Ask/Ply/Record recovery evidence. Do not
rewrite native history into a fabricated Hone recovery or copy host memory
into the library. Native observations can inform an explicit Hire authoring
request with provenance. Preserve the original source lock while authoring;
local edits no longer match that pin. Refresh installed packages deliberately
after export, preserving private changes and keeping all current job content
outside reusable source.
