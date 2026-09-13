# ADR 0010: Squarespace calendar JSON

- Status: Historical proposal; Herb's uses the HTML profile as of September 12, 2026
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## September 12 decision: use ordinary HTML for Herb's

The implemented source is `html-herbs`, under the
[HTML listing profile](0012-html-event-listings.md#herbs-implementation-and-admission-review--september-12-2026).
The JSON proposal below is retained as historical research, not operating guidance.
The inspected robots guidance excludes format=json, format=ical and month query
routes. The capture therefore reads only the ordinary calendar URL twice; it does
not request JSON, month selectors, incoming ICS, event details or ticket pages.

The site reports `America/New_York`, while the venue is in Denver. For example,
Mike Maurer Band prints September 12 at 9:30 PM, but the export epoch translates
to 7:30 PM in Denver. The user explicitly approved interpreting printed dates and
clocks as Denver-local. This is an approved assumption, not verified venue intent.
The implementation does not convert the site's epoch or use old URL slug years
as event dates. See the HTML profile for coverage and parent-admission limits.

## Historical context

Herb’s exposes calendar JSON through a page format parameter. The inspected response separates upcoming and past records.

## Historical proposed decision

Use a Squarespace calendar adapter that interprets calendar timestamps and follows only verified enumeration behavior.

## Historical retrieval and mapping contract

The verified URL is https://www.herbsbar.com/live-music-calendar-1?format=json. Parse upcoming and past arrays and pagination metadata explicitly. Preserve item IDs and URLs. startDate values are Unix milliseconds; URL slugs can retain old years and must not establish event dates. Use the source timezone policy to convert timestamps. Sample location coordinates did not represent the Denver venue, so do not accept coordinates without validation.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source | Raw observation | Supported finding | Material limit |
|---|---|---|---|
| [Calendar JSON](https://www.herbsbar.com/live-music-calendar-1?format=json) | Upcoming records included B3 Jazz Jam on September 29 and Hump Day Funk Jam on September 30, 2026. | Structured extraction works. | No later-month record was verified. |
| [Advertised next page](https://www.herbsbar.com/live-music-calendar-1?offset=1776981243495&format=json) | Returned past records and an empty upcoming array. | This pagination path does not establish future coverage. | A trial month=2026-10 request did not yield parseable JSON. |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Historical implementation requirements

Test millisecond conversion, harmless fractional-second remnants, stale URL date components, upcoming/past separation, offset direction, duplicate items, empty pages, and venue coordinates. Verify any proposed future-range mechanism before enabling it.

No importer, fixtures, or runtime checks are implemented by this ADR.

## Historical alternatives and consequences

HTML scraping is unnecessary for the verified records. Treating every nextPageUrl as forward event-time pagination would give a false completeness claim. Later-month retrieval remains a source-specific investigation item.
