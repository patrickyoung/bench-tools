# Long commands through an ordinary program

`job` starts one command under a private supervisor, returns an opaque handle,
and lets later shell actions inspect, wait for, or cancel it. It is an optional
Python 3 program on `PATH`; Ply gains no scheduler, task format, or daemon.
The supervisor exists only for its one command. It survives the `job start`
process and the foreground Ply action returning.

Use it when a worker should start a long local check and inspect the same
execution later. Use [Tend](https://github.com/patrickyoung/bench-tools/tree/main/tools/tend)
when the caller needs a durable queue, recorded attempts, or signal/timer
waits. Neither requires a second model loop. Agent experts can use the same
ordinary programs when their selected execution boundary permits them.

The program uses the Python standard library and POSIX process/socket APIs.
On macOS, `/bin/ps` distinguishes a zombie-only process group from a live
group when the kernel returns `EPERM` while cleaning up a finished command.

```sh
job start --timeout 1800 -- go test ./...
# Prints a handle such as 83a2cd458ed39b0c40593bde2b3327e5 on stdout.

job status 83a2cd458ed39b0c40593bde2b3327e5
job wait 83a2cd458ed39b0c40593bde2b3327e5 --timeout 5
job cancel 83a2cd458ed39b0c40593bde2b3327e5 --timeout 6
```

Arguments after `--` are passed directly to the executable. Use an explicit
interpreter when a pipeline, redirection, or shell expansion is required:

```sh
handle=$(job start --timeout 600 -- /bin/sh -c 'make build && make integration')
job wait "$handle" --timeout 5
```

The command inherits the launcher's environment and working directory, sees
EOF on stdin, and owns a separate process group. A script must wait for the
children whose work it wants included. When the command exits, the supervisor
kills remaining members of its group before recording the result. On cancel
or timeout it sends SIGINT, allows three seconds for cleanup, then sends
SIGKILL to the group. This lets a nested Ply cancel its own action groups.
Teardown remains bounded if a process refuses to exit; that exceptional case
is reported as an unknown outcome, never as completion.

`start` defaults to a one-hour command timeout. `wait` defaults to one second;
`cancel` waits up to five seconds after requesting cancellation. Each wait
may be set to at most 60 seconds. Repeated waits inspect the same execution
and return the same retained output after it finishes. They never rerun it.

| Command or state | stdout | stderr | Exit status |
| --- | --- | --- | --- |
| `start` | Handle | Launch status | 0 when the supervisor is observed; 1 if launch is uncertain |
| `status` | JSON state | Errors, if any | 0 for a live or completed job; 1 for an unknown outcome |
| `wait`, still running | Empty | Explicit JSON state | 2 |
| `wait`, naturally finished | Exact command stdout | State and exact command stderr | Actual command exit; signals use 128 + signal |
| `wait` or `cancel`, cancelled | Captured command stdout | State and command stderr | 130 |
| `wait`, command timed out | Captured command stdout | State and command stderr | 124 |
| Supervisor unavailable, output changed, or invalid evidence | Empty | Error or unknown state | 1 |

A completed JSON state carries both the actual `returncode` (negative for a
signal) and normalized `exit_code`. `wait_exit_code` distinguishes a timeout
or cancellation from ordinary completion, even if a command catches SIGINT
and exits zero. Thus command exit 2 and an unfinished wait are distinguishable
by their explicit state. A zero command exit is the command's result; Ply's
external `-check` still decides whether the larger task is done.

## Install and state

Use the program directly, or link it and its interpreter into a toolbox:

Run these from Ply's source directory (`cd tools/ply` from the monorepo root).
Create a fresh toolbox first:

```sh
mkdir tools
ln -s "$PWD/contrib/job" tools/job
ln -s "$(command -v python3)" tools/python3
```

`JOB_DIR` names the state directory. It defaults to
`~/.local/state/ply-jobs`. Set it outside a model-writable workspace when the
records need to be kept away from that workspace. The directory and each job
directory must be owned by the invoking user with mode 0700; files use 0600.
The program creates missing directories but refuses an existing directory
with broader permissions. Output is retained as files, including binary
bytes, and consumes disk space until the operator removes a completed job's
directory. The program does not silently discard output. `wait` may produce
large output; Ply's ordinary visible output cap still applies when it runs it.

Each job has a random 256-bit key and an owner-only Unix socket under
`/tmp/ply-job-UID/`. Requests and responses authenticate the handle, operation,
and nonce using HMAC-SHA256. Cancellation is sent through that live endpoint;
the client never kills a process based on a PID file. The supervisor keeps
its own process-group leader unreaped until it finishes signaling the group,
so PID reuse cannot redirect those signals. On completion it atomically
writes and fsyncs an authenticated terminal record binding the handle, actual
exit status, and each output stream's digest and byte count. Clients verify
the record and output before reporting completion.

This is process lifecycle management, not a sandbox. Commands retain their
ordinary filesystem and network access; a process can leave its group, and
another process running as the same user can read the private key. Do not
describe `job` as a replacement for a container, account boundary, or Cage.

## An unavailable supervisor is an uncertain outcome

Only an authenticated reply from the live supervisor proves a job is running.
Only the verified terminal record proves its recorded outcome. If the
supervisor is killed with SIGKILL, the machine restarts, or an endpoint is
lost before the result is sealed, `status`, `wait`, and `cancel` report
`unknown`: command effects may exist, and the command may still be running.
The original handle remains available even when startup could not be
confirmed. A durable launch claim prevents that handle from being restarted.

Inspect the work and use an external verifier before deciding whether a new
invocation is appropriate. This protocol is not exactly-once execution or an
automatic retry policy. In particular, an absent socket or a stored PID is
never evidence that replaying an external effect is safe.

Run the executable-seam tests with `python3 contrib/job_test.py`. They cover
binary output and its digests, actual failures, bounded waits, cancellation
after the leader exits, command timeouts, authenticated control, and a lost
supervisor without replay.
