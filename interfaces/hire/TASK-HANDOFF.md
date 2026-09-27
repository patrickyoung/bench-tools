# Work conversation team handoff

The UI invokes a team's `bin/task`; the adapter translates the selected team's
existing input, output and member contracts. It does not replace Agent's loop,
checkpoints or confinement, nor introduce a shared team scheduler. Command exit
status, a valid member report and completion of the user's goal are separate
observations.

A member may correctly return a structurally valid blocked report with exit 0.
The adapter must retain that meaning and the available partial work. Conversely,
an accepted member result does not establish team acceptance or goal completion.
Independent review remains necessary where the selected team requires it.

## Invocation and result

The capability declaration `task-runtime.json` is exactly:

```json
{"version":2,"resume":true,"local_only":true}
```

The controller supplies the existing `BENCH_TASK_FILE` prose goal,
`BENCH_TASK_WORK` working directory and optional `BENCH_TASK_RESUME=1`, plus:

| Variable | Binding |
| --- | --- |
| `BENCH_TASK_INVOCATION` | Unique controller identity for this invocation |
| `BENCH_TASK_GOAL_SHA256` | Digest of the selected goal bytes |
| `BENCH_TASK_DEFINITION_SHA256` | Controller digest of the complete selected definition |

The adapter atomically writes `BENCH_TASK_WORK/task-result.json` before a normal
return. All fields below are required; inactive text fields contain an empty
string. No unknown or duplicate fields are accepted.

```json
{
  "version": 1,
  "invocation": "<controller identity>",
  "goal_sha256": "<controller digest>",
  "definition_sha256": "<controller digest>",
  "status": "blocked",
  "stage": "Review deployment package",
  "message": "Source corrections and local checks are saved.",
  "reason": "The selected environment does not provide the required image builder.",
  "next": "Provide the required build environment before image validation.",
  "question": "",
  "files": [
    {"path": ".team/developer/app/config.py", "kind": "work", "sha256": "<digest>"},
    {"path": ".team/developer/checks.txt", "kind": "evidence", "sha256": "<digest>"}
  ]
}
```

Text is bounded plain display data, never executable commands or permission.
The report is at most 64 KiB: stage is at most 120 characters, message/reason/next
at most 2,000 each, and question at most 1,000. The manifest has at most 64 files,
25 MiB per file and 100 MiB total. At most 32 deliverables are supported; their
basenames must be distinct and use the UI’s supported artifact formats. Nested
source can be returned in an editable archive. The controller flattens declared
deliverables for download while preserving nested work/evidence in its snapshot.
File paths are unique relative paths under the selected working directory,
without parent/dot components, symlink components or nonregular files. Digests
are lowercase SHA-256. Controller file-count and byte limits apply; over-limit,
missing, stale or invalid files reject the report rather than silently removing
evidence. The report cannot name itself.

File kinds keep different observations separate:

| Kind | Meaning |
| --- | --- |
| `work` | Current source or candidate content, including partial work; used to observe progress |
| `evidence` | Native handoffs, reports and check receipts needed to understand that content |
| `deliverable` | Files offered for return to the user and final acceptance |

New transcript bytes, timestamps, request text or reworded summaries do not
establish progress. Keep them out of the work manifest. Selecting and hashing
evidence proves which bytes were observed, not that its claims are true.

## Outcome transitions

The observed public process outcome always takes precedence. Only confirmed
exit 0 or 2 may authorize automatic continuation. A question may accompany exit
75. Other exits retain their exact status; a diagnostic report cannot convert
failure, interruption or an unknown outcome into permission to retry.

| Report status | Required meaning |
| --- | --- |
| `complete` | Exit 0, team acceptance passed, result offered for independent goal review; reason, next and question empty |
| `continue` | Exit 0 or 2, concrete authorized work in next, empty reason/question; existing checkpoints can safely execute it |
| `blocked` | Any observed exit, concrete reason and remedy in next, empty question; no automatic replay |
| `needs_input` | Exit 0, 2 or 75, essential missing user fact in question, empty reason/next; no invented approval requirement |

The controller validates the report against its invocation and exact observed
outcome, then freezes the selected files before completion review. A capability
marker is not proof of this behavior. Missing, stale or malformed reports are
handoff failures, not generic requests to keep trying. No completed report can
override the independent final goal review.

Readiness for review is separate from readiness for release. If the producer
has usable candidate bytes and the remaining local work is an independent
review, pass those bytes and their supporting evidence to the reviewer. Do not
require a producer to claim that the review already happened. Unavailable
external checks remain explicit blockers; routing a review does not waive them.

## Protected member inputs

The UI enables `AGENT_PROTECT_INPUTS=1` for team execution. The selected runner
must preserve this setting and supply Agent/Cage versions supporting protected
inputs. Agent selects existing `inputs/` and `request.md` beneath each member's
work directory before invocation. Both model actions and checks receive a kernel
write boundary that denies changes to those selected inputs. Checks may write
only to their selected work/state and private temporary directory; networking
follows the member's explicit selection. This is an opt-in narrowing of the
ordinary Agent check boundary.

Stage inputs before launching the member. Workers copy or unpack into their own
writable output/scratch paths; they do not reorganize authoritative staged inputs.
Use explicit Agent `-read-only PATH` for other native input layouts. Preserve
selected input bytes and contract bindings during corrections. Do not disable
protection to make a worker or check pass.

This protection covers Agent member actions and checks. The enclosing team
coordinator still owns staging and must preserve its inputs; this does not claim
an immutable store against the coordinator or validate review quality. Missing
historical inputs do not gain valid provenance merely by being restored.

## Member continuity

Before launching a member, persist a started receipt naming its assignment,
checkpoint and full definition/input bindings. After return, persist the exact
observed exit and accepted output hashes. Record member and check outcomes
separately. A normal rejection under the native check contract may permit a
correction; a broken, interrupted or unknown check does not authorize replay,
even when the member process itself succeeded. A missing or started-only receipt is
unknown, even if an earlier invocation succeeded. Never overwrite the sole
record of an uncertain attempt with an older accepted result. Before any reuse,
correction or acceptance decision, validate contiguous attempt history and require
the latest receipt to agree with its newest record. Missing pointers, gaps or
contradictory history are uncertainty, not first use; they must not launch members
or license downstream work. The same validation applies to retained reviews.

Retain explicit checkpoints from the first invocation, the original edit
baseline and current partial work. Resume only observed 0/2 outcomes with a
specific correction. A correction requiring work uses Agent `-B` even if a
structural precheck already passes. Reuse requires unchanged selected inputs,
full definition including executable modes, and accepted output bytes. A changed
producer invalidates downstream acceptance. Stage the current request, selected
source and every referenced review input together; source lineage and supporting
evidence cannot be replaced by a summary.

## Conformance and migration

New and adapted UI team entries use version 2. A retained version 1 declaration
does not establish version 2 compatibility. Adapt its UI entry through Hire
before new execution, preserving the roster, native contracts and immutable
reused members. Retained uncertain attempts remain uncertain; migration does
not authorize their replay.

Offline executable fixtures must cover actual adapter behavior with stub public
member commands: partial continuation, blocked-with-exit-0, correction after a
passing precheck, changed producer/reviewer bindings, missing capabilities,
unchanged content with changed prose, stale hashes, unknown/interrupted exits,
termination between started and observed receipts, and missing/stale latest
receipts while numbered attempt history remains. Assert no unsafe replay,
exact outcomes, current report bindings and preserved partial/editable inputs.

Structural verification does not execute generated checks. Any executable
conformance run is separately admitted under the selected execution boundary,
without live models or external effects. Author-supplied fixtures help test native
schemas, but cannot certify themselves. Independent controller rejection tests
and public-executable integration tests establish the UI boundary; specialist
quality and actual goal completion need their own evidence.
