# Security

An agent home is code and model input. Inspect it with `agent show` and
validate it with `agent check` before running it.

## What the default run confines

Model-authored command blocks run through Cage. On a supported backend they
may persist filesystem writes only beneath `work/`, `state/`, and Cage's
private temporary directory. Host networking is denied unless the caller
passes `-net`. Backend setup failure returns 125 and never falls back to an
unconfined action.

Agent creates that action temporary directory beneath a private run directory
outside the agent home and pins Cage's `TMPDIR` to it. A caller-supplied
`TMPDIR` inside the home is rejected. The action wrapper also uses the exact
Cage executable resolved before Ply prepends the agent-controlled toolbox
to `PATH`. The native interpreter scans hard links itself using filesystem
metadata, so a toolbox program cannot replace that pre-confinement check.

Toolbox entries may be symlinks to reviewed programs, but Agent resolves each
one during validation and rejects targets beneath model-writable `work/` or
`state/`, non-executable/non-regular targets, multiply-linked targets, and an
unbounded catalogue. This prevents one run from rewriting a tool synopsis or
verifier-path program that would gain controller/system-prompt authority on
the next run.

`agent check` and the action wrapper conservatively refuse every regular file
with more than one hard link under `work/` or `state/`. Hard links alias
inodes, so pathname-only write policy cannot safely distinguish an outside
name. Copy such files to fresh inodes before running the home.

Definition files and `.agent/` evidence are outside Cage's writable roots.
The home cannot grant itself network access, choose `-no-cage`, select a
model, or change its action boundary through Markdown.

A child is a foreground process with explicit input and separate context.
Ordinary recursion requires a caller-selected host boundary. Agent preserves
Ply's inherited depth, model selection and approval gate. A custom inherited
action interpreter is refused rather than rebound to different mutable roots.
Default Cage does not grant nested provider or controller-evidence access.

With `-C`, the definition stays separate from the selected workspace, mutable
state, and controller evidence. Agent refuses overlapping definition/evidence
and mutable roots, validates controller subdirectories before creating state,
and leaves reusable definition files unchanged. `check` and `show` accept the
same path flags without creating those roots.

Authoring, learning and home-controller effects are separate Hire commands.
See [Hire's authoring boundary](../hire/SECURITY.md). Agent scrubs controller
connector and home-maintenance authority variables from model execution while
retaining any inherited Ply action approval gate.

## What it does not confine

Cage is not a confidentiality, identity, CPU, memory, process-count, syscall,
or credential boundary. The model action can read files and inherited
environment values available to the current operating-system identity and,
with `-sh`, can invoke the host's programs. Run under an appropriately narrow
identity and environment; use a container or virtual machine when reads or
secrets must be isolated.

`-net` permits host networking generally; it is not a destination allowlist.
`-no-cage` deliberately restores ordinary host write and network reach.

## The verifier is controller-authority code

`bin/check` runs outside the model-action Cage so the worker cannot rewrite
its judge. If the check executes scripts, binaries, build hooks, or other code
from `work/` or `state/`, that code executes with the controller's authority.
Keep checks operator-owned, inspect their transitive execution, or run the
whole controller under a stronger outer boundary for adversarial work.

Ask sessions and Ply verifier receipts under `.agent/runs/` are evidence of
the exact conversation and verdict. They prove replay integrity and the
recorded check outcome, not that the model, home, or external data was
trustworthy.

Named checkpoint pointers live under `.agent/checkpoints/`, outside Cage's
writable roots, and may name only sessions under the same home's
`.agent/runs/`. Agent rejects symlinked, malformed, oversized, or escaping
pointers before invoking Ply. Ply locks the checkpoint for the whole run and
publishes pointer changes durably. A checkpoint preserves conversation
context; it neither rolls back the work tree nor proves whether an external
effect interrupted in flight happened.
