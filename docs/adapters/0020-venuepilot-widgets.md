# ADR 0020: Evaluate VenuePilot event widgets as a separate adapter family

Operational profiles now follow [ADR 0023](../adr/0023-locale-isolation.md).
`adapter_options.event_base` supplies the event-link prefix, including any hash
route and trailing separator. Venue name and timezone are configured. Capture
requires `SITE_DIR`; `CAPTURE_SOURCE` selects a different configured source.
Its GraphQL endpoint, account ID and timezone come from `capture.json`.

- Status: Implemented — Levitt capture, replay, and admission configuration
- Date: 2026-09-10
- Related: [Source evaluation](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

Levitt's calendar is hosted on Squarespace, but its raw page embeds a VenuePilot
event widget. Hosting technology alone does not determine the event-data adapter.
The supplied `#/events` fragment is client-side navigation, not an HTTP range
parameter.

## Decision

Use a separate VenuePilot adapter with the public widget's verified GraphQL
contract. The September 12 implementation supersedes the initial unresolved
retrieval questions below. Squarespace hosting does not define this interface.

## Verified contract and admission review — September 12, 2026

The official calendar's normal browser flow sends a read-only POST to
`https://www.venuepilot.co/graphql`. No authentication is required. The capture
uses `paginatedEvents` with `accountIds: [1105]`, `startDate`, `endDate`,
`limit: 5`, and one-based `page`. It selects event identity, title, local date,
doors/show time, minimum age, description, CTA status, venue name and ticket URL.
Images, social data, generic footer content and prices are not selected.

| Evidence source | Raw observation or test result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Official widget request and direct scoped API requests | No authorization headers; September 11 pagination returned 5, 5, 1 records; total 11 | Public structured retrieval supports pagination | Undocumented public interface, not a stability guarantee |
| Same API with October 10 bounds and January 2027 bounds | One October 10 event; January returned zero records and totalPages 0 | Date bounds and successful empty responses are distinguishable | Available listings do not fill the full 12-month window |
| September 12 paired capture; all ten raw records inspected | Matching passes, ten records through October 11; eight explicit All Ages and two explicit 21+ | Current scope includes free concerts, wellness and paid events | September 11 event is outside the new upcoming window |
| Event records and widget display | minimumAge null even for both Beer Festival days; descriptions say guests must be 21+ | Null must never imply All Ages | Event-specific admission text is required when the structured age is absent |
| [Levitt FAQ](https://www.levittdenver.org/faq) | Free concerts are All Ages; guests 16 and younger need an adult | Explicit All Ages events support reviewed adult clearance | No adult waiver for restricted events; ticket and capacity rules still apply |
| Rez Metal raw record | Event date October 1; ticket slug still September 25 | Preserve the API date and exact ticket link independently | URL text is not a reliable date source |

`tests/venuepilot/capture.mjs` validates page number, size, exact count, page count,
unique numeric IDs, HTTP success and absence of GraphQL errors. Two full passes
must agree. It caps retrieval at 500 events and the snapshot at 4 MiB. Errors
stop the capture; they do not become a successful empty feed. The operator
snapshot contains `from`, `through`, `total`, `events`, and `check`.

`internal/venuepilot` validates the paired envelope again and maps only the
reviewed Levitt venue. Dates must fall inside the captured Denver-local window.
Local `doorTime` and `startTime` become America/Denver timestamps with daylight
saving offsets. Missing times remain absent. Provider numeric IDs supply identity;
two dates with separate IDs remain separate even when their ticket URL is shared.
The venue event link is
`https://www.levittdenver.org/summer-concert-series#/events/<id>`.
HTTPS ticket links are preserved, not fetched or reconstructed.

The observed `FREE RSVP`, `FREE RSVP/UPGRADE`, and `TICKETS` CTA values map to
Scheduled. Explicit Cancelled/Canceled values are covered by synthetic tests,
not a live cancelled sample. Unreviewed statuses reject the record.

Admission extraction recognizes a standalone All Ages line or explicit
guest/patron/attendee minimum-age wording in the event description. It does not
use generic footer text, scripts, or an artist biography's passing mention of
All Ages. Supported explicit numeric minimum ages are also handled defensively;
the live sample contains only null numeric ages. Conflicting restrictions reject
the record. Unknown admission remains unknown and receives no adult clearance.

The venue configuration links the FAQ. For explicit All Ages events, ages 0–16
require an adult; age 17 uses the event's ticket requirements without an adult
requirement. All ticket, section and capacity details remain on the venue site.
Restricted categories receive no inferred waiver. Configured overrides retain
their existing precedence. No price data is published.

Use `replay-venuepilot` through the existing snapshot/reconciliation/publication
workflow. New-source staging uses `internal/venuepilot/testdata/levitt.yaml`;
refreshes require an operator config with `state: established`. The adjacent JSON
is synthetic test data and must never be published. Tests cover capture failures,
mapping, overrides, last-valid retention, successful absence, API, ICS export,
and desktop/phone age-14 filtering. See README for commands and ADR 0019 for the
local publication and verification record. No recurring job is enabled.

## Initial discovery evidence — September 10, 2026

| Evidence source                                                                      | Raw observation                                                                                                                                                        | Supported finding                                           | Material limit                                                                                                                                                    |
| ------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [Levitt calendar markup](https://www.levittdenver.org/summer-concert-series#/events) | `widgetElementId1` config uses `type: 'widget'` and `eventsType: 'event-grid'`; loader URL is `https://widget.staging.venuepilot.com/2.0.0-0fc7f2c0/widget-loader.js`. | VenuePilot supplies the embedded event UI.                  | Loader was identified in markup, not validated as a stable data interface. The staging hostname must not be silently replaced with a guessed production hostname. |
| Same page's widget configuration                                                     | Pagination has `show: false` and `perPage: 24`; calendar has `hidePastEvents: false` and `onlyShowCurrentMonth: false`.                                                | UI configuration exists for pagination and date visibility. | These settings do not prove response size, completeness, historical availability, or a range API. No event count or maximum horizon was measured.                 |

## Initial retrieval and mapping questions (resolved above)

Inspect the public loader and widget requests to identify the publisher scope,
read-only endpoint, method, parameters, access requirements, and response schema.
Do not invent endpoint names or use privileged credentials. Check both free and
ticketed events against the official calendar.

Validate representative raw records before asserting coverage. Identify provider
IDs, venue attribution, event and ticket URLs, event titles, dates, doors/show
times, timezone conventions, restrictions, cancellation, and optional prices.
Verify page/range controls, caps, duplicate handling, and failure versus empty
responses. Apply the 12-month ingestion window locally where needed; do not
claim the upstream publishes that entire window.

All artifact, reconciliation, and access requirements in ADR 0001 apply. Unknown
retrieval is not a successful empty calendar. The registry records the separate
admission-policy review gaps; free events do not imply unrestricted admission.

## Initial implementation verification requirements (implemented above)

Capture authorized representative fixtures only after confirming the interface.
Test scope, pagination, malformed responses, identity, time normalization,
optional fields, cancellations, and source-policy overrides. Verify the resulting
artifact through the existing consumer and With adult filter before onboarding.
The initial research added no importer or publication; the implementation above
now supplies the capture, decoder and configuration.

## Alternatives and consequences

Squarespace JSON is not selected because the inspected event UI uses VenuePilot.
Static HTML extraction did not expose event records in the retrieved text.
Browser-rendered extraction remains a fallback if no suitable public structured
interface can be verified. Reuse an existing adapter only if the observed record
contract actually fits it; shared hosting or presentation is insufficient.
