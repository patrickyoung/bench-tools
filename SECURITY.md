# Security

## What is stored

Each named profile has a mode-0700 directory. `profile.json` and
`credential.json` are mode 0600. The latter may contain an access token,
rotating refresh token, and—only when the operator supplied one—a client
secret. Writes are fsynced and atomically renamed.

State readers open files with `O_NOFOLLOW`, require one hard link, require a
regular file, enforce private permissions, and bound input to 1 MiB. Refresh
uses a stable `flock` inode, re-reads after acquiring it, and saves the rotated
token before releasing it.

The profile and credential records share a fresh random binding value. A
reader refuses a mismatched pair, so an interrupted or concurrent profile
replacement fails closed instead of combining one profile with another
credential.

## Secret movement

- Client secrets enter from stdin or a protected file, never argv.
- Tokens never enter argv, environment variables, authorization URLs, status
  output, login output, or diagnostics.
- `oauth with` sends one header through a kernel pipe inherited as descriptor
  3 by one literal child process. It does not invoke a shell.
- `oauth header` is an explicit secret-export command. Its stdout must be
  treated as a credential. It exists for Unix composition but is not the
  recommended ordinary path.

The invoked child is inside the grant: it can read descriptor 3 and can pass
the credential to its own descendants. Do not put `oauth` itself in a model's
toolbox. Put only reviewed wrappers around exact authenticated operations
there.

## Network behavior

- Authorization, metadata, token, and device endpoints require HTTPS. Exact
  loopback HTTP is allowed for local callbacks and test/development issuers.
- Discovered private, loopback, link-local, unspecified, and multicast network
  addresses are refused by default. `-allow-private` is an explicit operator
  decision for internal identity systems.
- HTTP redirects are never followed. This is especially important for token
  requests whose body can carry an authorization code, refresh token, device
  code, or client secret.
- Responses are bounded to 1 MiB and network operations have explicit
  timeouts.
- Metadata resource and issuer values must exactly match the identities from
  which they were discovered.
- Tokens are requested with the exact RFC 8707 `resource` value and are
  delivered only under the profile selected by the operator.

## Login behavior

Authorization-code login uses fresh cryptographic randomness for PKCE and
state, requires PKCE S256 support, listens only on `127.0.0.1`, and validates
the authorization-response issuer. Device login bounds polling by both the
device-code expiry and the command context.

`oauth` refreshes an expired credential before starting a protected command.
It never repeats the command after a 401, 403, refresh, scope upgrade, broken
connection, or uncertain effect.

## Reporting

Report exploitable issues privately through the repository's GitHub security
advisory page once published. Do not paste credential files, authorization
headers, callback URLs, or raw token-endpoint responses into an issue.
