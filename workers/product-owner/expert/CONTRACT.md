# Product Owner artifact contract

The worker writes exactly two regular files in `output/`:

- `response.md`: nonempty human-readable recommendation or answer.
- `decision.json`: UTF-8 JSON, no larger than 256 KiB.

Source and response files are limited to 1 MiB each. At most 64 input files,
256 total input entries (files and directories), and 16 nested directories are
accepted. Symlinks, duplicate JSON keys and non-JSON numeric constants are rejected.
The request binding is SHA-256 of the exact `request.md` bytes. `inputs` lists
every regular file recursively below `inputs/`, sorted lexically by its
slash-separated path. Paths begin `inputs/`; absolute paths, `.` and `..`
segments are invalid. Empty `inputs/` is represented by `[]`.

## Core schema

`decision.json` has exactly these keys:

- `schema`: `"bench.product-owner/v2"`
- `mode`: `"intake"`, `"planning"`, `"review"`, or `"interview"`
- `status`: `"ready"` or `"needs-input"`
- `request_sha256`: lowercase SHA-256 hex
- `inputs`: array of `{"path": string, "sha256": lowercase SHA-256 hex}`
- `response_sha256`: lowercase SHA-256 hex
- `summary`: nonempty string
- `recommendation`: nonempty string
- `assumptions`: array of strings
- `questions`: array of strings
- `next_actions`: array of
  `{"owner": string, "action": string, "acceptance": string}`
- `details`: the mode object below

Owners may be `"Unassigned"` when the needed decision is identified. A
`needs-input` package has at least one nonempty blocking question and one next
action. `ready` means only ready for human review.

## Mode details

### intake

    {
      "problem": "...",
      "customers": ["..."],
      "desired_outcome": "...",
      "triage": [
        {"demand": "...", "decision": "discover|plan|defer|reject",
         "rationale": "..."}
      ]
    }

All fields are present. For `ready`, the problem, customers, desired outcome and
triage are nonempty. For `needs-input`, unknown strings and customer lists may
be empty; questions identify what blocks completion. Explain evidence and
material constraints in the response where they affect the decision. Urgent
incidents should be routed to incident response.

Version 2 replaces the original mandatory SIPOC intake fields with this product
decision. Version 1 packages do not satisfy this check; keep an older definition
pin for an existing v1 consumer, or update that consumer to the v2 contract.

### planning

    {
      "goal": "...",
      "backlog": [
        {"item": "...", "outcome": "...", "rationale": "...",
         "acceptance": ["..."]}
      ],
      "forecast": {"basis": "...", "uncertainty": "..."}
    }

All fields and item fields are present. A `ready` plan has a nonempty goal and
backlog; every item has testable acceptance. Ordering is array order. Explain
dependencies, size uncertainty, evidence, and displaced work in the response
where relevant. When history is absent, the forecast states that limitation
and does not invent numerical confidence.

### review

    {
      "decision": "proceed|revise|hold",
      "findings": [
        {"finding": "...", "evidence": "...", "action": "..."}
      ]
    }

A `ready` review has at least one complete finding. Evidence can explicitly say
that it is absent or uncertain; it must not imply research that was not supplied.

### interview

    {
      "answers": [
        {"question": "...", "answer": "...",
         "experience_basis": "hypothetical|supplied-evidence"}
      ]
    }

A `ready` interview has at least one complete answer. Use `supplied-evidence`
only when an input explicitly supports the experience claim. Otherwise answer
with a clearly hypothetical example, concrete first action, evidence needed,
trade-off, stakeholders, and learning/decision rule.
