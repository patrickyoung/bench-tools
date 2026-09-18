---
name: css-first-layout
description: Choose CSS-first layout, responsive typography, native state and progressive enhancement. Use when building new frontend pages or changing layout and design through refactors; not for exact-copy post-review assignments.
---

# CSS-first layout judgment

## Boundary and priorities

For a passing-review copy assignment, stop here: copy approved bytes exactly,
manifest them, and never restyle to apply this teaching. Repairs change the
reviewed result and require another controller-owned review. This skill changes
judgment, not the single-file output, creative-byte, interaction or handoff
contracts. It introduces no dependency or permission to fetch sources.

For new construction, layout work or refactoring, read
`references/semantics.md` alongside this skill (via Brief or the definition's
skill directory). The notes derive from the operator-curated packet reviewed
2026-09-17; they are not a browser support certificate.

1. Start with the brief's content hierarchy, primary task, reading order and
   art direction. Choose a clear type hierarchy, readable measure, spacing
   rhythm and deliberate alignment before effects. Do not turn every section
   into interchangeable cards or adopt features for their count.
2. Establish semantic HTML and a usable static baseline. Choose ordinary flow
   unless a relationship needs layout machinery; then choose Flex for one
   dimension, Grid for two. Explain the problem solved, not the novelty used.
3. Make content resilient before adding breakpoints: intrinsic sizes, wrapping,
   flexible gaps, suitable minimums and overflow diagnosis. Components should
   respond to available space, not guessed device brands.
4. Add the smallest enhancement with visible benefit. Keep modest tokens,
   layers and selectors; no framework installation to access native CSS.
   Compare complexity against a simpler baseline and remove redundant JS.
5. Establish target browser versions from the brief. When unspecified/offline,
   assume semantic HTML, ordinary flow and established Grid/Flex as the usable
   baseline; treat advanced features as optional with uncertainty disclosed.
   Use supplied dated target-browser evidence if available. Do not invent
   current compatibility, network access or cross-browser test results.

## Layout and typography

Use flow for prose, Flex with wrap/gap for toolbars and navigation, Grid for
aligned page regions or repeated tiles. Subgrid is useful when nested items
must share parent tracks; independent grids suffice when alignment is local.
Preserve DOM reading and focus order rather than fixing semantics with `order`
or grid placement. Choose content-driven breakpoints at the point relationships
fail, not fixed phone/tablet categories.

Automatic minimums can cause overflow even with fractional tracks. For a
deliberate two-column composition, illustrate the shrink boundary explicitly:

```css
.layout { display: grid; grid-template-columns: minmax(0, 2fr) minmax(0, 1fr); }
.layout > * { min-inline-size: 0; }
.prose { max-inline-size: 65ch; overflow-wrap: anywhere; }
```

This is not a universal template: collapse columns when content needs it.
Fix the actual long token, media or minimum-size cause; never conceal broken
layout with global overflow clipping. Avoid fixed heights for text cards.
Use responsive media, reserve space with aspect-ratio or intrinsic dimensions,
and crop with object-fit only when the brief permits cropping.

Container size queries serve reusable components placed in differently sized
regions. Establish a suitable ancestor wrapper, usually
`container-type: inline-size`, and query its descendants; a container does not
query itself. Keep a functional base stack. Containment changes sizing, so do
not place size containment everywhere. Container units, style queries and
scroll-state queries require separate evidence, not assumed coverage from size
queries.

Prefer inline/block logical sizing and spacing to physical left/right unless
the design truly requires physical placement. Inspect RTL text, directional
icons and mixed-direction data where relevant. Logical properties alone do not
certify localization.

Choose a few distinct type roles, comfortable leading and a prose measure
around 60–75ch adjusted for the font/content. Use relative endpoints for fluid
type, for example `font-size: clamp(1.75rem, 1.2rem + 2vw, 3rem)`, not viewport-only
type; test enlargement rather than assuming clamp guarantees accessibility.
Fluid spacing can share a small rhythm without making every value fluid.
Balance short headings optionally; normal wrapping remains valid. Avoid manual
line breaks that only fit one viewport.

For mobile-height regions, choose svh for a stable small viewport, lvh for the
large viewport, dvh for dynamic chrome changes. Prefer min-block-size with a
base fallback and allow content growth; never lock a text page into 100vh.
Use containment/content-visibility only for measured rendering needs, reserving
space and checking navigation, focus and accessibility effects.

## CSS/native HTML instead of presentation scripts

Replace resize listeners that count columns or equalize card heights with
Grid/Flex, wrapping and intrinsic sizing. Replace component-width JS classes
with container queries plus a base layout. Replace margin calculations with
gap; replace direction-specific positioning arithmetic with logical properties.
Replace redundant “contains checked input” presentation classes with :has()
when supported; maintain native input state and a clear fallback style.
Use details/summary for ordinary disclosures, not custom click handlers and
ARIA state mirroring when native behavior meets the brief.

Native scroll snapping can replace carousel presentation/scroll positioning,
not business state or every accessible control. Choose it only for intentional
stops; prefer proximity when strict stops impede reading. Use scroll-padding
and scroll-margin for intended visible positions. Test keyboard/touch,
oversized items, focus and ordinary scroll access. Never default to full-page
mandatory snapping. Snapping is not smooth scrolling; do not enable smooth
motion unconditionally under reduced motion.

These substitutions reduce synchronization bugs; they do not erase behavior.
Keep small purpose-bound JS for application state, persistence, data validation
or synchronization beyond native constraints, playback controls/state,
asynchronous feedback, focus restoration and complex dialog interactions.
Use native buttons, links and controls; do not fake an application with
inaccessible checkbox hacks. CSS cannot create accessible names or manage focus.
A same-document View Transition still wraps a real JS state update.

## Enhancement and review

Keep baseline content visible with no JS and unsupported enhancements. Gate
actual enhanced syntax, not a vaguely related feature. `@supports` and
`CSS.supports` detect parsing/support, not correct behavior, contrast,
accessibility or freedom from bugs. Some at-rules lack a useful direct test:
use dated compatibility evidence and a tested baseline, not invented detection.

Motion is optional and subordinate to reading and tasks. Default to static;
enable decorative motion only under no-preference and suitable support.
Reduced motion must stop nonessential continuous scheduling, retain meaningful
static states and preserve required pause controls. Scroll timelines may replace
decorative scroll listeners, not application logic. Do not leave content at
opacity zero when the effect cannot run.

Follow the existing review boundary: prepare representative journeys and local
checks; do not launch controller-owned browser review. Where observations are
authorized/available, inspect actual geometry and behavior at 320px,
intermediate and wide sizes, long/empty/extreme content, 200% zoom or text
enlargement, keyboard use, no-JS, reduced motion and unsupported enhancements.
Include RTL, forced colors, print and coarse pointers as relevant. A Chromium
observation proves neither Safari nor Firefox behavior. Record untested cases.

When useful/requested, give a short engineering note outside the product UI:
chosen techniques and reasons, retained JS and reasons, target versions,
support evidence/date/uncertainty, fallback/static behavior and checks actually
performed. Do not add required artifacts or developer messaging to the product
flow. Static acceptance does not certify design or interaction quality.
