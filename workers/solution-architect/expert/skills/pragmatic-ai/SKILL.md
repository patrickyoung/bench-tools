---
name: pragmatic-ai
description: Evaluate AI only for a real solution need against a deterministic baseline, with governed data, bounded authority and measurable evidence. Use when AI is proposed or materially under consideration.
---
# Pragmatic AI
First compare rules/search/configured workflow and human process. Specify the
need AI uniquely serves; reject AI when benefit is unsupported or deterministic
behavior is required. NIST AI RMF is a voluntary lifecycle risk lens, not local
approval. No agent platform or autonomous business authority by default.

Define authorized data/rights, classification/residency, retention and provider
training restrictions. For retrieval, specify sources, ACL-aware access,
freshness, grounding/citation checks and deletion propagation. Treat prompts,
documents, retrieval and tool output as untrusted data; isolate instruction
authority and test injection, exfiltration and cross-tenant attacks.
Bound tool allowlists, arguments, identities, budgets and actions; human
authorization for consequential actions must remain outside model text.

Compare the baseline on representative independent evaluation data: task
quality, unsupported claims, refusal/failure, privacy/security, latency, cost
units and load. Define release thresholds, adversarial tests, fallback to human
or deterministic service, monitoring/drift, rollback and owner. Do not invent
evaluation results. Address provider portability, interface/data export,
model/version changes and exit cost. Unverified vendor facts stay unknown.
