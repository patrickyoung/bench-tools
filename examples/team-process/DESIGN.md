# Team adapters and accountable work

New roots use the independent Agenda tool and Agenda UI interface. Read their
own DESIGN.md files for record, projection, view and MCP contracts. The
[Unix architecture review](UNIX-REVIEW.md) explains the extraction.

`runner.py` owns only the pinned admission/execution/receipt contract of this
application. Its source contains no web rendering or HTTP server. Existing team
commands and Tend retain all execution authority. `team.py` invokes the runner
and Agenda as public subprocesses; admission binds one external Agenda item to
an immutable execution, and status becomes an expiring supplied observation.
Changing the business item cannot silently reuse acceptance for a new promise.

`human.py` is a separate application adapter connecting human Agenda items to
existing Tend waits and exact May requests. These links and responses do not
change human business reports or external acceptance. Agenda never reads May,
Tend, team directories or model state. Calendar and Kanban never execute a team.

New admission pins the runner, not the viewer. A partial Agenda admission is
recoverable through stable IDs before any submit. Existing admissions can be
indexed without re-pinning or resubmission. Legacy roots, calendar histories and
the original `process.py` remain available; no unverified history translation or
dual-write migration is performed. This is an explicit compatibility boundary,
not a claim that all legacy app features migrated into the first core release.

## Retained v0.4.0 design

The remainder documents the original `process.py` application and legacy roots.

## Outcome

A standing team can publish its standard process and working agreement, bind
concrete work to owners, inputs, expectations, milestones and business dates,
plan daily or weekly occurrences, submit the existing team command once, and
inspect execution, acceptance and timeliness independently.

The first two process definitions describe the page team and vendor comparison
team. They retain their existing public commands, handoffs and completion gates.
Stages describe the existing implementation; this application does not execute
individual stages, plan specialist tasks or interpret a second dependency graph.

## Boundaries

The portable Python standard-library application lives entirely in this
directory. It imports no other Bench implementation. It invokes Tend through
its public executable and runs reviewed team entry/check commands. Weave,
Agent, Manage, providers, approvals and execution confinement remain where
they already live. There is no required daemon or new installed core filter.

Reusable `expert/process.json` and `expert/PROCESS.md` are approved team source
and travel in the existing pinned exports. Concrete commitment, selected
configuration, source/input hashes and receipt files belong outside the source
library. The original team lock remains authoritative for exported source.

## Operations

- `validate`: inspect a process definition without executing its commands.
- `plan`: expand a finite local calendar with explicit timezone, weekdays,
  excluded dates, local start/due times and missed-occurrence policy. Emit
  occurrence proposals; caller supplies current inputs and admits chosen work.
- `admit`: freeze a commitment and snapshot its selected input files, pin the
  exported team, application, Python and Tend bytes, and retain exact argv and
  configuration. Repeated identical admission returns the same directory;
  conflicting reuse of its ID fails. A local advisory lock serializes writers.
- `submit`: pass that immutable request to `tend submit -id` with a stable ID,
  not-before timestamp and caller-selected standing-team serialization key.
  Safe to repeat after a lost submission response; never executes a job.
- `status`: use public Tend observations and sealed output identities to show
  execution, team acceptance, deadline/milestone lateness, and escalation
  candidates. Inspection never invokes models, team checks or execution.
- `register-calendar`: retain an owner-attributed calendar/process snapshot in
  an immutable revision chain. Future effective dates preserve prior obligations;
  changed source or dates never rewrite admitted commitments.
- `record-disposition`: retain an explicit skipped/cancelled/reopened business
  decision with a reason. This grants no execution cancellation or recovery.
- `calendar`: reconcile all expected occurrences with admitted work and decisions,
  independently of catch-up policy. Render JSON, self-contained HTML or an ICS
  snapshot. Include missing work, milestones, source conflicts and monitor age.
- `reconcile`: perform the same current-time observation, verify stable tracking
  inputs before/after inspection and under the admission lock, then atomically
  checkpoint the last successful check. Exceptions are findings; unverifiable
  records prevent a successful checkpoint. The host supplies periodic invocation.
- `_execute`: the private submitted wrapper verifies frozen bindings, invokes
  the original team command, and only after exit zero runs the declared final
  verification. It seals accepted artifact identities and the completion time.
  Normal completion checks run here, not in Tend's unknown-resolution `-check`.
- `record-activity`: append an attributed human activity revision linked to an
  existing commitment/occurrence, with explicit assignee, date, reason and selected
  completion evidence. History is retained separately from agent execution.
- `kanban`: project the same calendar/commitment/activity records into five work
  columns, retaining exceptions and cancelled/skipped history. Read existing
  Page Team assignments through explicitly selected public status observations.
- `serve`: optionally publish this read-only projection on loopback and refresh
  it every few seconds. No write endpoints, scheduler, required daemon or remote
  hosting authority is introduced. Connection freshness and periodic
  reconciliation freshness remain distinct.
- `link-human`: bind an existing activity to one current Tend signal wait and,
  for approval, the exact pending May request. Retain job/wait identity and
  selected executable pins. The operator explicitly selects the controller;
  linking does not change team wiring or install a new gate.
- `respond-human`: retain an attributed response, invoke May's own terminal
  decision or publish selected input to a write-once mailbox, then send a
  stable, deduplicated Tend signal. Never consume a grant, run a worker, retry
  a job or resolve an unknown outcome. The resumed controller validates input
  or asks May for the identical action and consumes its one-use grant.

Human links are operator-owned adapters. They use public Tend list/events/signal
and May pending/decide, with no database or approval-state writes. May selects
the OS account independently of HOME; approval links pin that operator UID.
Each interaction requires a unique signal name and immutable link. Tend has
no expected-wait compare-and-set; rechecking before wakeup narrows races but
does not replace controller validation. Signal receipt and actual wakeup are
reported separately. An absent May pending record never establishes approval.
The board observes linked work but cannot approve through a status change.
Generic human completion remains a report, separate from linked job completion.

Tend's stdout/stderr capture remains intact. The original team exit is retained
in the receipt. Ordinary exits propagate; interruptions and nested unknown
execution become 125. A read-only team adapter inspects unsuccessful outcomes.
Status reads the wrapper's receipt to distinguish a team needing
input/revision from a broken verifier. Missing trusted completion remains
unaccepted even if a job is marked done by an external operator.

## Time and working agreements

Business due time, execution not-before time, and execution budgets are distinct.
Team-specific execution limits are selected through the existing team settings;
the outer Tend operator sets TEND_JOB_MAX. Business lateness does not kill a
process, weaken a check or authorize a retry. Explicit as-of is a projection
time, not permission to execute early or a reconstruction of historical state.

The calendar supports bounded weekday/daily/weekly patterns and explicit
holiday exclusions, with catch-up-all, latest-only or skip-missed policies.
Ambiguous or nonexistent local start/due instants fail rather than silently
choosing a DST interpretation. Identity is schedule ID plus local date, stable
across ticks and schedule edits; changed bytes conflict at admission. Each
occurrence binds the calendar bytes. There is no infinite task graph.

Registered schedules enumerate all occurrences, including upcoming and skipped
catch-up proposals. First registration can expose a past backlog. Revisions
take effect after today; every earlier date retains its prior definition.
Existing future commitments and waivers that disagree with a new revision stay
visible as conflicts. Expiry, approaching horizon and uncovered renewal gaps are
reported. Active business horizon follows as-of; monitoring cadence follows the
revision effective at actual observation time, never a future relaxed interval.

Calendar and disposition records are immutable, hash-chained revisions with
optimistic previous-revision checks and serialized writes. Newly created parent
links and complete records are synced before success. The replaceable monitoring
checkpoint is derived observation data, not task authority or completion proof.
Views are read-only, report stale or absent checks and never manufacture success.
Static HTML and ICS are timestamped snapshots; the HTML freshness banner expires
without fetching anything. An external observer is necessary to notify users
when the host itself stops. There is no installed scheduler, calendar account
connection or implicit notification channel.

The operating agreement declares owner roles, review independence, required
deliverables and escalation expectations. Enforced checks are distinguished
from guidance. Milestone confirmations are explicit controller-selected
evidence bindings in a separate input snapshot; their existence does not
establish team completion. Unsupported stage progress stays unknown.

## Failure and authority

An instance's exact definition is durable before submission. Its stable Tend ID
is derived from the standing-team namespace and commitment ID, not the input
hash; changing inputs must never evade duplicate admission detection. Same-key
unknown work remains fenced by Tend even when another occurrence is queued.

Subsequent submissions do not retry or resolve. Deliberate continuation uses
the existing team's rules: page-team supports unchanged-run resume; comparison
requires a fresh run. The wrapper refuses repeat invocation for new-run teams.
Business changes require a new, explicitly superseding commitment, preserving
the old promise and evidence. No cancellation, deadline amendment, waiver or
notification is silently performed.

Checks and commands are trusted operator-selected code. Hashes detect drift,
not malicious administrators. The application does not establish confidentiality
or sandboxing. Credentials are passed by selected names only. Source pins do
not freeze external services, native packages, model output or business truth.

## Verification required

Unit coverage: strict bounded JSON; missing/stale inputs and source; finite
calendar and DST edges; stable occurrence IDs; missed-tick policies; separate
late/accepted states; milestones and escalation; changed accepted artifacts.

Executable integration: real Tend submit/work/show/events/check; concurrent
duplicate and conflicting admission; lost submit response; exit 2/75/125;
unknown fencing; verifier rejection; source drift before execution; two distinct
team contracts; unchanged team stream/exit semantics. Existing team fixture
suites cover their actual handoffs separately. No paid model run is implied
by deterministic orchestration coverage.
