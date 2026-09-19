# Development guidance

Weigh reads one explicit request, obtains typed judgments, validates the whole
response, and prints one result. Keep that sentence true.

- Keep this module independently buildable, testable, usable, and versioned.
- Stdout is one validated JSON result. Diagnostics use stderr. Exit 0 means
  valid inference, 1 runtime/provider failure, and 2 invalid invocation/input.
  Low confidence and negative judgments are successful inference, not errors.
- Preserve complete native distributions. Never fabricate, fill, renormalize,
  or convert a selected option into probabilities. Keep provider confidence
  distinct from probability of correctness.
- Make at most one inference HTTP request. No retries, redirects, fallback,
  generated explanations, thresholds, actions, or agent loop.
- Keep credentials off argv, stdin, diagnostics, and results. Private header
  descriptors take precedence over the API-key environment variable.
- Validate and bound input before networking; never truncate. Preserve state
  numeric literals. Reject ambiguous JSON and unsupported contract fields.
- No other component imports, shared runtime, configuration discovery, session
  format, database, daemon, plugin system, or worker-specific behavior.
- Record captures invocations externally. A checker owns threshold policy and
  translates Weigh failures into its own broken-check status.
- Use offline HTTP fixtures, never ambient credentials or paid model calls.
- Run go test ./..., go test -race ./..., and go vet ./... before completion.
- Keep help, README, DESIGN, and weigh.1 consistent with public behavior.
