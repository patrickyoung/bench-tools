# Human requests through Agenda, Tend and May

`human.py` is an optional application adapter. It binds an existing Agenda human
item to one existing Tend signal wait. Input responses are retained and delivered
to a selected mailbox. Approval responses use May's terminal decision; the
resumed controller must consume the grant for the identical action bytes.

The adapter reads Agenda only through the selected executable's public `export`
command. Its handoff records live in a separately selected directory. It never
changes an Agenda item, report or acceptance state. A completed human report
cannot grant approval or wake a job. May remains outside an AI's toolbox and
MCP; this adapter supplies no model-callable approval path.

## Bind a waiting job

Create the human item with `agenda apply` and retain its returned revision. The
job must already be waiting on a signal whose name is unique to this request.
Use the exact current item revision in a request such as:

```json
{
  "schema": "bench.agenda-human-link/v1",
  "id": "review-mockup",
  "item_revision": "EXACT_REVISION_FROM_AGENDA",
  "kind": "input",
  "tend": "/absolute/bin/tend",
  "queue": "/absolute/team-queue",
  "job": "review-job",
  "signal": "review-input-1",
  "response_path": "/absolute/job-work/review-response.txt",
  "by": "Team coordinator",
  "reason": "The team is waiting for this review."
}
```

The response path must not exist and must be inside the job's working directory.
For an approval, use `"kind": "approval"` and replace `response_path` with
`may`, `may_job` and `action`: the absolute May executable, May's exact job name,
and the selected UTF-8 action file. That exact request must already be pending
in the operator's May store.

```sh
python3 human.py link /absolute/handoffs link.json \
  --agenda /absolute/bin/agenda --agenda-root /absolute/work-agenda
```

The returned binding pins the Agenda/Tend executables, selected Agenda root,
item revision, job identity and exact wait evidence. May bindings additionally
pin the May executable, operator identity and exact request digest. Repeating
an identical link is idempotent; one wait cannot be linked to two items within
the selected handoff directory. Coordinate through one handoff directory for
that job collection. A different request needs its own item and unique wait.

## Inspect or respond

Inspection is read-only. It prints the current coordination state and an
`item_current` flag. A changed Agenda item remains visibly different from the
bound revision, and response attempts refuse the changed binding.

```sh
python3 human.py inspect /absolute/handoffs review-mockup \
  --agenda /absolute/bin/agenda --agenda-root /absolute/work-agenda
```

Prepare an input response:

```json
{
  "schema": "bench.agenda-human-response/v1",
  "id": "review-mockup",
  "by": "Reviewer",
  "reason": "Reviewed the selected mockup.",
  "input": "/absolute/review-response.txt"
}
```

For an approval response, omit `input`. May will ask the person on their terminal;
stdin, JSON fields and card state cannot answer the prompt.

```sh
python3 human.py respond /absolute/handoffs response.json \
  --agenda /absolute/bin/agenda --agenda-root /absolute/work-agenda
```

Input is limited to 2 MiB, retained before publishing, and never overwrites a
conflicting mailbox. Response attribution and bytes become immutable when
prepared. A stable Tend signal ID permits recovery after a lost reply without
duplicate wakeups. A previously prepared input can be recovered after its
original source file disappears. Changed attribution or bytes are rejected.

The adapter rechecks the item and wait before signalling. These independent
programs do not provide a distributed transaction; a later revision can still
occur after the last check. The resumed controller must validate the request
and supplied input, or consume May's matching grant. A refusal also wakes the
controller so it can observe May's refusal. The adapter never executes a job,
retries a failed job, resolves an uncertain outcome or marks business work done.

## Supply coordination observations to a view

`observe` prints one Agenda observation with the full inspection result in
`extensions.human_coordination` and a content-hash reference. The observation
expires after 60 seconds. Refresh it independently of browser refresh; a viewer
must not treat a retained observation as a fresh inspection.

```sh
python3 human.py observe /absolute/handoffs review-mockup \
  --agenda /absolute/bin/agenda --agenda-root /absolute/work-agenda > human.jsonl
cat team.jsonl human.jsonl > observations.jsonl
agenda export /absolute/work-agenda > snapshot.json
agenda project snapshot.json --as-of "$AS_OF" < observations.jsonl
```

Here `team.jsonl` is output from `team.py observe` and `AS_OF` is an explicitly
selected RFC3339 timestamp. Supply one observation per item. Coordination
observations can report waiting, queued, running, rejected, cancelled, unknown
or unverified work; they never claim acceptance. Job completion becomes
`unsubmitted` coordination state because the Agenda human report owns business
completion. A changed item revision or failed inspection is retained as
unverified evidence. An unreadable binding cannot yield an observation and
returns an error.

Commands emit JSON on stdout, diagnostics on stderr, and return 0 on success or
2 on invalid input, changed bindings or failed operations. `inspect` prints
`null` when the selected item has no link. The reported `by` text is attribution,
not authentication; access to the selected state and tools remains an operator
responsibility.
