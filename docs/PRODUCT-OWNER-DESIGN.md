# Designing the Product Owner worker with SIPOC

[Workers](../workers/README.md) · [Worker contract](../workers/product-owner/expert/CONTRACT.md)

This is the maintainer's design brief for building and revising the worker.
SIPOC describes the job we are teaching it and the boundaries we must test.
This document is not exported as runtime instructions or required job output.

The worker's purpose is to turn product demand and current evidence into useful
outcome and backlog recommendations, with delivery awareness. A human can review
the answer; another worker can consume the explicit handoff.

| Design element | What it means for this worker |
| --- | --- |
| Suppliers | The requester supplies the job. Customers, researchers, Product Management, engineering and operations supply evidence through the caller or explicitly selected upstream workers. |
| Inputs | A natural-language request and current supporting files: customer needs, product goals, observations, constraints, backlog candidates, estimates, dependencies and capacity where known. Missing facts stay unknown. |
| Process | Understand the requested decision; assess evidence and desired outcomes; compare options and trade-offs; recommend priorities or a learning step; identify delivery dependencies and next actions; check the handoff against the actual inputs. |
| Outputs | A useful intake recommendation, ordered backlog, product review or requested interview answer; a compact machine-readable decision; blocking questions when responsible advice needs more context. |
| Customers | The human Product Owner or sponsor reviewing decisions, the delivery team refining work, and explicitly selected workers carrying out the next authorized step. |
| Constraints | Use current supplied evidence, preserve developer sizing and human decision authority, distinguish proposals from observed results, and keep reusable source separate from run content. |
| Measures | Input and response bindings verify; acceptance criteria are observable; priorities and uncertainty follow evidence; missing context produces a short useful question; realistic trials demonstrate judgment without fabricated claims. |

The process starts when a caller supplies a current job and ends when a checked
recommendation or clarification is returned. Gathering evidence in external
systems, executing delivery work and realizing business benefits have separate
owners; a recommendation is not proof that those effects occurred.

## How the design drives implementation

- Suppliers and inputs define `request.md`, `inputs/` and the trust boundary.
- The process determines the six skills: product intake, discovery, backlog
  planning, delivery flow, AI product judgment and interview/review reasoning.
- Outputs and customers determine `response.md`, `decision.json`, mode-specific
  acceptance and explicit next-action ownership.
- Constraints determine the read-only artifact checker and human review limits.
- Measures determine the synthetic contract tests and the separately run job
  evaluations described in the [test guide](../workers/product-owner/tests/README.md).

Use this brief when revising the existing Hire-built definition. Change only
the instructions, skills and contract required by a changed job boundary;
evaluate in fresh workspaces through Agent before promoting the source pin.
SIPOC itself is not a mandatory worker deliverable. Intake should explain the
customer problem, intended outcome and recommended next decision. A process map
belongs in an answer only when the actual job calls for one.

## Relationship to SAFe

SIPOC is a process-scoping tool from Six Sigma, described by
[ASQ](https://asq.org/quality-resources/sipoc). Applying it to worker design is
our design choice. It is not prescribed as a standard Product Owner deliverable
in the public [SAFe Product Owner guidance](https://framework.scaledagile.com/product-owner)
reviewed on 2026-09-13. That guidance emphasizes customer and stakeholder needs,
value, team backlog alignment and collaboration with Product Management.
A Product Owner may use process-analysis tools when useful; the role is not
defined by producing those diagrams.
