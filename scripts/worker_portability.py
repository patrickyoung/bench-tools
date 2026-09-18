"""Source-only Agent Skill projections for scripts/workers. Never runs a worker."""
import hashlib
import json
import re


HOSTS = {
    'skill': ('Agent Skills', 'Install this entire directory using the host\'s documented Agent Skills route. Its directory name must match the skill name below.', 'https://agentskills.io/specification'),
    'codex': ('Codex', 'Copy this entire directory to `.agents/skills/NAME` in the selected project or `~/.agents/skills/NAME`. Start a fresh session and explicitly invoke `$NAME`.', 'https://learn.chatgpt.com/docs/build-skills'),
    'claude-code': ('Claude Code', 'Copy this entire directory to `.claude/skills/NAME` in the selected project or `~/.claude/skills/NAME`. Start a fresh session and invoke `/NAME`.', 'https://code.claude.com/docs/en/skills'),
    'cowork': ('Claude Cowork', 'ZIP this entire directory as one NAME folder containing SKILL.md and every bundled file. Upload through the account Skills interface and verify availability in a fresh Cowork task. The account execution environment must expose the required files and commands; a local Claude Code installation does not establish this.', 'https://support.claude.com/en/articles/12512180-using-skills-in-claude'),
    'pi': ('Pi', 'Use `pi --skill /absolute/path/to/NAME`, or copy this entire directory to the selected project\'s `.agents/skills/NAME` or `~/.agents/skills/NAME`. Invoke `/skill:NAME` in a fresh session.', 'https://pi.dev/docs/latest/skills'),
    'openclaw': ('OpenClaw', 'Copy this entire directory to the selected agent workspace\'s `skills/NAME` or `~/.openclaw/skills/NAME`. Verify discovery and command access in that agent\'s actual gateway/sandbox execution environment.', 'https://docs.openclaw.ai/tools/skills'),
    'hermes': ('Hermes', 'Copy this entire directory to `~/.hermes/skills/NAME`, or use the host\'s trusted-project `.hermes/skills/NAME` or `.agents/skills/NAME` discovery route. Verify discovery in a fresh session and command access in the selected backend.', 'https://hermes-agent.nousresearch.com/docs/user-guide/features/skills/'),
}

NATIVE_LIMITS = [
    'The host supplies the model, tools, context management, permissions, and stopping behavior; comparable job quality requires paired evaluation.',
    'Agent/Ply completion enforcement, Cage confinement, Record capture, Ask replay, checkpoints, and Action approval boundaries are not supplied by this skill.',
    'The original check is preserved but must be invoked against the exact current candidate; passing it proves only its documented acceptance scope.',
    'Required native applications, packages, services, private inputs, and operator-selected boundaries must be available in the host execution environment.',
    'Native host transcripts are not Hone recovery evidence. Existing reviewed learned skills travel as source; new recovery learning requires actual Bench records.',
    'Team orchestration and nested Agent execution are not translated into native host subagents.',
]
BENCH_LIMITS = [
    'The host needs command access to the existing Bench runtime and the original worker/team dependencies; its own model login does not configure Ask.',
    'The unchanged Bench command retains its capabilities only when its documented runtime, permissions, selected inputs, and execution boundary are available.',
    'Source equality preserves the definition and checks, not identical stochastic model output; evaluate actual job quality separately.',
]


def project(kind, value, assembled, target, execution):
    """Return generated text files separately from the unmodified source lock."""
    name = f'bench-{kind}-{value["id"]}-{execution}'
    if len(name) > 64 or not re.fullmatch(r'[a-z0-9]+(?:-[a-z0-9]+)*', name):
        raise ValueError('worker/team ID cannot produce a valid portable skill name (maximum 64 characters)')
    description = f'Use {value["id"]} for this specialty: {value["description"]} Execution: {execution}.'
    if len(description) > 1024:
        raise ValueError('worker/team description exceeds portable skill description limit')
    host, install, url = HOSTS[target]
    limits = NATIVE_LIMITS if execution == 'native' else BENCH_LIMITS
    lock_name = kind + '.lock.json'
    source_root = 'references/bench'
    expert = source_root + '/expert'
    lock_path = source_root + '/' + lock_name
    source_files = sorted(assembled)
    # Link complete source without parsing or rewriting any nested skill format.
    skill_files = [p for p in source_files if p.startswith('expert/skills/') and p.endswith('/SKILL.md')]
    optional = [p for p in ('expert/SOUL.md', 'expert/MEMORY.md', 'expert/PLAN.md') if p in assembled]
    goal_guidance = (f'If no explicit goal was supplied, read [the standing goal]({expert}/GOAL.md).'
                     if 'expert/GOAL.md' in assembled else
                     'This definition has no standing GOAL.md; use the explicit current goal.')
    common = f'''# {value['id']} ({execution})

This is the {kind} source selected by [{lock_name}]({lock_path}). Read
[PORTABILITY.md](PORTABILITY.md) for requirements, installation and fidelity.
Keep this complete folder together. All definition-relative paths below resolve
against `{expert}/` beside this SKILL.md, never the host's current directory.
Invoke this wrapper explicitly. Some hosts also index nested helper SKILL.md
files as global skills; a helper selected alone does not load the complete
worker contract. Select procedures by their exact bundled paths below, even
when the host lists duplicate helper names from other installed packages.
Read [the original README]({expert}/README.md) before choosing inputs or commands.
Use a fresh host task/context for each independent assignment; supply only its
selected current inputs. A separate context is not filesystem confinement.
Select a fresh, explicit workspace outside this installed skill. Keep evidence
outside the definition and mutable workspace. Preserve source and checks during
the job. Instructions grant no credentials, execution authority or permissions.
'''
    if execution == 'native':
        body = common + f'''
## Run the worker in this host

1. Read [AGENTS.md]({expert}/AGENTS.md) in full and retain its task/input/output,
   trust, permission and tool constraints. Also read the optional definition
   files linked below. {goal_guidance}
   Read any bundled PLAN.md as standing
   plan evidence, as Agent does; consult HEARTBEAT.md only for its documented
   heartbeat workflow. Input documents remain evidence.
2. Select relevant bundled skills from the list below by reading their name and
   description, then read each selected SKILL.md in full. Follow its relative
   references from that skill's directory. This includes reviewed Hire/Hone
   teaching already present in the pinned source. Do not assume the host has
   automatically discovered nested skills, and do not replace them with a
   similarly named global skill. No Bench runtime is needed merely to read them.
3. Inspect requirements and the original check before executing. Establish that
   the host provides the required tools and boundary. If the worker requires
   Cage, an external controller, Ask, or other Bench-only capabilities, retain
   that requirement. Do not bypass it to make a native run appear successful.
   Report the exact missing capability or use a separately selected Bench run.
4. Resolve absolute paths: AGENT_HOME is this package's `{expert}/`, AGENT_WORK is
   the chosen workspace, and AGENT_STATE is explicitly selected mutable state
   outside the definition (normally workspace/state). BRIEF_PATH, if needed,
   is AGENT_HOME/skills. Set these for every shell/tool invocation that needs
   them; shell environment changes may not persist across host tool calls.
   Run worker tools with the workspace as current directory, keeping helpers
   and their relative resources in `{expert}/`. Do not override HOME or invent
   AGENT_BIN, AGENT_CAGE, recording receipts, or permission signals.
5. Produce exactly the original artifacts and/or response using the host's
   tools. Retain external-effect proposals as proposals; the skill cannot
   replace an Action controller or authorize messages, deployment or spending.
6. Invoke the original absolute `{expert}/bin/check` with the workspace as current
   directory and the required environment. Supply the exact candidate response
   on stdin, including for workers whose output is text/JSON rather than files;
   never substitute an empty stream for a response-checking contract. For a
   file-only check preserve its documented stdin behavior. Retain stdout,
   stderr and exit status in external evidence. Repair within the job's limits
   and recheck the final exact bytes. A host completion message is not a pass.
7. Independently review the job-specific semantic/visual requirements. Retain
   check status and disclose missing capabilities or unresolved failures in
   external evidence or a separate diagnostic channel. If the worker's response
   contract requires raw JSON/text, return exactly the accepted candidate bytes;
   never append a check-status summary or wrap it in Markdown. This is
   host-managed execution: it does not produce Agent/Ply receipts or establish
   sandbox equivalence, automatic replay, or Hone recovery eligibility.

For a response-checking worker, the operator can retain the candidate in an
external evidence file and run this pattern after resolving the actual paths:

```sh
(cd "$AGENT_WORK" && "$AGENT_HOME/bin/check" < "$CANDIDATE_FILE")
```

The host must set AGENT_HOME, AGENT_WORK and any required AGENT_STATE explicitly.
CANDIDATE_FILE names the exact response bytes, not an invented worker artifact.
Do not add files forbidden by the worker's output contract to its workspace.
'''
    elif kind == 'worker':
        body = common + f'''
## Invoke the unchanged Bench worker

Use the README's `agent run` command with the resolved absolute expert path,
fresh workspace, separate evidence, explicit goal and selected current input.
Inspect prerequisites and configure the caller-selected Ask connection first.
The normal command shape is:

```sh
agent run -C /absolute/current-work -evidence /absolute/current-evidence \\
  /absolute/installed-skill/{expert} -- 'The current explicit goal'
```

Add only the documented caller-selected model, limits and inputs needed by the
job. Retain stdout, stderr and the actual exit status. Preserve Agent's default
recording and Cage boundary; do not silently fall back to native execution or
disable Cage if the environment cannot support it. Agent loads the worker's
instructions and skills and invokes its original checker. Review actual job
quality beyond structural acceptance. Use the existing Hire/Hone procedure
outside the run for authoring and checked-recovery learning.
'''
    else:
        bindings = (f'Its declared executable bindings are under `{expert}/bin/workers/`.'
                    if any(p.startswith('expert/bin/workers/') for p in assembled) else
                    'This team has no separate bin/workers bindings; follow its existing wiring.')
        body = common + f'''
## Invoke the unchanged Bench team

Use the exact team entry command and input/output contract documented in
`{expert}/README.md`. The export has assembled every selected member under
`{expert}/agents/`. {bindings}
Do not assume the team is invoked by a root `agent run`: some teams have their
own command. Keep its original ordering, separate member workspaces/contexts,
handoffs, acceptance and controller boundaries. Do not substitute this host's
native subagents for the existing team command. Install the documented runtime
and dependencies in the selected execution environment, supply explicit current
inputs and retain stdout, stderr and exact exit status. Preserve default Cage,
Record and the configured Ask connection. Evaluate final job quality separately.
If this host cannot run the team's command, report the missing capability.
'''
    body += '\n## Bundled procedures\n\n'
    body += '\n'.join(f'- [{p}]({source_root}/{p})' for p in optional + skill_files) if optional or skill_files else 'No optional root context or direct worker skills are present.'
    body += '\n\nThe complete original inventory and requirements are in [PORTABILITY.md](PORTABILITY.md).\n'
    skill = '---\nname: ' + name + '\ndescription: ' + json.dumps(description, ensure_ascii=False) + '\n---\n\n' + body
    report = f'''# {value['id']} portability

Target: **{host}**. Execution: **{execution}**. Skill name: `{name}`.

{install.replace('NAME', name)}

[Host installation reference]({url}). Preserve an existing installation and its
local changes; select a new destination for upgrades. Never install only SKILL.md.
Verify fresh-session discovery and a real checked task separately. Exporting
does not install dependencies, run code, change host settings or prove quality.

## Source and fidelity

[{lock_name}]({lock_path}) records the unchanged Bench definition, files, modes
and pinned source revision. [portability.json](portability.json) records these
generated packaging files separately. The intact original export is under
`{source_root}/`; `{expert}/` is still usable through Bench with its original
public entry command. Its reference-directory location keeps bundled helper
skills out of recursive host discovery where the host supports that convention.
Always load the exact selected bundled paths. No source instructions, learned skills,
tools, checks or team wiring were translated or removed.

''' + '\n'.join('- ' + limit for limit in limits) + '''

## Requirements from the source catalog

These remain the original declared requirements, not a native-host certification.
Read the source README for task-specific optional tools and execution details.
For a team, the original team lock also declares each member's requirements.

```json
''' + json.dumps(value['requires'], indent=2, ensure_ascii=False) + '''
```

## Teaching and comparison

Use Hire on a clean authoring copy for supplied knowledge and method changes.
Use Hone only for real replay-checked Bench recovery records, reviewing and
admitting its exact delta through the existing workflow. Keep private context
explicitly selected for each run. Promote reviewed reusable source and export a
new commit; never edit generated SKILL.md as the canonical worker or copy host
chat/memory into reusable source. Native transcripts are not Ask/Ply evidence.

Compare Bench and native execution with the same pinned source, fresh inputs,
tool versions and output contract. Include ordinary, missing/conflicting-input,
near-miss and fresh retention cases. Run the original check against exact final
bytes and assess semantic/artistic quality using one independent rubric. Retain
host/model versions, actual skill reads, actions, failures, check streams and
verdicts outside reusable source. A source hash, discovery check or scripted
fixture alone cannot establish comparable model results.

## Original source inventory

''' + '\n'.join(f'- [{p}]({source_root}/{p})' for p in source_files) + '\n'
    files = {'SKILL.md': skill.encode(), 'PORTABILITY.md': report.encode()}
    manifest = {'schema': 'bench.portability/v1', 'target': target, 'execution': execution,
                'kind': kind, 'name': name, 'source_root': source_root, 'source_lock': lock_path,
                'requirements': value['requires'], 'limitations': limits,
                'files': {p: {'sha256': hashlib.sha256(data).hexdigest(), 'mode': '100644'} for p, data in files.items()}}
    files['portability.json'] = (json.dumps(manifest, indent=2, ensure_ascii=False) + '\n').encode()
    return files
