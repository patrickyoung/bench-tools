# Build the solution with the existing runner

Establish the actual inputs, expected deliverable, workspace, permitted effects,
limits, and acceptance evidence. Infer routine choices from the request and
sample files; ask only when a missing decision changes the result or authority.
Turn vague quality words into examples, mechanical checks, or an explicit
review rubric. Do not invent policy or expected facts.

For meaning, evidence fidelity or visual quality that deterministic checks
cannot establish, follow [semantic checks](semantic-checks.md). Give Hire the
rubric, backend choice, evidence boundary and acceptance policy; Weigh is
optional, and an Ask-only checker remains a valid composition.

For choosing a repair from check findings, give Hire an explicit allowed action
menu, selector model, separately selected repair runner and action limits. Use
rules for established mechanical fixes and optional `select-fix.py` from
`tools/weigh/examples/improve-checks/` for semantic choices. The caller dispatches
the selected ID through existing commands and always runs its independent final
check, including when no change was selected. See [improve](improve.md) for the
snapshot contract, evidence and evaluation; this adds no worker-specific loop.

## Find or assemble first

Follow [library discovery](library.md) before writing a new definition. Select
an existing worker, assembled team or useful source to adapt. An unchanged
suitable export runs directly. Read `docs/BUILD-WITH-AN-LLM.md` in the selected
checkout for the human walkthrough; `docs/WORKER-LIBRARY.md` owns packaging.

When adapting a team, give Hire the clean assembled authoring directory and
the desired roster, handoffs and acceptance. Reuse existing worker sources and
command adapters. Promote changes back to the owning `workers/ID` or `teams/ID`
entry after evaluation. The original lock still identifies the starting source;
record later edits separately. Do not promote a whole working directory.

For a teaching example only, `examples/support-reply` includes explicit practice
inputs and a complete expert. Examples are outside the clean source catalog;
never silently copy their sample content into a user's worker. The large
Architect, Writer and Present applications remain outside this library.

## Ask Hire to build a definition

Use a fresh solution directory outside the source checkout. Write `JOB.md`
with the outcome, input/output filenames and formats, procedure, allowed
tools, a good case, a rejection case, and the check's limits. Keep those cases
and authoring records outside the reusable definition. Then:

```sh
mkdir authoring
hire build -C authoring -evidence build-records -goal-file JOB.md
hire verify authoring/expert
```

These commands use the selected PATH from setup. Hire invokes the existing
Agent runner. Its generated `authoring/expert` must be inspected before use:
`hire verify` validates structure without executing generated verifier code.
Follow `docs/BUILD-WITH-AN-LLM.md` and `tools/hire/README.md` for a full brief.

For direct authoring use `hire new expert 'JOB'`, add its required README and
finish the instructions and deliberately rejecting scaffold check before
`hire verify`. A portable expert needs `AGENTS.md`
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
