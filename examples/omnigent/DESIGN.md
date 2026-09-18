# Deployment application boundaries

This example composes existing Unix programs. It adds application admission
and deployment configuration, without becoming a shared Bench runtime.
Durable submission is the recommended local and remote path.

| Executable | Owns | Calls through process boundaries |
| --- | --- | --- |
| `service` | Client ownership, admission limits, JSON request/query operations, explicit recovery orchestration | Tend; `sandbox-job stop` before releasing an uncertain job |
| `service-mcp` | Maps five MCP tool names to ordinary command names and wraps JSON results | `mcpserve`; `service` |
| `service-worker` | Shell repetition | One `service work` transition at a time |
| `sandbox-job` | Job files, selected credentials, Docker invocation and owned-container cleanup | Docker; image commands run Agent or the existing team entrypoint |
| `sandbox-builder.py` | Container-only Hire authoring and bounded candidate export | `hire build`, `hire verify` through public processes |
| `service-chat` | Client-specific Omnigent specification and chat state | Omnigent, configured to launch `service-mcp` |

The source implementations of these executables do not import each other.
`contracts.py` contains only this application's file formats, bounded file
reads, atomic writes and validation. It executes no commands, selects no
ambient state directory, and owns no queue, session runtime or model loop.
No code from an independent Bench tool is imported into this application.

## Streams and status

`service --state DIR OPERATION CLIENT` reads one JSON argument object and
writes one JSON result. Supported request operations are `submit`, `get`,
`list`, `cancel` and `read-file`. Client and state selection are trusted argv;
they are never accepted from request JSON.

Operation success is 0; request rejection is 1; configuration/controller
failure is 2. Errors leave stdout empty and use stderr. A successful query
does not imply a successful job. `work` and `check` retain Tend's contracts.

`service-mcp` translates process status into MCP `isError`. It neither calls
application functions in-process nor decides admission, retry or recovery.
The protocol implementation stays in the existing `mcpserve` executable.
The packaged `service-mcp.json` describes the same five operations.

Tend submits literal argv:

```text
sandbox-job --state DIR run CLIENT JOB SESSION KEY
```

Its stdin is one immutable JSON object with `client`, `session`, `key`,
`request` and `files` (relative paths to base64 bytes). These are application
data, not a shell program. The runner validates them again before creating
files. Its stdout/stderr are the worker's separate raw streams, captured by
Tend. The runner never queries the queue or opens its database.

The runner returns observed worker exit codes, including 75. Tend interprets
bare 75 as a manual wait. Missing trustworthy Docker results, a remaining
container or a signal-like exit are conservatively returned as 125. The
runner never resolves that uncertainty or retries itself.

`TEND_ATTEMPT_KEY` selects an attempt's evidence identity. The runner also
accepts an explicit `--attempt` for operator diagnostics without Tend; this
is an executable test/integration boundary, not a second recommended service
workflow. Such invocations do not provide admission, deduplication, session
ordering or durable outcome recording. The operator assumes those duties.

The original synchronous `run` delegates its prepared sandbox to
`sandbox-job execute`. Installation uses `sandbox-job doctor` and `probe`.
Docker launch policy and worker/team entry-command mapping have one owner.

The optional Matterbridge workflow selects `entry: hire` for the builder and
`entry: agent` for explicitly supplied worker/team candidates. Hire owns
authoring; generic candidate teams must be runnable root experts with their
own specialist handoffs. Builder inputs and the prior selected candidate are
explicit job files. Generated checks execute only inside the job boundary.
`install --without-chat` omits Omnigent; the durable service does not depend on
the chat front end. The source packager remains packaging-only.

The image build reads the packaged deployment selection. Ordinary workers,
teams and candidates receive execution tools only; `entry: hire` adds Hire
for the explicit builder service. Draft and Hone are not installed in either
image. Core tools retain independent builds and public process interfaces.

## Files and authority

`--state` is required and must select an absolute private local directory.
It may be outside the installed package. Initialization records the package's
absolute path and the digest of `deployment.json`; a mismatched package fails
closed. Moving or replacing a package/state directory with admitted work is
not an automatic migration. Existing template queues retain their original
bundle and contracts until reconciled.

```text
selected-state/
  config.json               deployment binding and operator limits
  clients/CLIENT.json       private provider configuration
  tend/                     Tend's authoritative queue and attempt streams
  runs/JOB/                 sandbox files and launch selection
  chat/CLIENT/              optional Omnigent specification
  omnigent/CLIENT/          optional Omnigent settings/conversations
```

Only a selected job's sandbox directory is mounted into Docker. The selected
state directory contributes to the container name and ownership label. Two
queues using the same package and identical client/request keys therefore
cannot target each other's containers during recovery.

`service recover` checks ownership and job status, invokes `sandbox-job stop`,
and only then asks Tend to retry/resolve/wake. A Docker cleanup failure retains
the unknown fence. Tend separately enforces launcher liveness and evidence
sealing. No application status database competes with Tend.

Session names choose Tend serialization keys. They do not introduce a second
model transcript or implicitly share files. Agent/Ply retain worker checkpoint
ownership; team retries use the team's existing entrypoint in a fresh attempt.

## Scale and verification

The execution primitive is separable from its supervisor, but removing Tend
does not establish a throughput gain. There are no throughput benchmarks in
this change. The deployment remains single-host; adding hosts requires explicit
routing and admission elsewhere. The shell/OS supplies worker lifetime and
Tend supplies local durable transitions.

Executable tests cover neutral commands without MCP or Omnigent, the runner
without a Tend binary, real MCP translation, SSH forced identity, independent
state/container recovery, concurrent sessions, output separation and unknown
outcomes. Docker is simulated in these application tests; they do not replace
a native confinement check or a provider-backed worker evaluation.
