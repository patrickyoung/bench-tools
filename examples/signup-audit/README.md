# Keep a durable signup audit

This starter runs an ordinary Python program through Tend. It counts exact
duplicate signup lines and retains the input, attempt, and output locally.
No model or account is needed.

Counting duplicate addresses is a fixed rule. Keeping its execution durable
is a separate job. This example lets Tend do that second job without turning
the first one into an AI task.

From the checkout root:

```sh
python3 scripts/install tend
export PATH="$HOME/.local/bin:$PATH"
practice=$(mktemp -d)
cp -R examples/signup-audit "$practice/signup-audit"
cd "$practice/signup-audit"
python3 run.py
```

Expected stdout:

```json
{"rows": 3, "unique": 2, "duplicates": 1}
```

The queue lives at `queue/` in the copied directory. Inspect the durable record:

```sh
TEND_ROOT="$PWD/queue" tend show signup-audit-001
TEND_ROOT="$PWD/queue" tend events signup-audit-001
TEND_ROOT="$PWD/queue" tend check
```

Run `python3 run.py` again with unchanged input: the same job ID returns the
existing result without another attempt. For different input or a new run,
choose a new ID:

```sh
python3 run.py signups.txt --job-id signup-audit-002
```

Reusing an ID with different bytes is an error. Input is saved at submission;
`audit.py` remains an external dependency, so keep it unchanged while pending.
This starter owns a dedicated queue and reads the latest observed successful
attempt. Use Tend's inspection commands if a job fails or is interrupted; the
starter never decides whether to retry uncertain work.

Each `tend work` performs one transition. A scheduler supplies repetition for
recurring work. Tend records execution facts; it cannot promise exactly-once
effects on an external service. This worker only computes a report.

The same queue can run `agent run` when interpretation is needed. Build its
expert with Hire and use [Tend's Agent guide](../../tools/tend/examples/agent-checkpoint/README.md)
to pass the model configuration and keep conversation checkpoints separate
from execution records.
