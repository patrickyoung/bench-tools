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
Cage and filesystem-scanner executables resolved before Ply prepends the
agent-controlled toolbox to `PATH`; toolbox programs therefore cannot run in
the wrapper before confinement.

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

`agent specialist` is a new foreground controller invocation, not a process
escape offered to a running parent. Each child receives caller-selected
authority and writes only its own work, state, and run evidence.

`agent learn` deliberately runs outside Cage because it may amend a local
skill definition. It requires an explicit portable skill name, accepts only a
regular non-symlinked session beneath the same home's `.agent/runs/`, rejects
symlinked skill directories, and delegates the fail-then-pass verdict to Hone.
Treat it as a controller-authorized definition amendment, never an automatic
end-of-run hook.

`agent history` is read-only and delegates archive parsing to Trail. The
wrapper scopes archive operations and session paths to the selected home's
`.agent/runs/`, refuses symlinked or outside session files, and delegates
replay checking to Ask. Session output is sensitive and is not redacted.

`agent amend` also runs outside Cage because it is the controller operation
that can change definition. It accepts only one existing root definition file
per regular, non-symlinked proposal beneath `work/proposals/`, refuses a
multiply-linked target, and dry-runs with Git before asking. May binds a human
answer to the physical home, current definition hash, target, proposal path,
proposal hash, and stated effect. A grant is single-use. Agent rechecks those
inputs after spending it, validates the whole home after applying, rolls the
target back on any validation or evidence-publication failure, and records a
receipt outside the model-writable roots. May is not passed into Ply's
environment, and no Markdown instruction can approve a request. Review the
actual diff and exact May action before deciding; approval proves consent to
those bytes, not that they are wise.

`agent proposals` is the read-only inspection half of that boundary. It uses
the same parser and exact-action builder as `amend`, but never resolves or
invokes May and never writes a receipt. The catalogue is capped at 16 portable
`.patch` names, 32 KiB per file, and 64 KiB combined so a writable work tree
cannot turn the TUI into an unbounded output sink.

`agent actions` and `agent act` provide the equivalent split for external
effects. The worker may write a strict `{version, connector, input}` proposal
under `work/actions/`; that file grants no executable, policy, approval mode,
credential, or connector path. Read-only review validates it through Action's
public parser without resolving a connector. `agent act` is an explicit
controller invocation outside Cage: it selects `AGENT_ACTION_PATH`, policy,
May, and Ask; binds a stable job to the current definition and proposal hashes;
and records Action's typed receipts in an existing home Ask session. Action,
May, and connector-path environment are scrubbed before Ply starts. Exit 125
means the effect may exist and must not be retried automatically.

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
