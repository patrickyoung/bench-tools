# Ply verbosity comparison, 2026-09-09

The final default used **3,762 provider output tokens**, compared with **5,125
for contemporaneous Pi** and **10,703 for original Ply**: reductions of 26.6%
and 64.9%, respectively. All nine tasks passed for each tool. Median driver
time fell from 47.1 seconds for original Ply to 15.7 seconds, versus 23.3
seconds for Pi. This is a small, repeatedly used development corpus, not a
held-out capability benchmark or a causal estimate of latency improvement.

## Method and results

Both arms used `openai-codex/gpt-6-astra` at medium reasoning effort. The
[task harness](../README.md) gave each tool the same three fixtures, three
repetitions each, fresh work and session state, an external outcome oracle,
and a 180-second wall limit. The final cohort used frozen source and private
builds with the exact new default prompt. Native prompts and tool protocols
differ, and Pi has no equivalent adapter-enforced model-turn cap. Subscription
cost is unknown. The original baseline was a separate earlier cohort.

Output tokens count all reported provider output across model requests,
including reasoning where included by the provider. They do not measure only
the last visible report. No response truncation, hidden logs, or lower reasoning
effort produced the reduction. Report words are whitespace-separated and
include the harness success marker when present.

| Tool / cohort | Accepted / scheduled | Total output tokens | Median driver seconds | Median report words |
| --- | ---: | ---: | ---: | ---: |
| Original Ply | 9 / 9 | 10,703 | 47.1 | 53 |
| Final Ply | 9 / 9 | 3,762 | 15.7 | 20 |
| Contemporaneous Pi 0.81.1 | 9 / 9 | 5,125 | 23.3 | 25 |

The final cohort recorded all 18 scheduled trials, with no invalid runs,
timeouts, native process failures, or false completion markers. Every task
below was accepted in all three repetitions for both tools. Token columns
are totals across the three repetitions; time columns are their medians.

| Task | Original Ply tokens | Final Ply tokens | Pi tokens | Ply / Pi | Original Ply seconds | Final Ply seconds | Pi seconds |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Multi-file repair | 3,770 | 2,424 | 3,418 | 0.709 | 48.2 | 33.0 | 49.4 |
| Clarification | 2,055 | 363 | 444 | 0.818 | 36.1 | 12.4 | 15.9 |
| Large output | 4,878 | 975 | 1,263 | 0.772 | 52.6 | 15.7 | 23.3 |

Final Ply per-repetition token counts were `821, 798, 805` for repair,
`122, 121, 120` for clarification, and `330, 315, 330` for large output.
Pi's corresponding counts were `1081, 1117, 1220`, `148, 151, 145`, and
`440, 414, 409`.

The criteria were fixed before the repeated comparisons: nine accepted Ply
tasks, no false completion claims, an aggregate Ply/Pi output ratio at most
1.25, and each task's ratio at most 1.5. The final aggregate ratio was 0.734;
all criteria passed. Three earlier repeated cohorts also accepted all 18
tasks each but failed the clarification threshold. Their Ply/Pi totals and
clarification ratios were `4550/5226; 2.246`, `4927/5208; 1.519`, and
`5096/5208; 2.005`. Separate pilots informed the changes and are not pooled
into the scored cohort. The original three-tool comparison accepted 27/27
tasks; a planned Claude arm failed login preflight and scored no task trials.

## What changed and what Pi's source showed

Ask now accepts and records `-verbosity`, mapping it to Responses
`text.verbosity`; standalone Ask keeps the provider default unless configured.
Ply defaults to low, passes overrides through to Ask, and preserves the
policy for compaction and nested Ply processes. Its prompt calls for concise
reports, direct reads, economical shell writes, and proportional verification.
Requested detail, complete artifacts, uncertainty, and numeric precision take
priority. Rebuild Ask and Ply together: old Ask binaries do not recognize the
new flag. `ply -verbosity ""` omits the flag and lets Ask choose.

Inspection used the installed primary distribution of
`@earendil-works/pi-coding-agent`, `@earendil-works/pi-ai`, and
`@earendil-works/pi-agent-core`, all version **0.81.1**. Their declared
[upstream is earendil-works/pi](https://github.com/earendil-works/pi); these
findings describe that installed version, not the latest upstream source.
The package-relative references below make the inspection portable.

| Finding | Package and distributed source reference |
| --- | --- |
| Codex requests default to low verbosity | `pi-ai/dist/api/openai-codex-responses.js`, request builder around lines 341-355 |
| Concise generic guidance and short tool summaries | `pi-coding-agent/dist/core/system-prompt.js`, lines 39-69 |
| Writes carry a path and literal content; edits use small replacements | `pi-coding-agent/dist/core/tools/write.js`, lines 11, 131, 159; `edit.js`, lines 158, 212 |
| Typed actions precede actual tool results | `pi-agent-core/dist/agent-loop.js`, line 105; `pi-ai/dist/api/openai-responses-shared.js`, line 164 |
| Bash results do not repeat the submitted command | `pi-coding-agent/dist/core/tools/bash.js`, line 318 |
| Session WebSocket reuse and guarded delta requests exist | `pi-ai/dist/api/openai-codex-responses.js`, lines 786, 1024-1084; `pi-coding-agent/dist/core/settings-manager.js`, line 501 |

Pi's structural action boundary suggested a distinction worth expressing in
Ply's text protocol: a command block completes the current model request,
while the user's task may require another request containing its real result.
The concluding task report is a separate outcome. Inspection of 33 historical
Ply first responses found command blocks in `commentary`; all 17 with trailing
text also had a nonempty `final_answer`. Ask preserved these phases correctly.
Commentary also occurred in clean responses, so phase alone was not causal
proof. The final wording changed the request boundary and requested artifact
fields together; the experiment does not isolate one sentence's effect.

All 33 responses in the final Ply cohort contained exactly one `final_answer`
item: 24 actions and nine concluding reports. Manual review of all 18 tool
trajectories found no extra blocks, deferred tails, invented results, or
unsupported completion claims. This is observed behavior on this cohort,
not a guarantee of future model formatting.

Pi's transport is a separate unmeasured lead. It can reuse a session WebSocket
and send `previous_response_id` plus new input when request parameters and
the previous input prefix match. Ask starts fresh for each Ply turn and sends
full history. Actual transport selection, fallback, and reuse counters were
not captured, so no latency benefit is attributed to those mechanisms.
Neither transport changes nor built-in tools were added to Ply.

The original latency breakdown placed 97.7% of Ply wall time between model
request and response records, with about 1.1 seconds per trial outside those
windows. Those windows include networking, queueing, reasoning, generation,
and local response handling. They are not pure model computation. The observed
improvement retained the command-line architecture.

## Additional checks and limits

The final trace audit verified source preservation, unresolved clarification
choices, all 30,000 telemetry records, and exact incident IDs. All six repaired
implementations passed an additional decimal-rounding probe returning 44 cents
for `invoice.total(0.29, 3, 50)`. An earlier candidate returned 43 once; this
edge was outside the frozen oracle and does not prove a verbosity regression.
The second and third repeated cohorts recovered from five and three
first-turn response spillovers, respectively; those failures remain part of
the experiment history.

An explicit-detail task produced a correct 543-word artifact with all five
requested sections and a 32-word final report. It recovered from one nested
Markdown-fence mistake before completing and checking the artifact. Thus zero
deferrals applies to the final scored cohort, not every supplemental live
check. One compaction fixture recovered all 15 required fields, preserving
pending steps, uncertain external effects, and absent retry authorization.
This is one handoff check, not broad compaction-safety evidence.

Ask and Ply normal/race suites passed, as did documentation and whitespace
checks. Private-runtime cleanup checked 258 source/dependency hashes without
mismatches and scanned 1,213 retained files without credential matches.
Original native login files were unchanged and the private runtime was removed.
Raw local trajectories and the full experiment archive remain outside this
repository; no credentials or private runtime are included here.

The fixed corpus was reused while refining prompts. There is no held-out
generalization claim, statistical significance claim, billed-cost comparison,
or isolated transport experiment. Reproduce fresh measurements with the
[documented harness and explicit driver configuration](../README.md), retaining
all scheduled outcomes and native usage evidence.
