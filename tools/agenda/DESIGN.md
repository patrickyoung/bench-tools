# Accountable work

Agenda answers what is owed, by whom, and when, against an explicit account and
supplied observations. Durable recording and pure projection share one data
contract. The store is optional for the pure commands. No renderer, team format,
executable selection, approval service, queue, scheduler or model is involved.

## Authority

`apply` owns the append-only logical record history and selected evidence in a
caller-selected local directory. Item and schedule revisions are declarations;
reports and dispositions are attributed business records. They do not cause
execution or establish external acceptance. One program owns each store.

External observations are caller-supplied claims. Agenda validates references,
identity, revision and time; it never opens the referenced evidence or claims
to authenticate it. The selected adapter must check its own evidence before
supplying acceptance. A human report produces `reported-done`, never `accepted`.
Neither completing a child nor cancelling a promise changes its parent or an
execution system. Unknown effects remain unknown. Dates never authorize retries.

## Commit boundary

The explicitly selected root contains `.lock` and `snapshot.json`. `apply`
takes an exclusive Unix advisory lock, validates the entire current snapshot,
and compares the change's expected entity revision. A unique request ID binds
the exact normalized change. Repeating it returns the original record, including
after a lost output reply; differing reuse fails before reading new evidence.

Every record retains its change, actual recording time, global predecessor,
sequence, selected evidence bytes and a SHA-256 commitment. A new snapshot
contains the entire old history unchanged followed by one record. It is written
to a same-directory temporary file, synced, renamed, and the directory synced
before success. Newly created directory links are also synced. Temporary files
left by interruption are uncommitted and never enter an export.

This is one atomically replaced file with an append-only logical history, not
two independently authoritative logs. There is no deletion, pruning or
compaction command. Evidence is retained as base64 in the record so the exported
snapshot is self-contained. The initial size limits deliberately favor small
accounts over a hidden database or mandatory service.

A failure after replacement or after stdout begins does not prove absence.
Retry the identical request ID or export the store. Do not invent a new request
ID to evade uncertainty. Atomic replacement gives readers an old or new complete
snapshot without taking a writer lock. Source evidence files are unnecessary
after commitment. Filesystem permissions/backups protect the store; hashes do
not authenticate against an operator who can rewrite the entire account.

## Pure evaluation

`expand` uses explicit finite dates, weekdays, exclusions, IANA timezone and
evaluation time. The binary includes standard-library timezone data. Ambiguous
and nonexistent local times are errors. Output identity is schedule ID plus
local date. Catch-up policy annotates eligibility; every expected date survives.

`project` validates a complete snapshot and bounded observation stream before
emitting one JSON object. It joins stable identities and preserves obligations
without observations. Each schedule revision governs dates from its effective
date up to the next revision's effective date. Earlier obligations keep their
original declaration; a changed future declaration exposes conflicting admitted
items or decisions. Expiry, approaching horizons and renewal gaps remain visible.

Snapshot completeness is relative to the selected head and complete record
chain. Removing a retained record without replacing its commitments fails.
No program can infer an entirely omitted, independently rewritten account;
callers must select and preserve the authoritative snapshot, rather than feed
a query result pretending to be one.

Human report edits preserve the first completion time in the current
uninterrupted done state. Explicit reopening resets that time. A separately
provided completion date is an attributed report, not retroactive acceptance.
As-of compares current selected declarations and observations against time; it
does not reconstruct a historical execution state. Observation `valid_until`
lets the adapter declare freshness without imposing one global age policy.

## Boundaries and verification

Unix/WSL and an ordinary local filesystem are required. No distributed writes,
network-filesystem locking, authenticated identities, multi-record transaction,
exactly-once external effect, arbitrary recurrence language or notifications
are promised. The containing directory is trusted against concurrent hostile
replacement; path checks reject symlinks but are not a confinement mechanism.

Tests cover isolated source builds, public stream behavior, ambiguous JSON,
concurrent request deduplication and revision conflicts, missing/corrupt retained
evidence, schedule changes and gaps, DST ambiguity, missing/stale/unknown
observations, and human/external completion authority. They use no service,
model, queue or other Bench executable.
