# Prototype run record

September 6, 2026. Both workflows completed with the current local Weave, Tend,
Ask and Ply builds. All business data are frozen synthetic fixtures.

The subsequent [bounded invoice study](RESEARCH-RESULTS.md) uses a new fixed
intervention contract and live model comparisons. This historical demo allowed
editable handling-time assumptions and perfect review detection; its apparent
gains are not evidence that the newer study or a real operation improved.

| Workflow | Checked tasks | Result |
| --- | ---: | --- |
| Policy review | 4 | Revise the proposed approval rule; preserve the cumulative-budget objection and receipt/urgent-supply tradeoff |
| Invoice process | 9 | Propose a limited routing-method pilot, subject to empirical calibration |

Each workflow ran once in reference mode and once through real Ask/Ply commands
using a local Messages API fixture. The fixture supplies deterministic responses;
these runs establish CLI composition and evidence handling, not model reasoning
quality. No external inference was used for this run record.

The policy fixture's first synthesis omitted the budget guardrail. Ply's checker
rejected it, and a second candidate passed. The accepted synthesis retains the
single controls finding: 58,000 committed plus a 4,800 invoice exceeds a 60,000
monthly limit, although that invoice is below the 5,000 approval threshold.

The invoice experiment actually applies each proposed parameter set to the
simulator. Development results were:

| Method | Median minutes | p95 minutes | Labor minutes | Rework | Decision across all scenarios |
| --- | ---: | ---: | ---: | ---: | --- |
| Baseline | 31.5 | 44 | 228 | 0% | Reference |
| Routing | 24.5 | 32 | 204 | 0% | Candidate |
| Batching | 38.5 | 47 | 192 | 0% | Not improved |
| Risk selection | 6 | 144 | 144 | 16.667% | Not improved |

The comparison also checks demand +40%, staff absence, withheld cases, and a
combined withheld stress scenario. Risk selection illustrates why a better
median alone cannot justify a process change. The pilot report does not claim
observed business improvement; its first step is to calibrate the assumptions
against actual handling times and exceptions.

Raw evidence remains in the local archive `prototype-20260906` and is not
distributed with the release. It includes:

- Policy reference outcomes: `reference-policy-review-results.jsonl`.
- Invoice reference outcomes: `reference-invoice-process-results.jsonl`.
- Policy Ask/Ply fixture outcomes: `model-fixture-policy-review-results.jsonl`.
- Invoice Ask/Ply fixture outcomes: `model-fixture-invoice-process-results.jsonl`.

Each outcome points to its checked artifact and execution evidence. Model-mode
outcomes also point to the Ask session and sealed check events. This document
is a historical summary, not the complete audit evidence. Use the
[README setup](../README.md) to create reference runs in another checkout;
`make bootstrap` followed by `make check-examples` reproduces the fixture
integration with pinned public dependencies.

Validation covers the filter's malformed/stale input behavior, both workflows,
corrupted artifacts and receipts, rejected synthesis revision, pre-checks that
create no session, refusal of model shell actions, resumption without duplicate
attempts, driver death, and uncertain outcomes that are never retried. A real
Tend run completed 300 reference tasks with an eight-worker bound; concurrent
writers using the same Tend serialization key ran one at a time. This is an
execution test, not a 300-model capacity or provider-cost benchmark.
