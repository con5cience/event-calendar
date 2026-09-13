# ADR 0021: Live Nation venue events through the public browser flow

Operational profiles now follow [ADR 0023](../adr/0023-locale-isolation.md).
`adapter_options.venue_ids` is a comma-separated list of accepted upstream venue
IDs. Venue website, name and timezone replace the compiled Denver registry.
Capture requires `SITE_DIR` and a source CLI argument configured in that locale's
`capture.json`. Existing pagination and returned-identity checks remain required.

- Status: Implemented for Marquis, Summit, and Fillmore
- Date: 2026-09-11
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md), [JSON-LD exploration](0007-event-json-ld.md)

## Decision

Use an operator-triggered Playwright capture of Marquis's normal public scrolling
calendar. Read the event responses that the page itself requests. Normalize the
saved JSON with `livenation-venue-events`, exposed by `replay-livenation`.
Reuse the existing source configuration, admission rules and overrides, snapshot
reconciliation, schema validation, and generation-guarded atomic publication.

The original JSON-LD remains useful corroborating evidence but contains only the
first 36 records. It does not include the event notes needed for admission and
separate doors/show times. Direct requests returned 403 for later pages; the
unmodified browser's own scroll flow returned those pages successfully. The cause
of that transport difference was not isolated. It does not establish that all
access is blocked or that a standalone API client is supported.

No stealth mode, copied authentication, CAPTCHA solving, header spoofing, or API
requests injected into the browser are used. Stop on an event-response failure.
Browser capture is a separate operator step; Chromium is not added to the app or
Go ingestion image. The app still consumes JSON files and makes no venue requests.

## Fillmore profile and admission review — September 11, 2026

Use the official `https://www.fillmoredenver.com/shows`, not the originally
supplied independent resale guide. The page requests
`https://content.livenationapi.com/v1/venues/KovZpZAE6eJA/events` with the same
offset/limit and terminal-empty pagination as Marquis and Summit.
Run `node tests/marquis/capture.mjs fillmore` and replay with
`internal/livenation/testdata/fillmore.yaml`. Only venue ID `KovZpZAE6eJA`
is accepted, published as Fillmore Auditorium at 1510 North Clarkson Street.
No unrelated venue or aggregator is inferred.

The [official visit policy](https://www.fillmoredenver.com/visit), reviewed
September 11, requires a ticket for every guest at All Ages events. For 16+ events,
all guests must be at least 16 and provide valid ID; a parent or guardian does
not waive that minimum. Configure All Ages clearance for ages 0–17 with the
ticket condition, and 16+ clearance only for ages 16–17 with ticket and valid ID.
This is eligibility at the minimum age, not an accompaniment exception.
No clearance is inferred for 18+, 21+, unclear, or missing restrictions.
Unlike Summit and Marquis, Fillmore has no inferred All Ages default.
Configured event overrides still take precedence.

The FAQ also confirms that the ticket time means doors. Event notes provide
separately labeled doors and show times. Existing timestamp validation applies.
Do not reinterpret midnight pass timestamps or combine several dated doors/show
labels into one event time. Reject those records; separate daily listings remain.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Two complete public browser captures | Identical pages of 36, 13, and 0 | 49 currently announced listings enumerated | Website-internal API, no service or reuse guarantee |
| Six raw and six staging API records | Correct venue IDs, Ticketmaster URLs, summer/winter offsets and explicit restrictions | Existing profile-based normalization fits | No prices are published |
| Staged replay | 46 valid records through April 3, 2027: 26 All Ages, 16 at 16+, two at 18+, two at 21+ | Age-14 clearance applies to the 26 All Ages events | Three multi-day passes rejected |
| Shpongle pass `1E006477A5E8755A` | Midnight listing, 7 p.m. doors note | Doors/listing mismatch rejects | Daily tickets remain |
| Decibel passes `1E006490A2EB96C9`, `1E006490A26A95ED` | Several dates with different doors/show times; three-day pass also includes Ratio Beerworks | Conflicting labeled times reject both | No off-site pre-fest is assigned to Fillmore |

Tests cover missing/unknown policies, ages 0, 2, 3, 14, 15, 16, and 17, configured
overrides, off-site IDs, midnight passes, conflicting clocks, reconciliation,
API consumption, and export. Set `FILLMORE_BASE_URL` for the shared
`tests/browser/marquis.spec.ts`. The existing `test:marquis` and
`test:contracts` commands cover the shared capture and Go interfaces.

## Summit profile and admission review — September 11, 2026

Summit reuses the same capture, page validation, decoder, and replay command.
Invoke `node tests/marquis/capture.mjs summit`; the default remains Marquis.
Only explicitly reviewed source profiles are accepted. Summit's normal All Rooms
page requests `https://content.livenationapi.com/v1/venues/KovZpZAFFt1A/events`.
The reviewed room IDs are Summit `KovZpZAFFt1A` and Moonroom `KovZ917AQXY`.
Both are at 1902 Blake Street and publish under the Summit venue filter.
Separate ticket IDs remain separate events. Other venue IDs reject.

The [Summit FAQ](https://www.summitdenver.com/visit), reviewed September 11,
states that an event without a listed restriction is All Ages. Ages three and
older require tickets; ear protection is requested for children. It publishes no
adult waiver for restricted events. Summit has its own configuration and policy
URL; it does not inherit Marquis's policy by provider association. All Ages
receives reviewed With adult ranges 0–2 and 3–17, with a ticket required in the
latter range. Explicit restricted or unclear records receive no inferred waiver.
Configured overrides still win.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Two fresh public browser enumerations | Identical pages of 36, 18, and 0 | Current listing enumerated to its empty terminal page | Website-internal interface, not a supported API guarantee |
| Six sampled records and full venue fields | Local/UTC clocks, labeled doors/show times; 52 Summit and two Moonroom records | Shared field semantics and reviewed room attribution fit | No unrelated/off-site venue inference |
| Staged replay | 53 valid events through May 2, 2027; one rejected | 48 All Ages, four 18+, one 21+; no prices | Coverage is current announcements, not future availability |
| Rejected pass `1E006504E6F142AB` | October 30 start is 00:00:01, but notes say doors 7PM | Existing doors/listing consistency check rejects the two-day pass | No special pass-time interpretation was added; separate daily listings remain |

Run the shared capture checks with `npm run test:marquis`, full Go/handoff checks
with `npm run test:contracts`, and the shared browser spec
`tests/browser/marquis.spec.ts` with `SUMMIT_BASE_URL` set to the staging or main
app. Tests include both room IDs, unrelated venue rejection, unknown/default/
restricted admission, configured overrides, retained URLs, failed jobs, and removal.
Capture is operator-only and introduces no scheduling or browser dependency in
the app. Public retrieval does not establish a reuse license.

## Retrieval and enumeration

- Page: https://www.marquisdenver.com/shows
- Observed event endpoint: `https://content.livenationapi.com/v1/venues/KovZpZAJeFkA/events`
- Requests: `offset=0&limit=36`, then offsets 36, 72, 108, and so on.
- Scroll normally until an explicit empty response. Require contiguous pages;
  full pages except for the last nonempty page; unique Ticketmaster event IDs.
- Repeat enumeration in a fresh browser context and require identical page data.
  Repeated responses at the same offset must also agree.
- Reject missing terminal pages, gaps, HTTP failures, changed data, malformed or
  oversized responses, duplicate IDs, and a capture that exceeds the page/time
  limits. Maximum 32 pages; maximum serialized snapshot 4 MiB.
- Filter valid records locally from the capture's Denver date through the next
  12 months. Coverage refers to current published listings, not future announcements.

The operator capture stores raw event response objects in `snapshot.json`, plus
the source, endpoint, and capture timestamp in `report.json`. Raw optional fields
remain evidence only; no price field enters the published artifact.

## Mapping

Accept primary `REGULAR` discovery events with venue ID `KovZpZAJeFkA`. Nested
upsells are not separate events. Venue, required identity, UTC/local date/time,
and Ticketmaster URL identity must agree. Reject unsupported venue or record types
instead of inventing off-site attribution. The calendar's Ticketmaster destination
is both the event and ticket link. Performers come from `artists`.

The reviewed listing timestamp represents doors. Parse explicit `Doors` and
`Show` labels from `important_info`; check doors against the UTC timestamp and
convert to America/Denver. Reject conflicting labeled times and show-before-doors
records. Missing show time remains omitted. Overnight labeled shows are not yet
supported. Explicit cancelled status maps to Cancelled; onsale, offsale,
rescheduled, and postponed map to Scheduled under the existing lightweight model.

## Required Marquis admission review

The [official visit FAQ](https://www.marquisdenver.com/visit), reviewed September
11, says an event without a listed restriction is All Ages. Ages three and older
need tickets. It recommends ear protection for children. It does not state that
an adult can waive an event's minimum age.

Configure the venue default as `All ages`, with With adult ranges 0–2 and 3–17;
the latter requires a ticket. Preserve explicit event restrictions. In this
implementation only All Ages receives reviewed clearance. Restricted and unclear
policies have no inferred accompaniment exception. Configured overrides win.
Unknown policy language does not silently receive the venue default. This parser
recognizes the reviewed statement forms, not arbitrary admission prose.

| Evidence source                               | Raw observation                                                        | Supported finding                                                           | Material limit                                                    |
| --------------------------------------------- | ---------------------------------------------------------------------- | --------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| Two complete browser captures                 | Pages 36, 36, 2, 0; 74 unique IDs                                      | Normal scrolling reaches February 10, 2027                                  | No service guarantee or automatic refresh                         |
| Eight raw/normalized samples                  | Correct venue IDs; labeled doors/show times; summer and winter offsets | Structured payload supports required fields                                 | Overnight and inconsistent times reject                           |
| Captured primary event notes                  | 73 All Ages; Cannibal Ox explicitly 18+                                | Age-14 filter includes supported All Ages events and excludes the 18+ event | No guardian exception for restricted shows                        |
| Venue robots, earlier September 11 inspection | `User-agent: *`, `Allow: /`                                            | No venue-path robots exclusion observed                                     | Not a reuse license; no claim of permission for direct API access |

## Verification and operating limits

Capture validation tests cover complete/empty, partial, changed, duplicate, missing
identity, and oversized-page cases. Go tests cover mapping, date consistency,
restriction conflicts, unknown/default admission, configured overrides, and record
rejection. CLI tests exercise real serialization, file publication, app API, `.ics`,
failed-job preservation, invalid-record retention, and successful removal.
Browser tests exercise ages 0, 2, 3, 14, and 17 against All Ages and 18+ records.

Run `npm run test:marquis`, `npm run test:contracts`, and the browser test with
`MARQUIS_BASE_URL` pointing at the selected staging or local app. This is a one-time
local workflow, not a schedule or remote deployment. Summit's separate review is
above; Fillmore's review is also above. Ball uses the KSE adapter.
