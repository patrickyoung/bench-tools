# Solution Architect

The Solution Architect turns a business need into an implementable end-to-end
solution using the organization's supported technology platforms and standards.
It consumes Enterprise Architecture direction and returns traceable design,
conformance findings, decisions and delivery/operations handoffs.

The Enterprise Architect owns enterprise direction, portfolio/platform choices
and standards. The Solution Architect applies that direction to a particular
problem and identifies justified gaps for accountable owners. Engineering and
operations teams own implementation and service operation; the worker cannot
approve its own exceptions, spending, delivery capacity or production release.

## Design scope

| Input or collaborator | Contribution |
| --- | --- |
| Business/product owner | Problem, users, outcomes, scope, priorities and acceptance |
| Enterprise Architecture | Applicable standards, approved platforms, patterns and decisions |
| Security/data/platform owners | Controls, information lifecycle, service contracts and constraints |
| Solution Architect | Alternatives, architecture views, decisions, requirement/control traceability and measurable validation |
| Engineering/QA/operations | Implementation, observed verification, rollout and operational acceptance |

The worker covers packaged SaaS/COTS, low-code, custom applications, cloud,
on-premises/hybrid/edge, identity, data and integration. It favors reuse and
configuration when these meet the need, with bounded exceptions where they do
not. AI is one optional design choice, assessed against a non-AI alternative.
Contemporary methods do not imply that new technology is always appropriate.

Organization-specific facts are supplied per job, not embedded into reusable
source. Industry guidance is a design reference, not evidence of enterprise
approval. Unknown or conflicting mandatory requirements remain explicit.

## Acceptance

The worker's CONTRACT.md defines bounded, current input/artifact bindings and
status-dependent requirements. Its read-only check validates the package and
mechanically enforceable consistency; QUALITY.md defines independent semantic
review. No passing file check proves that a design is sound or approved.

Synthetic tests live beside the definition. Authoring, live model cases,
reviewer judgments and process records remain outside the source library.
See the [worker](../workers/solution-architect/expert/README.md) and
[evaluation runbook](WORKER-EVALUATIONS.md) for repeatable use and checks.
