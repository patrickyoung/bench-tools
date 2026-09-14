# Enterprise Architect artifact contract

A run writes nonempty UTF-8 artifacts only under `output/`. It always writes:

- `output/report.md`, role `report`: the human recommendation.
- `output/architecture.json`: the compact bound handoff; it never lists itself.

Choose one primary `deliverable`. A `ready` package also contains artifacts with
all roles shown:

| deliverable | required roles |
|---|---|
| `advice` | `report` |
| `north-star` | `report`, `diagram` |
| `capability-map` | `report`, `catalog` |
| `roadmap` | `report`, `diagram` |
| `guardrails` | `report`, `policy`, `tests`, `ci` |
| `decision` | `report`, `decision-record` |
| `integration-catalog` | `report`, `catalog`, `api-contract` |
| `finops` | `report`, `allocation` |
| `data-ai-governance` | `report`, `catalog`, `policy` |
| `portfolio` | `report`, `catalog` |

Additional files may use role `supporting`. Roles describe artifact purpose;
they do not prove semantics. For `needs-input`, only `report` is required,
outcomes and decisions may be empty, and one to three decisive questions plus a
next action are mandatory. Its report is normally at most 250 words; an explicit
request for a longer diagnostic may override that writing limit. The checker
does not infer this semantic exception from natural language.

## Closed JSON shape

`architecture.json` is UTF-8 JSON with exactly these keys:

- `schema`: `"bench.enterprise-architect/v1"`
- `deliverable`: one value from the table
- `status`: `"ready"` or `"needs-input"`
- `request_sha256`: SHA-256 of exact `request.md` bytes
- `profile_sha256`: SHA-256 of exact definition `PROFILE.md` bytes
- `inputs`: complete lexically sorted current input records:
  `{"path":"inputs/...", "sha256":"..."}`
- `artifacts`: complete lexically sorted records for every `output/` file except
  `architecture.json`: `{"path":"output/...", "sha256":"...", "role":"..."}`
- `summary`: nonempty string
- `recommendation`: nonempty string
- `lenses`: nonempty array of unique selected skill IDs
- `outcomes`: array of exact objects
  `{"outcome":"...", "measure":"...", "evidence":"..."}`
- `decisions`: array of exact objects
  `{"decision":"...", "rationale":"...", "alternatives":"...",
  "tradeoffs":"...", "review_trigger":"..."}`
- `assumptions`: array of strings
- `questions`: array of at most three strings
- `next_actions`: nonempty array of exact objects
  `{"owner":"...", "action":"...", "acceptance":"..."}`

Every string inside outcome, decision, and next-action objects is nonempty.
Measures and evidence may explicitly state that a value is unknown. A meaningful
no-op alternative can be `"Do nothing"` with its consequence. A `ready` package
has at least one outcome and one decision.

Allowed lens IDs are `product-capabilities`, `business-outcomes`,
`value-stream-flow`, `cloud-edge`, `data-ai`, `security-design`, `finops`,
`api-ecosystem`, `collaborative-governance`, and
`architecture-storytelling`. Allowed artifact roles are the table roles plus
`supporting`.

## Bounds and path rules

`request.md`, `PROFILE.md`, each input, and each artifact are at most 1 MiB;
`architecture.json` is at most 256 KiB. Inputs allow at most 64 files, 256 total
files/directories, and 16 directory levels. Output allows at most 32 files
including the manifest, 128 total files/directories, and 8 directory levels.
Inputs may be binary; request, manifest, profile, and generated artifacts must
be UTF-8.

All listed paths are canonical slash-separated relative paths under their
stated root: no absolute path, backslash, empty/`.`/`..` segment, or trailing
slash. Files and directories must not be symlinks; files must be regular.
Hashes bind bytes, not truth or trust. The checker rejects duplicate JSON keys,
nonfinite constants, stale hashes, malformed or extra schema fields, incomplete
or unsorted lists, missing/extra artifacts, empty files, and wrong roles. It
never executes or imports artifact code.
