# Standing team commitments

This is an experimental, independently runnable application, not a new Bench
tool or an interpreter for team stages. Read DESIGN.md before editing.

- Invoke existing team entry commands and Tend using literal argv.
- Tend owns attempts, waits, serialization and uncertain effects. Never open
  its database, retry jobs, signal waits, or resolve unknown outcomes here.
- Team entry commands own their internal workflow and acceptance. A roster,
  milestone report, model claim or passing structural check is not acceptance.
- Kanban projects existing records. Human activity completion is an attributed
  report with retained evidence; it never accepts or unblocks agent work. Read
  assignments only through bound existing controller observations, not invented
  stage tasks. Optional live serving is loopback-only and read-only.
- The application owns immutable commitments and their source/input bindings.
  Runtime data stays in explicitly selected directories outside source.
- Calendar expansion and inspection are finite, read-only and use explicit
  evaluation time. Recurrence proposes occurrences; it never submits them.
  Registered calendar history and explicit dispositions are separate writes.
  Reconciliation records a successful actual-time observation only after a
  consistent read. Missing work must remain visible regardless of catch-up policy.
- Keep configuration values and credential names separate. Never retain secret
  environment values. A caller supplies execution authority and isolation.
- Run unit tests and real Tend integration tests. Label deterministic team
  fixtures separately from actual team contract tests and live model trials.
