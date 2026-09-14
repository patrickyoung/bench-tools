# Keep an expert's attempts durable

A customer reply has two kinds of history. Ask records what the model saw and
said. Tend records the attempt to run the worker: its exact command, input,
output, and execution outcome. A conversation checkpoint does not replace a
job record, and a job record does not replace model context.

## Submit an existing expert

First run the [support-reply starter](https://github.com/patrickyoung/bench-tools/tree/main/examples/support-reply)
or build a suitable expert with Hire. Install Tend plus Agent's companions
(`agent ask brief ply cage record`, and `cite` for that starter). Configure Ask in
the operator environment.

The following assumes an Anthropic model. `TEND_PASS` contains environment
**names**, not secrets; use the matching names for your provider instead.
Replace the expert path with the inspected folder you want to run:

```sh
expert=/absolute/path/to/support-reply/expert
case_dir=$(mktemp -d)
mkdir "$case_dir/work"
printf '%s\n' 'Can I export before closing? How long does it take?' > "$case_dir/work/question.txt"
export TEND_ROOT="$case_dir/queue"
export TEND_PASS='ASK_MODEL ANTHROPIC_API_KEY'
job=customer-001

tend submit -id "$job" -C "$case_dir/work" -- \
  "$(command -v agent)" run -C . -evidence "$case_dir/control" \
  -checkpoint "$job" -turns 8 "$expert" -- \
  'Draft reply.md from the supplied policy for this customer.' < /dev/null
tend work
tend show "$job"
```

`submit` saves the request; `work` runs one attempt. The job record should
report `done` only if Agent exited successfully. Inspect the deliverable and
this first attempt's final report:

```sh
cat "$case_dir/work/reply.md"
cat "$TEND_ROOT/jobs/$job/attempts/001.out"
tend events "$job"
tend check
```

A successful `tend work` means Tend performed a transition; read `show` for
the worker's outcome. The report in `001.out` is Agent's stdout, while the
reply is the workspace artifact. Model configuration must still be available
when `tend work` runs; submission does not freeze secret values in its record.

## Continue only the work that needs continuing

If Agent returns a known unfinished outcome, Tend records a failed attempt.
After inspecting its output and choosing to continue the same task:

```sh
tend retry "$job"
tend work
tend show "$job"
```

The next attempt uses the same command and checkpoint. For a portable Agent,
that pointer is under the selected evidence root's `checkpoints/` directory;
Ply owns its lock and the current Ask session, including after compaction.
The output is now a later attempt file, such as `002.out`; consult the job
record instead of always reading the first attempt.

An interrupted or lost execution can be **unknown**. Inspect the workspace,
Tend artifacts, Agent records, and any external effects before choosing a
resolution. Only after that decision would you use
`tend resolve "$job" retry`. Tend never makes it automatically.

A new customer is a new workspace and job ID. An unchanged accepted workspace
may satisfy Agent's pre-check with no model call. Neither the checkpoint nor
that pre-check proves that an uncertain external operation is safe to repeat.

## Existing recurring homes

The home layout remains supported. With a valid inspected home:

```sh
tend submit -id daily-report -C /absolute/path/to/home -- \
  "$(command -v agent)" run -checkpoint daily-report . < /dev/null
```

That Agent stores its checkpoint under `.agent/checkpoints/` in the home.
Tend still sees only an executable and its arguments. Your OS scheduler
supplies recurring `tend work` calls; Agent and Hire need no scheduler added
inside them.
