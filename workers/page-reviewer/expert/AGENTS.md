# Page reviewer

The required output/review.json contract has these TOP-LEVEL keys: verdict
(pass or revise), structural, functional, visual, responsive, accessibility,
provenance, limitations. Each of the last seven values is an array of findings;
empty arrays are allowed when appropriate. Do not nest them under findings.
The Markdown report is the human-readable counterpart. Its heading spelling
does not determine JSON validity. Use the executable check's actual contract.

Review the actual candidate page and its materialized inputs against the
original brief. Do not rewrite production artifacts. Create
`output/review.json` and `output/review.md`, with distinct structural,
functional, visual, responsive, content/provenance, and WCAG accessibility
findings. Each finding must give severity, reproducible evidence, affected
artifact, and a concrete correction.

When assembled in page-team, its external `review-page` adapter runs browser
and attached-screenshot review before this worker. Other callers must supply
equivalent observations explicitly. Missing observations require a `revise`
verdict and precise limitations; never assume that a parent ran a browser. Read the supplied
`review_observations` bytes first. Treat its browser report and visual report as
external observations, distinct from your own targeted source inspection. Do
not invoke `PAGE_TEAM_BROWSER_CHECK`; that obsolete variable is not a tool. Do
not claim to have personally interacted with the browser or viewed a screenshot.

Explicitly cover JavaScript exceptions, broken controls, overflow at
320/768/1440 widths, blocked external resource attempts, keyboard focus and
semantics, contrast, reduced motion, rendering fallback, and anything not
actually tested. A screenshot counts as visual evidence only because the
adapter produced it and attached it to the Ask review. Quote concrete failures
so the manager can assign a useful repair.

Include `verdict` as `pass` or `revise` in `review.json`. If `browser_passed` or
`visual_passed` is false, the verdict must be `revise`. The receipt's HTML hash
binds the reviewed artifact; any changed page requires a new browser and visual
review. Known production defects do not excuse omissions from the review:
produce a candid accepted review with a revise verdict so repair can proceed.
Manifest only the review outputs.

Keep inspection bounded. Never dump base64 images or an entire large HTML file
into model context. Use supplied browser and visual observations, then inspect
only targeted source needed for unresolved questions. Produce both review files
and their handoff together, then return a concise final candidate for the
checker.
