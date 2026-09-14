# Record

Retain one process invocation in a self-contained Ask session. Replay its
observed bytes and status offline, without repeating the work.

```sh
go build -o record .
record run -f lookup.jsonl -- context get policy < query.json
record check -f lookup.jsonl
record replay -f lookup.jsonl
```

Record needs [Ask](https://github.com/patrickyoung/ask) 0.3 or later on PATH.
The `-ask` flag selects another executable. Record checks the required features
through `ask help` before using new verbs, so an older Ask cannot interpret
`init` as a model prompt. There are no Go dependencies.
Each recording uses a new explicit file; existing destinations are refused.
Ask owns creation, locks, events, and prefix seals. Record uses `ask init`,
`ask note`, and `ask replay -check -json` through public subprocesses.

`record run` executes literal argv in the physical current directory with
the caller's environment. There is no implicit shell, retry, policy, or
model. stdout is the child's stdout; stderr includes child stderr and any
recorder failure diagnostic. A completed capture returns the child's status,
including 1, 2, 75, and 125. Signals use 128 plus the signal number. Recorder
errors return 125. A complete receipt of exit 125 does not resolve the child's
uncertainty about external effects.

The session contains a sealed intent before execution, binary stream chunks,
and a sealed terminal receipt with lengths and SHA-256 commitments. It records
literal argv and paths as base64 because Unix filenames need not be UTF-8.
Executable identity is a hash of the selected file observed before execution;
it is not executable attestation, an interpreter/dependency snapshot, or a
guarantee against concurrent replacement.

`record replay` first asks Ask to emit its verified event snapshot, then
checks Record's ordering, stream offsets, commitments, and terminal outcome.
It writes nothing from the child until all checks pass. A sealed prefix with
no terminal receipt is incomplete, never a successful run or permission to
retry. Ask's hashes detect divergence relative to retained commitments; they
are not signatures protecting against an actor able to rewrite the archive.

Replay writes the retained stdout and stderr and returns the recorded exit.
It does not reproduce scheduling across the two pipes, send signals, or
execute commands. Each stream's byte order is exact. These forms extract data
to ordinary filters instead:

```sh
record replay -f lookup.jsonl -stream stdin > query.bin
record replay -f lookup.jsonl -stream stdout | context check
record replay -f lookup.jsonl -json > receipt.json
```

Extraction and JSON output return 0 after verification, irrespective of the
recorded child's status. `record check` verifies without emitting child data.

## Files and child conversations

The caller chooses which filesystem observations belong in the recording:

```sh
record run -f build.jsonl -input plan.json -output report.bin \
  -session worker.jsonl -- sh -c 'ask -f worker.jsonl < plan.json > report.bin'
record replay -f build.jsonl -stream artifact:0 > saved-plan.json
record replay -f build.jsonl -stream artifact:1 > saved-report.bin
record replay -f build.jsonl -stream artifact:2 > saved-worker.jsonl
ask replay -check saved-worker.jsonl
```

Use the actual child program's flags when composing it. `-input` snapshots a
regular file before starting. `-output` snapshots after termination. `-session`
also verifies a private snapshot through Ask before retaining its exact bytes.
Flags repeat; artifact numbers list inputs, outputs, then sessions, in each
group's flag order. The JSON receipt includes names, original paths, and phases.
Missing or changing selected files make the recording incomplete. Input file
snapshots are observations, not a guarantee that the child read those bytes.

The recording contains all retained bytes, including selected child sessions.
Moving this one file is sufficient for offline verification; original paths,
services, and executables are not consulted. Select all relevant child files
explicitly, including parents and summaries when a tool compacts a session.
Unselected filesystem changes, environment-dependent behavior, dependency
versions, kernel enforcement, and network effects are outside this receipt.

## Pipes and credentials

stdin streams while the child runs, so bidirectional protocol servers can
respond before EOF. The input stream retains bytes read for forwarding;
`stdin_delivered` gives the prefix successfully written to the child's pipe.
It does not prove how many bytes the child consumed. `stdin_eof` distinguishes
an exhausted input from a child that stopped accepting it. Unread upstream
bytes are not captured. Recording introduces pipe buffering and backpressure;
it is intended for Unix filters and protocol pipes, not interactive TTYs.
Output pipes have a two-second drain bound after child termination; descendants
that keep them open make evidence incomplete. `-timeout` optionally limits the
process group. Interrupt/terminate/hangup signals are forwarded to that group.

Environment values are never recorded. A private inherited descriptor is
closed in the child unless explicitly selected with `-pass-fd N`; selected
descriptors retain their number and are not read by Record. For example:

```sh
oauth with profile -- record run -f request.jsonl -pass-fd 3 \
  -label resource=example -- ask -header-fd 3 -f answer.jsonl 'question'
```

Do not put credentials in recorded argv, stdin, output, labels, or selected
files. Record cannot distinguish a token from other application bytes and
does not claim to redact arbitrary streams. Record OAuth's `with` child
boundary; do not record a command whose purpose is to print a secret header.
Labels are optional, explicit non-secret identity/resource context.

Run `go test ./...`, `go test -race ./...`, and `go vet ./...` for local checks.
The public executable contract also needs `RECORD_TEST_ASK` set to a freshly
built Ask executable; the repository integration gate requires it, with no skip.

When supervised, `-grace D` bounds cleanup after a forwarded signal (default
one second). Record kills remaining members of the child process group even
if its leader exits first. The supervisor's grace must be longer so Record
can finish cleanup and seal the observed outcome. Ordinary completion does
not kill detached work.

Agent enables this recording boundary automatically through Ply. Install all
three together with Ask; the Bench harness instructions describe retained
indexes, explicit task-file selection and offline replay. Direct Unix commands
and plain Ply can still use Record explicitly without Agent.
