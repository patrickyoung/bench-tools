# Semantics and evidence ledger

Scope: concise teaching derived from the supplied curated primary-semantics
packet, reviewed **2026-09-17**. Links identify sources,
not instructions to fetch them. Living/draft modules may change. Reassess against
dated target-browser evidence when authorized; do not hardcode “latest forever.”

The [CSS Snapshot 2026, 22 June 2026 Group Note](https://www.w3.org/TR/2026/NOTE-css-2026-20260622/)
catalogs specification stability, not implementation adoption; CSS 2026 is not
one new version. The [2 August 2026 priority article](https://cssawwwards.com/blog/20-css-features-every-frontend-developer-should-know-2026)
supplies topics, not normative semantics. [Baseline](https://web.dev/baseline)
is a compatibility signal for a defined browser set, not audience minimums,
assistive technology certification or performance proof. “Newly available”
and “widely available” labels are distinct from standardization.

User-supplied [additional topic checklist](https://dev.to/digitalunicon/essential-modern-css-features-for-2026-5835),
reviewed **2026-09-17**, is secondary evidence, not syntax, compatibility or
accessibility authority. Apply the primary corrections below for container
shorthand, subgrid spanning, dynamic contrast and selector specificity.
Recheck dated “coming soon” or broad support claims against current evidence.

## Cascade and native state: correct the shortcuts

[Cascade 5](https://www.w3.org/TR/css-cascade-5/#layer-order):
within the same origin/context/importance, later layers win for normal rules,
but **unlayered normal rules beat all layered normal rules**. Important layer
order reverses, and layered important rules beat unlayered important rules.
Layer order precedes specificity; other cascade axes still apply. A small
declared order such as reset, base, components, utilities is enough. Don't add
!important to repair an unexplained ordering problem.

[Cascade 6](https://www.w3.org/TR/css-cascade-6/#scoped-styles):
@scope bounds selector matching and introduces scoping proximity into cascade
resolution. It does not block outside selectors or inherited values.
It is not Shadow DOM or bidirectional isolation. Explicit component roots
remain a sensible baseline.

[Native nesting](https://www.w3.org/TR/css-nesting-1/) is browser parsing, not
Sass preprocessing. `&` denotes a nesting selector, not string concatenation:
do not port `&__title` from Sass expecting a generated class name. Keep nesting
shallow; selector lists can raise nested specificity unexpectedly. Supply
equivalent flat rules when targets need them.

[Selectors 4](https://www.w3.org/TR/selectors-4/#relational):
:has() tests relative selector relationships, not application truth.
:focus-visible uses browser heuristics, **not strictly keyboard-only input**.
Keep native focus or an equally visible indicator; never globally remove
outlines and assume a hover style substitutes. A conservative base `:focus`
outline may remain while supported :focus-visible rules refine presentation.
[Specificity rules](https://www.w3.org/TR/selectors-4/#specificity-rules):
:where() and its arguments contribute zero; :is() and :has() use the most
specific selector argument, not necessarily only the matched branch. Prefer
scoped :where() defaults for easy overrides; avoid accidental IDs in grouped
selectors. These are not interchangeable specificity tools.

## Color, tokens and text

[Variables](https://www.w3.org/TR/css-variables-1/) inherit and participate in
the cascade. Choose a few semantic surface/text/accent and spacing tokens.
[Color 4](https://www.w3.org/TR/css-color-4/) offers OKLCH perceptual coordinates,
not guaranteed gamut or contrast. Check rendered text and controls in all
states, including focus/disabled treatment and forced colors; provide sRGB
fallbacks where needed. Aim for applicable WCAG contrast requirements, not
visual intuition alone.

[Color 5](https://www.w3.org/TR/css-color-5/) color-mix/relative syntax needs
its own support review. Unsupported syntax inside a custom property can
invalidate its consumer at computed-value time; an earlier declaration does
not necessarily rescue it. Gate the advanced token itself:

```css
:root { --accent: #2455a4; }
@supports (color: oklch(45% 0.15 260)) {
  :root { --accent: oklch(45% 0.15 260); }
}
a { color: var(--accent); }
```

The example demonstrates syntax fallback, not certified contrast for an unknown
background. Test each derived color/state.
[contrast-color()](https://www.w3.org/TR/css-color-5/#contrast-color) in current
Level 5 takes a color and produces black or white for a solid input background;
the precise contrast algorithm is UA-defined at this level. Do not teach old
`color-contrast(bg vs black,white)` as widely supported production syntax.
Gate actual syntax/targets and retain deliberately measured foreground/background
tokens. A function name cannot certify all-state readability: translucency,
imagery, typography and controls still need checks. [Values 4](https://www.w3.org/TR/css-values-4/)
covers fluid constraints and viewport units; container units are specified in
[Contain 3](https://www.w3.org/TR/css-contain-3/#container-lengths). Relative units still
require zoom testing. [Text 4](https://www.w3.org/TR/css-text-4/) heading balance
does not promise an exact line count or replace typography.

## Layout and motion source map

[Grid 2](https://www.w3.org/TR/css-grid-2/) and
[Flexbox 1](https://www.w3.org/TR/css-flexbox-1/) ground track alignment,
subgrid, wrapping and automatic minimums. To inherit multiple subgrid columns,
span appropriate parent tracks (e.g. `grid-column: 1 / -1`), not a single-track
area. Keep a viable independent nested-grid fallback.
[Contain 3 shorthand](https://www.w3.org/TR/css-contain-3/#container-shorthand):
`container: inline-size` sets a container **name** with default type, not size
containment. Use `container-type: inline-size` or `container: card / inline-size`
on the ancestor wrapper queried by descendants; size queries do not query the
element's own size. Containment affects sizing. [Sizing 4](https://www.w3.org/TR/css-sizing-4/)
defines aspect-ratio as preferred, not an absolute dimension lock.
[Logical 1](https://www.w3.org/TR/css-logical-1/) follows writing mode/direction.

[Scroll Snap 1](https://www.w3.org/TR/css-scroll-snap-1/) grounds intentional
native snap positions, not carousel state or unconditional smooth motion.
[Scroll animations](https://drafts.csswg.org/scroll-animations-1/) use scroll/view
progress rather than ordinary time-based animation triggered at a position.
Declare animation-timeline after animation shorthand, which can reset it.
Keep static content visible outside enhancement rules.
[View Transitions 1](https://www.w3.org/TR/css-view-transitions-1/) same-document
transitions use a JS API around state updates: detect that actual API, skip
nonessential transitions for reduced motion, and perform the update directly
when unavailable. Different transition features need separate support evidence;
neither transition family makes application state CSS-only.
[Media Queries 5](https://www.w3.org/TR/mediaqueries-5/#prefers-reduced-motion)
grounds reduced-motion handling.
[HTML interactive elements](https://html.spec.whatwg.org/multipage/interactive-elements.html)
ground native disclosure/control choices, not a blanket zero-JS rule.
