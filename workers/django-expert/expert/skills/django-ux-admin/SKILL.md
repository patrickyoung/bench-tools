---
name: django-ux-admin
description: Build and iterate professional Django admin and accessible task-first customer journeys using server rendering, modern CSS and selective htmx.
---
# Two interfaces, one domain
Admin is for trusted internal staff, not the default customer UI. Prefer native
ModelAdmin list_display, search_fields, list_filter, autocomplete_fields,
readonly/audit fields and usable formsets. Scope get_queryset, related field
choices, object permissions, exports and actions to staff/tenant authority.
Optimize relations and measure query budgets on representative list/change pages.
Safe bulk actions recheck authorization for each target, validate inputs, show
consequences and require confirmation for destructive effects; use transactions/
batching appropriately. Audit actor/time/change without leaking sensitive values.
Test that crafted object IDs and posted relations cannot cross tenant boundaries.

## Iteration loop
1. Identify user, task, context (device/connectivity/assistive technology) and
   measurable acceptance criteria. Separate evidence from assumptions.
2. Offer low-cost prototype/alternatives and explain tradeoffs. Seek actual
   feedback where possible; otherwise record hypotheses and requested review.
3. Ship a thin usable vertical slice, including empty/loading/error/success/
   forbidden states, helpful copy, understandable navigation and recovery.
4. Review keyboard, focus order, mobile layout, slow networks, no-JS behavior and
   supported browsers. Evaluate real task completion, not just attractive images.
5. Record observations, changes and next experiment. Never fabricate user studies,
   feedback, task metrics, screenshots or accessibility certificates.

## Rendering and interaction
Default to Django views/forms/templates and built-in partialdef/partial and
template.html#partial. Preserve escaping; only mark safe justified sanitized
content. Avoid redundant legacy partial packages. Use semantic HTML/native
controls, labeled forms, field errors plus linked error summary, retained safe
input, clear destructive confirmation and consistent success/permission feedback.
Use supported container size queries with sensible fallback layouts; do not assume
style/scroll-state queries have equal browser support. Use native dialogs only
when a modal helps the task; ordinary pages/forms are often simpler. Test focus,
dismissal and keyboard behavior. Use responsive CSS grid/flex and design tokens for
typography, spacing, color, focus and states. Restrained modern styling, readable
content, responsive tables with intelligible headers and overflow alternatives.
Search/filter/sort/pagination state belongs in URLs for back/forward/share behavior.

Target WCAG 2.2 AA: keyboard-only use, visible unobscured focus, contrast,
names/labels, target sizes, reduced motion, errors and recovery, status messages
and live regions. Avoid focus traps or unexpected focus movement; restore focus
deliberately after replacement. Automated tools supplement manual testing, never
prove conformance. Native flows should remain useful without JS.

Add selective htmx only when it improves a task. Pin self-hosted assets; use
external CSP-compatible scripts, allowEval=false, allowScriptTags=false,
selfRequestsOnly=true. Send Django CSRF on unsafe requests; never disable CSRF
for partial endpoints. Server-side auth/authorization applies equally to full and
partial responses. Set Vary for request headers that select representations
(e.g. HX-Request), combine existing Vary values correctly, and control private
cache behavior. Disable private history snapshots (hx-history=false), avoid
sensitive browser storage, and test history/back navigation across logout.
Handle auth expiry without injecting login pages into arbitrary fragments,
status/error recovery, focus and live-region updates. Consider full redirect on
expired authentication. Test no-JS alternatives and duplicate/retried requests.

Use Django native CSP middleware deliberately: start with a reasoned policy,
report-only rollout if needed, then enforcement. Check nonce/cache interaction;
prefer external scripts, no unsafe-inline/unsafe-eval shortcuts. Add richer
islands/APIs, CSS frameworks or a build stack only for demonstrated benefit with
bundle/performance, accessibility, dependency and delivery costs explained.


## Frontend performance
Set explicit CSS/JS/images/fonts budgets for the supported devices and networks;
minimize transfers and unnecessary work, reserve media dimensions to avoid shifts.
Measure actual LCP/INP/CLS on representative journeys. Starting field budgets at
p75: LCP <=2.5s, INP <=200ms, CLS <=0.1. Record workload, device/network, tool,
sample/window and evidence. Distinguish field user measurements from controlled
lab diagnostics; lab results do not establish field p75 or actual user INP.
Never claim measured scores without measurements; disclose unavailable field
evidence and next measurement actions. See references.md for MDN and web.dev.
