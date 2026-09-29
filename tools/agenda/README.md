# Agenda

**Record what is owed, by whom and when; account for it against evidence.**

Agenda is an independent Unix command with no external Go dependencies. It
retains promises, schedule revisions, human reports and business decisions.
Its pure commands also work directly on ordinary saved JSON files. It never
executes work, grants approval, sends notifications or hosts an interface.

Build with Go 1.26 or later on Unix/WSL:

```sh
go build -o agenda .
./agenda version
make check
```

## A human-only account

Choose a physical absolute directory outside source (on macOS use `/private/tmp`
instead of the symlink `/tmp`). Create a promise:

```sh
agenda apply /private/tmp/my-account <<'JSON'
{"schema":"agenda.change/v1","request_id":"review-1","kind":"item","id":"review","previous":null,"by":"Morgan","reason":"Agree review date","value":{"title":"Review the release notes","owner":"Morgan","not_before":"2026-09-29T09:00:00Z","due_at":"2026-09-29T17:00:00Z","timezone":"UTC","basis":"human"}}
JSON
agenda export /private/tmp/my-account > account.json
agenda project account.json --as-of 2026-09-29T18:00:00Z < /dev/null
```

The projection retains the obligation and reports the missing human report and
overdue date. For an `external` item, missing execution observations remain
visible instead. An item revision changes a declaration with explicit reason
and expected prior revision; the complete earlier declaration remains retained.
An application requiring immutable promises should create a new item identity
and keep its supersession relationship in extensions.

## Commands and streams

```text
agenda apply ROOT                  # one change JSON object on stdin
agenda export ROOT                 # one complete snapshot JSON object
agenda expand --as-of TIME          # one schedule on stdin; occurrences JSONL
agenda project SNAPSHOT --as-of TIME # observations JSONL on stdin; one projection
agenda version
agenda help
```

`ROOT` for writes must be an explicit physical absolute path. Reads accept a
selected path; all selected file/directory paths refuse symlink components.
Only `apply` writes. `project` does not consult a store, original evidence files,
the wall clock or external programs. `expand` requires no store at all.

Exit 0 means the operation succeeded, including a projection containing overdue,
missing or uncertain work. Exit 1 means rejected, conflicting or inconsistent
input; exit 2 means usage or filesystem/output failure. Diagnostics use stderr.
An output failure can leave partial bytes: check exit status before consuming
results. A failed reply from `apply` can follow commitment; retry the identical
request ID. No empty output or successful exit establishes completed work.

## Change contract

`agenda.change/v1` has `request_id`, `kind`, `id`, `previous` (null initially or
the last revision for that kind/ID), `by`, `reason`, and typed `value`. Optional
`evidence` is up to 16 `{path,sha256}` selections of absolute regular files.
Their exact bytes are checked and retained. IDs are nonempty UTF-8 strings up
to 256 bytes without NUL/newlines. Attribution is not authenticated identity.

All kinds accept an optional `extensions` object inside `value`. It is retained
as opaque data. Agenda never follows paths, executes commands or interprets
application policy there. The only external file reads during `apply` are
explicit `evidence[].path` selections and its own store.

| Kind | Value fields |
| --- | --- |
| `item` | `title`, `owner`, `not_before`, `due_at`, `timezone`, `basis` (`human` or `external`); optional `parent`, `occurrence`, `extensions` |
| `schedule` | `title`, `owner`, `timezone`, `start_date`, `end_date`, `weekdays`, `excluded_dates`, `start_time`, `due_time`, `due_day_offset`, `missed`, `effective_from`, `check_every_seconds`; optional `id` matching change ID, `extensions` |
| `report` | `target` matching change ID, `state` (`planned`, `ready`, `in-progress`, `needs-attention`, `done`, `cancelled`); optional `completed_at`, `extensions` |
| `disposition` | `target` matching change ID, `kind` (`skipped`, `cancelled`, `reopened`); `schedule_revision` required for a scheduled occurrence; optional `extensions` |

Item timestamps use RFC3339 with explicit offsets and seconds. `due_at` is at
or after `not_before`; timezone is an IANA name. Basis and parent cannot change
under an existing identity. Parent completion does not flow in either direction.
Optional occurrence is `{schedule_id,date,schedule_revision}`; item ID must be
`schedule_id/YYYY-MM-DD`. A matching occurrence merges with its declared item;
changed dates or source revisions produce a visible conflict.

Reports target existing human items. `done` requires selected evidence. It
produces `reported-done`, separately from externally observed acceptance.
`completed_at`, when supplied, must be between item start and actual recording
time and is allowed only for done. Editing an uninterrupted done report preserves
its first completion; reopening starts a new completion interval. Historical
selected evidence remains checked even when a newer report supersedes it.

Skipping an admitted item is refused. Reopening needs a preceding disposition.
A cancellation is a business decision, never execution cancellation. A changed
schedule cannot silently make an earlier waiver valid for another declaration.

## Finite calendars

`expand` reads the schedule value above with required `id`. Dates are inclusive,
at most 366 days. Weekdays use Monday=0. Exclusions are explicit ISO dates; local
times use HH:MM. The implicit machine zone `Local` is refused. Due-day offset
is 0–30. DST gaps/ambiguities are errors. Timezone rule data comes from the Go
standard lookup with embedded fallback; archive the chosen rules/tool when
long-term reproduction across timezone database updates matters.
`missed` is `all`, `latest` or `skip`; it changes only the `eligible` field.
Every date remains in the output, including future and missed occurrences.
`check_every_seconds` is a declared observation cadence, 30–604800 seconds;
Agenda does not install or run a monitor. This cadence is guidance; it does
not create a successful-check heartbeat or automatically set observation
freshness. A caller can enforce it by supplying explicit `valid_until` values.

The initial registered schedule starts at `effective_from == start_date`.
Subsequent revisions preserve timezone and take effect after the current local
day and after their predecessor's effective date. Projection uses the complete
revision history so earlier obligations and conflicting admissions remain.

Occurrence JSONL fields: `id`, `schedule_id`, local `date`, `not_before`, `due_at`,
`timezone`, declaration `revision`, `eligible`, `overdue`. Standalone expansion
uses the canonical schedule hash as revision; registered occurrences use their
record revision. Do not interchange these two bindings.

## Snapshots and observations

`agenda.snapshot/v1` contains `records` and `head` (null or final revision).
Each `agenda.record/v1` contains `sequence`, global `previous`, actual
`recorded_at`, complete `change`, optional `data` mapping evidence digest to
base64 bytes, and `revision`. The revision hashes compact Go JSON serialization
of that record with `revision` omitted, followed by LF. Consumers should retain
the returned revision instead of duplicating the encoder. The change's
`previous` is the prior revision of its kind/ID, distinct from the global chain.
Snapshot validation covers every historical record and all retained evidence.

`project` reads an observation snapshot as JSONL. Empty stdin is valid and makes
missing observations visible. Each record has `id`, item/occurrence `revision`,
`state`, `observed_at`, and `evidence` (`[{kind,ref}]`). Optional fields are
`completed_at`, `valid_until`, and opaque `extensions`. States are `unsubmitted`,
`queued`, `running`, `waiting`, `cancelled`, `unverified`, `accepted`, `rejected`,
`unknown`. Accepted/rejected/unknown observations require evidence references.
Only accepted has required `completed_at`, at or before observed_at.

Observation time must not follow as-of. Optional `valid_until` must not precede
observation time; exceeding it marks the observation stale. Revision mismatch
also marks it stale and cannot establish acceptance. Unknown IDs remain visible
as coverage errors. These checks do not authenticate evidence: the caller's
adapter owns its meaning, integrity and freshness policy.

`agenda.projection/v1` has `as_of`, `snapshot_sha256`, `coverage`, `obligations`,
`attention`, `errors`, and an authority `notice`. Every row retains identity,
revision, owner/title/dates/timezone, basis, calendar ID, extensions, and separate
`state`, `execution`, `acceptance`, `timeliness`, `completed_at`, `attention`.
Supplied external `observation` is retained on its row. Missing observations,
schedule gaps, stale bindings and unknown effects do not disappear. Due equality
is on time. As-of is evaluation against selected current facts, not historical
execution reconstruction. Human rows additionally carry `reported_state` and
`report_revision`. A supplied observation on a human row can show a linked
wait/failure or stale evidence but never completes its human report. Coverage
describes the read, not whether work is done; an empty account is unverified.

## Bounds and guarantees

JSON rejects duplicate/unknown fields outside extensions, invalid UTF-8 or
surrogates, nonfinite/out-of-range numbers and depth beyond 32. Change/schedule
inputs and observation records are at most 2 MiB; the observation stream is at
most 16 MiB/10,000 records. Accounts retain at most 2,000 records and 64 MiB of
evidence across history in a snapshot of at most 128 MiB. Projection permits
at most 20,000 scheduled obligations. There is no automatic pruning.

The local store uses a writer lock and atomic durable snapshot replacement.
Only committed records are exported. The exported snapshot contains all selected
evidence, so it can be moved and inspected without original paths. Hashes detect
inconsistency against retained commitments; they do not defeat an administrator
able to replace the whole account. Keep directories trusted and backed up.

See [DESIGN.md](DESIGN.md), [agenda.1](agenda.1), and [AGENTS.md](AGENTS.md).
`make check` includes a build from copied standalone sources and no other tools.
