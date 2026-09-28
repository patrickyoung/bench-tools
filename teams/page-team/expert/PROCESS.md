# Page delivery operating agreement

`process.json` describes this team's existing process for the optional standing
team application. It does not replace `bin/page-team` or interpret the manager's
task graph. The manager selects useful specialists; Frontend integrates;
independent browser and visual review covers the exact HTML and original brief.
Manage's original root check owns final acceptance.

The reusable process contains roles, outcomes, handoff expectations, configuration
names, final verification and delivery artifacts. Actual owner names, dates,
milestones, daily calendars, credentials and current briefs belong in external
commitments. The operating owner reviews blockers and dates each working day.
Instructions about this daily review are guidance, not proof that it happened.

For each commitment, supply the complete current brief as input `brief`. Include
the actual task-specific expectations in that brief; the commitment's objective
is a tracking summary, not an extra prompt silently sent to specialists. An
operator may select an additional acceptance command for requirements beyond
the existing team gate. Existing team instructions and checks remain in force.

`bin/check-process RUN` queries the configured public Bench Manage inspection
command for an accepted root result, then verifies that its accepted manifest,
source HTML and delivered `result.html` agree. It does not substitute the simpler
HTML `bin/check` for integrated browser/visual acceptance or rerun model review.
It is also independently usable by an operator after a normal page-team run.

Business due time is separate from `PAGE_TEAM_DEADLINE` (the admitted execution
budget), action timeouts and the outer Tend duration limit. An overdue page may
still be running. An accepted page delivered late remains visibly late. Never
lower review standards to meet a date. Escalation is an unsent report to the
named owner until the caller deliberately communicates it.

The existing page entry supports unchanged-run resume through Manage. The
process application does not retry or resolve unknown work itself; an operator
must inspect the original evidence and follow the existing continuation rules.
Changing the brief, process or requirements requires an explicit new commitment
and run, retaining the old promise and any unresolved execution fence.
