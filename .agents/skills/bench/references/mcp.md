# Extend a harness with MCP capabilities

Read `tools/mcp/README.md` in the selected checkout. Bench already supplies
the protocol edge: `mcpserve` exposes a program; `mcp`, `mcp-legacy`, and
`mcpbox` consume or admit service capabilities. Do not add an MCP SDK to the
expert runner, another provider client, or a new server framework.

## Expose a program

Start with `tools/mcp/examples/filter-server` for a complete offline example.
Install the `mcp` component and, from the checkout's `tools/mcp` directory:

```sh
printf '%s\n' '{"name":"hello","arguments":{"name":"Unix"}}' |
  mcp request tools/call -- \
  mcpserve examples/filter-server/manifest.json -- examples/filter-server/dispatch
```

The manifest declares the tool and its JSON input schema. MCPserve appends the
method to the dispatcher's literal argv, supplies params JSON on stdin, and
expects one MCP result object on stdout. Diagnostics go to stderr. The SDK
already owns discovery and framing.

For a real capability, write only the input/output adapter the application
needs. Select fixed programs and reviewed paths; never turn a caller's argument
into a shell command. An expert adapter supplies input files in a fresh
workspace, invokes Agent with literal argv and bounds, and maps its artifact
and exact outcome into the declared tool result. Preserve separate records.
Long work needs an explicit task/continuation contract; do not claim a timed-out
request was canceled or automatically retry it. A2A already supplies a remote
agent task interface when that fits the job better.

## Prove the public interface

Test a valid request, invalid input, tool-level rejection, and subprocess
failure through the actual MCP client and server. Check JSON structure and
artifact contents, not a success phrase. A result's `isError` is distinct from
transport/process success. Keep stdout free of startup banners and progress.
If the target host uses a legacy MCP lifecycle, test with `mcp-legacy` too;
select `mcpserve -allow-legacy` for that server. Do not assume a modern-client
pass establishes host compatibility. The SDK owns both protocol paths.

## Attach it to this harness

Use the host setup reference for its connection mechanism. A local stdio
registration has an absolute `mcpserve` command plus absolute manifest,
dispatcher, and resource paths in its argument array. A remote connector needs
an endpoint reachable from the actual host environment and the selected
authentication. Do not give Cowork a localhost URL on the user's laptop.

Create or update only the requested server entry, preserving existing settings.
Verify the host lists the capability, then make a harmless representative call
through the host before claiming it works there. If host-side access is absent,
leave the tested server, exact registration, and the remaining connection step;
do not report it as installed or connected.

## Consume a service instead

`mcpbox make` discovers a service into a folder. Inspect and admit only the
capabilities needed. Direct tools and Action connectors grant different paths:
use the latter for operations requiring Action policy and May decisions.
OAuth handles resource-bound login and credential handoff independently.
Descriptor discovery is not permission, and an uncertain request is not safe
to repeat simply because the process returned an error.
