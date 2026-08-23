# May

May asks a human before one exact action proceeds:

```sh
printf '%s\n' 'publish release v1.4.2' | may
printf '%s\n' 'publish release v1.4.2' | may release-142
```

The action is stdin, never an argument. At a terminal, May shows its quoted
bytes on `/dev/tty` and asks `y/N`. With a job name, May instead records the
request and exits 75. A human can inspect and decide it later; when the exact
job and action are retried, May atomically spends the one matching grant and
exits 0.

May does not decide whether an action is safe. It binds a human answer to
exact bytes.

## Interface

```text
printf '%s\n' ACTION | may [JOB]
printf '%s\n' ACTION | may request JOB
may pending
may decide DIGEST
may check
may help
may version
```

Without `JOB`, a controlling terminal is required. Stdin is already the
action, so approval is read from `/dev/tty`; a pipe, model, or redirected
stdin cannot answer its own question. `y` and `yes`, case-insensitively, are
the only approvals. Anything else is a refusal.

With `JOB`, May computes lowercase hexadecimal SHA-256 over the exact byte
sequence `may-v1`, NUL, `JOB`, NUL, `ACTION`. This is the public v1 digest
wire contract. A matching grant is consumed by atomic rename before exit 0. If
there is no decision, the exact words and digest are written as a pending
request and May exits 75. A recorded decline exits 3.

`request JOB` performs that identical state transition and writes one strict
JSON object containing `version`, `job`, `digest`, the exact `action`, and
`verdict` (`parked`, `declined`, or `spent`). Its exit status remains
0/3/75/2. This is for a supervisor that must bind the machine result before it
acts; JSON does not add an approval path and cannot answer May's human prompt.

The human side is:

```sh
may pending
may decide 7d6f...the-complete-64-character-digest
```

`pending` emits one JSON object per line. `decide` reloads and validates the
request, displays the quoted job, action, and digest on `/dev/tty`, and asks
`y/N`. It never reads a decision from stdin.

Job names that collide with commands or begin with a hyphen use `--`:

```sh
printf '%s\n' 'rotate signing key' | may -- check
```

## State and audit

State is ordinary files under:

```text
~/.local/state/may/
  pending/       requests awaiting a person
  granted/       single-use grants
  declined/      declined requests
  spent/         consumed grants
  audit.jsonl    append-only decisions and transitions
```

Each state file records the exact words as well as their digest. May validates
both before using a decision. The audit is append-only JSONL and includes the
time, job, digest, exact action, and verdict. There is no state-directory
environment variable or config file: an invocation cannot select a different
store.

The newline from `printf '%s\n'` is one of the approved bytes. A retry must
produce the same bytes. Actions must be non-empty UTF-8 without NUL and are
bounded at 16 KiB. Job names are non-empty UTF-8 bounded at 1 KiB.

## Exit status

| status | meaning |
| --- | --- |
| 0 | a human approved the exact action; a job grant has been spent |
| 1 | `may check` found a failure |
| 2 | bad usage, invalid input, corrupt state, or an I/O failure |
| 3 | a human declined, or no controlling terminal could ask one |
| 75 | the exact job request is durably parked for a human |

A supervisor should park on 75, stop on 3, and treat 2 as a broken gate. It
must not turn any of those into approval.

## Install and check

May targets Unix and WSL, requires Go 1.26 or later, and has no external
dependencies. Windows builds honestly refuse terminal decisions because the
contract requires `/dev/tty`; job state remains a Unix/WSL deployment.

```sh
go install github.com/patrickyoung/may@latest
may check
```

The acceptance check is offline and uses a temporary state directory. It
proves terminal yes/no, fail-closed operation without a terminal, job parking,
human grant and decline, exact-byte digests, one-shot grant consumption,
pending JSONL, and append-only audit records.

May itself and its state are operator capabilities. Never put either in a
model toolbox or writable sandbox. Consequential connector programs call May
by an absolute operator-controlled path. See [SECURITY.md](SECURITY.md).

Clerk also contains a worker-specific gate named `may` whose chat transport
and state belong to Clerk. This standalone May closes Bench's general gate
gap; it does not import Clerk's supervisor or silently alter Clerk jobs.

## License

MIT. See [LICENSE](LICENSE).
