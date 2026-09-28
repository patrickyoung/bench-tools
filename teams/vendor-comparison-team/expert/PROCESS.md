# Vendor evaluation operating agreement

`process.json` describes the fixed five-stage process already executed by
`bin/compare-team`: Product Manager framing, Polars analysis, comparison,
Product Manager synthesis, and independent Product Owner review. The same
manager definition acts in two fresh contexts. Existing checked handoffs own
progress; this description introduces no second stage executor or scheduler.

The reusable process contains roles, outcomes, handoff expectations, configuration
names, final verification and delivery artifacts. Actual owners, commitments,
due dates, milestones, daily calendars and current case data live outside the
source export. The operating owner reviews blockers and dates each working day;
that agreement is guidance until supported by explicit evidence.

Supply the complete current job as input `job`, plus every selected local
material/dataset as a supporting file at its original path relative to the job.
Include actual decision expectations in the job's business case, criteria and
gates. The commitment objective is a tracking summary, not another prompt.
Inputs are snapshotted at admission. A later source change cannot silently
change that case. Remote material acquisition retains the existing team's
explicit behavior and occurs at execution; use selected local snapshots when
the evidence must be frozen before admission.

`bin/check-process RUN` invokes the existing full structural/handoff checker
and additionally requires final status `reviewed` and independent review
`proceed`. The original `bin/check` intentionally accepts coherent packages
whose verdict still requires revision or input; those are not completed delivery.
A structurally valid recommendation is still advice, not purchase authority.

Business due time is independent of Agent's `COMPARISON_TIMEOUT`, turn budgets
and the outer Tend duration limit. A valid late delivery stays visibly late;
deadline pressure cannot upgrade confidence or remove a reviewer objection.
The application reports escalation candidates; it sends nothing externally.

The entry requires a new run directory. Exit 75 preserves a need for input, but
does not make the command safely resumable. Inspect records and create an
explicitly superseding commitment with selected new inputs and a fresh run.
Never blindly wake/retry the old command. Unknown execution must be resolved
deliberately under Tend's own rules before conflicting work can proceed.
