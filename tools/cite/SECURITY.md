# Security

Cite is a validator, not a sanitizer or trust engine.

It opens one caller-named regular evidence file and reads the candidate on
stdin. It makes no network calls, executes no programs, writes no state, and
does not open citation URLs. Evidence and candidates may be sensitive; their
filesystem permissions, shell pipelines, logs, and retention remain the
caller's responsibility.

An exact ref and URL proves only that a candidate points at a record in the
given file. A malicious source can still provide a malicious URL or content,
and a model can attach a real citation to an unsupported claim. Cite does not
render Markdown, establish source authority, check prompt injection, or prove
semantic entailment. Render untrusted Markdown with an appropriate sanitizer
and use a task-specific verifier for consequential claims.

Both inputs are bounded. A rejected candidate leaves stdout empty, including
when the final citation is bad, so downstream programs cannot mistake a valid
prefix for an accepted answer.
