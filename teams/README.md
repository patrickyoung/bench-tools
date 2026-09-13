# Team recipes

A recipe describes useful roles, inputs, handoffs and checks. It contains no
customer brief or previous output. Hire reads the recipe as normal instructions;
Agent and existing Bench commands execute the resulting expert folder.

- [Simple site](simple-site.md): frontend and reviewer, with optional creative
  specialists only when the current job benefits from them.
- [Creative site](creative-site.md): visual and native artwork contributions,
  frontend integration and review, with explicit source handoffs.

Start from a clean [worker export](../workers/README.md). An unchanged suitable
team can run directly without calling Hire again. For changes, use `hire build`
against the exported authoring workspace, inspect the diff, run `hire verify`
and the relevant acceptance cases. Preserve the source lock. Record adaptations
and the revised file digests before using an altered assembly; the original
export's hashes do not describe later edits or installed dependencies.

Recipes are not automatically installed or executed. They do not grant tools,
credentials, filesystem access or scheduling authority.
