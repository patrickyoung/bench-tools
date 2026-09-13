# Wait indefinitely for user input

This example runs an ordinary shell script, parks it without a deadline, then
wakes it with user input and lets it continue.

No model or account is needed. Run from the Tend source directory
(`cd tools/tend` from the monorepo root):

```sh
go build -o ./tend .

demo=$(mktemp -d)
export TEND_ROOT="$demo/state"
worker=$PWD/examples/wait-for-input/worker

job=$(./tend submit -C "$demo" -- "$worker")
./tend work
./tend show "$job"             # status is waiting
```

Nothing is polling and no Tend daemon is required. The job remains `waiting`
indefinitely. In the same shell, provide input below. If using a second shell,
copy the printed/inspected values of `demo`, `job`, and `TEND_ROOT` and use the
same source directory first; shell variables do not travel between terminals.

```sh
printf 'please continue\n' |
    ./examples/wait-for-input/answer "$demo" "$job" ./tend
```

The `signal` command performs the durable wakeup transaction. Let one worker
perform the resumed attempt:

```sh
./tend events "$job"            # includes job.woken
./tend work                     # resumed script observes input -> done
./tend show "$job"
cat "$demo/.tend-wait/$job.observed"
```

The response travels through a normal file written atomically from stdin. The
signal is only the durable wakeup. This separation lets the script observe the
input without receiving `TEND_ROOT` or direct access to SQLite. A signal sent
just before the script finishes parking is also safe: Tend consumes it in the
same transaction that records the wait, so it is not lost.

An expert can use this kind of wait when it needs missing business input.
Tend retains the wait and the later attempt; Ask does not need to poll a model
while nobody has supplied the answer. See [Agent checkpoints](../agent-checkpoint/README.md)
when conversation continuity matters too.
