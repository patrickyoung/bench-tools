---
name: common-workflows
description: Use for creation or prompt edits involving budgets, reporting, joins, projects, inventory, capacity, marketing/research or invoices; apply domain invariants without imposing fixed templates.
---

# Common spreadsheet workflows

Read the active contract: creation uses `CONTRACT.md` and workbook-design;
`mode=edit` uses `EDITING.md` and workbook-editing. Domain advice grants no extra
operations. On edits inspect the bound workbook first, preserve its conventions,
and change only requested scope. Combine relevant families instead of forcing
every request into a dashboard template.

## Evidence and defaults
The supplied research is dated 15 September 2026. AFP's 2025 practitioner survey
supports planning/reporting priority within finance; Vena's 2026 findings are
finance-specific vendor evidence. Microsoft's templates show supported demand,
not prevalence, and its Copilot documentation/article show workflow expectations,
not this worker's capabilities. This is directional evidence, NOT a universal
2026 ranking or certification of specialized models. See PROVENANCE.md.

Infer useful routine choices from the actual data and record the interpretation.
Escalate material ambiguity with the exact decision needed, not a generic
questionnaire. Do not invent accounting/tax policy, dates, owners, currency
conversions or business definitions. Separate known facts, supported indications,
unknowns and explicit assumptions.

## Budgets, forecasts and cash plans
Establish period start/end, fiscal versus calendar basis, currency, units, entity
and scenario. Distinguish actuals from forecasts and assumptions from observed
values. Check month additions do not duplicate/skip periods and references
include the intended new period. Respect supplied sign and variance conventions;
“favorable” depends on the measure. Verify driver propagation independently.
For cash plans reconcile opening balance plus signed inflows/outflows to closing
balance; do not substitute accrual revenue for cash without evidence. Aggregate
matching units and periods only. Test zero/missing bases and rate changes; do
not mask missing assumptions as zero. Do not invent depreciation or tax rules.

## Recurring sales and management reports
Identify transaction versus period-summary grain, stable IDs, reporting cutoff,
time basis, currency and included population. Map renamed columns using normalized
names, semantics, types, units and examples, with confidence/reasons logged.
Reconcile source rows to retained/appended/duplicate/unresolved records and
independently sum measures. Keep like-for-like periods and explain partial periods.
Weighted rates require matching numerator and denominator, not averages of
percentages by default. Chart sources must include new data only within verified
capacity. Native table filters do not automatically change all totals; state the
actual filter scope. Pivots are only the bounded native EDITING.md adapter:
source edits require explicit refresh and cache/source verification, not fake
formula pivots or claims that recalculation refreshes a pivot.

## Joins, cleanup and reconciliation
Determine key grain and expected one-to-one, many-to-one or other cardinality.
Preserve original fields and source locators. Check key uniqueness, unmatched
records and before/after row counts; avoid multiplying parent amounts during
one-to-many expansion. Normalize safely, retaining raw values; IDs remain text
with leading zeros and required precision. Resolve dates/locales and units from
evidence, not guesswork. Equal-plausibility mappings remain unresolved; never
use fuzzy strings alone or drop unmapped data silently.
Use independently known keys for append deduplication, not row position.
Separate exact copies, conflicting records and repeated observations. Reconcile
both sides of a join, counts and totals; no silent inner-join loss. Unknown,
invalid, zero and not-applicable are different states.

## Projects, actions and dependencies
Identify task IDs, owners, statuses, deadlines, prerequisites, effort units and
supplied reporting date. Distinguish source deadlines from target overrides and
keep conflicts traceable. Adding an owner or changing a due date must not sever
ID-bound notes. A completed subtask does not satisfy unrelated prerequisites.
Check remaining effort, dependencies and warning text when state changes.
Do not make missing effort equal zero or invent owner/date facts. Preserve
historical notes separately from live overdue/missing/conflict warnings.
A requested total hours column versus total row must be resolved from context
or explicitly left pending when ambiguous.

## Inventory and assets
Establish item/location/lot grain, movement IDs, time cutoff, stock unit,
pack conversions and valuation basis. Opening stock plus receipts minus issues
and authorized adjustments must reconcile to closing stock at the same grain.
Do not sum pieces and boxes without a supplied conversion. Separate on-hand,
reserved, available and reorder thresholds; do not invent their business policy.
Append receipts using movement keys, not just item IDs that legitimately repeat.
Check sign, quantity and price units in stock-value formulas, missing costs,
threshold equality and whether negative stock is allowed by supplied rules.
Do not invent costing methods, depreciation or accounting treatment.

## Schedules, time and capacity
Establish date boundaries, time zones if relevant, work calendars, availability
units, resource IDs and shift/task grain. Check overlaps, cross-midnight shifts,
period boundaries and duplicate allocations according to supplied rules.
Demand and capacity must use compatible hours/people/periods. Utilization needs
a defined denominator; zero capacity is not a healthy zero-percent utilization,
and unknown availability is not zero. Avoid double-counting shared resources.
Adding people/shifts must extend the right ranges and maintain associations.
Do not invent labor rules, holidays, overtime policies or daylight-saving facts.

## Marketing, CRM and research logs
Separate contacts, organizations, events, campaigns and observations; preserve
IDs and observation dates. Joins must not inflate conversion counts or duplicate
measurements. Define stages, eligible population, numerator, denominator and
attribution window for each rate. Distinguish repeat events from unique people
and nonresponse/missing from zero. Respect measurement units, sample/grain
differences and supplied consent/sensitive-data boundaries. Charts need comparable
periods; do not infer causality or universal representativeness from a trend.
Do not invent attribution policy or impute unsupported research measurements.

## Invoices, expenses and administrative lists
Preserve invoice/line/vendor/currency/date identifiers and text codes.
Establish quantity, unit price, discount basis, tax basis/rate, inclusivity,
signs, line versus document rounding and supplied currency precision before
formulas. Use only supplied policy; never invent jurisdictional tax/accounting
rules. Independently reconcile line extensions, authorized discounts/taxes,
credits and document totals. Clarify whether “total this column” means selected
rows, all records or visible rows when context is insufficient; never silently
include subtotals twice. Check missing/zero quantities, negative adjustments and
rounding differences without changing raw evidence.

## Shared quality bar
Keep current operational warnings tied to the same facts and selected population
as results. Test both warning text and counts when resolving an issue; an unrelated
unresolved prerequisite must remain visible. Static historical notes say
“At import” or “Original source”; neutral definitions are not alarm banners.
Use explicit units, sensible precision and dates, retain numeric blank-versus-zero
checks and do not use scalar equality-to-empty-string for numeric missingness.

Choose the smallest useful plan and independent expectations. Changes need
baseline assertions, before/after preservation checks and proportionate live
tests. New periods, append/join ranges, controls, rates and refresh behavior need
relevant boundary tests; a narrow cell fix does not need unrelated dashboards.
Trace intentional changed ranges and every mapping with confidence/reason.
Preserve unmapped inputs, report unresolved decisions precisely, and state actual
capacity, filter scope, refresh and engine limitations. Supported saved native
features, readable images and executed tests are separate evidence; never claim
new evaluations, native-app behavior or visual inspection without observing them.
