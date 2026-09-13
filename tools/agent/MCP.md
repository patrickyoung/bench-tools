# Give an expert service tools through ordinary programs

An expert may know how to review a support case but need a policy lookup from
an MCP service. Keep that connection at the edge: provision the reviewed
capability as a program, then let Agent use it through its normal toolbox.

The current implementation lives in the monorepo's
[MCP component](https://github.com/patrickyoung/bench-tools/tree/main/tools/mcp).
It uses the official Go SDK and supplies four commands:

| Command | Responsibility |
| --- | --- |
| `mcp` | One explicit request or foreground event stream using the implemented modern stateless protocol |
| `mcp-legacy` | An explicit client for supported earlier server lifecycles |
| `mcpbox` | Discovery into a folder, inspection, and admission of selected capabilities |
| `mcpserve` | Expose an operator-selected dispatcher program through MCP |

Agent remains the runner. Brief owns skills, Ask the model connection, Ply the
loop and verifier, Cage the action boundary, and Hire the definition builder.
No MCP-specific provider or tool loop is needed in those programs.

## Try the local example first

The [hello-server walkthrough](https://github.com/patrickyoung/bench-tools/tree/main/tools/mcp#make-your-first-request)
needs no model or service account. From the monorepo root, after installing MCP:

```sh
cd tools/mcp
mcpbox make hello.mcp -- go run ./examples/hello-server
mcpbox show hello.mcp
mcpbox admit hello.mcp tools hello
printf '%s\n' '{"name":"Pike"}' | ./hello.mcp/tools/hello
```

Expect an MCP result containing the greeting. The generated program accepts a
JSON argument object on stdin; it does not infer positional shell arguments.
This demo records a relative `go run` endpoint, so keep its working directory.
For deployment, build the server and provision against a stable absolute path.

## Select the capability before running the expert

A real service may offer many tools; admit only the ones the job needs. The
capability folder retains the endpoint, tool name, and reviewed descriptor
digest. A wrapper verifies that descriptor before use. Changed discovery is a
new proposal, not an automatic expansion of access.

Place the reviewed executable tools in the expert's `tools/` namespace using
operator-controlled paths. Agent prepends them to PATH. Its default Cage still
denies action networking, so a remote tool needs an explicitly selected network
boundary or an external controller. A PATH entry grants no escape from Cage.

For a tool that changes an external service, admit an Action connector instead:

```sh
mcpbox admit service.mcp actions create_ticket
```

This assumes an inspected folder exposing `create_ticket`. The controller
selects its `actions/` directory for Action, applies policy, and uses May when
review is required. Admitting the same tool under `tools/` deliberately permits
direct invocation. Server annotations cannot approve their own use.

## Keep protocol state and credentials explicit

Tools, prompts, exact resources, resource templates, completion, subscriptions,
and supported extension methods travel through the MCP client boundary.
Input-required or unfinished task results remain explicit handles and exit 75.
The caller supplies a later request; the client does not answer or retry it
silently. An uncertain transmitted effect is exit 125 and requires inspection.

OAuth provides login and refresh through its separate credential handoff.
Neither the expert folder nor the generated capability folder should contain
a copied service token. See [MCP authentication](https://github.com/patrickyoung/bench-tools/tree/main/tools/mcp#authenticate-without-putting-tokens-in-argv)
for descriptor and generated-wrapper configuration.

Rich content and extension metadata are protocol data, not a terminal UI.
These commands do not render MCP Apps or MCP-UI views. A graphical host still
needs its own extension support and rendering boundary; preserving result
metadata alone does not establish a complete UI integration.

## Expose a tool or expose an expert

Use `mcpserve` when an MCP client should call your program as a tool. The
manifest describes the capability and the dispatcher owns the actual work.
For a remote agent's task lifecycle, identity, continuation, and artifacts,
use [A2A](https://github.com/patrickyoung/bench-tools/tree/main/tools/a2a).
Its listener can invoke Agent through Tend without changing the expert's
local execution loop.

The Unix design question is concrete: can this piece be inspected and used
without the rest? Here discovery is data, a capability is a program, a request
is JSON, and an unfinished task has a handle. Files and process interfaces
make those boundaries available to both people and other programs.
