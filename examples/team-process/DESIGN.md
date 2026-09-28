# Team process application

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
- `_execute`: the private submitted wrapper verifies frozen bindings, invokes
  the original team command, and only after exit zero runs the declared final
  verification. It seals accepted artifact identities and the completion time.
  Normal completion checks run here, not in Tend's unknown-resolution `-check`.

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
