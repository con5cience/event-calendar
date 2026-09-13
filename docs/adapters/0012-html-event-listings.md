# ADR 0012: HTML event listings

- Status: Ophelia's, Black Buzzard and Herb's implemented; 19hz remains proposed
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

Ophelia’s, Black Buzzard, and 19hz expose useful event records in server-returned HTML. No suitable standalone event endpoint was verified for these listings.

## Decision

Share HTTP fetching and safe DOM parsing, but implement an explicit parser profile per source. Do not create a universal selector configuration that hides distinct date, venue, or enumeration rules.

## Retrieval and mapping contract

Ophelia’s profile extracts events-item records with title, date, time, age restriction, image, and ticket link. Black Buzzard’s profile extracts cal-info event cards with title, date, venue, image, and Tixr links; its month controls filter existing cards. The 19hz profile extracts table rows with date/time, event and venue, genre, price/age, promoter, and links, retaining the full date value where present.

Resolve relative links and decode text entities. Never execute source scripts.
Preserve raw date text in operator evidence alongside normalized dates. Venue,
date and title are required; show time is optional under the product contract.
Distinguish an empty calendar from a selector or schema failure.

## Herb's implementation and admission review — September 12, 2026

`tests/herbs/capture.mjs` reads only the ordinary
[calendar](https://www.herbsbar.com/live-music-calendar-1) twice. It reuses the
bounded HTTP transport: no redirects, 30-second timeout, UTF-8 HTML, 1 MiB per
page and 4 MiB per snapshot. `internal/herbs` compares parsed records across the
two passes; it never executes scripts. The
[earlier Squarespace JSON proposal](0010-squarespace-calendar-json.md) is superseded
for this venue. Robots guidance excludes JSON, month and incoming ICS routes.
This guidance does not establish redistribution rights or approve scheduled jobs.

Mapping uses the upcoming event list, item IDs, title links and printed start
dates. Both 12-hour and 24-hour start clocks must agree. Same-day and overnight
layouts have distinct selectors; end clocks and export timestamps are ignored.
Per explicit user approval, printed clocks mean Denver-local time even though the
site is configured to New York. This assumption is not independently confirmed by
the venue. DST gaps, repeated clocks and conflicting clocks reject the record.
Missing times remain missing and receive no parent exception. Only `show_at` is
populated; doors are not inferred. URL slug years are not date evidence. No price
or ticket link is inferred. Original event links remain available.

The calendar footer at 2057 Larimer Street states a 21+ establishment with minors
allowed until 10:30 PM when accompanied by a parent. The same policy is available
on the [venue homepage](https://www.herbsbar.com/). Replay checks the inspected
footer wording and address before applying the reviewed rule. Policy changes
stop the refresh for review. This is a parent exception, not permission to attend
with any older friend or to remain for the whole show.

For a scheduled on-site event with a known start strictly before 22:30, the event
policy adds ages 0–17 with the exact condition
`Parent required; minors must leave by 10:30 PM`. At or after 22:30, missing times,
cancellations and unreviewed restrictions get no exception. The 0–17 range follows
the existing child-age schema. Visible age-related text in titles, excerpts or
descriptions becomes an unclassified event policy pending review; configured
policy overrides still win. Off-site locations reject the record instead of
borrowing Herb's policy. The 21+ venue default has no blanket admission rule.

The approved capture supplies 22 events from September 12 through September 30,
2026. All start before the cutoff. Six raw records, six normalized records and six
records from each app API were inspected. A stale September 11 upcoming row is
excluded by its printed start date. Coverage is the ordinary page's available
upcoming list inside the next 12 months, not evidence of twelve populated months.
No forward pagination was present. Unknown empty layout, pagination, duplicate or
missing IDs, unsafe links, policy changes and disagreement between passes fail
the whole refresh. Five hundred cards is a conservative cap guard, not a measured
upstream limit. Invalid identified records retain their last valid version;
successful absence unlists previous events through the existing reconciler.
Old records expire 90 days after their dates; the initial capture does not backfill
the site's older event list.

Use `replay-herbs` with `internal/herbs/testdata/herbs.yaml` and an actual captured
snapshot. The adjacent HTML fixture is synthetic and must never be published.
Use `state: established` for subsequent refreshes. Mapping, cutoff, DST, override,
retention, paired capture and app boundary checks are recorded in
[ADR 0019](../adr/0019-implementation-and-verification-plan.md#herbs-integration--september-12-2026).

## Ophelia's implementation and admission review — September 12, 2026

The approved implementation uses two ordinary GET requests to
`https://opheliasdenver.com/calendar/`, with no credentials, browser navigation,
Ticketmaster requests or access-control bypass. The inspected robots file allows
the calendar path and restricts wp-admin except admin-ajax. This is access guidance,
not a redistribution license. One-time local ingestion is enabled; scheduling and
remote deployment remain outside this change.

The public page has one `#primary.eventspage`, an Upcoming Events heading and
`events-item` cards. The inspected calendar-specific scripts do not enumerate
additional pages. No standalone event API or event JSON-LD was verified. Reuse
x/net/html and the established inert DOM traversal convention, with explicit
Ophelia's selectors rather than a universal selector configuration. A browser
would add a dependency without improving this server-rendered listing contract.

Capture stores a `from`, `through`, `html`, `check_html` envelope. Replay parses
both pages, sorts by Ticketmaster event ID, and requires equal selected fields.
Unrelated dynamic scripts do not affect equality and are never executed. Limits
are 1 MiB per page, 4 MiB per snapshot and 500 cards. HTTP failures, redirects,
duplicate IDs, changed structure, unreviewed pagination and changed captures fail
the job. Zero cards also fail: no live empty-calendar marker has been reviewed.
A successful nonempty refresh can remove absent events using the existing retention
contract. Invalid known records retain their last valid version.

Select title, age label, scoped event excerpt, full local date, MT clock and action
link. The Ticketmaster URL supplies a per-performance ID. It is both the event
and ticket link because these cards have no separate venue detail page. Preserve
the URL as supplied; do not derive the event date from its slug. Map America/Denver
with DST. Three live excerpts explicitly identify doors and show clocks and agree
with the displayed show clock. Keep separate labeled doors where supplied; do not
calculate doors by subtracting the FAQ's general one-hour interval. Missing times
remain missing. Cancelled events have no ticket link; Sold Out stays Scheduled.
Do not publish descriptions, images, prices or unrelated page metadata.

The [official FAQ](https://opheliasdenver.com/faq/) gives a default of 21+, permits
underage attendance at 18+ shows with a ticket and legal guardian, excludes everyone
under 13, and permits no exception at 21+ shows. Configure the 18+ rule for ages
13–17. This is not permission to attend with any older friend.

Seven [calendar listings](https://opheliasdenver.com/calendar/) explicitly permit
under-16 attendance with a guardian at their 16+ events. Recognize those scoped
event statements only, retaining the venue's 13-year floor. Ages 13–15 require
a ticket and legal guardian; ages 16–17 retain ordinary ticket/ID conditions.
Do not create a blanket 16+ accompaniment rule from those examples. Unrecognized
restrictions grant no exception. Missing age data falls back to the reviewed 21+
venue default. Configured overrides retain their existing precedence.

Eight captured and eight normalized records were inspected before aggregate
reporting, including explicit times, HTML entities and guardian terms. All 38
records passed paired replay, with seven 16+, four 18+ and 27 21+. The dates span
September 18 through December 12, 2026. This establishes coverage of the inspected
public listing, not a guarantee of unpublished inventory or a full year of events.
The app consumes the new artifact without provider-specific app changes. See
[ADR 0019](../adr/0019-implementation-and-verification-plan.md) for verification.

## Black Buzzard implementation and admission review — September 12, 2026

Use the original `https://www.theblackbuzzard.com/event-calendar` plus
`https://www.theblackbuzzard.com/`. Both respond to ordinary unauthenticated GETs.
The [robots file](https://www.theblackbuzzard.com/robots.txt) disallows `/events/`
and allows other paths. The importer does not request the event-detail paths or
Tixr pages. A research attempt to open a Tixr short link through web retrieval
failed; no ticket-level enrichment is claimed. Robots guidance does not establish
a redistribution license. This workflow is an approved one-time local import,
not scheduled retrieval or remote deployment.

The original calendar has `cal-info` cards with full dates, title, explicit venue
and Tixr ID links. The homepage has richer Event JSON-LD inside `event-item` cards,
plus separate age cards. Six raw JSON-LD records were inspected: they identify
individual performances at 1624 Market St, Denver, with date-only startDate,
status and ticket URLs. HTML entities remain encoded inside some JSON strings.
The homepage and calendar have the same 13 IDs, through December 5, 2026. The
two Tim Butterly performances have distinct IDs and remain separate events.

The homepage includes a CMS pagination attribute and loader script, but the
inspected navigation container is empty. The calendar month controls filter
existing cards. Require equal identity sets across all three representations and
both capture passes. Reject next-page links, duplicate IDs and disagreements.
Fail at 100 calendar cards as a conservative operational guard, not a measured
provider limit. Zero cards also fail until real empty markup is reviewed. No claim
is made about unpublished inventory or a full year of available listings.

The shared bounded HTTP helper comes from Ophelia's tested transport. Parsing is
a source-specific Go profile using the established x/net/html traversal convention.
The snapshot has `from`, `through`, `pages` and `check`; each pair has `calendar`
and `home`. Limits are 1 MiB per page and 4 MiB per envelope. Raw pages remain
temporary operator evidence. No embedded script is executed. Only scoped Event
JSON-LD is decoded; unrelated scripts are ignored. Changes to ignored price data
do not affect normalized events. A general JSON-LD importer alone cannot establish
the required coverage and age-card joins, and browser automation is unnecessary.

Require the reviewed venue names and Denver address. Decode entity-encoded titles,
parse the full dates, and use the Tixr ID as upstream identity. Keep Tixr URLs as
both event and ticket links; no ambiguous show or doors clock is inferred from
the homepage's unlabeled time. Ignore endDate, prices, images and descriptions.
Recognized EventCancelled removes the ticket link; sold-out does not mean cancelled.
Unknown status or invalid known fields reject the record. Identity and enumeration
failures reject the whole capture. Existing retention and configured overrides apply.

The [official FAQ](https://www.theblackbuzzard.com/faq) says shows are generally
18+ unless otherwise posted and requires ID for entry. It publishes no guardian
exception. This is not proof that exceptions are prohibited; it means none can be
granted by this importer. Do not apply other CoClubs venues' rules from shared
marketing text. The inspected age cards contain the literal placeholder
`This is some text inside of a div block.`. Treat that exact placeholder or blank
text as missing and apply the venue default. Preserve recognized event-specific
categories; keep other text unknown. No adult-clearance rule is configured.

Paired replay accepted all 13 events with no rejects. Staged desktop/phone checks
passed, and the one-time main publication is verified. All records currently use
the 18+ venue default with no adult clearance. See ADR 0019 for operating results.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source                                                 | Raw observation                                                                                                                                 | Supported finding                                                        | Material limit                                                            |
| --------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------------------- |
| [Ophelia’s](https://opheliasdenver.com/calendar/)               | Server HTML included Reprise on December 12, 2026 with a 9 PM time.                                                                             | HTML can supply future-month records.                                    | No standalone API was verified.                                           |
| [Black Buzzard](https://www.theblackbuzzard.com/event-calendar) | Cards included early and late Tim Butterly shows on December 5, 2026.                                                                           | Date plus title alone is insufficient to collapse multiple performances. | Listing cards lacked show times.                                          |
| [19hz Denver](https://19hz.info/eventlisting_Denver.php)        | Table included FKJ at Mission Ballroom on April 29, 2027 and a warning that listings are automatically generated and not reviewed for accuracy. | Useful discovery and enrichment source.                                  | Do not treat aggregator data as authoritative or independently confirmed. |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test one fixture set per source, multiple shows per day, year boundaries, overnight times, missing values, ticket links, layout drift, and pagination if found. Check selectors against raw records before reporting counts. Compare aggregator observations with direct publisher evidence and retain conflicts.

Ophelia's and Black Buzzard have importer, fixture, capture, CLI and runtime checks.
The 19hz profile remains unimplemented;
aggregators remain deferred.

## Roxy follow-up — September 10, 2026

Superseded September 12: browser inspection verified the embedded Afton widget's
public paginated API. Roxy is implemented under [ADR 0022](0022-afton-venue-events.md)
with linked Afton HTML/JSON-LD admission enrichment. The historical investigation
below does not describe the current integration status.

The resubmitted [Roxy calendar](https://www.theroxydenver.com/calendar) remains the
existing unresolved source. Retrieved text contains Wix assets and an Afton
Tickets website credit, but no event records. The credit does not establish a
ticketing API, and an empty extraction does not establish an empty calendar.

Inspect the public calendar's network requests or another official public listing
before selecting an adapter. Do not create an HTML profile without usable records
and tested selectors. The [rules](https://www.theroxydenver.com/rules) and
[box-office page](https://www.theroxydenver.com/box-office-tickets) did not establish
an accompaniment exception in the extracted evidence. The
[registry](../adr/0013-source-adapter-registry.md#september-10-2026-candidate-expansion)
records that admission review as incomplete, not an explicit prohibition.

## Alternatives and consequences

Structured endpoints remain preferable when later verified with adequate coverage. Browser automation is not required for these inspected server-rendered listings. Roxy is assigned to the Afton JSON listing with HTML detail enrichment in ADR 0022, not a calendar-cell scraper. Site-specific parsers carry higher layout-maintenance cost.
