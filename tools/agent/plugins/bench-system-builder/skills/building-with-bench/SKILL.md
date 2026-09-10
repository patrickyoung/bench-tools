---
name: building-with-bench
description: Guides a business expert from a process through verified Bench setup, a governed Agent home, checks, and a first local run or resumable model-access gate. Use for a new Bench system or redesign.
license: MIT
metadata:
  author: patrickyoung
  version: "0.2.0"
  bench-suite: "0.13.0"
  agent: "0.2.1"
---

# Building with Bench

Turn a process the user understands into a reviewed, bounded, independently
checked Bench system. Work in plain business language. Preserve the durable
decisions in files; do not make this conversation the system of record.

This skill contains the operating knowledge needed to design the system. It
does not contain the Bench runtime and does not turn a successful model response
into proof.

Claude Code or Cowork is the authoring and control surface. The system being
built is always an Agent home executed by the pinned Bench suite's `agent` or
transparent `bench home` entry point. Never substitute a Claude-native agent,
custom loop, scheduler, configuration format, or state store.

## Route the request

Use this skill for a new or substantially redesigned system through first local
run and controlled-pilot readiness. With the complete plugin, keep one
continuous journey: call on the platform-steward procedure for an approved
runtime setup, then resume the build at the exact prior gate; call on the
operator procedure to leave a concrete runbook and disabled schedule plan. Do
not make a nontechnical user restart the tutorial or translate a handoff between
these skills. A standalone copy can build and make a first run when an exact
compatible suite is already reachable; without the steward it cannot install or
govern a missing runtime, and without the operator it leaves local operation and
scheduling as explicit handoffs. It remains design-only only when no compatible
runtime is reachable.

## Standing rules

- Preserve the user's outcome and authority. Do not enlarge the job because a
  tool or component exists.
- Begin with a narrow recommendation, read-only analysis, or draft workflow.
  Add effects only after evidence and proof are credible.
- Ask one decision-sized question at a time unless the user supplied a complete
  process pack. Explain why an answer matters when the connection is not obvious.
- Record unknowns as unknowns. Never invent policy, source precedence, expected
  answers, credentials, approvals, or business thresholds.
- Never read or print secret values. Report only whether a credential capability
  exists, its resource, account identity, and scope.
- Preflight is read-only. After it, do not install, authenticate, widen
  permissions, activate a schedule, execute an effect, or alter an existing
  Agent without a bounded plan and the approval that owns that exact change.
- Keep withheld cases and their expected answers outside every worker-readable
  root. A hidden label inside the Agent home is not a holdout.
- Treat a toolbox as tool selection, Cage as a current write/network boundary,
  and the Claude surface as an outer interface. None is by itself a complete
  confidentiality, identity, credential, or truth boundary.
- A model may propose work. Only an executable external check and retained
  evidence may accept it.

## 1. Establish the workspace and execution lane

Before creating or changing artifacts, read
[compatibility.md](references/compatibility.md) and
[surface-routing.md](references/surface-routing.md). Perform only their
read-only preflight.

Classify the session exactly:

- **BUILD-READY** — a compatible pinned suite or managed runtime is reachable,
  a dedicated workspace exists, and the user has authorized build work.
- **DESIGN-ONLY** — the skill can create the complete business design and case
  package, but no compatible Bench runtime is reachable.
- **STEWARD-REQUIRED** — runtime installation/governance, a meaningful verifier,
  a connector/effect boundary, or another authority decision prevents safe
  build work and requires a named platform/security owner.

State the classification, supporting observations, and claim boundary. Local
files connected to Cowork do not imply access to a host-installed binary.
Report **MODEL-READY** or **MODEL-ACCESS-REQUIRED** as an orthogonal milestone;
a BUILD-READY lane may still be blocked at its first model-backed gate.

When a local Claude Code shell is supported but the exact suite is absent, do
not strand the user at **STEWARD-REQUIRED**. Invoke the
`stewarding-bench-platform` procedure and its bundled pinned setup helper. Run
the helper's read-only plan, present its exact target, hash, prefix, writes,
collision behavior, and rollback, then ask for one installation decision. If
approved, install and verify with that helper, rerun this skill's
`doctor.sh --prefix ABS`, review and approve the steward helper's transient
`cage-check`, and automatically resume here. The doctor proves exact suite
bytes and Cage backend discovery; only a literal 13/13 target-host Cage check
earns **SUITE-READY**. If a nested Claude/Cowork sandbox blocks it, report
**CAGE-HOST-CHECK-REQUIRED** with the exact host-terminal command and saved
resume gate. Never do this in a Cowork cloud task or on an unsupported host.
Never turn a failed prebuilt install into an implicit source build.

Immediately after suite setup, establish a separate model-access milestone
before any Bench contract, `draft new` with a description, `draft build`,
`draft prove`, or Agent run that can call Ask. Read
[model-readiness.md](references/model-readiness.md) and use its bundled
read-only-plan/approved-probe helper; a Claude Code or Cowork login is not
evidence that Ask can reach a provider. Report this progression explicitly:

- **SUITE-READY** — the exact suite, all public commands and internal checksums
  are verified, and Cage passed 13/13 on the target host. `cage status` alone is
  only **SUITE-INSTALLED**. This makes no model-access or Agent-result claim.
- **MODEL-READY** — the non-secret provider/model identifier and approved
  controller-owned access complete the approved, schema-constrained Ask probe
  in this execution lane without exposing a credential value. Interactive
  readiness does not prove fresh-scheduler readiness.
- **AGENT-FIRST-RUN** — after home proof, one bounded fixture invocation through
  `bench home run` has a literal recorded terminal result.

If model access is unavailable, keep **SUITE-READY** (or the narrower
**SUITE-INSTALLED** / **CAGE-HOST-CHECK-REQUIRED** state actually proved), record
**MODEL-ACCESS-REQUIRED** rather than calling the setup a failure, and continue
only work that does not invoke the Bench model boundary. Put the access owner,
non-secret model identifier, earliest blocked gate, and exact command to resume
in `STEWARD-HANDOFF.md` and `FIRST-RUN-REPORT.md`; do not request a credential
value in chat. When the owner supplies an approved boundary, prove
**MODEL-READY** and automatically resume at that saved command instead of
restarting the interview.

## 2. Build the business process pack

Read [lifecycle.md](references/lifecycle.md). Copy the relevant templates from
`assets/` into a new `bench-system/` directory; do not overwrite existing work.
Interview the accountable process owner and fill the artifacts in this order:

1. `00-job-brief.md` — trigger, finished result, owner, boundaries, escalation.
2. `01-process-map.md` — decisions, normal path, exceptions, stop conditions.
3. `02-case-library.csv` — teaching, regression, edge, adversarial, and withheld
   case identifiers without exposing withheld answers to the worker.
4. `03-source-register.csv` — resource, owner, authority, freshness, conflict
   order, sensitivity, and failure behavior.
5. `04-authority-register.csv` — read, recommend, draft, propose, affect, and
   forbidden capabilities with approval and uncertainty handling.

Use contrastive examples: ask why one near-neighbor is acceptable and another
is not. Convert adjectives such as “good,” “urgent,” or “appropriate” into
observable rules, examples, or explicit human judgment.

Stop for owner review. Do not proceed while the owner, outcome, source conflict
order, high-risk boundaries, or success evidence is unresolved.

## 3. Design the Bench composition

Read [platform-owners.md](references/platform-owners.md). Fill
`05-system-design.md` by assigning each required guarantee to one named owner.
Use the smallest composition that preserves the guarantees; “full platform”
means no responsibility is silently assigned to the model, not that every
component runs on every turn.

The ordinary promotion path is:

`Bench contract → Draft design → Build → Prove → Agent home → controlled pilot`

Document why every selected component is needed and why every unselected one is
not needed yet. Keep deterministic transformation, model judgment, evidence
retrieval, effect execution, approval, supervision, and acceptance distinct.

## 4. Define proof before model work

Read [evidence-and-proof.md](references/evidence-and-proof.md). Fill
`06-proof-plan.md` with:

- mechanical invariants and the exact checker that owns each one;
- withheld business cases and their external oracle;
- calibrated human-review rubric for judgment that cannot be mechanical;
- false-success, escalation, stale/conflicting-source, prompt-injection, and
  mutation cases;
- pass thresholds, budgets, stop rules, and regression policy;
- what each check proves and what it does not prove.

Reject constant checks, checks controlled by the worker, placeholder goals,
structure-only checks presented as business quality, and evaluation on examples
the worker can read. The author of a candidate result must not be its sole judge.

## 5. Review the contract and authority

Read [authority-and-effects.md](references/authority-and-effects.md). Present a
plain-language review containing:

- what the system owes and explicitly does not own;
- the evidence it may trust and how conflicts fail;
- what it can read, write, propose, and affect;
- where human decisions remain mandatory;
- budgets for time, turns, cost, and effect count;
- behavior for denied, timed-out, interrupted, and effect-unknown work.

Record the approved result in `05-system-design.md`. A conversational “yes” is
not permission to install, authenticate, publish, pay, delete, schedule, or
widen authority.

## 6. Materialize only in a build-ready lane

If the lane is not **BUILD-READY** and the continuous local setup path above is
not available or was declined, read [steward-handoff.md](references/steward-handoff.md),
create `bench-system/STEWARD-HANDOFF.md`, and stop after the design package. Say
precisely which claim remains unproved and preserve the gate from which a later
session should resume.

If it is **BUILD-READY**, read [suite-commands.md](references/suite-commands.md)
before running anything. Use the compatible runtime's public commands; never
read source as an undocumented substitute. Show the user the proposed build
sequence and obtain approval before writes or model-backed work.

Materialize in gates:

1. Negotiate and admit the exact Bench contract in Review mode.
2. Create and review the Draft `DESIGN.md` and its executable check.
3. Run `draft check`, explicitly admit the verifier, then build.
4. Run `draft prove` and inspect what the check fails to detect.
5. Read [home-materialization.md](references/home-materialization.md), create
   the home with `bench home new`, and map the reviewed design into the public
   Agent-home files. Keep governed definition, mutable work/state, executable
   check, and controller evidence in separate write domains.
6. Run `bench home check` and inspect `bench home show` before the first
   model-backed run. A fresh scaffold is unfinished, not run-ready.
7. Prove the check and optional wake gate with fixtures, network off, and no
   external effects.
8. Require the saved **MODEL-READY** gate, then make one bounded fixture run
   through `bench home run`. Fill `FIRST-RUN-REPORT.md` with the literal status
   and evidence. A provider-access failure returns to the resumable
   **MODEL-ACCESS-REQUIRED** gate; only the recorded invocation earns
   **AGENT-FIRST-RUN**.
9. Use the operator procedure to create a concrete local runbook for
   interactive, distinct on-demand, named-resume, tick, history, pause, and
   retirement. Render—but do not activate—host scheduling when requested.

Record commands, versions, exit statuses, and evidence paths in
`BUILD-REPORT.md`. Do not translate a nonzero or unknown result into success.

## 7. Evaluate and prepare a controlled pilot

Run the external case harness, not the Agent's self-description. Compare results
with the withheld oracle outside worker-readable roots. Record every case as
pass, fail, or unknown with evidence.

Fill `07-pilot-plan.md`. Begin with simulation, shadow, or draft-only work. Set a
bounded population, volume, duration, review cadence, escalation threshold,
pause rule, rollback, owner, and retirement path. Widen one authority boundary
at a time only after rerunning the complete regression set.

End with one readiness state:

- **DESIGN COMPLETE** — business artifacts are reviewed; runtime proof absent.
- **STRUCTURE VALID** — a scaffold and offline structural checks pass.
- **CHECK PROVED** — Draft mutation work demonstrates meaningful detection.
- **PILOT READY** — withheld outcomes, authority review, and operating controls
  meet their admitted thresholds.

Never use “production ready” for a scaffold or a passing structure check.

## User-visible handoff

Summarize:

1. the current lane and readiness state;
2. the exact artifacts created or reviewed;
3. the strongest evidence and largest unknown;
4. granted and explicitly absent authority;
5. the smallest next decision and its accountable owner.

## Additional references

- [worked-example.md](references/worked-example.md) — use only when the user
  needs a concrete invoice-exception example.
- [steward-handoff.md](references/steward-handoff.md) — required format when the
  build cannot continue safely.
- [home-materialization.md](references/home-materialization.md) — reviewed
  Draft-to-Agent mapping, proof gates, and first local run.
- [model-readiness.md](references/model-readiness.md) — bounded Ask probe,
  evidence, exact resume gate, and claim limits.

## Bundled resources

- [doctor.sh](scripts/doctor.sh) — inspect before running; offline, read-only
  suite-integrity, exact component-version, and Cage-status preflight. It
  accepts a prefix, run-in-place suite, explicit Bench path, or PATH selection.
- [probe-model-access.sh](scripts/probe-model-access.sh) — read-only plan and one
  approved, timeout-bounded, schema-constrained Ask call that records
  **MODEL-READY** or **MODEL-ACCESS-REQUIRED** without retaining provider errors.
- [00-job-brief.md](assets/00-job-brief.md) and
  [01-process-map.md](assets/01-process-map.md) — owner interview artifacts.
- [02-case-library.csv](assets/02-case-library.csv),
  [03-source-register.csv](assets/03-source-register.csv), and
  [04-authority-register.csv](assets/04-authority-register.csv) — case,
  evidence, and capability registers.
- [05-system-design.md](assets/05-system-design.md),
  [06-proof-plan.md](assets/06-proof-plan.md), and
  [07-pilot-plan.md](assets/07-pilot-plan.md) — reviewed architecture, proof,
  and pilot templates.
- [BUILD-REPORT.md](assets/BUILD-REPORT.md) and
  [STEWARD-HANDOFF.md](assets/STEWARD-HANDOFF.md) — terminal build or blocked
  handoff records.
- [FIRST-RUN-REPORT.md](assets/FIRST-RUN-REPORT.md) — separate runtime, home,
  model-access, check, and first-result claims.
