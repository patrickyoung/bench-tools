# Security

Trail reads conversation archives. Those files may contain prompts, answers,
reasoning, credentials pasted by a user, provider state, and complete binary
attachments. Treat Trail's JSONL output as at least as sensitive as the source
archive.

## Boundary

Trail opens session files read-only and writes results to standard output. It
does not use the network, call a model, create an index, cache content, repair
files, or follow archive symlinks. The sole child process is the executable
selected by `ASK` for `trail check`; it receives exactly `replay -check FILE`.

An explicitly named session path may itself resolve through operating-system
path semantics. Archive scans, by contrast, consider only regular `.jsonl`
directory entries and skip symbolic links.

## Output is not redacted

`show` and `window` deliberately preserve complete events, including media
bytes and opaque provider fields. `find` omits those fields from search text,
but a matching user or assistant event can still contain sensitive prose.
`ls` exposes paths, models, lineage ids, timestamps, usage, and costs.

Do not pipe Trail into an untrusted program, shared log, issue tracker, or
remote service unless that destination is authorized to receive the archive.
Trail does not claim that a terminal, shell history, pager, or downstream
consumer is private.

## Integrity

Trail is an inspector, not an integrity authority. `check` delegates to Ask's
replay verifier. A successful check proves Ask's recorded request-fold
invariant; it is not a signature and does not prove who created the file.

Archive damage is emitted as a per-file error record and status 1. An invalid
last record is treated as a torn append, matching Ask's reader, and is reported
as a warning without changing the file. Event lines larger than 64 MiB are
refused rather than truncated.

## Reporting

When reporting a vulnerability, include the Trail version, operating system,
the smallest synthetic session that reproduces the problem, the command, and
the observed exit status. Remove real conversation content and credentials.
