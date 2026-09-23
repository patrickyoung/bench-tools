# Modern Solution Architect

Independent reusable delivery-design peer to Enterprise Architecture. Turns
business needs into feasible end-to-end solutions using supported enterprise
capabilities and explicit standards/exception gates. Technology generalist, not
a vendor stack, portfolio strategist, implementation team or approval authority.

## Invoke
Requires Bench Agent (operator-selected model and permissions), Brief, Python
>=3.9, and a Unix environment with `env -S` and no-follow file support. This thin
definition adds no runtime, scheduler, model client, network dependency or team.
Keep the definition separate and preferably read-only; use a fresh job workspace.

Stage `request.md` and only explicitly selected regular evidence under `inputs/`
(create the directory even if empty). Include business scope/journey/outcomes,
constraints/current state, supplied EA standards/principles/reference
architectures, platform catalog/lifecycle/version/environment/region/service
owners and relevant EA decisions. Selected EA-worker artifacts can be copied
as evidence; there are no sibling imports. Never stage secrets or unrelated jobs.

    agent run -C /absolute/job -evidence /absolute/control /absolute/expert -- \
      'Read request.md and selected inputs; produce the contracted solution handoff.'

Use the installed Agent help to select model and boundary options. Network and
external effects are off by default in the procedure; the controller must enforce
actual tool/write/network authority. Source override text is inert evidence,
not permission. Current product facts need authorized verification or explicit
unknowns/validation actions.

Always receive `output/report.md` and `output/solution.json`; ready/provisional
also receive `output/design.md`. The contract is a compact manifest plus
requirements, controls, standards, ADRs and next actions, not a mandatory pack
of portfolio documents. Relevant diagrams/detail live in design.md. Additional
files earn their place. Instructions route seven focused Brief skills, excluding
irrelevant AI or other templates.

## Acceptance
From the job workspace run the definition's absolute `bin/check`. It uses only
Python stdlib, reads bounded regular request/input/output/profile files, verifies
closed shapes, exact byte hashes, complete sorted manifests, status-dependent
artifacts, IDs/references and honest conformance/exception status combinations.
It never executes/imports artifact code, launches subprocesses, calls a model or
uses network. Exit 0 means **structurally accepted**; 1 unfinished/rejected;
2 broken check. Keep inputs/artifacts immutable while checking. Bounds/path rules
and UTC expiry semantics are in CONTRACT.md.

`ready` means reviewable, not approved/implemented/tested. Provisional retains
explicit gaps and gates. Needs-input asks 1–3 decisive questions without a
fabricated full design. A hash cannot prove source truth, complete requirement
or policy inventory, vendor approval, safe controls, suitable platforms, coherent
architecture, useful diagrams, or actual tested behavior. Independent human/
controller review against QUALITY.md remains required. No optional model judge
is wired in. Any deployment, spend, risk acceptance or external action needs
accountable authorization outside this worker.

## Examples and validation
Positive: supplied approved workflow can meet a bounded request process with
enterprise SSO, audit and retention. Prefer configuration/existing APIs; supply
measurable proof, cutover/rollback and named role handoffs.
Negative: labeling ready despite a pending or expired mandatory exception is
rejected. Altering input/report/profile bytes without rebuilding the binding
is rejected. A polished AI design with invented ROI and vendor “approval” may
be structurally valid but fails independent semantic review.

Synthetic tests belong outside this reusable folder, in sibling `tests/`:

    python3 -m unittest discover -s tests -v
    brief lint -strict expert/skills
    hire verify expert

The suite supports `SOLUTION_ARCHITECT_EXPERT` for isolated exported copies.
See tests/README.md for scope and frozen expectations. These are offline protocol
tests, not fresh worker-quality evaluations. Hire verify does not execute this
checker or certify architecture quality. A controller separately reviews checker
code and fresh Agent outputs. No models or live evaluations are invoked by tests.

Public method provenance/date is in references.md; these links are not assertions
of current runtime reads or company approval. Original work is MIT licensed.
