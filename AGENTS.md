# Development guidance

`oauth` owns OAuth login, token refresh, and the narrow transfer of an HTTP
authorization header to an explicitly invoked Unix process. It is a sibling of
Ask and MCP; neither protocol client owns its credential lifecycle.

- Credentials never appear in argv, environment variables, diagnostics, status
  output, authorization URLs, or ordinary command output.
- `oauth with` invokes literal argv without a shell, passes one header on file
  descriptor 3, preserves the child's standard streams, and never retries it.
- Bind every credential to an exact resource, issuer, client, and requested
  scope set. Never send a token to another resource.
- Authorization-code login uses PKCE S256, a random state value, an exact
  loopback redirect, and issuer validation when the server advertises it.
- Token and registration requests never follow redirects. HTTPS is required
  except for exact loopback endpoints.
- Refresh happens before invocation, under a per-profile `flock`. Re-read after
  taking the lock so a rotating refresh token is spent once.
- Definition and secret state are separate atomic files in a mode-0700 profile
  directory. Secret files are mode 0600. Refuse symlinks and multiply-linked
  files.
- Discovery grants no authority. Discovered resource and issuer identities must
  exactly match the URL from which their metadata was derived.
- stdout is requested data, stderr is interaction and diagnostics, and exit
  status is the outcome. Login interaction never prints a token.
- No daemon, proxy, SDK, endpoint registry, hidden retry, ambient browser
  session, global token cache, or provider-specific behavior.
- Run `go test ./...` and `go test -race ./...` before reporting success.
