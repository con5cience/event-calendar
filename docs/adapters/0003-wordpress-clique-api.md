# ADR 0003: WordPress Clique event API

- Status: Implemented for Red Rocks; operator-triggered capture and replay
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

Red Rocks publishes custom WordPress routes. These are not interchangeable with WordPress core posts or every WordPress event plugin.

## Decision

Use a Clique-specific adapter with explicit upcoming-list and calendar-range operations. Keep their response mappings separate where schemas differ.

## Retrieval and mapping contract

Verified routes are /wp-json/clique/v1/get_upcoming_events and /wp-json/clique/v1/get_calendar_events_range?start=2026-09-08&end=2026-10-01 on www.redrocksonline.com. Map observed WordPress and acf fields; preserve IDs, links, venue, status, and source time data. Do not assume that arbitrary query parameters are supported.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source                                                                                                          | Raw observation                                                                      | Supported finding                            | Material limit                                          |
| ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------ | -------------------------------------------- | ------------------------------------------------------- |
| [Upcoming JSON](https://www.redrocksonline.com/wp-json/clique/v1/get_upcoming_events)                                    | Returned WordPress-shaped records and acf data, including a September 3, 2027 event. | Upcoming retrieval extends beyond one month. | Complete future publication was not established.        |
| [Range JSON](https://www.redrocksonline.com/wp-json/clique/v1/get_calendar_events_range?start=2026-09-08&end=2026-10-01) | Returned calendar records with offset-bearing ISO times.                             | A date-range operation is available.         | Exact boundary inclusivity and range limits need tests. |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test both response shapes, range boundaries, overlaps, DST offsets, empty periods, and canceled/rescheduled records. Verify that successive ranges neither omit nor duplicate boundary events.

The September 11 implementation below supersedes this initial verification list.

## Red Rocks implementation — September 11, 2026

`tests/clique/capture.mjs` captures upcoming JSON, a full-year calendar range,
and two populated partitions of that range. `internal/clique` maps the snapshot;
`ingest replay-clique` uses the existing reconciliation and atomic publication flow.
The configuration is `internal/clique/testdata/red-rocks.yaml`. Its JSON neighbor
is synthetic test data, not a live publication input.

The requested horizon is today through the same date next year. Range requests
start one day early, then filter to their logical half-open dates. The importer
requires agreement between upcoming identities, the full range, and partition
coverage. It rejects gaps, mismatches, malformed envelopes, and duplicate identities.
HTTP failures abort capture; they never become empty results. An explicit empty
full-year result needs no additional empty-month probes. Bounds are 4 MiB per
response and snapshot, 30 seconds per request, and fewer than 10,000 records.
These are defensive limits, not documented provider limits.

Use `acf.showing_id` as identity, canonical event links for navigation, and
Denver-local ACF dates checked against offset-bearing calendar times. Doors take
precedence in the existing consumer. Explicit `Canceled:` or `Cancelled:` title
prefixes set cancellation status and are removed from the display title. Historical
postponement prose does not change the current published date or status.
Descriptions, images, prices, and promotional subtitles are not published.

| Evidence source                  | Raw observation                                                                                                                        | Supported finding                                                   | Material limit                                                 |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- | -------------------------------------------------------------- |
| September 11 live range requests | December and January requests repeatedly returned HTTP 500; populated broader ranges returned 200.                                     | Avoid empty-month probes; use full-year and populated partitions.   | Server-side cause is unknown. A failed request still aborts.   |
| September 11 boundary probe      | A September 11 lower bound omitted the 08:00 stair climb; September 10 included it. Explicit midnight did not fix the omission.        | One-day overlap plus logical-date filtering is required.            | No general provider boundary guarantee exists.                 |
| Final captured snapshot          | Upcoming and range identities agree on 71 records, including the stair climb, REZZ cancellation, and September 2027 Rod Stewart dates. | Capture covers the agreed current listing in the requested horizon. | Does not prove that upstream has published every future event. |

### Required venue admission review

Reviewed September 11 against the official [FAQ](https://www.redrocksonline.com/plan-your-visit/faq/)
and [Hasan Minhaj & Ronny Chieng event](https://www.redrocksonline.com/events/hasan-hates-ronny-ronny-hates-hasan-1479450/).
The FAQ supplies the All Ages default and under-two adult-lap ticket condition.
The event explicitly supplies a 13+ restriction. Event restrictions override the
venue default; configured overrides remain authoritative.

All Ages rules cover ages 0–1 with the lap condition and 2–17 with ordinary ticket
conditions. Explicit 13+ or 16+ floors admit minors at or above that floor, with
the event page as evidence. They do not create a below-floor adult exception.
18+, 21+, unknown, or conflicting restrictions do not inherit All Ages permission.
Unknown age wording remains visible with its event link but has no positive age
classification. No older-friend or guardian exception is inferred.

### Access and alternatives

The API is public and requires no credentials. The inspected robots file permits
these paths for the general user agent. The [terms](https://www.redrocksonline.com/termsandconditions/)
restrict reuse of site materials; technical access is not evidence of legal
permission. The approved integration publishes factual calendar fields and links,
not descriptions or images. Raw responses remain local operator capture artifacts.

HTML scraping adds layout dependence without improving the observed API contract.
A universal WordPress adapter would conceal Clique-specific identity and boundary
rules. Scheduled capture, ticket-page extraction, and remote deployment are outside
this implementation.

### Verification

`npm run test:clique` tests bounded capture, overlap requests, explicit empty data,
and failure handling. `npm run test:contracts` includes Go mapping, DST, coverage,
cancellation, policy overrides, reconciliation, publication, HTTP, and ICS tests.
`CLIQUE_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/clique.spec.ts`
checks live details, links, cancellation, exports, and age boundaries on desktop
and phone. Publication results are recorded in ADR 0019.

## Alternatives and consequences

HTML parsing remains a fallback, but the inspected routes avoid layout dependence. A universal WordPress adapter would hide custom plugin contracts. The custom route can change without a public compatibility guarantee.
