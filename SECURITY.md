# Security

Context is an execution and data boundary, not a sandbox.

Every executable on `CONTEXT_PATH` is code the caller has chosen to trust.
Running `context ls` invokes each one with `describe`; running `query` invokes
the named connector. Use project- or user-owned connector directories and do
not include world-writable directories. Files with invalid names or without an
execute bit are ignored.

Context stores no credentials. A connector uses its provider's ordinary
credential mechanism and must preserve the caller's source-system permissions.
Do not put tokens or secrets in result records. Connector diagnostics go to
standard error and may still be captured by a supervisor.

The bundled Wikipedia connector sends its query to English Wikipedia's public
Action API. Its `WIKIPEDIA_API` override changes where that query is sent; use
only an endpoint or proxy you trust. `WIKIPEDIA_USER_AGENT` is sent as an HTTP
header and should contain public contact information, never a secret.

Retrieved content is untrusted data even when its provenance is valid. Keep it
in model input, not system instructions, and assume documents may contain prompt
injection. A citation establishes identity and location; it does not establish
truth, freshness, authorization, or fitness for a particular decision.

Queries and results may be sensitive. They travel through process pipes and may
be written into Ask session history when used as model input. That persistence
is valuable for review but must follow the same filesystem access and retention
policy as the source material. `context query source` reads a query from stdin
when keeping it out of argv matters.

Input is bounded to prevent accidental unbounded reads. Context validates all
query output before printing any of it, so a malformed final record does not
leave an apparently successful prefix on stdout. It does not confine connector
network or filesystem access; use Cage, a container, or the operating system
when confinement is required.
