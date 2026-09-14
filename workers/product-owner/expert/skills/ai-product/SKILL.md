---
name: ai-product
description: Shape evidence-led AI-enabled product decisions with human control, evaluation slices, privacy, accessibility, reliability, cost, latency, guardrails, and rollback. Use when a request proposes or evaluates AI.
---

# AI-enabled product ownership

Begin with the user decision or task and why AI is preferable to a deterministic
or non-product alternative. Never make AI adoption itself the outcome.

Define a representative evaluation matrix across customer segments, common
tasks, edge cases and plausible harms. Bind each slice to supplied test data or
a plan to obtain lawful representative data. Evaluate task success and failure,
not only model-style scores. Include where relevant:

- false-positive/false-negative consequences and escalation;
- privacy, retention, provenance, security and data rights;
- accessibility and language performance;
- quality drift and abuse;
- latency, availability, unit cost and total operating cost;
- explainability appropriate to the decision and user recourse.

Specify explicit human control: what the system may suggest, what a person must
review or approve, how they override it, and who is accountable. Avoid nominal
human-in-the-loop controls that lack time, context or authority.

Use shadow mode, limited cohorts, feature flags or another reversible slice.
Define release gates, monitoring, stop conditions, rollback/fallback and
incident ownership. Do not claim current model capability, legal compliance,
fairness, ROI or production readiness without supplied evidence.

NIST AI RMF materials are optional risk guidance, not certification. DORA's
AI research supports examining the surrounding work system, not arbitrary
productivity promises. External model, browsing, analytics and account access
are unavailable unless the caller explicitly supplies admitted evidence; never
imply they ran.
