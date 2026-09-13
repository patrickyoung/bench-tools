# Hire authoring boundary

Hire's model-backed build is an ordinary Agent run. Agent/Cage owns its
write and network boundary, Ply owns execution, and Ask owns sessions.
The generated definition is an artifact, not a controller authorization.
`hire verify` delegates structural inspection to `agent check`; it never
executes the generated verifier or other generated code. Structural readiness
does not prove that an expert's check is sound or its output is correct.

## Existing recurring-home maintenance

`hire learn` deliberately runs outside Cage because it may amend a local
skill definition. It requires an explicit portable skill name, accepts only a
regular non-symlinked session beneath the same home's `.agent/runs/`, rejects
symlinked skill directories, and delegates the fail-then-pass verdict to Hone.
Treat it as a controller-authorized definition amendment, never an automatic
end-of-run hook.

`hire history` is read-only and delegates archive parsing to Trail. The
wrapper scopes archive operations and session paths to the selected home's
`.agent/runs/`, refuses symlinked or outside session files, and delegates
replay checking to Ask. Session output is sensitive and is not redacted.

`hire amend` also runs outside Cage because it is the controller operation
that can change definition. It accepts only one existing root definition file
per regular, non-symlinked proposal beneath `work/proposals/`, refuses a
multiply-linked target, and dry-runs with Git before asking. May binds a human
answer to the physical home, current definition hash, target, proposal path,
proposal hash, and stated effect. A grant is single-use. Agent rechecks those
inputs after spending it, validates the whole home after applying, rolls the
target back on any validation or evidence-publication failure, and records a
receipt outside the model-writable roots. Home-maintenance May selection is not passed into Ply's environment. An
inherited Ply action gate retains its own May selection; no Markdown
instruction can approve a request. Review the
actual diff and exact May action before deciding; approval proves consent to
those bytes, not that they are wise.

`hire proposals` is the read-only inspection half of that boundary. It uses
the same parser and exact-action builder as `amend`, but never resolves or
invokes May and never writes a receipt. The catalogue is capped at 16 portable
`.patch` names, 32 KiB per file, and 64 KiB combined so a writable work tree
cannot turn the TUI into an unbounded output sink.

`hire actions` and `hire act` provide the equivalent split for external
effects. The worker may write a strict `{version, connector, input}` proposal
under `work/actions/`; that file grants no executable, policy, approval mode,
credential, or connector path. Read-only review validates it through Action's
public parser without resolving a connector. `hire act` is an explicit
controller invocation outside Cage: it selects `AGENT_ACTION_PATH`, policy,
May, and Ask; binds a stable job to the current definition and proposal hashes;
and records Action's typed receipts in an existing home Ask session. Action,
May, and connector-path environment are scrubbed before Ply starts. Exit 125
means the effect may exist and must not be retried automatically.
