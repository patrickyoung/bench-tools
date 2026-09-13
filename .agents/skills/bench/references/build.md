# Build the solution with the existing runner

Establish the actual inputs, expected deliverable, workspace, permitted effects,
limits, and acceptance evidence. Infer routine choices from the request and
sample files; ask only when a missing decision changes the result or authority.
Turn vague quality words into examples, mechanical checks, or an explicit
review rubric. Do not invent policy or expected facts.

First inspect an existing solution or relevant example. The selected checkout's
`examples/support-reply` is a complete expert, while `examples/meeting-brief`
uses a single model request. Keep useful renderers, parsers, and checks when
adapting a worker; remove duplicate context/loop code by using Agent.

## Reuse the source library first

Read `workers/README.md` and `teams/README.md`. Use `python3 scripts/workers
list --all` to inspect status, then select a full reviewed source commit and
export approved files into a new authoring directory. The exact command and
experimental opt-in are in the catalog. Never copy example showcases, an old
run or a development checkout into a new team. The export contains the expert
and its source lock; current job inputs are supplied separately.

Use an unchanged suitable expert directly. If adaptation is needed, give Hire
the existing exported expert and a short recipe. Preserve its source lock,
inspect changes and evaluate them before promoting revised source through Git.
The large Architect, Writer and Present applications are not library entries.

## Ask Hire to build a definition

Use a fresh solution directory outside the source checkout. Write `JOB.md`
with the outcome, input/output filenames and formats, procedure, allowed
tools, a good case, a rejection case, and the check's limits. Then:

```sh
mkdir authoring
hire build -C authoring -evidence build-records -goal-file JOB.md
hire verify authoring/expert
```

These commands use the selected PATH from setup. Hire invokes the existing
Agent runner. Its generated `authoring/expert` must be inspected before use:
`hire verify` validates structure without executing generated verifier code.
Follow `docs/BUILD-WITH-AN-LLM.md` and `tools/hire/README.md` for a full brief.

For direct authoring use `hire new expert 'JOB'`, then finish its instructions
and deliberately rejecting scaffold check. A portable expert needs `AGENTS.md`
and executable `bin/check`; additional definition files should earn their place.
Do not add another provider SDK or an expert-specific runtime.

## Run outside the build workspace

Create `case-01/` and copy the actual test inputs into it. Keep evaluation
expected answers and the definition outside worker-writable paths:

```sh
agent show -C case-01 authoring/expert
agent run -C case-01 -evidence case-01-records -turns 8 \
  authoring/expert -- 'Read the supplied case inputs and produce the deliverable defined in your expert instructions.'
```

Use the user's specific execution goal in a real run. `-goal-file FILE` can
supply it without putting private text in arguments; keep it separate from a
brief that asks Hire to build a definition. Stdin can carry evidence, or a goal when no other
goal was supplied. Read the Agent manual before changing that contract.

Evaluate the output, repair the definition or check when warranted, and run a
second case in a fresh workspace. A previously accepted output can satisfy the
pre-check without a model call. Bind checks to the current input when workspace
reuse is a requirement.

## Add capabilities, not parallel infrastructure

Private procedures go under `expert/skills/`; deterministic helpers under its
reviewed tools namespace. Agent loads memory and procedures and gives child
experts their own task/session context. Source data remains input evidence.
Do not describe a directory as confidential merely because it is outside the
workspace: Cage restricts writes/network, not host reads.

Use MCP for a tool connection and A2A for remote agent work. Add Action/May
when an external operation needs policy or a person's decision. Add Tend for
retained attempts, or Weave for actual task dependencies. Unix callers supply
process composition and scheduling; none of these additions needs a new loop.
