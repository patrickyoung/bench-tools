# Moniker design

One call reserves one readable team name. The caller chooses a private registry
explicitly; no default path, home discovery, model, network, daemon or shared
runtime is involved. The independent module imports only Go's standard library.

The CLI emits a name or the four-field JSON result. Diagnostics use stderr;
usage and runtime failures have distinct exit statuses. The MCP adapter is a
Unix dispatcher, not a server. The separate `mcpserve` owns discovery and
transport using the static manifest. The dispatcher accepts one tool and one
optional theme in bounded stdin JSON, never filesystem authority from tool
arguments. Reservations are writes and are deliberately non-idempotent.

## Reservation invariant

Generated display names use fixed ASCII words with one space between words.
Optional suffixes contain eight lowercase hex digits. Lowercasing and replacing
spaces with hyphens maps each possible display name to one slug. The path
`SLUG.json` is opened with `O_CREATE|O_EXCL` inside an anchored `os.Root`.
Existing entries, including incomplete reservations, occupy that spelling.
There is no read-then-create race, process-local lock or scan used for exclusion.

The process checks that the selected registry is a real 0700 directory, writes
the record with mode 0600, syncs the file and directory, then prints the result.
Creation is the reservation boundary. A later error never releases it: another
process may already rely on the name being occupied. A lost stdout response is
an uncertain reservation, not grounds for replay or automatic reclamation.

Collision work is bounded: 32 dictionary candidates, then 16 suffix candidates.
IDs use 128 random bits; suffixes use 32 random bits and still undergo exclusive
reservation. Name exclusion holds in an intact selected registry; IDs are random
identifiers, not cross-registry coordination. Users must retain records to retain
the guarantee. Other processes with the same filesystem authority can change
the registry; Moniker is not a multi-user access-control service.

## Verification

Offline tests exercise process invocations, private-path refusal, exact result
shape, concurrent reservations, restarts, collision fallback and bounded
exhaustion. Adapter tests verify strict JSON, no authority in arguments, MCP
error results and ordinary dispatcher errors. Tests use temporary registries
and bounded fake entropy for controlled collisions. Run unit, race and vet
checks plus standalone builds. Adapter changes also need public MCP integration.
