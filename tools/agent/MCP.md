# MCP, the Unix way

Bench should use MCP fully without making MCP its internal architecture.

The boundary is simple:

> MCP is a wire protocol at the edge. Inside Bench, it becomes programs,
> streams, files, explicit handles, signals, and exit status.

That preserves both systems. MCP servers remain usable at their complete
protocol boundary. Bench workers continue to see a Unix machine.

## Implementation

The first edge release now lives in
[`patrickyoung/mcp`](https://github.com/patrickyoung/mcp). It implements the
modern stateless stdio transport, exact result preservation, bounded input and
output, explicit progress and subscription streams, honest exit 125 effect
semantics, process-group cleanup, paginated catalogues, staged folder
generation, descriptor-digest admission, runtime verification, executable
Tools and Prompts, exact admitted Resource readers, explicit MRTR continuation,
and extension Task requests.

Streamable HTTP and credential-helper authorization, safe Resource Template
expansion, binary unpacking, Registry proposals, and the reverse `mcpserve`
adapter remain later slices. They do not require a change to the boundary
described here.

## Why the current direction is right

`tools/list` is a directory listing waiting to happen. A tool name,
description, and JSON Schema become a program name, synopsis, help text, and
invocation contract. Putting only selected generated programs on `PATH` is
better than injecting a remote server's entire catalogue into a model runtime.

The standalone edge makes that selection durable with descriptor-digest
admission. Bench programs consume only the resulting capability directory;
they neither discover servers nor depend on the compiler that produced it.

The mistake would be to stop at tools and call every other MCP primitive a
Bench deficiency. The fix is to finish the edge projection.

The current MCP `2026-07-28` specification makes this easier. The core is now
stateless: initialization and protocol sessions are gone; each request is
self-describing; application state travels as explicit handles; server input
requests return as explicit `input_required` results; list and resource results
are cacheable; subscriptions are foreground streams; and long-running work is
the Tasks extension. Roots, Sampling, and Logging are deprecated rather than
features a new Bench adapter should chase. See the
[2026-07-28 release](https://blog.modelcontextprotocol.io/posts/2026-07-28/),
[Tasks extension](https://modelcontextprotocol.io/extensions/tasks/overview),
and [deprecation decision](https://modelcontextprotocol.io/seps/2577-deprecate-roots-sampling-and-logging).

## Two programs, two sentences

Do not turn the existing Python proof into one large MCP host. Split mechanism
from projection.

### `mcp`: the transport filter

`mcp` sends one self-contained MCP request to one explicit server and prints
the exact result.

```text
mcp discover [-timeout D] -- SERVER [ARG...]
mcp request [-timeout D] METHOD -- SERVER [ARG...]
mcp listen [-timeout D] -- SERVER [ARG...]
```

- stdin is empty or one bounded JSON params object;
- stdout is one lossless compact result record;
- `listen` emits notification JSONL until its pipe closes;
- stderr is diagnostics and server stderr;
- an optional explicit event descriptor carries progress JSONL;
- the endpoint is exact stdio argv or an operator-owned HTTP wrapper;
- credentials never enter argv, generated files, model context, Agent state,
  Ask events, or Tend artifacts;
- it never retries automatically.

The implementation should use the official Tier-1
[MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk). Bench should own
the Unix process contract, not hand-roll JSON-RPC framing, version negotiation,
Streamable HTTP, OAuth, or future extension codecs.

### The namespace compiler (`mcpbox`)

`mcpbox` discovers one server and writes a reviewable directory of Unix
capabilities that call `mcp`.

```text
mcpbox make DIR [-u URL | -- SERVER ARG...]
mcpbox show DIR
mcpbox diff OLD NEW
mcpbox tools DIR
mcpbox prompts DIR
mcpbox resources DIR
mcpbox templates DIR
```

Its durable product is a folder, not a daemon, registry, or database:

```text
github.mcp/
  endpoint.json              endpoint identity; never credentials
  discover.json              exact canonical discovery result
  catalog/
    tools.jsonl
    prompts.jsonl
    resources.jsonl
    templates.jsonl
  admit/
    tools.tsv                name + reviewed descriptor digest
    prompts.tsv
    resources.tsv
  tools/
    search                    generated executable
    create-issue
  prompts/
    review-pr                 generated prompt filter
  bin/
    read                     resource reader
    complete                 argument completion
    listen                   catalogue/resource change stream
```

Generation happens in a private staging directory followed by an atomic
rename. Refresh creates a new directory for `diff -ru`; it never mutates a
live toolbox. A catalogue change may invalidate or remove an admitted program,
but it may never grant a new one.

The directory remains the effective allowlist. The admission files merely
make reviewed deletion reproducible. Each entry binds the capability name to
the digest of its canonical description, full schemas, annotations, server
identity, and endpoint identity. A server that changes a tool without changing
its name therefore produces a reviewable mismatch instead of silently
inheriting old authority.

## The complete MCP projection

| MCP surface | Unix / Bench projection | Owner |
| --- | --- | --- |
| Tools | One reviewed executable per tool; only selected wrappers enter a worker's `PATH` | capability compiler + Ply toolbox |
| Tool input/output schema | Full JSON Schema 2020-12 retained byte-for-byte; simple flat schemas may offer positional convenience; every schema accepts exact JSON on stdin | generated wrapper |
| Text and structured results | Exact result JSON on stdout; an explicit `mcp text` filter may print only supported text blocks and must refuse unhandled content | `mcp` |
| Images, audio, blobs, embedded resources | Lossless result records; a separate `mcp-unpack -C DIR` filter materializes digest-named files and emits a JSONL manifest | `mcp-unpack` |
| Resources and templates | Catalogue JSONL plus an explicit `read URI` filter; Context connectors normalize selected reads into replayable cited evidence | capability directory + Context program |
| Prompts | Operator-invoked filters that emit an attributed prompt envelope; never silently installed as a Skill, system prompt, or prior assistant message | capability directory + caller |
| Completion | An ordinary completion filter for shell/editor use | capability directory |
| `input_required` / MRTR | Exact continuation data on stdout with a nonterminal exit; caller supplies response JSON and retries explicitly | `mcp` + shell/controller |
| Elicitation | Additional input, never confused with authorization | caller; May remains separate |
| Sampling request | Explicit data that an operator-selected adapter may pipe to Ask; never hidden recursive model access | Ask adapter outside `mcp` |
| Roots request | Legacy compatibility from an explicit handler program; no ambient filesystem discovery | operator handler |
| Tasks | Remote task handles remain ordinary data; `get`, `update`, and `cancel` are requests; polling may run under Tend | MCP server + `mcp` + Tend |
| Subscriptions | A foreground JSONL stream whose lifetime is its pipe; gaps cause exit rather than hidden reconnect | `mcp listen` |
| Progress and cancellation | Progress on an explicit event stream; SIGINT maps to cancellation and bounded process-group cleanup | `mcp` |
| Cache hints | Catalogue snapshots retain `ttlMs` and `cacheScope`; cache freshness never changes admission | capability compiler |
| Trace context | W3C trace fields pass through; Ask and Tend evidence remain authoritative local records | `mcp` |
| OAuth and enterprise auth | The standalone [`oauth`](https://github.com/patrickyoung/oauth) filter owns discovery, PKCE/device/client-credentials login, issuer/resource binding, secure refresh, and descriptor-based header transfer; `mcp` remains credential-blind | `oauth` + transport edge |
| MCP Registry | Search produces a proposed endpoint descriptor for review; discovery never installs or runs a server | optional registry filter |
| MCP Apps | Transport UI resources and structured/text fallbacks losslessly; rendering stays in a browser/host outside Bench | external host |

Prompts, Resources, and Tools keep MCP's own control distinction: prompts are
user-controlled, resources are application-controlled, and tools are
model-controlled. Flattening all three into model tools would use more MCP
while understanding it less.

## Honest effect semantics

The most important addition is not another primitive. It is a trustworthy
answer to: did the remote action happen?

Suggested process outcomes:

| Exit | Meaning |
| --- | --- |
| `0` | complete positive result |
| `1` | complete peer/application negative, including tool `isError` |
| `2` | local usage, schema, or protocol failure proven before transmission |
| `75` | valid but unfinished: `input_required` or a nonterminal Task |
| `125` | request may have reached the server, but no trustworthy terminal result arrived |
| `130` | interrupted before transmission, or after a confirmed cancellation |

`mcp` maintains one irreversible `sent` bit at the successful write boundary.
EOF, timeout, malformed framing, wrong response ID, duplicate terminal result,
stream loss, or child death after that point is `125`. No annotation such as
`readOnlyHint` weakens this rule, and no request is automatically retried.

That composes directly with Tend's existing `unknown` state. Tend supervises
the process; it does not need an MCP table. A remote Task handle can be written
to a file and polled by shell, cron, or a Tend job. MCP server durability and
Tend process durability remain separate, honest facts.

## Input required is data, not a callback

An MCP server must not gain silent access to Ask, May, the terminal, roots, or
ambient credentials. When a request returns `input_required`, `mcp` prints the
complete `inputRequests` and `requestState`, exits nonterminal, and stops.

An optional handler directory is explicit composition:

```text
handlers/
  elicitation
  sampling
  roots
```

Each handler reads one request JSON object and writes one response JSON object.
Only supplied handlers are advertised. A boolean elicitation saying “confirm”
is still not May approval: May binds a human decision to the exact action bytes
and remains outside the MCP server's control.

For an indefinite human wait, persist the continuation record, use Tend to
wait for an explicit signal or response artifact, then invoke the continuation
as a new process. No held-open hidden callback and no new workflow engine are
needed.

## Export Unix in the other direction

Full MCP use is bidirectional. After the client edge is solid, add a small
foreground CGI-like server adapter:

```text
mcpserve -stdio CAPDIR
mcpserve -listen 127.0.0.1:PORT CAPDIR
```

`CAPDIR` contains explicit method handlers and standard JSON Schema sidecars.
For each request, `mcpserve` starts one handler in its own process group. The
handler reads params JSON on stdin, writes one result JSON object on stdout,
uses stderr for diagnostics, and receives cancellation as a signal. Reverse
proxies own public TLS, authentication, rate limits, and deployment.

Do not infer a rich MCP tool from every program on `PATH`. When no specific
schema is supplied, the honest generic schema is an argv array plus optional
stdin. Never auto-export the whole host toolbox.

Tend can back the Tasks extension without being changed: the server adapter
may translate an explicitly configured handler's Tend job ID into an MCP Task
handle and delegate status/cancel/result to Tend commands. The mapping must
retain Tend's honest `unknown` state rather than pretending every local job
status has an exact MCP equivalent.

## Authority invariants

- Discovery and registry metadata are untrusted descriptions, never authority.
- Tool annotations never choose May policy, Cage roots, or network access.
- The generic `mcp request`, arbitrary `rpc`, server selector, credential
  helper, May, and Cage never enter a model toolbox.
- Generated wrappers pin the endpoint, server identity, method, capability
  name, and reviewed descriptor digest.
- External JSON Schema references are disabled unless explicitly admitted.
- Resource URIs are data, never interpreted as host filesystem paths.
- Icons, remote resources, and MCP App assets are not fetched automatically.
- A Ply toolbox remains scope, not adversarial confinement. Hard secrecy or
  destination-specific network policy still requires an OS identity,
  container, VM, or stronger Cage adapter.
- Ask remains the only conversation/event writer. `mcp` creates no second
  transcript.

## Pike and Thompson tests

These are design lenses, not attributed quotations.

The Pike test asks whether the protocol becomes a namespace of small,
independently useful pieces. It does: tools are programs, resources are named
readers, prompts are filters, catalogues are directory listings, subscriptions
are streams, and refresh is a new folder plus `diff`.

The Thompson test asks whether the mechanism can be smaller and whether state
is visible. It can: one request is one process; params and results are JSON in
pipes; endpoint reuse is a wrapper program; remote state is an explicit handle;
process lifetime belongs to the kernel; durability belongs to Tend; and an
uncertain effect is `125`, never a hopeful retry.

Both tests reject the same design: a resident MCP host hidden inside Ply with a
private registry, connection pool, callback loop, credential store, task DB,
prompt installer, and second trace format.

## Implementation order

1. Build `mcp discover` and `mcp request` over stdio for MCP `2026-07-28`,
   using the official Go SDK. Preserve all content kinds, unknown `_meta`, and
   full schemas. Prove process cleanup and pre-send/post-send outcomes first.
2. Build the capability compiler on that filter. Support paginated `tools/list`, atomic
   directory generation, descriptor-digest admission, JSON-on-stdin, and
   exact tool results. Remove shallow coercion for complex schemas.
3. Add Resources, resource templates, Prompts, and Completion as separate
   projections. Add Context connector examples rather than teaching Context
   MCP.
4. Add explicit MRTR continuation and Tasks verbs. Compose waits through
   files, shell, and Tend; do not add a poller daemon or task database.
5. Add foreground subscriptions with gap semantics and catalogue refresh that
   can revoke but never silently grant.
6. Add Streamable HTTP and compose authorization through the operator-owned
   `oauth` filter. Support legacy servers only through an explicit
   compatibility process.
7. Add `mcpserve` after the consuming edge is proven. Export only explicitly
   described capability directories.

The first release is deliberately narrow: stdio discovery, paginated
catalogues, exact requests, honest unknown-effect handling, and a rebuilt
capability compiler. That slice establishes the hardest invariant while
keeping the final architecture open to every current MCP primitive and
extension.

## What not to build

No MCP code in Ply, Ask, Agent, Bench, Context, May, Cage, or Tend. No daemon,
connection pool, endpoint registry, hidden cache, credential database,
automatic retry, automatic task poller, automatic MRTR answer, automatic
prompt-to-Skill installation, automatic resource index, universal
program-to-schema inference, MCP Apps renderer, or second event log.

The fix is one protocol filter, one namespace compiler, and later one server
filter. Everything else already has an owner in Unix or Bench.
