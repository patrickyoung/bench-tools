# Development guidance

`action` gives one explicitly named connector one exact JSON request only
after an operator-owned policy and, when required, May authorize it. Keep that
sentence true.

- A connector is an executable on `ACTION_PATH`. It supports `describe` and
  `run`; Action contains no provider or MCP branches.
- Discovery and descriptions are data, not authority. The caller names the
  connector and policy. The model never selects or changes either boundary.
- Canonicalize one bounded JSON object before policy, approval, recording, or
  execution. Every later stage binds those same bytes.
- Policy is a program. Exit 0 is allow, 3 is deny, and 75 is exact human
  review through May. Require one strict JSON result matching the action
  digest and exit status.
- May remains the only human approval path. Verify its strict result and exact
  exit status; never add an approval flag, prompt, or model classifier.
- Record proposal, decision, prepared attempt, sent boundary, and terminal
  result as sealed Ask notes when `-record` is selected. A missing terminal
  record after an attempt is conservatively unknown.
- Start the connector behind a closed stdin pipe, seal the attempt, then
  release the exact canonical request. The connector contract forbids effects
  before it has read its complete stdin object.
- Once the request bytes are released, loss, cancellation, oversized output,
  or failure to seal the terminal receipt is exit 125. Never retry.
- stdout is the connector's exact result, stderr is connector diagnostics,
  and exit status is the outcome. Buffer both before publishing stdout.
- Keep Action, policy, May, Ask, and their state outside model-writable
  sandboxes. Agent-facing execution belongs in Agent's controller path, not
  inside its Cage action child.
- Run `go test ./...`, `go test -race ./...`, and `go vet ./...` before
  reporting success.

Do not add a daemon, registry, credential store, MCP client, task scheduler,
automatic retry, hidden policy, model call, sandbox, or second event log.

