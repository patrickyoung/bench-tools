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
