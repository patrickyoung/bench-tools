# oauth

OAuth at the edge; ordinary Unix inside.

`oauth` logs a person or machine into one exact resource, refreshes rotating
credentials safely, and gives one exact child process an HTTP Authorization
header on file descriptor 3. It is not an HTTP proxy, SDK, daemon, identity
provider, browser host, or secret service.

## Install

Requires Go 1.26 or newer.

```sh
./install.sh
```

The default destination is `$HOME/.local/bin/oauth`. Use
`./install.sh -prefix DIR` to install as `DIR/bin/oauth`.

## The shortest MCP use

First log in. The resource publishes its authorization server and endpoints;
the CLI uses authorization code with PKCE S256 when available:

```sh
oauth login github -client-id YOUR_CLIENT_ID https://mcp.example.com/mcp
```

That form is for a **public CLI client**, where PKCE replaces a client secret.
If the MCP provider issued both a client ID and a client secret, keep the
secret out of shell arguments and supply it on stdin:

```sh
read -s -p 'Client secret: ' secret; printf '\n' >&2; \
  printf %s "$secret" | oauth login github \
    -client-id YOUR_CLIENT_ID \
    -client-auth client_secret_basic \
    -client-secret-stdin \
    https://mcp.example.com/mcp; unset secret
```

There is deliberately no `-client-secret VALUE` option because process
arguments are visible to other local tooling and commonly retained in shell
history. The secret may instead come from a protected file with
`-client-secret-file`.

Then run the exact MCP command. The access token exists only in the pipe
behind descriptor 3:

```sh
oauth with github -- \
  mcp request -header-fd 3 tools/list -- https://mcp.example.com/mcp
```

`oauth` refreshes before starting `mcp` when necessary. It never retries
`mcp`; a possibly effectful request remains the caller's decision.

## Public browser clients

Native command-line applications are public clients: an embedded client
secret would not remain secret. They use authorization code with PKCE S256:

```sh
oauth login docs -flow code -client-id public-cli \
  -scope 'files:read files:write' https://docs.example.com/mcp
```

The callback listener binds to a random port on `127.0.0.1`, checks a random
state, uses the exact redirect URI, and validates the response issuer whenever
it is present or advertised as required. `-no-browser` prints the URL without
opening it, which is useful over SSH.

## Device authorization

When server metadata advertises a device endpoint:

```sh
oauth login tv -flow device -client-id public-cli \
  https://device.example.com/api
```

The verification URI and user code go to stderr. Polling honors
`authorization_pending`, `slow_down`, expiry, cancellation, and the login
timeout.

## Confidential and machine clients

Never put a client secret in argv. Read it from stdin:

```sh
secret-command | oauth login build \
  -flow client-credentials \
  -client-id build-worker \
  -client-auth client_secret_basic \
  -client-secret-stdin \
  https://build.example.com/api
```

Or use an operator-owned file whose mode is 0600 or stricter:

```sh
oauth login build -flow client-credentials \
  -client-id build-worker \
  -client-secret-file /run/secrets/build-oauth \
  https://build.example.com/api
```

`client_secret_post` is available for authorization servers that require it.
`none`, `client_secret_basic`, and `client_secret_post` are checked against
server metadata when it advertises supported methods.

## Providers without resource metadata

Modern discovery is preferred. For a provider that does not publish RFC 9728
and RFC 8414 metadata, make every boundary explicit:

```sh
oauth login legacy \
  -issuer https://login.example.com \
  -authorization-endpoint https://login.example.com/authorize \
  -token-endpoint https://login.example.com/token \
  -client-id public-cli \
  https://api.example.com
```

Explicit endpoints are still validated before storage and again before use.

## Commands

```text
oauth discover [flags] RESOURCE
oauth login NAME [flags] RESOURCE
oauth refresh [flags] NAME
oauth status [NAME]
oauth logout NAME
oauth header [flags] NAME
oauth with [flags] NAME -- COMMAND [ARG ...]
```

`status` is tab-separated and never prints credentials. `header` deliberately
prints a secret header for expert pipeline use; prefer `with`, which cannot
accidentally send it to an ordinary stdout log.

State lives under `$OAUTH_HOME`, `$XDG_STATE_HOME/oauth`, or
`~/.local/state/oauth`. Each profile is a directory containing separate
definition and credential JSON files plus a stable lock. See
[SECURITY.md](SECURITY.md) and [DESIGN.md](DESIGN.md).

## Supported standards and boundaries

- OAuth 2.1 authorization code behavior with PKCE S256.
- OAuth 2.0 Protected Resource Metadata (RFC 9728).
- OAuth 2.0 Authorization Server Metadata (RFC 8414) and OIDC discovery.
- Resource Indicators (RFC 8707).
- Authorization Server Issuer Identification (RFC 9207).
- Device Authorization Grant (RFC 8628).
- Client Credentials and Refresh Token grants.
- Bearer Token Usage (RFC 6750).

The implicit and password grants are deliberately absent. DPoP, mutual-TLS
sender-constrained tokens, token exchange, private-key client assertions,
dynamic client registration, and provider-specific login imports are not
silently approximated; a server requiring one is refused until that contract
is implemented explicitly.
