# ADR 0006: HoldMyTicket iCalendar feeds

Operational profiles now follow [ADR 0023](../adr/0023-locale-isolation.md).
`adapter_options.feed_id`, venue name and timezone select the calendar.
Federal additionally sets `adapter_options.layout: federal` to retain its
reviewed malformed-description and admission handling. Other calendars do not
inherit that specialist behavior. Keep fixture configs separate from operational configs.

- Status: Snapshot adapter and explicit operator capture implemented; scheduled retrieval not enabled
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

September 13 update: the common capture wrapper uses each locale's configured
feed URL, discovers public HoldMyTicket event URLs, and extracts Event JSON-LD
from detached HTML documents. It preserves the original feed, including Federal
descriptions. URL discovery is not a second calendar parser: the existing Go
adapter still validates the full calendar and detail agreement before publication.
Missing details remain rejected observations, not silently removed records.
The local refresh coordinator can run this path; no schedule or remote deployment
is enabled. See ADR 0023 and README for commands and verification limits.

Oriental and HQ publish HoldMyTicket calendar subscriptions using the same format.

## Decision

Use a shared standards-aware iCalendar parser with a HoldMyTicket source profile. Keep provider timezone conventions separate from general iCalendar parsing.

## Retrieval and mapping contract

Fetch https://holdmyticket.com/ics/{venue_id}: Oriental is 801 and HQ is 6457. Parse VEVENT entries, unfolded lines, escaped text, UID where supplied, DTSTART, DTEND, URL, and status fields where present. Respect value types and explicit timezone information. The inspected feeds declare X-WR-TIMEZONE:America/Denver and use floating event times; apply that provider convention explicitly rather than treating floating times as UTC.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source                                  | Raw observation                                                      | Supported finding                          | Material limit                                          |
| ------------------------------------------------ | -------------------------------------------------------------------- | ------------------------------------------ | ------------------------------------------------------- |
| [Oriental ICS](https://holdmyticket.com/ics/801) | Returned VEVENT records including Steve Hofstetter on July 17, 2027. | A subscription feed reaches future months. | Sample descriptions were empty.                         |
| [HQ ICS](https://holdmyticket.com/ics/6457)      | Returned HQ entries including Nervosa on March 8, 2027.              | Same parser can serve HQ by configuration. | Feed completeness and removal semantics are unverified. |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test line folding, escaping, floating and explicit-zone times, DST, date-only entries, missing end times, recurrence if encountered, UID stability, and canceled entries. Verify provider timezone fallback against public event details. Test detail enrichment independently.

## Implementation update — September 10, 2026

`internal/hmt` and `ingest replay-hmt` implement local snapshot normalization.
They reuse the existing golang-ical library and the AEG replay publication workflow.
No schema or reader change was required. The input bundle contains complete feed
text and one saved Event JSON-LD object per provider event ID. Capture remains
outside the importer. Synthetic fixtures exercise both configured venues.

Use the numeric event ID in the feed URL for stable provider identity, not a hash
of mutable data. Require unique nonempty UIDs as an additional consistency check.
Reject duplicate identities, incomplete envelopes, unknown recurrence constructs,
wrong calendar names/timezones, and details belonging to another feed.

The detail page confirms `startDate` as show time and supplies `doorTime` separately.
Normalize to America/Denver, reject ambiguous/nonexistent local times, and require
calendar/detail agreement. Date-only entries have no invented time. Ignore DTEND
for application duration; no verified duration contract is added.

Use the exact detail venue as a guard against off-site misattribution. Preserve
published age prose and map only explicit All Ages and 13/16/18/21+ label forms.
Do not infer accompaniment exceptions. Ignore offer prices and associated price labels;
price metadata is retired across all venues. The feed's HoldMyTicket detail URL is the event link; a validated
offer URL supplies the purchase link. No venue detail route is invented.

Failed individual observations use the shared rejection/last-valid behavior.
`coverage.complete` describes full parsing of the supplied feed, not proven venue
publication completeness. Scheduled refresh and absence-based removal require a
separate coverage/access review. Both sources were added through an approved
one-time local capture. Public redistribution and recurring access are not claimed
to be authorized merely because the feed and pages can be read.

The live capture used public calendar and event paths. Observed robots guidance
disallowed box-office, venue administration, staff, and admin paths, none of which
were used. No login, private credential, checkout, or bypass was involved.
See ADR 0019 for validation results and retained snapshot paths.

## Federal candidate — September 10, 2026

The [Federal calendar](https://thefederaltheatre.com/calendar_list) links
[subscription 8693](https://holdmyticket.com/ics_user/8693). This is an observed
`/ics_user/` route, unlike HQ and Oriental's `/ics/` routes. Preserve the exact
configured URL; do not infer that the numeric identifiers are interchangeable.

The retrieved feed declares `X-WR-CALNAME:The Federal Theatre` and
`X-WR-TIMEZONE:America/Denver`. Entries include Extortionist on September 10,
Federal Nights on September 12, Mad Caddies on September 13, and The Black Queen
on September 17, 2026, with UIDs, floating DTSTART values, and HoldMyTicket event
URLs. This establishes a candidate for parser reuse, not tested compatibility or
a complete horizon. Some descriptions advertise a Chapel Perilous VIP upgrade;
do not publish an upgrade as a separate performance without ticket/event evidence.

Before onboarding, validate the full feed's venue scope, detail JSON-LD agreement,
calendar naming, numeric event identities, time semantics, and ticket links with
Federal-specific fixtures and the existing replay/consumer checks. Do not infer
the meaning or scope of `ics_user` from its name alone. The
[registry's admission review](../adr/0013-source-adapter-registry.md#september-10-2026-candidate-expansion)
remains incomplete; HoldMyTicket membership does not imply a shared age policy.
No Federal source configuration or published events are added by this ADR update.

## Federal implementation — September 10, 2026

The `8693` profile now supports Federal snapshots through `replay-hmt`. The
candidate limits above record the earlier research, not the implementation state.
The original live feed failed the standard parser at a multiline DESCRIPTION.
Removing descriptions in a diagnostic copy allowed all 35 VEVENT records to parse.
The approved solution keeps the feed and does not switch to HTML discovery.

Before standard parsing, only Federal discards DESCRIPTION blocks in memory.
Require an unindented `CREATED:YYYYMMDDTHHMMSSZ` boundary with a valid timestamp.
Reject other property-looking lines inside a block, missing boundaries, blocks
outside events, and duplicate descriptions. Preserve raw snapshots. Continue
validating envelope, event count, identity, recurrence, venue, timezone, and detail
agreement. A future layout change must fail visibly, not silently remove fields.
HQ/Oriental parsing and application `.ics` exports are unchanged.

Admission review used the official venue listing/about pages and the captured
HoldMyTicket details. Explicit All Ages labels support ages 0–17 with condition
`All ages event; attend with adult and follow event conditions`, linked to each
event page and reviewed September 10. The captured text review found no additional
accompaniment requirement; it is not a guarantee against future event conditions.
No exception is inferred for 13+/16+/18+/21+ or ambiguous labels. Configured event
policy overrides win; mismatched physical venues are rejected before enrichment.

Tests cover age 14 and boundaries, restricted/unknown labels, overrides, off-site
rejection, malformed/empty/folded descriptions, unsafe boundaries, and unchanged
HQ behavior. See [ADR 0019](../adr/0019-implementation-and-verification-plan.md)
for capture, staging, publication, and consumer verification results.

## Alternatives and consequences

HTML scraping duplicates the feed for basic scheduling fields. A handwritten line splitter is not a sufficient iCalendar parser. The feed may require enrichment for prices, lineup, images, or age limits; missing descriptions are not empty event facts.
