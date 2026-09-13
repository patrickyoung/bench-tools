# One worker, existing protocol edges

These commands assume an installed, unchanged expert copy and exported tool,
model, browser and creative-capability configuration from the main README.
Set `PAGE_EXPERT` to that copy's absolute `expert` directory. Ordinary local
execution requires no listener. Protocol listeners remain operator-managed.

## MCP

The server manifest describes one `build_page` tool. The dispatcher translates
its arguments, calls the same page filter, and reports an MCP tool error for an
unfinished build. A protocol error is distinct from a failed application run.

```sh
export PAGE_TEAM_MCP_RUN_ROOT=/absolute/private/mcp-runs
printf '%s\n' '{}' | mcp request tools/list -- \
  mcpserve "$PAGE_EXPERT/interfaces/mcp/manifest.json" -- \
  "$PAGE_EXPERT/interfaces/mcp/dispatch"
```

Save a request with a fresh `run_id` and actual `brief` in `request.json`, then:

```sh
mcp request tools/call -- \
  mcpserve "$PAGE_EXPERT/interfaces/mcp/manifest.json" -- \
  "$PAGE_EXPERT/interfaces/mcp/dispatch" < request.json > response.json
```

The request shape is:

```json
{"name":"build_page","arguments":{"run_id":"page-001","brief":"The actual page brief"}}
```

The successful response contains HTML in `content[0].text`. Reusing the same
run ID with the same brief resumes its retained run; a different brief fails.
Do not automatically retry an unknown outcome. Use `-allow-legacy` only when
the MCP host needs the earlier lifecycle; the [MCP manual](../../tools/mcp/README.md)
explains that explicit compatibility choice.

## Authenticated A2A and REST

Use the [existing A2A server](../../tools/a2a/README.md). Provide issued server
and client certificates, a private state directory, and an mTLS policy file:

```json
{"mode":"mtls","clientCA":"client-ca.pem"}
```

With your actual hostname, paths and model credential variable, start:

```sh
a2aserve -listen 0.0.0.0:8443 -public-url https://worker.example:8443 \
  -state /absolute/private/page-service -auth /absolute/private/auth.json \
  -tls-cert /absolute/private/server.pem -tls-key /absolute/private/server.key \
  -timeout 1h -pass-env AGENT -pass-env BENCH_MANAGE \
  -pass-env TEND -pass-env WEAVE -pass-env PAGE_TEAM_MODEL \
  -pass-env OPENAI_API_KEY -pass-env PAGE_TEAM_BROWSER \
  -pass-env BLENDER -pass-env GIMP_CONSOLE -pass-env PAGE_TEAM_IMAGEGEN \
  -artifact controller/result.html "$PAGE_EXPERT/interfaces/a2a/card.json" -- \
  "$PAGE_EXPERT/interfaces/a2a/dispatch"
```

Replace `OPENAI_API_KEY` with the names required by your existing provider
configuration. For a selected credential wrapper, pass `AGENT_ASK` and any
nonsecret selectors that wrapper requires. The observed setup passed
`PRESENT_ASK` and `PRESENT_IMAGE_CODEX_MODEL`; these are optional Present bindings,
not requirements of Agent. Pass any intentionally changed page limits by name.
The caller's mTLS or OAuth identity never becomes a model credential.

Send a unique message using the existing client:

```sh
a2a request -ca ca.pem -cert client.pem -key client.key \
  send https://worker.example:8443/rpc < message.json > task.json
```

`message.json` contains:

```json
{"message":{"messageId":"page-001","role":"ROLE_USER","parts":[{"text":"The actual page brief"}]}}
```

The server exports HTML as a text artifact and `controller/result.html` as an
inline file artifact. Retain the returned task ID. Save `{"id":"RETURNED_ID"}`
in `task-id.json` to query the same task through its REST API:

```sh
a2a request -transport rest -ca ca.pem -cert client.pem -key client.key \
  get https://worker.example:8443/rest < task-id.json
```

The observed local TLS evaluation completed a fresh build, matched both
artifact hashes, queried it through REST, rejected a client without a
certificate, rejected an unrelated browser Origin, and hid the task from a
different valid client identity. It did not deploy a public service or test a
second physical machine. OAuth uses the existing server policy and OAuth tool;
see its manual rather than adding authentication to this dispatcher.

## Independent specialists and context

Each `expert/agents/NAME` is a valid Agent definition. Supply its assignment
and actual inputs in a separate workspace, select `PAGE_TEAM_TASK_ID`, and run
Agent with a separate evidence directory. The manifest maker uses that task
identity. Keep the containing expert bundle available for shared validators.
Review and image generation have additional fixed external capabilities owned
by the team adapter, documented in the worker README.

The team filter can itself be a subprocess of another externally admitted
worker. Each Agent has its own conversation. Default Cage does not grant
recursive model-network access or controller-write authority, and separate
contexts do not provide host-read secrecy. Use the existing external controller
to establish those separate work and evidence roots.
