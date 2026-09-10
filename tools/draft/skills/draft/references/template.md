# NAME

One sentence. What somebody types, and what they get back.

## What it is

The shape of the answer, in two or three sentences. If this is hard to
write, the requirements are not understood yet, and no amount of building
will fix that.

## Requirements

What must be true for this to be worth having. Write them as statements
somebody could disagree with, not as features.

- [ ]
- [ ]

## Not doing

The cuts, on purpose, so nobody quietly re-adds them. This section is worth
more than the one above it: a system is defined by what it refuses. Give
each cut its reason.

-

## The split

Which parts need a model and which are a script. Be hard about this. Most
of a working system is not an agent problem, and reaching for the loop where
`curl` would do is slow, expensive, and nondeterministic for no gain.

| stage | tool | why |
| --- | --- | --- |
|  | none -- a script |  |
|  | ask |  |
|  | ply |  |

## Data

What is raw and never rewritten, and what is derived and can be deleted and
rebuilt. Get this right and the rest is self-evident; get it wrong and no
amount of code recovers it.

    raw:
    derived:

## Check

The command whose exit status says the work is done. This section is
required, and `draft check` refuses a design without it. If no program can
decide, say so here and say what a human looks at instead -- but look hard
first, because a program can decide far more often than it seems.

Keep the semantic assertions here or in an operator-controlled program when
the build needs an admitted verifier. A project-local check script is writable
by the builder unless the operating-system boundary protects it.

```sh
false
```

## Layout

    dir/
      bin/
      ...

## Traps

Name the failure cases and how the system detects or handles them:

- plain `ask` starts fresh; `-c` continues `current`; `-f FILE` creates or
  continues only that named session. Choose a new filename for independent work;
- a passing task check can avoid repeated work, but file existence alone is
  insufficient. Inspect an uncertain external effect before considering a retry;
- explicitly select the environment available to a scheduled job or worker.
  Keep credentials and trusted evidence outside its writable boundary.
