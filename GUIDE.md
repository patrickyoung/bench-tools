# May guide

## Attended use

Pipe the exact human-readable action to May:

```sh
printf '%s\n' 'publish release v1.4.2' | \
  /usr/local/bin/may && publish-release v1.4.2
```

May reads the action completely, then opens `/dev/tty` and displays its quoted
form, digest, and `y/N` prompt. This separation matters: stdin may be a pipe,
and the bytes being approved must not double as the answer. Only `y` or `yes`
allows the following command.

If there is no controlling terminal, May exits 3. It never interprets
non-interactive execution as implied approval.

## Parked jobs

A durable job gives the request a stable name:

```sh
action='publish release v1.4.2'
rc=0
printf '%s\n' "$action" | /usr/local/bin/may release-142 || rc=$?
case $rc in
  0)  publish-release v1.4.2 ;;
  3)  echo 'declined' >&2; exit 3 ;;
  75) echo 'waiting for a human' >&2; exit 75 ;;
  *)  echo "approval gate failed: $rc" >&2; exit "$rc" ;;
esac
```

On the first run, this creates a pending request and returns 75. The action
does not run. From an operator terminal:

```sh
may pending
may decide DIGEST
```

The decision prompt repeats the job, the quoted exact action, and the full
digest. A yes moves the request to `granted`; a no moves it to `declined`.

The supervisor then reruns the same checked job. Identical job and action
bytes consume the grant by atomic rename and return 0. A second attempt cannot
reuse it: it creates a new pending request and returns 75. Different spacing,
case, or a missing final newline also produces a different request.

## Inspect the record

All authority remains ordinary files:

```sh
may pending | jq .
jq . ~/.local/state/may/audit.jsonl
find ~/.local/state/may -type f -print
```

The audit records the words, not merely a hash. It is append-only; state files
move between directories but audit lines are never edited. Back up or inspect
the directory with normal file tools.

## Put the boundary in the right process

`may` is not a model toolbox program. A connector that can publish, send, buy,
delete, or otherwise cross an irreversible edge invokes an absolute May path
itself. The model receives the connector, not May, its decision commands, or
its state directory.

Filesystem permissions under one Unix account are not an identity boundary.
Run the agent in Cage or a container whose writable roots exclude May state,
and keep the operator side outside that confinement. If the model can alter
`~/.local/state/may`, it can forge the files May is meant to trust.

Clerk workers should continue using Clerk's own gate because its requests,
chat replies, and job lifecycle share Clerk state. Standalone May is for a
connector or supervisor that needs the general file-and-exit-status contract.

## Prove it

```sh
may check
```

The check uses no network, model, default state, or real approval. It exercises
the same state machine with a temporary directory and deterministic terminal
answers, then removes the fixture.
