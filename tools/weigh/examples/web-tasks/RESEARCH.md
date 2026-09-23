# What people were building with JEV, September 23, 2026

Research question: what practical browser work do the September demonstrations
support, and what can be composed with the existing Web and Weigh programs?
Directories and roundup videos were leads; the conclusions below use the
builders' code, reports and an original video transcript. Published results
are author reports, not measurements reproduced by this project.

## Browser builds worth borrowing from

| Original source | Actual problem and implementation | Evidence and limit | Lesson used here |
| --- | --- | --- | --- |
| [Browser Use: jev-ultrafast](https://github.com/browser-use/jev-ultrafast), [measurements](https://github.com/browser-use/jev-ultrafast/blob/1231850a0bf1a0c0341fe408ef1668dbbfdfac46/docs/performance.md) | Navigate Google Flights from a goal. JEV chooses an operation and compatible target from a freshly observed element table; a separate small LLM writes field values. | The published run is 7.073 seconds after initial observation, with 17 JEV requests and two text calls. Three alternating pairs passed both versions; this is one repeated task, not general browser reliability. It searches; it does not book. Browser cost is excluded and JEV's dollar charge was estimated, not provider-reported. | Observe once, bind choices to real candidates, ask independent next-step questions together, verify the destination separately. |
| [AICodeKing, September 17 video](https://www.youtube.com/watch?v=SNJ3yuJ_QwY&t=297s) | At about 4:57 the creator explains the flight build and the JEV/text-model split. | Original auto-caption transcript retrieved and inspected. The creator explicitly says he did not reproduce the flight run. His earlier eight small playground calls do not establish browser reliability. | Trace a persuasive video back to its executable source and measurement boundaries. |
| [Tontoko: Jev Browser](https://github.com/tontoko/jev-browser) | Natural-language browser actions, form filling and extraction tied to observed DOM records. | The README separates model completion from UI readback and caller assertions. Missing fields and ambiguous saves remain explicit. Its feature surface is much broader than Web's. | Keep source text and URLs with every shortlisted card; never transfer a fact between neighboring records. |
| [Sightmap: JEV Turbo task reports](https://github.com/sightmap/jev-turbo/blob/main/bench/README.md) | Select the intended IKEA variant when many buttons share the same label. | The builder reports 10/15 success without the map and 15/15 with card-scoped context. Fifteen attempts are a small task-specific result. | Labels alone are insufficient; preserve the text of the card/row that owns each link. |

## Other concrete uses investigated

[Tony Dinh's Sponsor Skip](https://github.com/trungdq88/youtube-sponsor-detection)
uses transcript-line selection plus code-owned timestamps to skip sponsor reads.
It ships a Chrome extension and local app, with a SponsorBlock comparison
procedure. Its README documents transcript fetching failures and audio-mode
overshoot. This is a useful application, but video tooling was not selected for
this build.

[pg-jev](https://github.com/realZachi/pg-jev) puts semantic filters and scores
beside SQL rows. The relevant pattern is explicit records with questions whose
results ordinary code can filter; it does not require moving the database or
its access controls into the decision model.

[Nate Herk's September video](https://www.youtube.com/watch?v=ymgH8jS6Wb8)
was another discovery lead. YouTube metadata was reachable but subtitle
retrieval returned HTTP 429. A third-party transcript preview described an X
feed classifier and other experiments; it was not used as evidence of measured
browser performance. Some roundup headlines described the flight demo as
booking: the original project says otherwise.

## Fit for this repository

Weigh already had semantic/visual output checks, diagnosis, fix selection and
calibration examples. Repeating those would not answer the request. The new
work instead composes two end-user browser tasks with the existing `web` CLI:

1. Follow links to find requested information, retaining the path and source.
2. Read rendered rows/cards and shortlist those with evidence for every condition.

The essential Web addition is an atomic snapshot of final URL, page text,
observed links and scoped records. The existing separate reads could observe
different page versions and did not expose the final URL needed after redirects.
No borrowed browser runtime or provider SDK is needed.

[TypeSafe's confidence documentation](https://docs.typesafe.ai/confidence)
describes confidence as derived from the output distribution. Its
[JEV 1.13 limitations](https://docs.typesafe.ai/model-jaggedness/jev-1.13)
warn about arithmetic, irrelevant context, indirection and adversarial state.
Consequently the applications keep exact strings/URLs, budgets and thresholds
in code, preserve unknown evidence, and make no claim that a typed answer is
correct or authorizes an action.

The supplied local-browser integration checks are offline protocol tests with
scripted judgments. They establish that the applications really drive Web and
consume Weigh; they are not a reproduction of the community's live-model results.
