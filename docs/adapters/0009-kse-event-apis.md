# ADR 0009: KSE event APIs

- Status: Paramount venue-events and Ball calendar implemented
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

Ball Arena and Paramount expose public KSE JSON endpoints with materially different response shapes.

## Ball Arena implementation and admission review — September 11, 2026

Use the `kse-calendar` variant with the existing snapshot replay/publication flow,
exposed as `replay-kse-calendar`. It does not change Paramount's decoder.
`tests/kse/ball-capture.mjs` captures the calendar JSON and official listing twice.
Reject a changed array or changed semantic listing cards, duplicate/missing IDs,
invalid responses, oversized input, or incomplete enumeration. HTML uses the
existing `golang.org/x/net/html` parser dependency, not regular expressions over
rendered card markup.

The [official calendar script](https://www.ballarena.com/content/js/calendar.js)
uses `https://alttix.ksehq.com/api/tm/Calendar?Id=1`. Supply start/end parameters,
but filter locally: three tested ranges returned identical inventories. Enrich by
Ticketmaster event ID from the [official All Events listing](https://www.ballarena.com/misc/all-events/).
Require every dated card to match a feed record and every feed record to have a
card. A listing-only TBA/TBD record becomes a reported invalid observation. An
unrecognized empty HTML page fails closed; an empty list has no verified template.

Use the card's More Info URL as the event link when present, otherwise the feed's
Ticketmaster URL. Preserve the latter as the ticket link. Validate same-venue
detail URLs, matching titles and dates, and known status labels. The timestamp is
show/start time, not doors; localize to America/Denver. A date-only multi-day pass
uses its first date with no invented time; validate the feed's exclusive end
against the card's displayed inclusive range. Separate ticket IDs stay separate.
Do not add a pass classification, prices, genre filters, or a schema change.

### Required Ball Arena admission review

The [official FAQ](https://www.ballarena.com/arena-information/arena-policies-faq/)
requires tickets from age three for Nuggets, Avalanche, and Mammoth games and
permits younger children on a ticketed guest's lap. For concerts and special
events it gives general ticket-age guidance but explicitly says policies vary.
The reviewed Tame Impala, Weezer, and Billy Strings pages did not supply a
supported event age restriction. Do not turn this absence into All Ages clearance.

Only positively identified home games receive the reviewed All ages policy:
match both the listing's NBA/NHL/Lacrosse classification and the corresponding
home-team title prefix. With adult ranges are 0–2 (ticketed guest's lap) and 3–17
(ticket required), reviewed September 11. This is game-specific admission, not a
guardian waiver. Concert/special-event default text explains that policies vary;
category and With adult remain absent. Configured event overrides retain precedence.
No automatic event-detail admission extraction is included in this variant.

| Evidence source                  | Raw observation                                                                    | Supported finding                                                      | Material limit                      |
| -------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------------- | ----------------------------------- |
| Three calendar range requests    | Same 119 records and normalized inventory digest                                   | Range parameters do not bound the returned inventory                   | Current announcements only          |
| API and HTML identity comparison | 119 dated records plus one HTML-only NBA Cup record with date TBA                  | All dated cards reconcile; TBA is rejected                             | No publishable date for that record |
| Eight normalized samples         | Correct titles, links, and summer/winter show offsets                              | Feed plus HTML supplies the application contract                       | No observed doors time              |
| Staged policies                  | 85 recognized home games with clearance; 34 unknown concert/special-event policies | Positive-only admission filtering avoids unsupported concert clearance | Unknown does not mean prohibited    |

The endpoint is public and used by Ball's own page; no authentication or bypass
is involved. The venue robots route returned 404 during research. The API-host
robots policy was not verified; neither observation establishes a reuse license.
This remains an explicit one-time local capture, not scheduled access.

Tests cover cross-interface completeness, TBA reporting, multi-day date-only
records, status mapping, URL/date mismatches, game admission boundaries, override
precedence, publication, failed-job retention, and successful removal. Browser
checks use `BALL_BASE_URL`; operator instructions are in README. No other venue
inherits Ball's policy or parsing contract.

## Decision

Use a KSE adapter family with two named variants: calendar and venue-events. Share transport and normalization helpers but validate each schema separately.

## Retrieval and mapping contract

The calendar variant uses https://alttix.ksehq.com/api/tm/Calendar?Id=1&start=2026-09-01&end=2026-11-01 and returns title, url, start, and end fields. Parameters were needed for a working request, but returned records extended outside the requested bounds. Filter locally and report this behavior.

The venue-events variant uses https://alttix.ksehq.com/api/tm/venue/KovZpZAFa1nA and returns richer Ticketmaster-shaped records. Preserve event IDs, dates, timezone, status, doorsTimes, images, and URLs where supplied. In sampled records, dates.start.localTime carried an inconsistent date component; dates.start.dateTime and the corresponding calendar timestamp aligned. Use the validated UTC value with the declared timezone, not the spurious date component.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source                                                                                               | Raw observation                                                                  | Supported finding                                            | Material limit                                              |
| ------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- | ------------------------------------------------------------ | ----------------------------------------------------------- |
| [Ball Arena calendar endpoint](https://alttix.ksehq.com/api/tm/Calendar?Id=1&start=2026-09-01&end=2026-11-01) | A September–October request returned an Avalanche record dated January 20, 2027. | Returned data cannot be assumed to respect requested bounds. | Maximum horizon and enumeration behavior remain unverified. |
| [Paramount venue endpoint](https://alttix.ksehq.com/api/tm/venue/KovZpZAFa1nA)                                | Returned Elías Medina on April 10, 2027 and rich event fields.                   | The variant supports future-month event retrieval.           | Rich shape is not guaranteed for the calendar variant.      |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test both schemas, required query parameters, out-of-range responses, local filtering, UTC/local consistency, timezone transitions, missing IDs in calendar rows, and non-music categories. Verify caps and update/removal semantics separately for each variant.

The original exploration did not implement an importer. The September 11 update
records both implementations; the original exploratory limits remain historical.

## Paramount implementation and admission review — September 11, 2026

The `kse-venue-events` adapter accepts only the configured Paramount venue and
venue ID `KovZpZAFa1nA`. `tests/kse/capture.mjs` makes two unauthenticated requests
to the venue endpoint and requires matching arrays with unique IDs. The official
[calendar](https://www.paramountdenver.com/event-calendar/) and its
[widget](https://www.paramountdenver.com/scripts/TMEventWidget.js?2) consume this
whole array. The widget's load-more method repeats the same URL; it supplies no
pagination or range parameters. Coverage means this currently enumerated array,
filtered to the next 12 months, not a promise that unpublished future shows exist.
Reject capped (1,000 or more rows), oversized, malformed, or changing captures.

Reuse the existing snapshot replay, reconciliation, schema validation, and atomic
publisher through `replay-kse`. Keep UTC start and doors timestamps, checked against
the venue date and calendar timestamp. Ignore the incorrect date portion of
`localTime`. Reject inconsistent doors dates instead of moving them to a new date.
Scheduled/rescheduled/postponed rows remain Scheduled; explicit cancellations
become Cancelled. The calendar links to Ticketmaster, so event and ticket URLs
are the same official destination. No prices, images, or descriptions are published.

### Required venue admission review

The [official FAQ](https://www.paramountdenver.com/theatre-policies-faq/) delegates
age restrictions to each event's Ticketmaster page. It gives no blanket guardian
exception. Its ticket-purchase guidance is not an All Ages admission guarantee.
Default policy: `Age restrictions vary by event; check the event listing`.
Missing restrictions and `legalAgeEnforced: false` give no With adult clearance.

| Evidence source                                    | Raw observation                               | Supported mapping                                             | Material limit                                                                       |
| -------------------------------------------------- | --------------------------------------------- | ------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| Venue API, Brian Cox                               | Explicit 12+ statement                        | Raw 12+; With adult ages 12–17                                | No invented 13+ category; another performance without a restriction remains unknown. |
| Venue API, Ilana Glazer and Daniel Sloss           | Explicit 15+ and 16+ statements               | Preserve minimum age; eligible ages through 17                | An adult does not waive the minimum.                                                 |
| Venue API, Dan and Phil                            | Under-14 attendance requires someone aged 18+ | Ages 0–13 require that companion; ages 14–17 require a ticket | Event-specific, not a venue-wide exception.                                          |
| Venue API, Laura Ramoso and IMOMSOHARD             | Recommended ages, not minimum ages            | Preserve recommendation text only                             | No category or With adult clearance inferred.                                        |
| Venue API, George Lopez and A Drag Queen Christmas | Explicit 18+ statements                       | 18+ category; no child clearance                              | Do not infer a guardian exception.                                                   |

Event policies link to the actual Ticketmaster event. The review date is fixed at
September 11, not advanced by a capture. Configured admission overrides retain the
shared reconciler's precedence. Conflicting minimum-age or accompaniment statements
reject the record. Recognition covers reviewed statement forms, not arbitrary prose.

### Access and verification limits

Use only the public endpoint used by the venue. No login, token, browser bypass,
or ticket-price retrieval is involved. Venue robots returned 404; the API-host
robots request could not be verified. Neither result establishes reuse permission.
This is an operator-triggered local import, not authorization for scheduled access.
The capture contained 74 rows; eight raw/normalized samples were inspected. Two
rescheduled events retained doors dates from May and were rejected, leaving 72
valid records through April 10, 2027. Last-valid retention and failed-job protection
remain the shared publication rules. Ball's separate schema is described above.

## Paramount registry refresh — September 10, 2026

The resubmitted [Paramount calendar](https://www.paramountdenver.com/event-calendar/)
is the existing source, not a new integration. Retain the venue-events assignment
and endpoint above as September 8 evidence; this documentation update did not
repeat the endpoint request. Revalidate the response and coverage at implementation.

Admission readiness is separate from retrieval. The
[registry](../adr/0013-source-adapter-registry.md#september-10-2026-candidate-expansion)
records the outstanding official policy and event-restriction review. Do not infer
With adult permission from a Ticketmaster-shaped record, missing restrictions, or
Ball Arena's policy. No Paramount configuration or data publication is added here.

## Alternatives and consequences

A single assumed KSE schema would lose or misread fields. Two unrelated importers would duplicate host-level behavior. Direct Ticketmaster access remains a separate unverified alternative, not a dependency of these public routes.
