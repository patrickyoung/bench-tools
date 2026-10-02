# Prepare evaluation cases for Trait

The copied expert/ is the selected definition. Preserve EVERY expert file,
mode and byte. Your job is to create a small bounded evaluation recipe outside
it, under evaluation/cases/. Hire's outer verifier checks this unchanged expert.
Never execute the business job in place of preparing cases.
The authoring action Cage denies network and nested controllers. Do not run
live Ask, Agent, or team controllers here; Trait runs the actual entry afterward.
Use the writable workspace or $TMPDIR for helper scripts, never assume /tmp is
writable. Keep preparation programs/logs outside evaluation/. Its only root
entries may be run.json, CASES.md, and cases/. Preserve the copied expert as
received; it can intentionally differ from the original pre-edit input expert.

Read the supplied outcome, selected sources and the worker's documented contract.
Assess the complete capability and write CAPABILITIES.md beside expert/: methods,
checks, programs/native apps, specialists, dependencies, entry and handoffs.
Choose useful realistic examples yourself; do not require the user to write
test scripts or execution metadata. Cover the promised capability, not just
file existence. Mark assumptions and unavailable domain facts rather than inventing
them. Keep cases finite and inexpensive; normally two or three cases suffice.

Each evaluation/cases/ID has goal.md, optional input/, executable check,
case.json, and optional expected data. ID is lowercase letters/digits/hyphens.
case.json is {"purpose":"transfer","outputs":["answer.txt"]}. Use purpose
transfer for behavior the capability/teaching should enable and regression for
behavior it should preserve. Include BOTH purposes, even for initial creation.
Outputs must name 1–64 regular files, never directories. A known refusal test
may set "expected_exit":2 and "outputs":[]; omitted expected_exit means 0.
No other expected status is allowed. A matching status alone is insufficient:
the independent checker must verify the concrete expected refusal reason,
retained disposition and absence of a falsely accepted output. Do not credit
an infrastructure/model failure as a correct business refusal. For its evidence
the check receives TRAIT_EXECUTION_EXIT, TRAIT_EXECUTION_STATUS (recorded JSON),
TRAIT_EXECUTION_STATE and TRAIT_EXECUTION_EVIDENCE paths. State/child evidence
remain controller-writable, with the documented host boundary limitations.
The check runs without args from a frozen result workspace, with TRAIT_CASE_DIR
pointing to its frozen case. Exit 0 accepts, 1 rejects, other codes mean broken.
Networking and workspace writes are denied; temporary files under $TMPDIR are allowed.
Use executable deterministic checks where possible. Preserve caller-selected
expectations, handle missing outputs, and test positive and convincing incorrect
examples while preparing the check. Record those preparations in CASES.md inside
evaluation/; never pretend generated labels are independent domain truth.

For teams, exercise the real entry, specialist handoffs, native helpers, root
integration, and failure propagation. Do not merely inspect embedded folders.
For a documented controller entry, write evaluation/run.json with relative entry,
literal args and required command names using the described Trait placeholders.
Preserve any supplied run.json unchanged. Default to Agent only for a worker
whose capability can actually execute through one Agent. Missing required
compilers/apps or an unavailable controller are incomplete prerequisites.

Do not change expert/, supplied cases, source documents, model credentials, host
packages or installed tools. Finish with the generated evaluation paths and
what the cases do and do not prove. Trait freezes the recipe, separately reviews
the exact definition/checks, and runs fresh cases afterward.
