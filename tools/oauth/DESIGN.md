# OAuth, the Unix way

## Sentence

`oauth` obtains and refreshes one resource-bound credential, then gives its
authorization header to one exact process on a dedicated descriptor.

## Process contract

```text
oauth discover RESOURCE
oauth login NAME [options] RESOURCE
oauth refresh NAME
oauth status [NAME]
oauth logout NAME
oauth header NAME
oauth with NAME -- COMMAND [ARG ...]
```

`header` is the explicit bytes-to-bytes seam and prints a secret. `with` is the
safe ordinary composition: the child receives the same header on descriptor 3,
while its stdin, stdout, stderr, signals, and exit status remain its own. No
shell is involved and the command is never retried.

## State

Each profile is an inspectable directory under `$OAUTH_HOME`,
`$XDG_STATE_HOME/oauth`, or `~/.local/state/oauth`, in that order:

```text
NAME/
  profile.json       resource, issuer, endpoints, client, scopes
  credential.json    access token, refresh token, client secret, expiry
  lock               stable flock inode
```

The directory is 0700 and both JSON files are 0600. Files are written by
atomic rename. A lock covers refresh read-modify-write; after taking it the
credential is read again before a rotating refresh token is spent.
Profile and credential records also share a random binding value. Readers
reject a mismatched pair, so a crash during replacement cannot silently pair
new authority metadata with an old credential (or vice versa).

## Supported flows

- Authorization Code with PKCE S256 and a temporary loopback callback.
- Device Authorization Grant when advertised by server metadata.
- Client Credentials for machine identities.
- Refresh Token rotation for every flow that receives one.

Client authentication is `none`, `client_secret_basic`, or
`client_secret_post`. A public native client uses `none` plus PKCE. Secrets are
read from stdin or an operator-owned file, never argv.

## Discovery

The resource is resolved through RFC 9728 protected-resource metadata. Its
authorization server is resolved through RFC 8414 or OpenID Connect discovery.
The selected resource, issuer, endpoints, supported PKCE methods, grants,
client-auth methods, and scopes are retained in the profile. Explicit endpoint
flags are available for deployments that do not publish metadata; explicit
values are still checked before they are stored and again before use.

## Deliberate refusals

- No token in a URL, argv, environment variable, log, or model-visible file.
- No password grant or implicit grant.
- No automatic replay after refresh, reauthorization, 401, or 403.
- No accepting a non-Bearer token as if it were a Bearer token.
- No cleartext remote authorization, token, device, or metadata endpoint.
- No following redirects with a token request containing a code, refresh
  token, or client secret.
- No sharing Ask's private credential file or importing provider-specific
  headers into the OAuth mechanism.
