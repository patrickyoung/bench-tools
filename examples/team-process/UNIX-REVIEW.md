# Accountable work as reusable Unix capabilities

Status: architecture review, 2026-09-29; first extraction now implemented in
`tools/agenda`, `interfaces/agenda`, and this directory's runner/team/human
adapters. The recommendation below is retained as the review record; proposed
command spellings are superseded by each component's manual. Existing admission
adoption is supported without re-pinning. Bulk legacy calendar/history migration
and integration into the separate bench-hire application remain future work.

## Conclusion

The original decision to reuse team commands, Tend, Weave and May was sound.
The blanket decision that no new reusable tool was needed no longer fits the
scope. Calendar accountability, durable promises and human activity history
have grown into useful capabilities trapped inside one application.

Extract public command boundaries before expanding the web interface. Preserve
the existing functionality and safety tests; do not move the entire application
under `tools/`, turn it into a generic workflow engine, or merely divide its
Python file into modules. Another caller must be able to use the capability
without adopting this application's team exports, directory layout or UI.

Independent Thompson-inspired and Pike-inspired reviews reached this shared
conclusion. These are engineering review perspectives, not statements from
the actual people. The reviewers differed on extraction order and whether
calendar expansion should initially ship separately. Tool count is subordinate
to a clear, independently usable contract.

## Concrete problems in the current boundaries

1. `admit` pins the whole `process.py`; `verify` checks that pin. HTML, CSS and
   polling code therefore participate in execution identity. A presentation
   edit can invalidate an existing commitment unless its complete original
   application remains available at the admitted path.
2. `register_calendar` requires a pinned team export. Calendar arithmetic and
   ordinary human obligations have no inherent dependency on a team roster.
3. `record_activity` calls the full calendar view while holding the writer
   lock. A business-record mutation consequently depends on live Tend/May
   inspection and application-wide reconciliation.
4. `assignment_snapshot` understands Page Team's manifest, Manage schema and
   brief representation. That knowledge is legitimate in a team adapter, but
   should not be required by a reusable board or work-record tool.
5. Business facts, execution observations, monitoring, rendering and HTTP
   serving share one implementation and release boundary. A command-line entry
   point alone does not make these responsibilities independently reusable.

The separate `bench-hire` application also has implemented cadence calculation,
routine records and scheduling. Its reviewed source at commit
`631194190f9690091d0eacb837bf5905c073f7cb` contains `schedule.go` (`nextDue`,
`occurrenceID`), `store.go` (routine persistence), and `runner.go`
(`queueScheduledRoutine`). This is a concrete second consumer, distinct from
this repository's `interfaces/hire` UI. It is not a dependency to import.
Its Sunday-based weekdays, hourly/duration cadences and occurrence identities
differ from this application's contracts; migration must translate deliberately.

## Responsibility map

| Responsibility | Owner |
| --- | --- |
| Worker execution and context | Agent and existing team entry commands |
| Finite dependency readiness | Weave, where a team uses that graph contract |
| Attempts, timers, waits, signals, uncertain effects | Tend |
| Exact human approval and single-use grants | May |
| Connector-operation gating | Action, when a connector effect is involved |
| Invocation recording and replay | Record, when retained invocation evidence is required |
| Team-specific procedure, continuation and delivery acceptance | Existing team/controller and its small adapters |
| Promises, owners, business dates, schedule revisions and human reports | New independently usable accountable-work capability |
| Finite calendar expansion | Pure reusable capability, independent of execution |
| Kanban lanes, page layout, HTML/ICS exports, live refresh and forms | Applications such as bench-hire |
| Reminder delivery and host invocation cadence | Explicitly configured application/host integrations |

Standard processes and working agreements remain reusable team source.
Descriptive stages do not become executable merely because a UI displays them.
A requested new workflow must be implemented through the team's existing
commands and, when suitable, Weave and Tend. Business deadlines are distinct
from Tend's wake times and execution budgets.

## Proposed core contract

Introduce one accountable-work component first, with a name selected when its
public contract is finalized. Its one responsibility is to record what is owed
by whom and when, and account for that work against supplied observations.
Stateful commands can be good Unix components: May and Tend already are.

Keep two capabilities separately usable even if they initially share one
component release:

- **Typed durable records.** Create promises; revise explicit assignments and
  schedules; record dispositions, supersession and attributed human reports;
  retain selected evidence; export a complete snapshot/history. An explicit
  caller-owned root selects state. Mutations use stdin JSON, expected revisions
  and stable identities. Success is emitted only after durable persistence.
  Identical retries recover lost replies; conflicts remain explicit. This is
  a domain record tool, not an arbitrary JSON database or event framework.
- **Pure expansion and projection.** Given finite declarations, an explicit
  evaluation time and observation records, emit versioned JSONL describing
  expected work, missing observations, deadline status and conflicts. This
  path needs no writable root, current clock, team export or execution tools.
  Calendar expansion must also be callable alone. Emit every obligation;
  catch-up policies such as latest/skip select execution separately.

Proposed command shapes, not installed commands:

```text
<work-tool> apply --root DIR < typed-change.json
<work-tool> export --root DIR > complete-snapshot.jsonl
<work-tool> expand --from DATE --through DATE < schedules.jsonl
<work-tool> project complete-snapshot.jsonl --as-of INSTANT < observations.jsonl
```

Use stdout only for records and stderr for diagnostics. Validate bounded finite
inputs before projection output. Proposed exits are 0 for valid results,
including overdue/missing work; 1 for invalid data or revision conflicts; 2 for
usage/I/O failure. Output can still be interrupted: a caller must check the
exit before treating redirected output as a usable snapshot. Existing tool
exit contracts remain unchanged.

The component knows no Page Team, Manage directory, `team.lock.json`, provider,
worker argv, approval state or browser. Source/controller bindings are opaque
references. Human completion remains an attributed report. Agent acceptance,
May approval, execution success and timeliness remain distinct facts.

Do not let `apply` execute programs or inspect controllers. Validate business
record structure, revision consistency and explicitly selected evidence bytes
locally. Querying remote/live facts belongs in observation adapters.

### Calendar packaging decision

Finite calendar expansion is a credible separate filter: this application and
bench-hire both use it. Start with an independently testable pure interface,
then extract a separate executable when both consumers exercise the same
contract. Keeping expansion inside the first work-tool release avoids making
two writers or duplicating schedule-revision rules merely to increase the
number of programs. A separately packaged filter is also acceptable once the
completeness seam below is proven. Extraction of business authority must not
be postponed while only the easy date arithmetic is separated.

## Completeness is part of the interface

Reconciliation cannot accept an arbitrary bag of expected rows and report a
healthy check. An omitted calendar could otherwise disappear without a trace.

- Export the complete registered schedule generation, every revision and
  effective boundary, and the finite coverage horizon.
- Bind expected output to that generation, including schedules producing zero
  occurrences. Preserve coverage gaps, expired horizons and skipped executions.
- Identify required observation sources and record missing, failed, stale and
  unknown observations explicitly. An unavailable adapter must not look like
  an empty successful result.
- Keep actual observation time separate from the requested business as-of time.
  Replaying an old export must not make it fresh.
- A monitoring checkpoint records a complete consistent inspection, not proof
  that all obligations are done. The host still invokes checks; an external
  observer is needed to detect a stopped host.

Adapters verify tool-specific evidence before constructing observations, as
with Weave today. The core validates the supplied identity, generation, evidence
bindings and coverage contract. Hashes and a source name do not authenticate
a factual claim; the caller must explicitly select and trust its adapters.

## Applications and adapters

The team execution adapter validates exports, binds inputs and literal commands,
executes the original entry, and verifies delivery. Its stable version and
receipts are pinned independently of calendars and renderers. Existing checks
remain authoritative; this extraction must not replace acceptance with a
generic exit-zero or model-reported completion.

Tend/Manage observation adapters produce normalized, evidence-bound records.
Rendering consumes those records and the core projection. A saved snapshot
must render without opening queues or invoking Manage. Live UI refresh collects
new observations and renders the same contract; watching only ledger mutations
would miss independent changes in running jobs.

The May/Tend response bridge stays an explicitly callable application adapter:
prepare a response durably, invoke May's terminal decision or publish selected
input, then signal the bound Tend wait. The resumed controller still checks
May or validates input. Preserve deduplication, refusal, lost-reply recovery
and unknown-outcome behavior. Do not make `apply` or moving a card an approval.

The current read-before-signal sequence cannot provide an atomic condition on
the observed wait. If that stronger guarantee is required, assess a narrowly
scoped conditional-signal extension in Tend itself. Its own transaction and
public contract must own that behavior. Do not simulate it in a generic ledger.

bench-hire should invoke these public executables with literal argv and bounded
records. It owns user interactions and presentation, while the tools own their
records and invariants. It must not import another component's implementation,
read private state or duplicate business transitions in JavaScript.

## Migration without losing the work

1. Preserve the current 91-case behavior baseline, schemas and retained legacy
   records. Keep the prototype available as a reference, not a second evolving
   authority.
2. Separate the versioned execution/receipt adapter from rendering. New
   admissions pin execution code only. Existing admissions still require their
   exact original executable and paths; never rewrite old hashes, silently
   repin them, relocate queues or resubmit jobs during migration.
3. Extract typed business records and pure projection behind public commands.
   Exercise a human-only process and a Bench-team process without a browser.
   Make the example call the component as an executable, not import its code.
4. Provide explicit read-only legacy adaptation and, if necessary, an offline
   importer that preserves original records, identities and provenance. Select
   one authoritative store after migration. No dual writes or automatic
   execution/approval effects.
5. Make static renderers and live viewers consume exported records. Integrate
   bench-hire through those commands rather than copying prototype internals.
6. Prove shared recurrence behavior with bench-hire's existing cadences before
   replacing them. Preserve IDs, timezone/weekday semantics and catch-up policy;
   then choose whether its pure interface merits its own executable release.

## Release gates for the revised design

- Build, install, version and test the new component outside this checkout,
  without sibling source or a running application.
- Track a human-only process without Tend, May, models or a team export.
- Pipe exported records into a shell report and a web renderer; preserve the
  same missing-work, history and deadline findings in both.
- Change CSS and refresh behavior without invalidating admitted execution.
- Delete an expected-calendar group or required observation from a test export;
  reconciliation must report incomplete coverage rather than healthy emptiness.
- Retain all existing DST, revision, evidence-corruption, uncertainty, human
  response, acceptance and concurrency behavior through public executable tests.
- Prove bench-hire integration without reading the component's private store.

PR #36 proves useful behavior and has passing scoped and cross-platform CI
checks at its implementation head. Passing tests do not settle these ownership
boundaries. The extraction and app integration above remain future work; the
current prototype should not be presented as the final reusable architecture.
