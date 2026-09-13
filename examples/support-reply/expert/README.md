# Support reply expert

Reads a workspace's `question.txt` and this folder's `sources.jsonl`; writes
`reply.md`. The supplied policy is fictional. Its URLs identify records for
the example and are not fetched.

Run with Agent in a separate workspace. See the
[complete walkthrough](https://github.com/patrickyoung/bench-tools/blob/main/examples/support-reply/README.md)
for installation, invocation, expected output, and negative checks.

`bin/check` rejects missing, empty, or symlinked replies and delegates citation
validation to Cite. At least one exact citation is required; malformed or
unknown `ctx:` references are rejected. A passing check does not establish
factual support, completeness, tone, or a correct answer to a new question.
Review those separately. The check reads source records from this definition,
outside the worker's allowed write roots under the default Cage boundary.

Adapt the instructions, skill, evidence, and check together for a different
job. A new specialty does not require a new execution loop.
