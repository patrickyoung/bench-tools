# Bounded invoice research run — September 7, 2026

**Keep the bounded experiment loop; defer a larger swarm and do not recommend a
business pilot from this benchmark.** The live comparison completed 81 candidate
slots and 54 real model calls. No selected method qualified in any of the nine
arm/trial confirmations. There were no failed proposals, failed measurements or
logged model retries. All results remain synthetic.

## What the comparison found

| Method | Feasible candidate slots on both search splits | Duplicate slots | Model calls | Input / output tokens | Median trial execution span | Confirmed selections |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Parameter sampling | 11/27 | 0/27 | 0 | 0 / 0 | 0.34 seconds | 0/3 |
| One adaptive agent | 20/27 | 0/27 | 27 | 255,346 / 23,541 | 302.73 seconds | 0/3 |
| Three-agent team | 27/27 | 14/27 | 27 | 235,959 / 22,371 | 121.83 seconds | 0/3 |

The agents found a better candidate than parameter sampling in two of three
trials; sampling found that same candidate in the third. Both agent arms selected
the exact same method in all three trials: **earliest-due work, receipt precheck,
review of flagged cases plus the fixed audit cohort, and single invoices**.
The team had no demonstrated selection-quality advantage over one agent.

Parallel calls shortened the team's measured execution spans. Its proposals
consistently passed the development/validation feasibility checks, but 14 of its
27 slots repeated a configuration already proposed within that trial. The single
agent repeated none. A team can therefore appear busy and competent while
covering fewer distinct options. The token totals do not establish equal dollar
cost; the team also recorded 25,088 cached input tokens, included in its input
total. Single and team reasoning tokens were 20,552 and 19,250 respectively,
already included in output totals.

## Why the attractive candidate failed

The selected agent method reduced staff labor and fixed the visible search-set
service problem, but its cycle-time benefit did not survive the final test:

| Evidence split | Baseline mean scenario median | Selected mean scenario median | Reduction | Worst p95: baseline → selected | Selected service / rework |
| --- | ---: | ---: | ---: | ---: | --- |
| Development | 23.17 min | 21.83 min | 5.76% | 93 → 45 min | Feasible in every scenario |
| Feedback-validation | 23.60 min | 20.57 min | 12.82% | 95 → 42 min | Feasible in every scenario |
| Final holdout | 23.50 min | 23.12 min | 1.62% | 94 → 132 min | 91.67% on time; 8.33% rework in every scenario |

The frozen requirement was at least 10% mean-median reduction on **both** search
splits, plus each scenario's service, rework, backlog, p95 and labor checks.
Development already missed the improvement hurdle. On the final cases, the
selected method additionally missed the 95% service floor and 5% rework ceiling,
and increased tail latency. Its final labor fell from 407 to 368 minutes, but
that saving did not override the other failures. The final fixture includes
defects outside the visible flag/audit groups, exposing the review shortcut.

Looking only at the 12.82% validation gain would have overstated success. The
separate final test and explicit rejection criteria were the useful autoresearch
adaptations in this study. A successful run of the software did not become a
false claim of a successful process change.

## Was there a qualifying option to find?

After every original selection and confirmation was complete, a separate
post-hoc diagnostic, included in the [portable aggregate summary](results/2026-09-07.json),
evaluated all 36 configurations on development and feedback-validation. It found
**11 feasible configurations and zero eligible configurations**. Among feasible
configurations passing the p95/labor guards, the largest weaker-split reduction
was **7.194245%**, below 10%. That configuration used shortest-ready work with
precheck, flagged-plus-audit review and single invoices; its validation reduction
was 12.815337%.

No amount of searching this fixed option set could have met the declared paired
search target. That conclusion comes from the later exhaustive diagnostic, not
from any arm's nine-slot allowance. It does not establish that no better real
process exists. The common validation ranking still selects earliest-due work
because validation objectives tie and canonical configuration bytes break the
tie; the post-hoc development maximum never revises the frozen selections.

The diagnostic used zero model calls and a 3.028-second Tend execution span.
Its local guarded runner, `research-diagnostic.py`, audited all nine original
confirmations before admitting its own tasks. Re-running it preserved the
original evidence and produced identical diagnostic results without new attempts.
That runner and its raw evidence remain in local archives and are not distributed.

For a space this small, ordinary enumeration is the economical first tool.
Use agents to suggest genuinely new intervention options when supported by
process evidence, then evaluate those options under a separately frozen contract.
Before any real pilot, replace the stipulated effects with a reviewed evidence
and calibration design. Do not expand concurrency on the strength of this run.

## Protocol and evidence

This run compares seeded parameter sampling, one adaptive researcher, and a
three-agent adaptive team. Each arm has nine candidate slots in each of three
trials. The baseline is shared; final confirmation is additional. Single and
team each have 27 model proposal invocations across the study. Every arm and
trial must freeze its selection before any final confirmation.

The model was `openai-codex/gpt-5.6-sol`, with medium reasoning effort pinned in a
local credential wrapper. Ask uses the existing Codex login. Credentials are
absent from the tracked recipe and run artifacts. A separate earlier `READY`
connectivity probe is outside the comparative proposal count.

The fixture contains 24 development, 24 feedback-validation, and 24 final cases.
Every duration, detection outcome and intervention effect is stipulated synthetic
evidence. There is no empirical calibration or measured company improvement.
The [fixed domain contract](RESEARCH-DOMAIN.md) and [operator protocol](RESEARCH.md)
describe the assumptions and acceptance requirements.

The release includes a [portable aggregate JSON summary](results/2026-09-07.json)
with all nine arm/trial results, selected-candidate scenario measurements,
recorded usage, and the 36-configuration diagnostic. It preserves the original
source, benchmark, experiment, selection and report hashes. It omits local paths,
authentication details, session contents and artifact references.

This export is a derived summary, **not the full audit evidence**. Raw Ask logs,
Tend events, checked artifacts, frozen inputs and audit files remain in the local
archives `research-live-20260907`, `research-live-20260907-diagnostic` and
`research-live-resume-audit.json`; they are not distributed. The published hashes
identify those original records but cannot independently reproduce their audit
without the records themselves.

Benchmark identity:
`sha256:09dbda71704aef7a4341f6691dbcd03a57a512f97d9789a675b7f16c3d784ddf`.

The identical live command was run again after completion. All 54 model-session
files and every Tend event snapshot remained byte-identical, and the rebuilt
report was identical. No new model calls or execution attempts were needed.

An independent audit checked all 47 round snapshots and all 145 Tend jobs;
every job finished in one attempt with exit 0. All 54 Ask sessions passed native
replay and matching sealed-receipt checks. The ledger, feedback, selections and
confirmations rederived correctly. Total recorded usage was 491,305 input and
45,912 output tokens, with zero retries. Peak active attempts was three.
The final job finished 1,260.513 seconds after manifest creation, within the
1,800-second deadline. The last sealed Ask event preceded first confirmation
admission by 2.684 seconds, and every confirmation bound the same complete
selection document. This is local event evidence, not external timestamp
attestation or a demonstration of hundreds of concurrent agents.

Go unit tests, race tests and vet passed. The 43-test offline suite and nine new
controller tests passed: **52 Python tests**, including a real CLI/fixture-inference
three-arm run, failure/duplicate accounting, holdout exclusion and recovery.
The historical check logs, `research-checks.log` and
`research-controller-checks.log`, remain in the undistributed local archive.

The JSONL ledger and report are derived from Tend execution evidence, checked
artifacts and replay-verified Ask sessions. They are not independent authorities.
An accepted experiment is a valid measured result; eligibility and final
confirmation are separate business criteria.

## Interpretation limits

Nine candidate slots per arm are comparable allowances, not equal realized
tokens, money or CPU work. Deterministic checker re-evaluation creates additional
verification work, without revealing new candidates to agents. Reported
execution spans sum each round's first attempt start to last attempt finish;
they exclude planning and verification gaps and are not end-to-end runtime.
Reasoning tokens are a subset of output tokens; cached tokens are a subset of
input tokens. No dollar cost is inferred from subscription-backed usage.

The team sees feedback only between batches of three; the single researcher
sees it after every proposal. This is a deliberate protocol difference. The
three trials measure search variability on the same fixture, not independent
business evidence. The finite 36-configuration space cannot establish discovery
of new process methods or performance of hundreds of agents.

The separately labelled exhaustive diagnostic is outside the comparison's
allowances. It did not change the frozen selections or expose additional final
cases to the model. These results support keeping the evidence and evaluation
recipe; they do not demonstrate a business improvement or a large-swarm benefit.
