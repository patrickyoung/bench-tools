# Replay across the tool inventory

Goal: the replay system must work with every Bench tool while preserving
independent Unix programs. Record supplies an optional process boundary; Ask
owns the sealed history, and Trail reads it. The required integration gate
records and replays meaningful invocations of every public command.

Replay means reconstructing and verifying the recorded execution offline.
Replaying a receipt must never rerun an external action, repeat an approval,
refresh a credential, or contact a model. Re-execution is a separate explicit
operation whose results may differ. This extends Ask's existing invariant:
the model conversation can be reconstructed exactly from its retained events.

For a visual browser, use [Bench Trace](../examples/event-browser/README.md).
Its single Python file reads one or several Agent evidence roots or Ask/Record
sessions, follows new events, displays recorded agent relationships and process
streams, and exports a self-contained offline HTML replay. It composes the
public read-only replay commands and never re-executes archived work.

## Completion requirements

1. Every command in `components.json` has a documented recording path and
   executable coverage of its meaningful boundary, not only `help` or
   `version`. New inventory entries must fail the coverage gate until mapped.
2. Deterministic work can create replayable history without a model call.
3. An ordinary executable can be recorded with literal argv, selected
   executable identity, physical working directory, and binary-safe stdin,
   stdout, stderr, and terminal outcome. stdout and stderr remain separate.
   Per-stream byte order is preserved; an observation order across pipes is
   not a claim about the child's internal write timing.
4. Presentation limits may shorten what a model sees, but must not silently
   shorten retained evidence. A recording failure or missing terminal record
   is explicitly incomplete. A valid prefix seal alone does not prove a
   completed run or the absence of an effect.
5. Recording preserves each tool's stream and exit semantics, including
   negative results, parked work, uncertain effects, signals, and startup
   failures. Evidence storage failure must not masquerade as tool success.
6. Capture starts before execution can occur. A crash at an ambiguous point
   must remain ambiguous; missing evidence is never a grant to retry.
7. Selected input/output artifacts and child-session links are retained with
   their bytes and identity. Replay must work after the original source files
   and live service are unavailable. Unobserved filesystem/network effects
   are explicitly outside a process receipt's evidence.
8. Credential handoffs remain on their existing private descriptor boundary.
   Private descriptor bytes and ambient credential values do not enter replay. Record the
   non-secret resource/identity context when supplied, not reusable authority.
9. Ask owns events, locks, folds, and seals. Tools compose its public commands;
   they never import its internals or implement their own model transcript.
   Trail remains read-only. Independent packaging and builds must pass.
10. Verify old records and existing pipelines as well as new recordings.
    Tests must cover corrupted/missing/reordered evidence, binary and large
    output, interrupted execution, unchanged live streams, and no effects
    during replay. A broad green build is not a substitute for these cases.

## Current inventory and evidence boundaries

The inventory contains 24 components and 28 executables. Wrap the selected
command with `record run -f FILE -- COMMAND ...`. Select relevant files with
`-input`, `-output`, and `-session`; process streams alone do not retain files
or discover nested conversations. This table identifies those boundaries.

| Component / executables | Meaningful boundary to record and replay |
| --- | --- |
| Ask | Exact model input, normalized request, response, sourced messages, sealed notes; model-free initialization |
| Weigh | Explicit state/questions on stdin, requested/reported model, validated judgments/distributions and process outcome; private authorization stays outside streams |
| Ply | Selected script/interpreter, action streams and outcome, verifier input/result, approval/confinement receipts, compaction/child-session links |
| Context | Exact query, source records, executable fingerprint, no-result and failed-retrieval outcomes |
| Cite | Selected evidence bytes, exact candidate, unchanged acceptance or empty rejected output and status |
| Brief | Selected procedure bytes; deterministic search input/result; model selector session when used |
| Rules | Instruction discovery inputs, selected file bytes and ordering, result stream |
| Improve | Frozen source/cases, literal proposer/trial/judge commands, paired observations, decisions and exact proposal; each command has a Record receipt |
| Hone | Verified source sessions, command or bound changed-content recovery, proposed lesson, accepted skill revision and delta |
| Agent | Definition/workspace selection, assembled context, child sessions, amendments and external-action receipts |
| Hire | Builder session, generated definition files, validation result, explicit updates |
| Draft | Intent and verifier definition, admission evidence, output artifacts and Ask/Ply histories |
| Action | Proposal/decision/attempt/sent/result sequence, exact request and captured streams; unknown effects remain unknown |
| May | Exact proposal digest and observed decision; recording/replay never spends another grant |
| Cage | Selected command and confinement policy, observed exit/failure; no claim of replaying kernel enforcement |
| MCP: mcp, mcp-legacy, mcpbox, mcpserve | Exact request/result, structured/binary payloads, admission descriptors, protocol stream and uncertain outcomes |
| A2A: a2a, a2aserve | Exact request/result, task/context handles, retained artifacts and local execution links; no implicit remote retries |
| Moniker | Selected registry and theme, reserved ID/name/slug and exit status; replay emits the observed name without reserving another |
| OAuth | Resource-bound non-secret profile/decision and child outcome; header descriptor stays private |
| Tend | Job definition/input, attempts, separate output artifacts, durable transitions, checks and unknown outcomes |
| Weave | Exact task and observation inputs, projection result, caller's execution links |
| Trail | Selected immutable archive snapshot and read-only verification/projection result |
| Record | Own process receipt semantics, full binary stream retention, explicit artifact/session snapshots, and offline extraction |

## Preserve existing responsibilities

- Ask now has explicit model-free `ask init -f FILE` and streaming typed
  notes with durable sequence acknowledgements. Prefix hashing is incremental
  and retains the existing canonical JSONL seal format.
- Ply's command runner combines stdout and stderr in `capBuf`, and elides
  the middle before recording its model-visible observation. That cannot
  recover original streams by itself. The optional Record interpreter adapter
  retains them separately, while preserving Ply's existing presentation and
  verifier rules.
- Context, Cite, Rules, Weave, and other direct filters have ordinary stream
  contracts. Record wraps them without adding history code to each filter.
- Action and Tend already retain valuable process/effect evidence, but
  neither is a generic, transparent recorder for the complete tool inventory.
- The required replay inventory gate builds on the existing semantic
  integration suite. Each recorded invocation retains its selected executable
  path, and replay must reproduce its live stdout, stderr, and exit status.
  Replacing binaries on PATH would change identity checks; the gate wraps
  invocations externally instead.

## Recording and verification

[Record](../tools/record/README.md) is an independent Go command. It wraps
literal argv, streams and retains separate binary input/output, records
executable identity and status, snapshots selected regular files and child
sessions, and replays only after Ask and receipt validation. No environment
values or private descriptor contents are captured. Observed input and the
prefix delivered to the child pipe are distinguished explicitly.

The required public Ask/Record fixture covers ordinary, negative, parked,
uncertain, signaled, and startup outcomes; binary output larger than Ask's
single-note limit; incomplete/corrupt/reordered records; bidirectional input;
selected files and child sessions after their originals are deleted; and a
private descriptor. Replay is checked not to repeat an observed file effect.
Tests recompute canonical prefix digests independently across reopen to ensure
the Ask optimization did not change existing seals.

The [Ply interpreter adapter](../tools/record/examples/README.md) uses Ply's
existing public `-shell` interface. Its executable fixture verifies that full
separate streams survive a smaller model presentation cap, while the original
Ply conversation still replay-checks. No Ply production code is changed.

The required inventory gate maps every command in `components.json` to a
meaningful case. A missing mapping or a command with no completed case fails.
Help, version, and capability-list output do not count. It retains a coverage
report and the complete recordings under the check run's `replay/` directory.

Additional cases force recording failure before intent, while capturing
output, and at terminal commit; kill the recorder after sealed output; close
the downstream pipe; and leave output descriptors open after both zero and
nonzero exits. None may claim complete replay. Timeout and literal non-UTF-8
argv cases preserve the observed invocation and outcome. An older Ask is
rejected through a safe feature probe before `init` could become a model call.

The OAuth fixture verifies the real private-descriptor handoff, retains only
non-secret profile/child-program snapshots, deletes the fixture credential
store, and replays successfully without it. Arbitrary credentials explicitly
placed in argv, public streams, or selected files would be retained like any
other bytes; use the private descriptor boundary, never `oauth header` as a
recorded public stream.

The compaction fixture creates a real model session and an explicit summary
through a loopback provider. It snapshots all three related sessions, deletes
the originals, extracts identical bytes, and verifies every session and both
lineage links through Ask and Trail after the provider has stopped.

## Use and limits

Keep a transparent process recorder separate from each tool. It invokes
literal argv, retains observations as sealed Ask notes, and preserves the
child's streams and outcome. It can wrap a deterministic filter, a model
client, an authenticated child, or a finite service session. It must support
streaming stdin; waiting for EOF before starting a bidirectional MCP server
would deadlock.

Use a dedicated recording session when a child owns its own Ask conversation.
Link those sessions explicitly instead of introducing competing writers to
one log. Selected file snapshots and typed process events complement the
existing model transcript. A recorder does not become a router, approval
authority, automatic retry loop, or shared runtime.

Run the required checks from the repository root:

```sh
python3 scripts/check
```

For an existing directory of independently built tools, run the inventory
stage directly with a new output directory:

```sh
python3 scripts/check-replay-inventory.py --bin-dir .build/bin --output /tmp/replay-check
```

The [Record contract test](../tools/record/record_test.go),
[Ply adapter test](../scripts/check-record-ply.py), and
[inventory gate](../scripts/check-replay-inventory.py) document executable
coverage. Native Cage kernel enforcement and live remote-provider behavior
remain separate proofs; observing a command's bytes is not a claim about
unobserved effects, authority, or business completion.

## Default Agent recording

Agent enables Ply's `-record-dir` seam for every action/check run. Record
captures full separate streams outside the action interpreter and Cage;
Ply keeps its presentation cap and original interpreter identity. Each
invocation has a sealed Ask index linking attempts, process receipts,
conversation sessions and nested invocation parents. Selected inputs are
snapshotted before work; outputs and used conversations before completion.
Recording errors stop with 125. Harness instructions route normal work
through Agent and teach explicit Record wrappers for direct Unix commands.
Compaction summaries are snapshotted through Ask's public JSON handoff.
Keep linked child evidence roots with the parent run.
Quiet heartbeat wakes still stop before Ply without creating a recording.
