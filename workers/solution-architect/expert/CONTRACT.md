# Small solution handoff contract — v1

Only `output/` holds generated job artifacts. Always supply nonempty UTF-8
`output/report.md` and `output/solution.json`. Ready and provisional also require
`output/design.md`; needs-input must not include that purported complete design.
Other useful supporting artifacts are optional, never a required document pack.

## Status semantics
- `ready`: reviewable design, required evidence present, no unresolved mandatory
  conformance issue. NOT approved, implemented, deployed or tested.
- `provisional`: useful conditional design, explicit gaps/assumptions and
  next-action gates; no unqualified conformance claim.
- `needs-input`: critical business/evidence gap prevents responsible design;
  report normally <=300 words, 1–3 decisive questions, concrete next action.
  Requirements/controls/standards/decisions may be empty; never invent completeness.

Report is normally <=600 words (writing guidance, not a keyword/word-count check).
Design contains scope/current/target, source-qualified alternatives and ADRs,
appropriate Markdown/Mermaid views, quality scenarios, standards mapping and
proof/migration/operations handoffs. Do not force irrelevant templates.
The manifest keeps essential rows; prose fields may refer to design sections.

## Closed JSON object
Exactly the following keys, all required:
- `schema`: literal `"solution-architect/v1"`
- `status`: one of the three statuses above
- `request_sha256`, `profile_sha256`: lowercase SHA-256, exact request and this
  definition's PROFILE.md bytes
- `inputs`: complete sorted records of all staged files under inputs/
- `artifacts`: complete sorted records of all output files except solution.json
- `summary`: nonempty concise decision and status text
- `lenses`: nonempty unique skill IDs selected from business-to-requirements,
  platform-options, solution-design, quality-security-operations, ea-conformance,
  delivery-migration, pragmatic-ai
- `requirements`, `controls`, `standards`, `decisions`, `next_actions`: rows below
- `assumptions`: unique nonempty strings, including unresolved gaps; provisional
  requires at least one
- `questions`: unique nonempty strings, maximum three; ready requires none

Each manifest record has exactly `path`, `sha256`. Paths use canonical slash
relative paths in their stated root; hashes bind exact bytes, not truth.

Each row has exactly the documented keys (no extras):
- Requirement: `id`, `kind` (`functional`|`quality`), `statement`, `outcome`,
  `measure`, `target_basis` (`supplied`|`proposed`|`unknown`), `verification`,
  `owner`, `control_ids` (references). Put load/conditions and target in measure,
  method/evidence in verification. Unknowns remain explicit, not invented.
- Control: `id`, `component`, `description`, `evidence`, `owner`. Evidence states
  source or planned proof and distinguishes design intent from measured behavior.
- Standard: `id`, `source`, `level` (`mandatory`|`advisory`),
  `applicability` (`applies`|`not-applicable`|`unknown`), `rationale`,
  `control_ids`, `verification`, `owner`,
  `status` (`conforms`|`fails`|`unknown`|`excepted`|`not-applicable`), `exception`.
  `id` is a stable local standard ID; retain authoritative standard ID in
  source.locator. `source` has exactly `path`, `locator`, `version`, `date`,
  `scope`. Path must be a bound inputs/ file; date is ISO YYYY-MM-DD or `unknown`.
  Locator/version/scope retain supplied authority and qualifications (explicit
  unknowns allowed). Missing pack is not fabricated as a standard.
- Decision (compact ADR): `id`, `choice`, `rationale`, `alternatives`, `tradeoffs`,
  `requirement_ids`, `standard_ids`, `review_trigger`. Alternatives/tradeoffs
  are concise strings or design-section references with meaningful summaries.
- Next action: `id`, `owner`, `action`, `acceptance`, `requirement_ids`,
  `standard_ids`. Each is an accountable proof, input or decision gate.

All scalar row fields not specified otherwise are nonempty strings, <=8000
characters. Arrays contain <=128 entries, except questions <=3. ID format:
`R-`, `C-`, `S-`, `D-`, `A-` respectively followed by 1–48 ASCII letters, digits,
hyphens or underscores. IDs are unique across rows. Prefer meaningful names,
stable across revisions. Reference lists contain unique existing IDs, no nulls.
Decisions reference at least one requirement. Applicable standards reference
controls, as do requirements. Unknown/not-applicable standards may have no
controls. Each requirement in a substantive design is covered by a decision.
Every status requires at least one next action.
Ready/provisional require at least one functional requirement, one quality
requirement, one control and one decision. Ready also requires standards rows;
completeness of a supplied pack remains a human review, not inferred from counts.

## Conformance and exceptions
Not-applicable applicability requires not-applicable status and vice versa.
Unknown applicability requires unknown status. Applicable means conforms,
fails, unknown or excepted. Conforms/excepted require mapped controls.
Use unknown/fails for unresolved policy conflicts; option scores cannot waive them.

`exception` is null or exactly:
`status` (`pending`|`approved`|`expired`), `risk`, `mitigation`, `decision_owner`,
`scope`, `review_trigger`, `expires` (ISO YYYY-MM-DD or null), `migration_exit`,
`approval_path` (bound inputs/ path or null), `approval_locator` (nonempty string
or null). All other fields are nonempty strings. Proposed exceptions remain
pending with approval fields null. Approved requires supplied approval path,
locator and expiry; scope must identify the design/environment covered.
Expired may retain historical approval evidence. Excepted requires an exception;
an exception requires excepted/fails/unknown status, never conforms.

Ready rejects ANY pending/expired/undated/elapsed exception (UTC date, expiry
must be strictly after today), and mandatory standards with unknown applicability
or applicable fails/unknown. Mandatory excepted can be ready only with an approved,
unexpired exception. Supplied approval is not the worker's approval. Unknown or
unscoped source date/version/scope prevents ready for applicable mandatory rows.
The checker recognizes literal `unknown`; semantic review must detect misleading
scope, incomplete policy inventory, conflicts and false approvals. Review triggers
can invalidate an exception before its date: record expired/unknown, do not hide it.

## Bounds and inspection
`request.md`, definition PROFILE.md, each input and artifact: 1 MiB maximum;
manifest: 256 KiB. Nonempty request/profile/output; binary or empty inputs allowed.
Inputs/ must exist even when empty; <=64 files, <=256 entries (files/directories),
<=16 nested directory levels. Output/ <=32 files including manifest, <=128 entries,
<=8 nested directory levels. Both trees total <=16 MiB each. Filenames/paths have
no absolute form, backslash, colon, NUL/control character, empty, `.` or `..`
segment. No symlinks (including root dirs), special files, escapes or hard-linked
regular files. No UTF-8 BOM in JSON, duplicate keys, NaN/Infinity or numbers
overflowing to infinity. Closed shapes accept no numbers or booleans as strings.
Lists must be sorted for manifests, unique everywhere specified.

`bin/check` inspects only these bounded files and its profile, read-only using
Python >=3.9 stdlib. No subprocess, artifact execution/import or network.
0 = structurally accepted; 1 = rejected/unfinished evidence; 2 = broken checker
(e.g. missing/unreadable definition profile). Use an immutable job snapshot while
checking; concurrent writers are not a supported trust boundary.
Human/controller review against QUALITY.md is independently required for source
entailment, completeness, platform suitability, diagrams and coherent safe design.
No shape, hash or field count establishes those properties.
