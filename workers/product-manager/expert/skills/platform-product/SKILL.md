---
name: platform-product
description: Shape internal and developer platform features around an evidenced consumer journey, self-service, contracts, safe migration and sustainable operations. Use relevant concerns for platform strategy; do not force a platform template onto vendor evaluation or synthesis.
---

# Platform as a consumer product

Use alongside feature-shaping for platform strategy and leadership-narrative
for the decision. Internal developers, operators and other internal consumers
have jobs and constraints; they are not merely implementation resources.
Use only concerns relevant to this decision. A small API improvement does not
require a portfolio inventory, funding model redesign or platform-wide roadmap.

## Start with one evidenced journey

Select one consumer cohort and job supported by supplied evidence. Follow its
trigger, discovery, onboarding/access, task completion and recovery/support;
identify the costly or unreliable moment, current workaround and desired
behavior. Cite the evidence rather than invent personas, interview quotes or
friction. If there is no journey evidence, describe a candidate journey as a
hypothesis and make observation/validation the first commitment. Do not call
a portal, catalog or mandated rollout the problem definition.

Use that journey to choose a narrow end-to-end slice and stable feature IDs.
Self-service means the consumer can find, understand and complete the task
safely within policy, not merely submit a ticket through a new interface.
Where human approval is necessary, expose state, next action and escalation.
Specify actionable failures: what failed, what the consumer can safely do,
what remains unchanged and how to get support without leaking sensitive data.

## Select consequential boundaries and guardrails

- **Product boundary and contracts:** who provides/consumes which service;
  supported APIs/events/data contracts; authentication/authorization;
  compatibility/versioning and limits. Reuse existing contracts where suitable.
  Treat proposed interfaces as proposals, not architect-approved designs.
- **Dependencies versus sequence:** identify hard provider prerequisites,
  consumer changes and independently valuable work. A shared dependency does
  not justify building a broad platform before testing the first journey.
  Tie technical enablers to consuming features and a testable need.
- **Migration and rollback:** coexistence with existing workflows, compatibility
  checks, opt-in/pilot path, state/data migration, safe reversal or recovery
  when irreversible, and conditions for retiring the old path. Name proposed
  responsibility and evidence needed; never fabricate a migration date.
- **Security, reliability and operability:** relevant access isolation, secrets
  or data protection, auditability, failure containment, recovery, service
  promises, observability and support/escalation. State supplied thresholds
  or unknown/proposed status. NFRs constrain the useful slice rather than
  become a detached wish list. Include failure behavior in feature acceptance.
- **Discoverability, adoption and lifecycle:** documentation/examples, onboarding,
  consumer feedback, support ownership, upgrades, deprecation and exit.
  Explain what would make consumers choose the path and how non-adoption
  would be investigated; mandated use is not evidence of preference or value.
- **Funding and unit costs:** identify unknown operating/support capacity,
  investment authority and cost drivers. Use finops only to the depth needed.
  A proposed useful unit (such as a successfully completed consumer task)
  needs a numerator, denominator, period and service-quality context; missing
  cost data remains unknown. Never invent savings or approved funding.

## Observe usefulness before expanding

Pair consumer task success, effort/time or failure recovery with feedback and
relevant delivery/operational guardrails. Preserve start/end events, cohort
and measurement window; waiting on one stage is not total journey lead time.
Keep adoption as a leading signal distinct from consumer outcomes and downstream
customer/financial benefit. Describe the uncertain mechanism to those benefits,
not a causal claim based on usage.

Recommend the smallest reversible journey test, with proposed collection owner,
review trigger and advance/change/stop conditions. Broader rollout needs
evidence of useful consumer behavior and sustainable support, not a completed
catalog. Apply feature-shaping's acceptance/outcome distinction. Summarize
relevant platform constraints and lifecycle in the existing JSON fields; no
additional artifacts, schema or platform implementation is authorized.
