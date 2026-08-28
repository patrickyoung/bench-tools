# Development guidance

Rob Pike is the bar, and `context`, `ask`, and `ply` are the siblings. `cite`
checks the literal seam between retrieved evidence and a model-written answer;
it does not become a source system or a judge.

The whole program is four steps: read Context records, index ref-to-URL pairs,
read a candidate, print it unchanged if every ctx ref is an exact cited link.
If a change makes that sentence longer, it needs a very good reason.

When changing `cite`:

- **preserve the identity-filter contract.** Successful stdout is the exact
  candidate bytes. Rejection stdout is empty. Diagnostics go to stderr;

- **keep evidence and candidate separate.** The evidence is one regular file
  argument because stdin is the candidate Ply supplies to its check. Do not add
  a second ambiguous input mode;

- **literal identity is the whole judgment.** Require exact
  `[ref](citation.url)` strings, reject every other `ctx:` occurrence, and
  require at least one. Never claim that this proves support, coverage, truth,
  freshness, authority, or safe rendering;

- **conflict is broken evidence.** One ref with two URLs is exit 2, not
  first-wins. A record without a URL may remain in evidence but cannot be cited
  as a Markdown link;

- **preserve the exit contract:** 0 valid, 1 rejected candidate or no citeable
  evidence, 2 usage, malformed evidence, I/O failure, or a bound exceeded. Ply
  treats 1 as feedback and any other nonzero as a broken checker;

- **buffer before printing and never truncate.** A bad final ref must not leave
  a plausible accepted prefix on stdout. Every bound is documented and tested;

- run `go test ./...`, `go test -race ./...`, and `go vet ./...` before
  reporting success;

- keep help, README, GUIDE, SECURITY, and `cite.1` true. Tests guard the command
  set, limits, version, help width, and portable ASCII manual.

Things left out on purpose: retrieval, connector discovery, a model, prompts,
semantic entailment, claim extraction, authority policy, automatic repair,
Markdown rendering, HTML sanitization, a config file, a cache, a database, a
daemon, a server, and a session format. Context retrieves, Ask writes, Ply
retries, and Brief can state policy.
