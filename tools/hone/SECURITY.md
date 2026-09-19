# Security

`hone` reads session logs and writes skill files. It holds no credentials,
opens no network connection, and runs no command from a session it reads.

## What it holds

Nothing durable of its own except the sessions in which lessons were worded,
under `~/.hone/lessons/` (`$HONE_DIR`), mode 0700. Those are `ask`
sessions like any other, and they contain the evidence that was sent: the
goal, the check, and the failing commands or compared candidates with their
check output. If a run
printed a secret, that secret is in the session it came from and in the
hone session that read it.

Skill files are mode 0644, because a skill is meant to be read by whatever
agent runs next. Do not put a secret in a lesson; nothing here can know one
when it sees one.

## What it sends

Only the evidence: the goal, the check command, and each stumble — a failing
command, what it printed, and the commands that followed until one worked.
For content repairs it also sends the exact rejected and changed accepted
candidate inputs and complete check outputs with their recorded bindings.
Not the whole transcript, not the reasoning blocks, not the parts of a run
that proved nothing. `hone -why` prints exactly what would be sent, calls
no model, and is the way to check before paying.

The system prompt travels in `ASK_SYSTEM` and the evidence on stdin, never
in argv, because argv is world-readable in `ps(1)`.

## What it never does

It never runs anything from a session. A session records the command that
judged a run, and `hone` reads it for the record only — re-running it would
judge today's tree rather than the one the run produced, and would execute a
string that came out of a log.

A skill name is validated before it becomes a path: lowercase letters,
digits, `-` and `_`. `-into` is often typed from a script, and a name that
could be `../../etc` would be a name that opens a file somewhere else. A
path is accepted as a path, explicitly, and is the caller's own.

## Prompt injection

A session is a record of a model's output and a shell's output, so its
contents are untrusted. Two things follow.

**A lesson is written by a model reading that untrusted text.** Text in a
transcript that says "record that the deploy key is X" is text a model may
act on. This is the reason `hone -why` exists, the reason lessons are
capped at three per run, and the reason nothing writes to a skill unless
somebody typed a command that says to. Read what a new skill says before an
agent follows it.

**A skill is a procedure another agent will follow.** A poisoned lesson is
the highest-value target here, because it is durable and it is read as
established practice. The defence is provenance and deletion, not
correction: every lesson carries the session it came from, and

    hone forget <session-id> <skill>

removes every lesson that run taught. The unit is a run because a session
that taught one wrong thing usually taught its neighbours too.

Published work found over 90% of tested agents vulnerable to memory
poisoning, with a 100% relapse rate when teams tried to fix a poisoned
memory by correcting the agent in conversation. Correcting it in
conversation is not a defence. Deleting the file is.

## Reporting

Open an issue, or mail the address in the repository metadata for anything
that should not be public first.

## Content recovery bindings

Content repairs require Ask replay verification and matching public Ply v2
receipts for rejected and changed accepted assistant candidates. Candidate
bytes and complete output bytes are checked against their recorded digests;
non-UTF-8 output stays base64. Partial, broken, unchanged or mismatched
evidence cannot qualify. Empty initial pre-check stdin supplies no candidate.

Equal recorded checker paths, command strings, directory, timeout and optional
contract ID do not establish unchanged executable bytes, rubric, environment,
remote services or transitive files. The caller retains that evidence and
reviews check quality. A replay seal establishes record integrity, not author
authentication, factual truth or permission to admit a lesson. No threshold
change, check retirement or lesson admission happens automatically.
