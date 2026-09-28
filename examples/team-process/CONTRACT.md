# Process and commitment contract v1

The application accepts UTF-8 JSON objects with unique keys, no NaN/Infinity,
at most 2 MiB and depth 32. Public input schemas reject unknown fields. Paths are
ordinary files; symlinked input, source and runtime paths are refused. Explicit
program selectors may be symlinks: invocation spelling is preserved for virtual
environments and basename-sensitive programs, while resolved target identity
and bytes are independently pinned.

## Reusable process: bench.team-process/v1

Required fields:

| Field | Meaning |
| --- | --- |
| `id`, `team`, `owner_role` | Stable process, exported team and roster role identities |
| `purpose` | Brief-independent outcome |
| `inputs` | 1–16 logical primary input names |
| `stages` | 1–32 ordered descriptions with `id`, `role`, earlier `needs`, `outcome`, `evidence` |
| `entry` | Existing team `argv`, optional `stdin` input name, run `environment` bindings, and `continuation` (`new-run` or `resume`) |
| `verification` | Literal argv for the final delivery verifier |
| `artifacts` | Required final relative files whose bytes are sealed on acceptance |
| `configuration` | Allowed environment selectors, typed `text`, `program` or `file` |
| `agreements` | Requirements and enforcement (`entry`, `verification` or `guidance`) |
| `escalation` | Guidance for the named business owner |

Optional `inspection` is a read-only command for unsuccessful team outcomes.
It emits an object with `outcome`: `unknown`, `unfinished`, `needs-input` or
`needs-revision`. An unavailable inspection leaves execution unknown. A successful
inspection does not release a nested unknown fence or establish acceptance.

Entry, verification and inspection select an inventoried executable `bin/`
path. Remaining argv may use whole-argument `{run}` or `{input.NAME}` placeholders.
No shell interpolation occurs. Entry environment can only map declared variable
names to `{run}`. The application does not execute `stages` individually: they
document the existing entry's process and handoff expectations.

Verification runs only after entry exit zero. It returns 0 for accepted delivery,
1 for rejected delivery, or another status for broken verification. Team-specific
adapters preserve stronger integrated gates instead of treating structural
validity as delivery acceptance. The final commitment checker, when selected,
runs after this verifier.

## Concrete work: bench.commitment/v1

Required: `id`, `namespace`, `owner`, `objective`, `not_before`, `due_at`,
`timezone`, `inputs`, `environment`, `pass_env`, `milestones`, and `schema`.
Timestamp fields require RFC3339 seconds and an explicit offset. The timezone
is an IANA name for calendar interpretation/display; concrete timestamp offsets
remain explicit. Due time must not precede not-before time.

`inputs` maps every logical primary name to an absolute selected file. Optional
`supporting_files` maps up to 64 relative sibling paths to absolute selected
files. Admission snapshots the complete set, bounded to 64 MiB. A comparison
job's material/dataset paths therefore retain their relative meaning. No
implicit directory discovery or fetching occurs at admission.

`environment` contains only declared nonsecret configuration. `program` selectors
pin the selected path, resolved target and hash; `file` selectors pin reviewed
external module bytes. `pass_env` declares credential names, never values;
it cannot override pinned configuration or interpreter/loader settings. The
child receives only those selected names, pinned configuration, run bindings,
and the admitted baseline PATH/HOME/LANG/LC_ALL/TMPDIR. Dependencies selected
through PATH remain the team's existing runtime prerequisite; declare program
selectors when their identity must be pinned individually.

Milestones contain unique `id`, named `owner`, `due_at` within commitment dates,
and `expectation`. These are business obligations, not executable substeps.
The objective summarizes tracking; actual job requirements must be in the
primary brief/job. Optional `additional_check` selects `argv` plus an explicit
`files` list for support code. Its absolute executable and listed file bytes
are pinned. Only `{run}` and `{commitment}` placeholders are expanded there.

Optional `occurrence` has `schedule_id`, local ISO `date`, and canonical
`calendar_sha256`; it requires the absolute selected `calendar` file. Admission
recomputes the proposal and retains the parsed calendar. Optional `supersedes`
records an older commitment ID; it does not cancel that case, change its dates
or release its execution fence.

## Calendar: bench.team-calendar/v1

Required: `schema`, `id`, `timezone`, `start_date`, `end_date`, `weekdays`,
`excluded_dates`, `start_time`, `due_time`, `due_day_offset`, `missed`.
Date range is inclusive and at most 366 dates. Weekdays use Monday=0; local
times use HH:MM. Due-day offset is 0–30. Ambiguous/nonexistent local instants
are rejected. Exclusions are explicit ISO local dates.

Planner output includes only occurrences whose start is at or before as-of.
`all` retains all; `latest` retains the latest; `skip` excludes those past due.
Occurrence ID is calendar ID/local date. Repeated evaluations preserve identity;
changing calendar bytes changes the binding, not the ID. Due equality is on time.
The planner's proposed process hash covers canonical process JSON; actual
admission additionally verifies the complete exported team lock and source.

## Registered calendars and explicit dispositions

`register-calendar EXPORT REGISTRATION ROOT` accepts
`bench.calendar-registration/v1`: required `schema`, `namespace`, `owner`,
`calendar` (selected file), `effective_from` (ISO local date), `reason` and
`check_every_seconds` (integer 30–604800). Optional `previous` selects the latest
revision hash for updates. The export must pass its original team lock. The
record retains parsed calendar/process snapshots and team/commit/lock identity.
Identical last payloads are idempotent; conflicting concurrent changes fail.

The initial effective date equals start_date. Later effective dates are within
their revision's calendar, strictly after today in that timezone and strictly
after the preceding revision's effective date. Timezone and team identity stay
fixed. Every date before the next effective date belongs to its original
revision. All expected occurrences are retained independently of `missed`.
Registration is limited to 100 calendars per root and 100 revisions per history;
the calendar view is bounded to 20,000 expected occurrences and 1,000 commitments.
Views flag upcoming horizon expiry (14 days), expiry and uncovered renewal gaps.

`record-disposition ROOT REQUEST` accepts `bench.calendar-disposition/v1`:
required `schema`, `namespace`, `calendar_id`, occurrence `id`,
`calendar_revision`, `kind` (`skipped`, `cancelled`, `reopened`), `by`, `reason`;
optional `previous` selects the last disposition hash. The selected expected
occurrence/revision must exist. Skipping admitted work is refused. Reopening
requires a preceding non-reopened decision. Cancellation is a recorded business
decision only: conflicting execution stays visible and Tend retains authority.
Changed-calendar dispositions remain conflicts instead of becoming silent waivers.

Records live under `ROOT/calendars/IDENTITY/NNNNNN.json` and
`ROOT/dispositions/IDENTITY/NNNNNN.json`. Identity hashes namespace and calendar
or occurrence ID. Each record has `schema` (`bench.calendar-revision/v1` or
`bench.disposition-revision/v1`), `sequence`, `previous` (previous record SHA-256,
or null), actual `recorded_at` and `payload`. The returned `revision` is the
record hash, not another stored field. Gaps, malformed records and chain errors
are visible failures. No history deletion, automatic retention or authority
authentication is implied. Caller-owned permissions/backups protect these files.

## Calendar views and monitoring

`calendar ROOT --as-of TIME [--milestones FILE] [--format json|html|ics]` is
read-only. `bench.team-calendar-view/v1` contains `as_of`, actual `observed_at`,
`input_sha256`, `monitor`, `calendars`, `obligations`, `attention`, `errors`,
and `notifications_sent` (false). It includes expected work, all admissions and
retained decisions, including those outside the current registered schedule.
Missing obligations never look complete. Source/date mismatches, failed/waiting
execution, unknown effects, stale evidence and overdue milestones remain visible.
Work carries separate `state`, `execution`, `acceptance` and `timeliness` fields.
Milestone input follows the existing board mapping. As-of projects deadlines
against currently observed facts, not historical execution reconstruction.

HTML is standalone, escaped, searchable and grouped into calendar months in
each event's local timezone. Its freshness banner expires client-side without
network requests. ICS emits escaped/folded VEVENTs for obligations and milestones
using distinct typed identities and UTC instants. Both outputs are snapshots;
neither is a subscribed calendar, authenticated portal or live synchronization.

`reconcile ROOT [--milestones FILE]` inspects actual current time and writes only
`ROOT/last-reconciliation.json` (`bench.reconciliation/v1`): `completed_at`,
`input_sha256`, obligation count and attention count, plus schema. Tracking
digests must agree before/after inspection and under the root's writer lock
before checkpoint replacement. Unverified records fail without refreshing the
last successful check. Missing/late work remains actionable but does not mean
the check itself failed. A current monitor does not mean all work is complete.

Freshness uses actual observation time and the minimum check interval currently
effective across calendars. Its state is `never-reconciled`, `current`, `stale`,
`changed` or `unverified`; exact equality with next-check time is still current.
Future revisions cannot relax today's cadence. Input digest covers registration
and disposition histories and admitted commitment/binding bytes; public Tend
and artifact observations are read on each invocation. The digest does not make
execution static between checks. The host must invoke reconciliation and publish
views periodically; a separate external observer must detect a stopped host.

## Authority and recorded evidence

One standing-team root owns immutable admissions. Its `tend/` directory belongs
only to the public Tend command. No application code opens the database. Stable
job IDs derive from namespace and commitment ID. `submit` has a fixed literal
command, admitted digest, cwd, not-before time and serialization key. Repeating
it is safe; changed bytes fail. The host runs Tend and supplies TEND_PASS and
execution duration limits separately.

The private wrapper refuses changed inputs/source/programs before execution.
It records original exits and verification, hashes accepted final artifacts,
and binds its receipt digest into Tend's retained stderr. Status verifies
public attempt-event output digests, the receipt marker and the admitted task
identity before accepting that evidence. `tend work` exit zero alone is never
delivery proof. Signal/interruption conventions and nested unknown outcomes
remain unknown in the outer queue.

Inspection returns independent execution, acceptance and timeliness dimensions.
Accepted timestamps establish on-time/late; current time establishes pending/
overdue for incomplete work. Missing or changed final artifacts become stale.
Invalid source, records or selected milestone evidence become an unverified row
with an error and escalation candidate. This preserves visibility without
inventing an outcome. Inspection does not run checkers or authenticate a human
attestation. Authority rests with the selected trusted controller and tools,
not with hashes alone or untrusted worker prose.
