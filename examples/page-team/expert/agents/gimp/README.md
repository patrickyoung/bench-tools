# gimp

Run independently with:

    agent run -C WORKSPACE /path/to/expert/agents/gimp -- "bounded assignment"

Supply the actual brief as the goal or stdin evidence and put accepted dependency
files under `inputs/`. The expert writes only its workspace, including
`output/` and `handoff.json`. Its executable check verifies the specialist's
required artifact types and does not prove subjective quality.

Positive example: provide a bounded page-related assignment and configured
tools, then inspect and run `bin/check` from the workspace. Negative example:
remove one required output or alter a manifested artifact; the check or parent
manifest validator rejects it. External effects, credentials, model choice, and
host capability remain operator-owned.
