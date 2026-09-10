# Security

May is an exact-action human gate, not a risk classifier. It cannot tell
whether the words are honest, sufficient, current, or safe. The connector
must give the person a complete action description and must perform exactly
that action after approval.

Action bytes come only from stdin. Terminal decisions come only from
`/dev/tty`. A job grant is bound to the job and exact action digest, records
the words too, and is atomically moved out of `granted` before May returns 0.
Corrupt or mismatched state fails closed.

`may request JOB` reports the existing job transition as strict JSON. The
reported verdict is useful to a supervisor only together with May's exit
status and exact action bytes. No JSON input is accepted, and the command does
not create a second way to approve.

May writes `~/.local/state/may` with private directory and file modes and
appends `audit.jsonl`. These permissions prevent accidental access by other
accounts; they do not protect against another process running as the same
user. The real boundary must be the process sandbox or identity:

- keep May, `may decide`, and the state directory out of the model toolbox;
- have consequential connectors invoke an absolute operator-controlled path;
- exclude the state directory from agent-writable Cage/container mounts;
- do not expose an operator shell or controlling terminal to unattended code;
- treat audit and pending action text as sensitive operational data.

There is no auto-approve option, environment bypass, state-location override,
configuration file, model judgment, network request, or daemon. If state or
audit cannot be validated and durably updated, May returns an operational
failure rather than approval.

A job grant is single-use but does not expire automatically. A human should
decide only current requests; operators may remove obsolete pending or granted
files while no job is running. Such maintenance is visible filesystem work,
not a May command, and audit history remains append-only.

Report vulnerabilities privately to the repository owner. Include the May
version, operating system, invoked command, state layout, and stderr. Redact
actions, job names, and credentials.
