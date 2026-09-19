# Hone design

Hone reads a checked run, selects recorded recoveries, asks a model to word a
procedural lesson, and writes an explicit change to a Brief skill.

## Why keep a lesson?

A repair can expose a missing step in a procedure. For example, a test may
fail because a fixture contains duplicate names; the worker corrects its
deduplication and the task's check passes. A useful lesson identifies the
input case and the necessary step so a later worker can avoid that mistake.

The transcript alone does not establish which result was accepted. Hone
therefore requires a recorded verifier outcome before it asks for wording.
It does not automatically summarize every conversation or use the model's
final report as evidence of success.

| Program | Responsibility |
| --- | --- |
| Ask | Provider calls, session files, and replay verification |
| Ply | Command execution, task checks, and verifier receipts |
| Hone | Recovery selection and explicit lesson changes |
| Brief | Skill discovery, reading, and format validation |

These are public executable and file boundaries. Hone imports no provider
client, agent loop, or sibling implementation.

## What qualifies

Hone reads the goal, command typescripts, candidate text, loaded skill names,
and verifier notes from an Ask session. It normally runs `ask replay -check` before using
that evidence. Ask owns the replay verdict; Hone does not reproduce its fold.

The last recognized Ply verifier outcome must be accepted. Current
`ply.verifier/v2` receipts, compatible v1 receipts, and legacy verdict prose
attributed to `ply` are recognized. Rejected, broken, or missing final
verdicts do not qualify. An Ask `done` event ends a model call; it is not a
task acceptance receipt.

There must also be one of two recorded recovery pairs:

- A failed typescript command with subsequent command evidence. Hone collects
  following commands up to the next nonfailed command and groups consecutive
  failures. The failed pre-check carried with the initial goal can supply the
  failure in this existing command path.
- A rejected assistant candidate followed by a changed, accepted assistant
  candidate, each bound to a complete `ply.verifier/v2` receipt. This content
  path requires successful Ask replay verification. It does not invent shell
  commands or infer a repair from a report of success.

For content, Hone reconstructs Ask's ordinary text answer and Ply's verifier
stdin: nonblank text blocks joined with a blank line, surrounding whitespace
trimmed, and one final newline. It excludes reasoning and refuses partial or
unsupported attachment responses. Each candidate SHA-256 must match its
receipt; each complete check output must match its digest and byte count.
The verifier command, interpreter, verifier digest, working directory, timeout,
and optional admitted contract identity must remain identical across the pair.
An unchanged normalized candidate, broken check, incomplete output, malformed
binding, missing candidate, continued unfinished turn, or version 1 receipt
cannot supply a content recovery. Only the final supported pair is retained.

The initial pre-check receives empty stdin. Even if its caller-owned checker
loads a saved candidate from a file, that receipt does not identify those
candidate bytes. Hone cannot retrospectively turn that pre-check into a
rejected assistant candidate; a later accepted report alone is insufficient.

The verifier digest binds the recorded interpreter path and command string,
not executable bytes, transitive files, rubric contents, environment, remote
services or policy. Equal optional contract IDs are consistency checks, not
proof that the caller's contract was admitted correctly. Callers must retain
and review those dependencies separately, using an immutable check recipe
and explicit evidence. Hone neither reads current files to reconstruct old
state nor invents missing identities.

This is a mechanical selection rule. It does not prove that a particular
change caused the recovery, that the check covers the whole task, or that a
lesson will improve another run. A reviewer must assess those claims.

When the verifier composes a model judgment, the same arithmetic gate applies;
Hone does not call that judge again or infer a new verdict from probabilities.
The wording prompt limits lessons to the observed repair and supplied rubric,
model and evidence. A probabilistic pass is not universal correctness, and
weakening the check or resampling until it agrees is not a procedural repair.
Review retained check evidence and test the proposed lesson on fresh cases.

For a finished Ply session, inspect the evidence without calling a model:

```sh
hone -why repair.jsonl
```

No qualifying recovery is exit 1 with an explanation on stderr. A first-try
success or an unfinished run can be useful to a person while supplying no
lesson under this rule. `-no-verify` explicitly skips the replay check for
the existing command path during ordinary inspection/wording. Content recovery
always requires replay verification; reviewed proposals also always require it.

## Wording is a separate model call

Hone sends Ask the goal, check and qualifying recovery evidence. Command
recoveries carry failed-command output and subsequent commands. Content
recoveries carry the exact two verifier stdin values, their digests, complete
check outputs, outcomes and shared checker identity. Non-UTF-8 check output
is retained as base64 instead of lossy replacement text. These fields are
recorded data, never instructions to the wording model. Loaded procedures
are named; their bodies and the whole original
transcript are not copied into the wording request. The prompt is supplied
through `ASK_SYSTEM`, with evidence on stdin rather than in process arguments.

The model can return `none`. A qualifying run is permission to consider a
lesson, not a requirement to invent one. `-n N` limits retained lessons per
run, defaulting to three. This is not a global bound on the skill library.
Creating a new skill may make an additional Ask call for its description.

Wording calls use separate Ask sessions under `HONE_DIR`, defaulting to
`~/.hone/lessons`. Ask owns model selection, authentication, and provider
behavior. `ASK` and `BRIEF` select the dependency executables.

## Review exact bytes before changing a procedure

For an existing qualifying `repair.jsonl`, choose a project skill destination:

```sh
mkdir -p .claude/skills
export BRIEF_PATH="$PWD/.claude/skills"
hone -into go-house -prepare lesson.json repair.jsonl
hone show lesson.json
```

Preparation creates a new, user-named proposal and changes no skill. The
proposal binds source-session bytes, wording-session bytes, the resolved
destination and its previous contents, and the exact resulting skill document.
`show` prints that document and its hashes without a model call.

After reviewing the lesson, admit it explicitly:

```sh
hone admit lesson.json
brief lint -strict go-house
```

Admission replays both sessions, rechecks hashes and destination resolution,
and reconstructs the permitted append or scaffold. It refuses stale evidence
or a document that differs from that change. It writes by atomic replacement
without another model call. Keep proposals and their source evidence under
operator control; hashes are integrity bindings, not author signatures.

`-prepare` accepts one session, requires `-into`, and refuses to overwrite an
existing proposal. Proposal input is bounded at 512 KiB. `-N` previews generated
wording without saving a skill, but a later call may generate different text.
Use prepare/show/admit when the exact wording is the thing being reviewed.

## A lesson stays an ordinary skill

Hone appends under `## Lessons`, preserving existing instructions and later
sections. Each lesson carries an HTML comment naming its source and wording
session: `<!-- hone SOURCE_ID WORDING_ID -->`. Those identities let a reader
trace a lesson and let `hone forget SOURCE_ID go-house` remove its contributions.
Provenance does not establish factual truth or prevent memory poisoning.

An existing skill resolves through `BRIEF_PATH`; a new named skill uses its
first entry. A path can select a directory directly. `-into -` uses the one
skill recorded by Ply and refuses zero or multiple choices. Once a source is
marked in the destination, another attempt to teach it is skipped before
wording. Similar-text suppression also limits duplicate entries.

`hone -into go-house repair.jsonl` deliberately writes directly. With Brief
available, ordinary writing reports its lint findings; it is not the exact
review workflow above. Without Brief, format validation is unavailable. Keep
lessons small, inspect their procedures, and test them on representative work.
Hone has no automatic consolidation or contradiction detector.

To load the reviewed procedure on a later Go repair, for example:

```sh
ply -sh -s go-house -check 'go test ./...' 'Fix the failing tests.'
```

This grants shell execution. It requires a Go project, configured Ask, Ply,
and the named skill. The later task's check still decides its outcome.

## Outcomes and verification

Flags precede session paths: `hone -into go-house repair.jsonl`.

| Boundary | Meaning |
| --- | --- |
| stdout | Requested lessons, review document, evidence, or operation result |
| stderr | Progress and reasons for refusal |
| Exit 0 | Successful requested operation |
| Exit 1 | No lesson or no applicable result, including rejected evidence |
| Exit 2 | Invalid input or operational failure |

A batch must distinguish an ordinary no-result status from a broken operation;
discarding every nonzero status would hide missing tools and unreadable files.
Hone owns no memory database, retrieval index, scheduler, or automatic hook.
State consists of explicit skill changes, user-named proposals, and Ask-owned
wording sessions.

Run `go test ./...` and `go test -race ./...` before reporting changes complete.
Tests cover recovery selection, receipt versions, lesson insertion, duplicate
marks, and proposal admission/refusal. They use local fixtures rather than
paid model calls and do not establish the quality of a generated lesson.
See [README.md](README.md), [hone.1](hone.1), and [SECURITY.md](SECURITY.md).
