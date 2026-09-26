# Independent human interfaces

Keep each interface independently buildable, testable, versioned and usable.
Read the selected application's AGENTS.md and DESIGN.md before changing it.

- Put application code, assets, tests and documentation in its own directory.
  Do not introduce a shared Go module, workspace, runtime or component import.
- Compose tools through literal executable arguments, separate streams, exact
  exit status and documented files. Source proximity grants no authority.
- Keep model clients, action loops, tool approvals and confinement in the
  public programs that own them. Do not recreate these inside an interface.
- Make source selection and writable runtime state explicit. Keep current
  inputs, results, logs and credentials outside the reusable source library.
- Use semantic server-rendered HTML, responsive CSS, visible keyboard focus,
  labeled forms and honest loading, empty, error and completion states.
- Prefer standard-library Go HTTP and templates, embedded assets, and small
  progressive enhancements. Add dependencies only for a demonstrated need.
- Bound request input and process output. Treat source text and command output
  as untrusted display content; never turn them into executable HTML or shell.
- Preserve interruption and unknown outcomes. Never retry model calls or
  effects automatically because a browser disconnected or a process restarted.
- Verify each application's standalone build and actual public-command
  boundaries. Keep live paid-model evaluation explicitly separate from tests.
