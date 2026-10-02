# Trait

Provide one Unix application for authoring and evaluating complete Bench
capabilities while preserving the independent programs that perform the work.

## Interface and defaults

```text
trait create|edit|train|eval [OPTIONS] INPUT_DIR NEW_OUTPUT_DIR
```

The caller supplies the outcome and selected source. Flags follow the action
and precede directories. A request override replaces admitted REQUEST.md
without editing input. Defaults are 12 worker turns and five minutes per
subprocess; the latter includes an entire controller entry. These are neither
a dollar cap nor a total case-suite deadline.
The model and effort flags are -m, -turns, and -timeout. Caller-selected
-entry-boundary host|cage controls a controller entry only, defaulting to host.

Create starts with a request; edit/train also need expert/. Eval needs expert/
and can obtain its goals from cases or generate cases from the documented
capability. Cases, capability assessment, and entry selection are optional
caller overrides. Missing cases or assessment are authored automatically.
Absent train.json selects knowledge/private; an explicit file must specify
valid kind and scope. Recovery additionally names the original session and
destination skill.

A prior authored result can be selected directly. Reuse its expert and saved
evaluation recipe while excluding old case outputs. New requests use -request.
Every action gets fresh disjoint output and private storage, never implicit
resume, in-place source amendment, terminal chat, or library publication.
Teaching from a prior result preserves the existing suite and adds a fresh
transfer case for the new request before amending the worker.

## Compose the complete capability

| Stage | Existing mechanism | Claim |
| --- | --- | --- |
| Author create/edit | Hire's embedded creator via Agent | A candidate addresses the requested capability. |
| Structural verification | Hire verify / Agent check | Definition structure; generated code is not executed. |
| Prepare missing evaluation | Separate Hire invocation | Model-authored cases and capability assessment. |
| Inspect exact selection | Bounded Ask schema decision | Automated source review, not human approval or safety proof. |
| Execute one case | Agent or selected external entry | Observed process outcome and artifacts. |
| Check frozen output | Record and read-only Cage | Only the coverage of the selected check. |

Hire may author instructions, Brief skills, deterministic helpers/native
programs, checks, and assembled specialists using existing public commands.
CAPABILITIES.md accounts for these categories, dependencies, and real entry and
handoff contracts, including categories that are unnecessary. Available
compilers may build source; missing host dependencies are reported rather than
installed. Neither adjacency nor a capability claim grants authority.

The evaluation author receives an expert copy and selected material. It must
preserve that expert and supplied evaluation files exactly. Generated cases
include transfer and regression examples with convincing rejection criteria.
Generated labels are not independent domain truth. Retain CASES.md and actual
process evidence separately from acceptance claims.

Before case execution, Ask reviews exact definition and evaluation inventories,
capabilities, and entry selection. Require an approval decision and reason,
retain the review, and recheck identities. A negative or invalid decision stops
evaluation. The source review does not certify behavior or create permission.

## Execution selection and authority

Without evaluation/run.json, run Agent with fresh work, state, evidence, and -B.
A controller recipe selects one executable relative to expert/, literal args,
and required command names. Whole-token substitutions provide goal-file, work,
state, child-evidence, and expert paths. Goal bytes also go to stdin. Metadata
cannot supply new write roots, networking flags, or host companion selectors.

The selected entry owns composition. Thin adapters preserve existing team
argv/stdin/environment contracts. Weave remains readiness selection, Tend
remains execution state, and an existing team controller retains scheduling.
Trait invokes a single process, adding no provider client, action loop,
topology language, or mandatory broker.

Run controllers through Record. Default host mode preserves existing team
and child-Agent compatibility, with ordinary host write/read/network authority.
Directory separation and postchecks do not protect source or evidence against
such a controller. Model review is advisory inspection, never confinement.

An explicit -entry-boundary cage adds Cage -net with fixed work, state,
child-evidence, and private temporary write roots. Keep Trait source, frozen
cases, outer records, and publication destinations outside those grants.
Unavailable confinement or unsupported nested sandboxing returns 125 without
falling back to host mode. This still permits full networking and unrestricted
host reads. The normal child Agent Cage is not enforceable against a controller
that selects another invocation; the optional outer grant is the actual write
boundary. Child records remain controller-writable in both modes. Do not call
them independent Trait evidence or claim credential isolation.

After execution, take a regular-file work snapshot in separate Trait storage,
outside the write roots in cage mode. This is not access isolation from a host
controller or its descendants; host mode cannot promise tamper-proof snapshots.
Run the independent check from the snapshot, with no argv and TRAIT_CASE_DIR
pointing to its frozen case. Record includes all case files, expected data,
and declared outputs. Check Cage denies networking and workspace writes while
allowing a private temporary directory. Exit 0 accepts, 1 rejects, and another
status means broken. Publish the captured bytes, not mutable execution work
read again after checking. Report the selected boundary and the limits of
separate evidence and source/hash checks, including change-and-restore races.

Ordinary Agent/Ply expert verifiers retain their host execution contract;
the independent case check is distinct. Host reads mean expectations are
visible regression data, not hidden cases. Use another access boundary for
withheld tests or confidentiality.

## Teaching and retention

Train freezes transfer/regression criteria before teaching. Use Hire for
selected knowledge, Hone for qualifying failed-then-accepted recoveries.
Private scope keeps facts in the selected specialization but grants no secrecy
or publication rights. Hone preparation, model review, and admission bind exact
proposal bytes. A no-lesson outcome changes nothing and never falls through to
another route.

Teaching changes only nonexecutable Markdown. Preserve all other files and all
executables, including executable Markdown, by path, mode, and bytes. Existing
runtime entry selection and frozen cases remain fixed. Code or checker changes
require edit. The change boundary preserves executable bytes, not a proof of
unchanged verifier semantics when code reads amended Markdown.

Only accepted fresh cases justify retained_on_cases. Structural validity,
procedure discovery, prior artifacts, or authoring prose do not demonstrate
retention. Case acceptance does not prove improvement over the original worker
or causal use of a changed skill. Link the actual execution evidence and its
trust limitations. No model weights or global library memory change.

## Artifacts and outcomes

Private admitted source, evaluation, authoring, work, state, and records live
under XDG state in trait/runs/RANDOM. Definition, admitted source, and evaluation
hashes are checked after child execution and independent checking. Drift stops
with 125; evaluated_sha256 identifies the selected definition.

Outputs contain valid authored expert/ (absent for eval), selected request,
CAPABILITIES.md, REVIEW.md, reusable evaluation/, checked cases/ID/work/,
result.json, and EVALUATION.md as stages complete. The same final JSON goes
to stdout; progress goes to stderr. Candidate availability does not imply
case acceptance. Record supplied/generated evaluation origin and actual
worker/controller/check outcomes.

Preserve child results including 2 unfinished, 3 declined, 75 waiting,
125 uncertain/boundary failure, and 130 interrupted. Independent rejection and
review rejection return 2; broken checks preserve their status. Usage is 2,
local failure 1, subprocess timeout 124, missing command 127. Do not retry
automatically or turn unknown effects into success.

The core builds independently. Root scripts/build trait bundle creates verified
packages; scripts/package-trait --runtime PREFIX --output FILE selects them
through .build/bin or --bin-dir and uses Bundle's argv transport. Build tools,
provider credentials, Cage backend, and authored-capability prerequisites have
separate responsibilities; packaging does not establish runtime availability.

## Verification

Cover automatic preparation, unchanged expert/criteria, prior-output admission,
strict explicit settings, literal entry substitutions, executable identity,
model review rejection and drift, nested Agent plus deterministic helpers,
write-boundary refusal, frozen-output acceptance, case origin, cancellation,
and exact status propagation. Run independent Go tests, race tests, vet, and
public-command loopback integrations. Live semantic/retention trials remain
explicit and separately evidenced.

## Expected refusal cases

Case metadata may set expected_exit to 2 for a known refusal; zero is the
default. Other statuses are invalid. Refusal cases may declare no regular output
files, but still need an independent check of the concrete disposition and no
false completion. A matching status runs that check; unexpected success rejects.
Trait records actual/expected status in execution.json outside case work and
binds it into the check receipt. Check environment supplies that file and the
controller state/evidence paths without granting writes. Original child status
is preserved even when the refusal test passes. Host-mode trust limits apply.
