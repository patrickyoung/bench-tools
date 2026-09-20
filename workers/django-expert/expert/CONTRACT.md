# Portable task contract, version 1

Run from a separate workspace. Required input: UTF-8 request.md, containing the
mode, desired outcome and acceptance criteria, explicit application path (or
none), selected inputs/ files, constraints and permitted actions. Optional files
live under inputs/. Only explicitly selected files are task dependencies; list
them in handoff.json. Selection completeness and fidelity to the brief require
human review, not keyword inspection. Do not silently consume undeclared inputs.
Repository source is inspected in the explicitly selected application directory;
changed/delivered files are individually manifested, not a snapshot of the whole
repository. No input, app or output path may escape the workspace or use symlinks.
Do not put runtime artifacts in the definition.

Required outputs: report.md (nonempty UTF-8), handoff.json and the app/code/docs
appropriate to the request. A plan may consist only of report.md; build output
must actually implement the agreed scope unless blocked. Static checking cannot
infer scope completion. A ready_for_review build requires a non-null existing
application directory and at least one manifested file below it. A report-only
build fails; blocked builds may have application=null or partial app files.
This proves artifact existence only, not correctness or completeness. Never manifest secrets, DB dumps, installed packages,
caches, .git internals or runtime state as deliverables.

handoff.json is one bounded JSON object with exactly these fields:
- schema_version: integer 1.
- mode: plan | build | iterate | review_debug.
- status: ready_for_review | blocked. The former means offered for review only.
- summary: nonempty string.
- application: null, or relative existing application-directory path.
- request: {"path":"request.md","sha256":"<lowercase 64 hex>"}.
- inputs: array of {"path":"inputs/<file>","sha256":"..."}; [] is valid.
- artifacts: nonempty array of {"path":"<file>","sha256":"..."}; includes report.md,
  all declared outputs and each validation evidence file. Excludes handoff.json,
  request.md and selected inputs. Files, not directory entries. List files changed
  in an existing app, not every untouched source file.
- environments: object with EXACT keys local, DevTest, UAT, production. Each value
  has exactly database, authentication, deployment, each a nonempty decision
  string. State unresolved decisions honestly; reference report detail as needed.
- validations: array of objects with exactly command, result, evidence. command
  is nonempty text describing an actual command or planned check, never executable
  by bin/check. result: passed | failed | not_run | blocked. evidence: relative
  manifested file path, required for passed/failed; otherwise null or such a path.
  No validations is valid for an unexecuted plan if its limits are stated.
- limitations: array of nonempty strings; [] only when genuinely none known.
- next_actions: array of nonempty strings.
- blockers: array of nonempty concrete strings. Nonempty for blocked, empty for
  ready_for_review. Blocked work may contain passed partial validations only with
  existing manifested evidence. State concrete outstanding scope in limitations
  and next_actions; local passes do not imply overall task or external integration
  success.

A ready_for_review handoff cannot have failed or blocked validations. It can
contain not_run checks if candid limitations and next actions identify outstanding
work. Blocked requires limitations and next_actions. A required acceptance check
that cannot run or fails ordinarily blocks implementation readiness; do not use
not_run to hide a required failure.

Limits: JSON 256 KiB, nesting 20, arrays 512 items, strings 8192 characters,
paths 512 characters, individual referenced files 64 MiB and total hashed bytes
256 MiB. Evidence is bounded text/artifacts, not huge captured logs. Duplicate
JSON keys, unknown fields, NaN/infinity, repeated paths, absolute/dot/parent paths,
backslashes, control characters, nonregular files and symlink components fail.
The workspace itself is resolved once; all child components must be real files
or directories. Files must remain stable during checking; do not run concurrent
writers. report.md is limited to 2 MiB.

Run /absolute/definition/bin/check with workspace as cwd. It uses Python standard
library only, ignores stdin, never executes application code or shell strings,
installs, imports project dependencies or contacts the network. Exit 0 means
structurally valid, explicitly labeled blocked when applicable; 1 means invalid/
unfinished; unexpected checker defects are not deliverable failures.

Hash bindings detect stale requests, selected-input/output changes, not truth.
Worker-authored evidence logs and hashes are not independent runtime proof.
The checker cannot establish selected-input completeness, source fidelity,
implementation quality, OAuth security, accessibility, visual quality or
acceptance-criterion satisfaction. Independent application checks run through
the caller-selected action boundary. Review actual external command receipts,
code, browser/user evidence and environment integration before acceptance.
