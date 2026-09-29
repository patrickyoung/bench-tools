# Give a standing team a process and commitments

This experimental application models the work a team owes without replacing
its existing execution. A reusable process describes roles, standard stages,
handoffs, working agreements and definition of done. A commitment supplies the
actual owner, objective, inputs, dates and milestones. Tend retains execution;
the existing team's command and checks establish acceptance.

Two shipped processes exercise different contracts: the page team accepts a
brief on stdin and supports unchanged-run resume; the vendor comparison team
accepts a job file, uses five fixed stages, and requires a fresh run. Their
`expert/process.json`, `PROCESS.md` and `bin/check-process` travel in normal
pinned team exports. Teams without a process retain their existing behavior.

## Install and select source

Requires Python 3.9+ with the IANA timezone database, Unix file locking, and an
installed Tend executable. The selected team's own tools and model access are
separate prerequisites. This directory can be copied independently; it imports
no other source package and needs no Python dependencies or daemon.

From the Bench checkout, build Tend and export a selected committed team:

```sh
python3 scripts/build tend
git rev-parse HEAD
python3 scripts/workers export-team page-team /absolute/new/page-team \
  --ref FULL_COMMIT --allow-experimental
python3 examples/team-process/process.py validate /absolute/new/page-team/expert/process.json
```

Use a reviewed commit containing the process definition. Uncommitted changes
are not exported. Install the team's documented dependencies in its deployment
copy, preserving inventoried source. Keep the application, export and selected
executables stable during a commitment. A changed source or executable is
reported explicitly; no silent upgrades or re-locking occur.

## Admit one commitment

Write a commitment JSON outside reusable source. The following is a template;
replace paths, dates, owner, namespace, objective and configuration with the
actual choices for your job:

```json
{
  "schema": "bench.commitment/v1",
  "id": "homepage/2026-09-29",
  "namespace": "web-team",
  "owner": "Delivery owner",
  "objective": "Deliver the page specified in the current brief",
  "not_before": "2026-09-29T09:00:00-04:00",
  "due_at": "2026-09-29T17:00:00-04:00",
  "timezone": "America/New_York",
  "inputs": {"brief": "/absolute/current/brief.md"},
  "environment": {
    "AGENT": "/absolute/bin/agent",
    "BENCH_MANAGE": "/absolute/bin/bench-manage",
    "ASK_MODEL": "provider/model",
    "PAGE_TEAM_DEADLINE": "1h"
  },
  "pass_env": [],
  "milestones": [
    {"id": "review", "owner": "Reviewer", "due_at": "2026-09-29T16:00:00-04:00",
     "expectation": "Independent review of the complete page is available"}
  ]
}
```

The objective is a tracking summary. Put all actual task expectations in the
team's brief/job; they are not silently injected into another model prompt.
For an additional executable definition of done, select `additional_check`:

```json
{"argv":["/absolute/reviewed/check", "{run}", "{commitment}"],
 "files":["/absolute/reviewed/check-support.py"]}
```

This optional field runs after the team's entry and delivery verifier both
succeed. Its executable and listed support files are hashed at admission.
Arguments are literal, with only the two whole-argument placeholders expanded.
Exit 0 accepts, 1 rejects, and other exits mean broken verification. The
application does not sandbox this trusted operator-selected checker.

```sh
python3 /absolute/team-process/process.py admit \
  /absolute/new/page-team /absolute/current/commitment.json \
  /absolute/standing-team/commitments --tend /absolute/bin/tend
```

The result prints the instance path and stable Tend job ID. Admission snapshots
input bytes and freezes the process, team lock, application, Python, Tend,
selected program paths and declared configuration. The same namespace/ID and
same bytes return the same instance; conflicting reuse fails. Use one stable
commitment root per standing-team namespace—moving to a new root creates a new
authority and is not a deduplication mechanism across machines or directories.

For vendor comparisons select the other exported team and `inputs.job` instead.
Provide `supporting_files`, mapping original relative material/dataset paths to
absolute selected files, for example `{"materials/spec.txt":"/absolute/spec.txt"}`.
The job and supporting files are copied together, preserving relative paths.
No directory is swept implicitly. The entire input set is bounded to 64 MiB.

`environment` admits only configuration names declared by that process. Program
selectors are resolved and hashed. Use `pass_env` for credential variable names
only; values remain outside request, admission and receipt files. Supply them
to the existing Tend worker through `TEND_PASS`. Other native packages and
external services retain the selected team's own dependency requirements.

## Submit and execute using Tend

```sh
python3 /absolute/team-process/process.py submit /absolute/standing-team/commitments/INSTANCE
TEND_ROOT=/absolute/standing-team/commitments/tend \
  TEND_PASS='YOUR_SELECTED_CREDENTIAL_NAME' TEND_JOB_MAX=2h \
  /absolute/bin/tend work
```

Submission is repeatable, even after a lost response. It does not run the job.
`tend work` performs one durable transition; use the existing host scheduler
or ordinary operator loop to invoke it again. A successful Tend invocation
does not necessarily mean a successful team result. Inspect the commitment.

All commitments in a namespace share Tend's serialization key. This first
version conservatively runs one at a time; an unknown attempt keeps the fence
even if a later daily occurrence is admitted. Do not change namespace merely
to bypass unresolved work. The queue is private to this standing-team application;
the page team's internal Manage/Tend queue remains separate.

Business `due_at` reports a promise. `not_before` prevents early execution.
The existing team's settings bound its execution; `TEND_JOB_MAX` bounds the
outer attempt. These are independent. Deadline expiry neither implies success
nor grants a retry. A started attempt interrupted by a timeout may be unknown.

## Inspect current work

```sh
python3 /absolute/team-process/process.py status /absolute/standing-team/commitments/INSTANCE \
  --as-of 2026-09-29T17:30:00-04:00
python3 /absolute/team-process/process.py board /absolute/standing-team/commitments \
  --as-of 2026-09-29T17:30:00-04:00
```

Both return JSON. Execution (`ready`, `running`, `waiting`, `done`, `failed`,
`cancelled`, `unknown`, `unsubmitted`, or `unverified`), acceptance and timeliness are separate.
An accepted late result remains accepted and late. A running overdue case
remains running. Changed delivery artifacts become stale; missing controller
receipts never become accepted. Broken source/program/record bindings produce
an `unverified` row with an error, keeping the affected commitment visible.
As-of evaluates deadlines against currently
observed facts; it is not historical state reconstruction or execution authority.

Milestones are declared obligations, not a second task graph. A status call
may select `--milestones /absolute/milestone-evidence.json`, containing an array:

```json
[{"id":"review","by":"Reviewer","at":"2026-09-29T15:30:00-04:00",
  "artifact":"/absolute/current/review.md","sha256":"FULL_SHA256"}]
```

The selected artifact must match its hash and attribution must match the
declared milestone owner. The view labels it **reported-met**; this does not
authenticate a person's identity or establish overall team acceptance.
Unconfirmed milestones remain visible. Escalation reasons and the responsible
owner are reported; no notification is sent and no business decision is made.
For `board --milestones FILE`, supply an object mapping instance directory names
to these observation arrays. Without selected evidence, the board deliberately
reports milestones as unconfirmed rather than inferring completion from files.

Inspection uses public Tend observations and retained bindings. It does not
invoke a model, rerun a semantic check, submit work or write an application
report. JSON views are disposable. Stale source prevents verified status and
refuses execution; inspection reports the problem rather than hiding the row.

## Daily and weekly work

Write an explicit finite calendar, for example:

```json
{
  "schema":"bench.team-calendar/v1", "id":"daily-site-review",
  "timezone":"America/New_York",
  "start_date":"2026-09-28", "end_date":"2026-10-02",
  "weekdays":[0,1,2,3,4], "excluded_dates":[],
  "start_time":"09:00", "due_time":"17:00", "due_day_offset":0,
  "missed":"latest"
}
```

```sh
python3 /absolute/team-process/process.py plan /absolute/new/page-team/expert/process.json \
  /absolute/current/calendar.json --as-of 2026-09-30T10:00:00-04:00
```

Weekdays use Monday=0. Select one weekday for weekly work; use all seven for
daily work. Explicit excluded dates express holidays. A calendar spans at most
366 dates; due-day offsets are 0–30. Nonexistent or ambiguous local times fail
explicitly. Normal local times preserve wall-clock meaning through DST.

Planning emits JSONL proposals whose start is at or before as-of. `all` emits
all such occurrences; `latest` emits only the latest; `skip` omits those already
past due. Identity is schedule ID plus local date, stable across repeated ticks.
Equal-to-due is on time. Changed calendar bytes have a different retained hash
but do not invent new IDs to evade an existing commitment.

For a selected proposal, copy `id`, `not_before`, `due_at`, `timezone` and
`occurrence` into a full commitment, add `calendar` with its absolute source
path, and supply current inputs and owner. Admission verifies the proposal
against those calendar bytes. Repeated ticks can plan and submit the same
occurrence safely. Planning never submits or invents current case inputs.

## Register calendars so missing work stays visible

The planning command proposes executions according to the missed-run policy.
The accountability view independently enumerates **every** expected occurrence,
including future work and missed occurrences omitted by `latest` or `skip`.
Register the calendar in the same root used for the standing team's commitments.
Write a registration file outside source:

```json
{
  "schema":"bench.calendar-registration/v1",
  "namespace":"web-team",
  "owner":"Delivery owner",
  "calendar":"/absolute/current/calendar.json",
  "effective_from":"2026-09-28",
  "reason":"Initial daily site review agreement",
  "check_every_seconds":3600
}
```

```sh
python3 /absolute/team-process/process.py register-calendar \
  /absolute/new/page-team /absolute/current/registration.json \
  /absolute/standing-team/commitments
```

The first effective date must equal the calendar's start date; past dates expose
the backlog. Registration snapshots the calendar, process, owner and pinned team
identity. Source files can then be moved without losing the registered schedule.
The result includes a `revision` hash. Repeating identical registration is safe.

To revise a calendar, retain its ID and namespace, supply the latest hash as
`previous`, give a reason, and select an effective date **after today** in that
calendar's timezone and after the preceding revision's effective date. Older
dates retain their original obligations. Revision does not modify existing
commitments: differing dates or source bindings become visible calendar
conflicts. Removed future dates with already-admitted work also stay visible.
Timezone/team changes require a separate calendar identity. Keep original
registrations; there is no history deletion command.

Each revision has a finite horizon. The view flags calendars ending within 14
days or already expired, and explicitly reports gaps between an old horizon
and a later renewal. A future revision does not hide current expiry or relax
the currently effective monitoring interval. A gap is a coverage warning;
the application does not invent obligations for dates nobody scheduled.

## Reconcile and show users the calendar

```sh
python3 /absolute/team-process/process.py reconcile /absolute/standing-team/commitments
python3 /absolute/team-process/process.py calendar /absolute/standing-team/commitments \
  --as-of 2026-09-29T17:30:00-04:00
python3 /absolute/team-process/process.py calendar /absolute/standing-team/commitments \
  --as-of 2026-09-29T17:30:00-04:00 --format html > /absolute/current/team-calendar.html
python3 /absolute/team-process/process.py calendar /absolute/standing-team/commitments \
  --as-of 2026-09-29T17:30:00-04:00 --format ics > /absolute/current/team-calendar.ics
```

The JSON view contains registered calendar histories, obligations, milestone
states, an owner-attributed attention list and monitoring freshness. Work is
upcoming, missing, unsubmitted, active, completed, explicitly skipped/cancelled,
or unverified. Execution, acceptance and timeliness remain separate. Independent
admitted work also appears; the view does not silently discard cases outside
the registered calendar. Supply `--milestones FILE` using the same mapping as
`board` to include selected milestone evidence.

The self-contained HTML view has a month selector, a searchable obligation
table, milestone due dates, change history and an attention list. No server or
external assets are needed. Dates use each calendar's timezone. The HTML is a
snapshot, not a live connection; its freshness banner turns stale when the
recorded next-check time passes. Regenerate it to see updates. The ICS export
contains work and milestone events with stable, distinct identities for calendar
clients. It is an importable snapshot, not a hosted subscription or two-way sync.
Regenerate/reimport as supported by the client; this application does not write
to Google Calendar or Outlook.

`reconcile` reads actual current work and records the last successful check with
the actual wall clock. It never creates commitments, executes teams, sends
reminders or cancels work. The checkpoint is written only if tracking inputs
remain unchanged throughout inspection and all records can be verified.
Missing/overdue obligations are a successful check with actionable findings;
unverifiable records fail the check and leave its prior successful timestamp
unchanged. A current check does **not** mean all obligations are complete.

Have the existing host scheduler run `reconcile` at least as often as the
smallest currently effective `check_every_seconds` across registered calendars,
and regenerate the user-facing view. For an hourly agreement, checking every
15 minutes leaves room for delays. The view reports never-checked, stale,
changed-input or unverified monitoring. Its freshness uses actual observation
time even when `--as-of` selects another deadline projection. This repository
does not install a scheduler or choose a user's real calendars. A stopped host
needs an external observer to alert anyone; an unread stale banner sends no
notification. Integrations can consume the attention list for explicitly
configured reminders without treating delivery/acknowledgement as completion.

## Explicitly skip, cancel or reopen an obligation

Select an occurrence and its `calendar_revision` from the calendar view. Record
the decision in a separate JSON file:

```json
{
  "schema":"bench.calendar-disposition/v1",
  "namespace":"web-team",
  "calendar_id":"daily-site-review",
  "id":"daily-site-review/2026-09-29",
  "calendar_revision":"FULL_REVISION_HASH_FROM_VIEW",
  "kind":"skipped",
  "by":"Delivery owner",
  "reason":"Review was explicitly waived for the planned maintenance day"
}
```

```sh
python3 /absolute/team-process/process.py record-disposition \
  /absolute/standing-team/commitments /absolute/current/disposition.json
```

`skipped` is allowed only before admission. `cancelled` records a business
decision; it does not cancel a Tend job, stop an attempt or release an unknown
fence. Contradictory live execution remains in the attention list. `reopened`
restores the obligation to ordinary tracking. Further decisions require
`previous` with the last disposition revision hash. Reasons, attribution and
recording times remain in the history, including after reopening. Attribution
does not authenticate a person's identity. A later calendar revision cannot
silently reuse an old waiver: mismatched decisions become conflicts.

## Watch work on a Kanban board

```sh
python3 /absolute/team-process/process.py kanban /absolute/standing-team/commitments \
  --as-of 2026-09-29T17:30:00-04:00 --format html > /absolute/current/team-board.html
python3 /absolute/team-process/process.py serve /absolute/standing-team/commitments \
  --port 8765 --poll-seconds 5
```

The first command exports a snapshot; omit `--format html` for JSON. The second
starts an optional **live read-only board** at `http://127.0.0.1:8765/`. It polls
recorded work every five seconds by default, automatically updates cards and
assignment observations, and preserves the viewer's filter and expanded details.
Refresh failures retain the last received view with a visible warning and the
last successful refresh time. This is near-real-time observation after the
underlying tools record state, not token streaming or an execution scheduler.
Refresh latency includes the public inspection commands' runtime.

| Column | Meaning |
| --- | --- |
| Planned | Future expected work or an admitted commitment awaiting submission |
| Ready | Submitted work ready to begin, or a human activity marked ready |
| In progress | Running execution, or a human activity reported in progress |
| Needs attention | Missing commitments, blockers, uncertain execution, rejected/stale evidence or inconsistent records |
| Done | Verified team delivery, or a separately labeled human completion report with retained evidence |

Overdue and late indicators remain on cards in their actual column. Running
overdue work remains In progress; an accepted late result remains Done. Skipped
and cancelled cards remain in history, unless contradictory execution or evidence
requires attention. The board and calendar share commitment and activity records;
there is no second task graph or independent drag-to-done state.

Team cards are read-only. Human activities are updated through the recorded
command below; their changes appear at the next refresh. Static HTML is also
read-only and must be regenerated. `--milestones FILE` works as on the calendar.
The original `board` JSON command remains an admitted-commitment status list.

The live viewer binds only to loopback, sends no notifications and has no write
endpoints. Stop it with Ctrl-C; no service or daemon is installed. It shares a
short-lived view cache across viewers. A malformed record stays visible as an
exception; a failed refresh never replaces the last view with apparent success.
Host reconciliation remains separate: a working live connection does not certify
that the standing team's periodic checks are running. Publishing to other users
requires separately selected authenticated hosting/proxy configuration. Do not
expose this local viewer directly to an untrusted network.

## Assign and update human activities

Create a JSON request outside source. Its `parent` must identify an existing
commitment or registered calendar occurrence in the same namespace:

```json
{
  "schema":"bench.human-activity/v1",
  "namespace":"web-team",
  "id":"stakeholder-approval",
  "parent":"daily-site-review/2026-09-29",
  "title":"Obtain stakeholder approval",
  "description":"Retain the named stakeholder's decision on the proposed page.",
  "assignee":"Delivery lead",
  "due_at":"2026-09-29T16:00:00-04:00",
  "timezone":"America/New_York",
  "state":"ready",
  "by":"Delivery owner",
  "reason":"Assign approval follow-up",
  "evidence":[]
}
```

```sh
python3 /absolute/team-process/process.py record-activity \
  /absolute/standing-team/commitments /absolute/current/activity.json
```

The returned `revision` is required as `previous` on a changed request. Send
the complete new record with a reason and attribution when changing assignee,
deadline, description or state. Allowed states are `planned`, `ready`,
`in-progress`, `needs-attention`, `done` and `cancelled`. Reopen by recording a
nonterminal state; cancellation and previous completion reports remain in
history. Concurrent conflicting edits are refused. An activity cannot change
parent after creation.

To report `done`, supply at least one evidence object:
`{"path":"/absolute/approval.txt","sha256":"FULL_SHA256"}`. Selected evidence
bytes are copied into the activity's retained history. Editing the original
file later does not alter that report. Corrupt or missing retained evidence,
including evidence from earlier revisions, becomes an exception and prevents
a successful reconciliation. Title/assignee corrections on continuously done
work preserve its completion time; reopening starts a new completion interval.

Human Done means **reported completion with retained evidence**, not authenticated
identity, verified business truth or accepted agent delivery. It cannot complete
the parent, waive a team check, cancel a Tend attempt or unblock dependencies.
Activities and their due dates appear in both calendar views and ICS exports.
The parent may finish while an outstanding human follow-up remains visible.

## Observe existing team assignments

The live viewer automatically inspects admitted Page Team runs through their
explicitly pinned `BENCH_MANAGE status -json RUN` executable. The adapter checks
`bench.manage.snapshot/v1`, binds `run_sha256` to exact `RUN/manifest.json`
bytes and checks the admitted brief and roster. It reports task IDs, assignees,
goals, dependencies and controller-reported states inside the parent card.
Assignments never independently establish parent completion. Unsupported teams
retain commitment visibility; descriptive process stages are not invented tasks.

For a one-time JSON/HTML board with current assignments, add `--live-assignments`.
Alternatively, pass `--assignments FILE`, mapping admitted instance directory
names to either `"manage-status"` or an absolute saved public status export:

```sh
/absolute/bin/bench-manage status -json /absolute/standing-team/commitments/INSTANCE/run \
  > /absolute/current/manage-status.json
```

Saved exports retain their original observation timestamp and are labeled as
retained snapshots. Reading them again does not make them live. Planning packets
under manager job work directories are a different envelope and are not accepted
as public status exports. Another run's snapshot is rejected even with the same
brief. Polling uses no worker/model execution, rejects an uninitialized existing
inner queue and disables Python bytecode writing. Existing controller/Tend
inspection may still open their normal database and lock files.

## Continue or change work deliberately

The application does not retry, signal, cancel, waive, reschedule or resolve
unknown jobs. Use Tend's documented operator commands and the selected team's
continuation contract. The comparison entry cannot resume a used run directory;
the wrapper refuses an attempted second invocation. The page team can resume
unchanged work through its existing Manage entry, preserving original limits.

For changed inputs, dates or process, create a new commitment ID with
`supersedes` naming the original. The original promise remains inspectable;
this field records lineage and does not cancel it or release an unknown fence.
Do not claim exactly-once external effects or treat a new ID as a recovery.

## Verify

```sh
make check-team-process                 # from the Bench checkout
# Or run this directory's tests using an already selected public Tend:
PROCESS_TEST_TEND=/absolute/bin/tend PYTHONDONTWRITEBYTECODE=1 \
  python3 -m unittest discover -s tests -v
```

Tests use real Tend and deterministic team fixtures, and exercise the actual
team entry/delivery adapters separately. They cover calendar boundaries,
duplicate admission, unknown fencing, business holds, source/input drift,
acceptance versus timeliness, and rejected/stale completion. Existing team
suites cover their underlying handoffs. These checks establish composition;
they do not establish model quality, business truth or an improved deadline
success rate. Live jobs use the caller's separately configured model account.

The public commands return 0 for valid results, including empty planning output
or a status showing unfinished/unverified work; 2 for invalid input/operating failure.
The private execution wrapper preserves ordinary nonzero team exits and uses
125 for interruptions, nested unknown execution, or an untrustworthy execution
result. The original team exit remains in its receipt. Stdout contains records or
the team's original stream; diagnostics and final-check text go to stderr.

See [CONTRACT.md](CONTRACT.md) for the input schemas and evidence protocol,
[DESIGN.md](DESIGN.md) for ownership boundaries, and [LICENSE](LICENSE).
