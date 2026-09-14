# ADR 0013: Register intended sources and adapter assignments

- Status: Proposed
- Date: 2026-09-08
- Related: [Source evaluation](0001-data-source-evaluation.md)

[ADR 0014](0014-product-and-delivery-contract.md) defines the accepted product scope and initial subset: Gothic, Mission, then HQ. This registry remains the full candidate inventory; aggregator-specific work is deferred and source identity is not a user-facing filter.

## Context

The original inventory contains 23 calendar URLs across three exploratory batches. The September 10 expansion below preserves seven additional supplied URLs, including updates to existing venues. All are intended ingestion candidates. Candidate inclusion does not authorize bypassing access controls or establish readiness to enable scheduled ingestion.

This registry preserves each original URL, assigns the verified retrieval family, and records known limits. The name Cervantes refers to the user-supplied Etix organizer listing even though the selected retrieval alternative is the official venue site.

## Decision

Use the assignments below as the proposed source inventory. Store operational configuration separately when implementation begins. Do not enable an unresolved source or claim complete ingestion from a partial listing.

Evidence states follow ADR 0001. Dates in the observation column are examples retrieved during the September 8, 2026 exploration, not guaranteed maximum horizons. The endpoints were not re-fetched during ADR authoring.

## Source inventory and inspected evidence

### September 14, 2026 integrations

Dazzle is integrated through its [VenuePilot profile](../adapters/0020-venuepilot-widgets.md#dazzle-implementation--september-14-2026), with 171 published events and the reviewed 11 PM admission condition.

Seventh Circle is integrated through its [HTML profile](../adapters/0012-html-event-listings.md#seventh-circle-implementation--september-14-2026), with six published events and the approved annual-fee condition.

Implementation update, September 9: Gothic, Mission, Bluebird, Ogden, and
Fiddler's Green now have tested AEG snapshot support and one-time live data in the
local app. The historical inventory below remains research evidence, not current
health. See [the expansion record](0019-implementation-and-verification-plan.md#aeg-source-expansion--2026-09-09).

September 10 update: HQ and Oriental now have tested HoldMyTicket snapshot support
with Event JSON-LD detail enrichment. The approved one-time local import and its
verification are recorded in ADR 0019. Recurring retrieval remains unimplemented.

### Admission-review readiness

September 12: Roxy now uses the [Afton venue adapter](../adapters/0022-afton-venue-events.md).
The embedded calendar provides a public, paginated listing, resolving the earlier
retrieval blocker below. All 26 available records through March 4, 2027 are now
published locally. Twenty Afton events explicitly allow All Ages, one is 21+, and
five external listings remain age-unknown. The venue has no inferred blanket age
policy or guardian exception. Native clocks are checked against structured
instants; ambiguous external clocks and all prices remain omitted. Both inspected
Afton page templates are supported. Unknown empty responses fail safely.

The catalog now contains all 26 dedicated venue candidates. All 25 earlier source
references are unchanged. No dedicated candidate remains unintegrated, but Roxy's
external-ticket admission enrichment remains follow-up work. Aggregators and
recurring jobs remain deferred. Older inventory rows below retain the research
state at their stated dates, not the current implementation status.

September 12: Herb's now uses the [ordinary HTML profile](../adapters/0012-html-event-listings.md#herbs-implementation-and-admission-review--september-12-2026),
superseding its historical Squarespace JSON assignment. Twenty-two events through
September 30 are published locally. Each starts before 22:30 and carries the
reviewed parent condition: minors must leave by 10:30 PM. Printed clocks are
treated as Denver-local by explicit user approval; the site's New York setting
is not used. No prices, ticket links or doors are inferred. The catalog contains
25 dedicated sources and all 24 previous references are unchanged. Roxy is the
only remaining dedicated candidate. Aggregators remain deferred. Unknown empty
responses and forward pagination still require review; recurring jobs are disabled.

September 12: Black Buzzard now has tested paired HTTP capture and HTML/JSON-LD
cross-checking under the [HTML profile](../adapters/0012-html-event-listings.md).
Thirteen events through December 5 are published locally, including separate early
and late Tim Butterly performances. Placeholder age text falls back to the FAQ's
18+ default with ID required. No guardian exception was found; none is inferred.
Times remain omitted because their meaning is unverified. No prices are published.
The catalog now contains 24 dedicated venue sources; all 23 previous references
remain unchanged. Herb's and Roxy remain, with aggregators deferred. Empty markup
and additional pagination require review before those responses can be accepted.

September 12: Ophelia's now has tested paired HTTP capture and scoped HTML replay.
The [HTML adapter and admission review](../adapters/0012-html-event-listings.md)
supersede the historical retrieval-only status. All 38 events are published locally:
seven 16+, four 18+, and 27 21+, through December 12. Eleven support the age-14
guardian use case. The FAQ permits a legal guardian exception at 18+ shows; seven
16+ listings explicitly state their own exception. Nobody under 13 is eligible,
and no exception applies at 21+. Empty markup remains unverified and fails safely.
There are now 23 dedicated venue sources, with all 22 previous references unchanged.
No prices, recurring jobs, aggregators or UI changes were added.

September 12: Meow Wolf Denver now has tested paired browser capture and replay
using the [embedded application JSON profile](../adapters/0011-embedded-application-json.md).
All 50 captured events are published locally through January 23, 2027: 24 All
Ages, 14 18+, 11 21+, and one unknown restriction. One is cancelled. The reviewed
event FAQ requires a ticketed guardian over 18 for guests under 16 at All Ages
shows; restricted and unknown events receive no inferred waiver. The catalog
contains 22 dedicated venue sources, with all 21 previous references unchanged.
This supersedes the historical unimplemented status. No recurring jobs are enabled.

September 12: Levitt now has tested, unauthenticated VenuePilot GraphQL capture
and replay. Its [adapter and admission review](../adapters/0020-venuepilot-widgets.md)
supersede the unresolved endpoint and policy entries in the historical tables.
The paired capture contains ten events through October 11: eight explicit All
Ages and two 21+ Beer Festival dates. The FAQ requires adult accompaniment for
ages 16 and younger; restricted or unknown events receive no inferred waiver.
Null API age values do not imply All Ages. No prices or recurring jobs are added.
All ten events are published locally. The catalog contains 21 dedicated venue
sources; all 20 previous source references remain unchanged.

September 11: Black Box's scoped Supabase adapter and ticket-page admission
enrichment are implemented and published locally.
The [adapter ADR](../adapters/0004-supabase-rest-api.md) records the dual-room
mapping, exact-count pagination, explicit doors/music times, 18+ venue policy,
and lack of a published adult exception. External ticket-provider redirects
reject instead of being bypassed. This supersedes the earlier unimplemented
adapter status. The local app has 43 Black Box events through November 21, 2026,
all 18+, with one external-provider record rejected. There are now 20 dedicated
sources; all 19 previous source references remain unchanged.

September 11: Fillmore has tested Live Nation browser-flow capture/replay against
the official venue site. Its [separate admission review](../adapters/0021-livenation-venue-events.md#fillmore-profile-and-admission-review--september-11-2026)
supports All Ages with tickets at every age and 16+ only for ages 16–17 with valid
ID. Under-16 guests have no adult exception. Missing restrictions remain unknown.
The local app contains 46 Fillmore events through April 3, 2027; three passes
reject on incompatible time data. This supersedes the unverified JSON-LD
assignment below; the originally supplied resale guide remains discovery evidence.
There are now 19 dedicated sources; all 18 prior source references are unchanged.

September 11: Summit has tested Live Nation browser-flow capture/replay, covering
both Summit and Moonroom at the same address under the Summit venue filter.
Its [separate admission review](../adapters/0021-livenation-venue-events.md#summit-profile-and-admission-review--september-11-2026)
supports All Ages clearance, not an adult waiver for restricted events.
The local app contains 53 valid Summit events through May 2, 2027; one two-day
pass rejects on inconsistent time data. This supersedes the original partial
JSON-LD assignment and unverified Moonroom coverage below.
The catalog now contains 18 dedicated sources. All 17 earlier references remain
unchanged; no aggregator was added.

September 11: Ball Arena has tested `kse-calendar` capture/replay with official
HTML listing enrichment. The [KSE ADR](../adapters/0009-kse-event-apis.md#ball-arena-implementation-and-admission-review--september-11-2026)
records the admission review: recognized home-game rules are supported; concert
and special-event admission remains unknown without an explicit override. The
local catalog contains 119 Ball events through May 16, 2027; one TBA listing is
rejected. There are now 17 dedicated venue sources.
This supersedes the earlier unimplemented/range-unverified status. No aggregator
or other venue policy is inferred from this work.

September 11: Marquis has tested `livenation-venue-events` capture/replay with
the venue's normal browser scroll flow. The [Live Nation ADR](../adapters/0021-livenation-venue-events.md)
supersedes the partial JSON-LD assignment and records the completed admission
review: explicit All Ages default, ticket requirement from age three, and no
guardian waiver of restricted shows. The local catalog contains 74 valid Marquis
events through February 10, 2027: 73 All Ages and one 18+. There are now 16 dedicated
venue sources. Ball remains the next candidate; no Ball implementation is included.

September 11: Paramount now has tested `kse-venue-events` capture and replay.
The [KSE ADR](../adapters/0009-kse-event-apis.md#paramount-implementation-and-admission-review--september-11-2026)
records the completed venue/event admission review. There is no blanket guardian
exception; only explicit event restrictions support With adult matches. The published
capture yielded 72 valid events through April 10, 2027 and two rejected records
with stale doors dates. This supersedes Paramount's historical pending review below.
The local catalog now has 15 dedicated sources. Ball remains a separate,
unimplemented KSE variant; aggregators remain deferred.

September 11: Hi-Dive is now published locally through the tested `plot-listings`
adapter. Its [Plot ADR](../adapters/0005-plot-listings-api.md#hi-dive-implementation-and-admission-review--september-11-2026)
records the reviewed parent/legal-guardian exception, preserved known 18+/21+
restrictions, unknown-restriction fallback, and age-boundary/override checks.
The one-time capture yielded 33 events through November 20; no prices or off-site
records were published. This supersedes the historical feasibility-only status.
That import brought the local catalog to 14 dedicated venue sources; aggregators remain deferred.

September 11: Red Rocks now uses tested `clique-calendar` capture and snapshot
ingestion. The [Clique adapter ADR](../adapters/0003-wordpress-clique-api.md#required-venue-admission-review)
records the completed All Ages default review, event-specific 13+ restriction,
and tested With adult age boundaries. This supersedes the historical range-testing
status below. Retrieval remains operator-triggered, not scheduled.

September 10 RHP implementation: Lost Lake, Larimer Lounge, and Globe Hall now
share `rhp-calendar` snapshot ingestion with event-detail enrichment. Their
admission reviews, precise guardian conditions, unknown-policy behavior, and
off-site isolation are recorded in [ADR 0008](../adapters/0008-rhp-calendar-api.md#required-venue-admission-review).
This supersedes Lost Lake's earlier unverified-endpoint status below. Cervantes
now also has on-site RHP support, as described below. See ADR 0019 for publication and runtime verification;
recurring retrieval remains outside this change.

Cervantes on-site onboarding is complete: Masterpiece Ballroom, Other Side, and
single-ticket dual-room events share venue `Cervantes`. The
[Cervantes admission review](../adapters/0008-rhp-calendar-api.md#cervantes-on-site-extension--september-10-2026)
supports ages 14–15 at 16+ shows with a parent or guardian; under-14s are excluded
from that exception. External promotions remain deliberately deferred until
cross-source deduplication and venue-specific admission handling are addressed.
They appear in operator rejection reports rather than the public calendar.

Accepted September 10, 2026: retrieval status is separate from venue-onboarding
readiness. Apply the [required admission-policy review](0001-data-source-evaluation.md#required-venue-admission-policy-review)
before marking a venue ready. Do not treat published event restrictions as a
substitute for reviewing accompaniment policy.

- **HQ: onboarding incomplete — With adult policy review pending.** Event ingestion
  and exact age filtering are implemented; official accompaniment-policy evidence,
  supported rules, and end-to-end filter verification remain required.
- **Oriental: onboarding incomplete — With adult policy review pending.** The same
  review and validation requirements remain outstanding.

Keep their existing events published. Unknown permissions remain excluded from
With adult. The completed policy work for Bluebird, Gothic, Mission, Ogden, and
Fiddler's Green is recorded in the product and verification ADRs; it does not prove
policy coverage for a new venue.

### Historical retrieval inventory

| Source key and original evidence URL                                               | Raw observation                                                                                                               | Assignment and supported finding                                                                                                    | Material limit / evidence state                                                                             |
| ---------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| gothic — [Gothic Theatre](https://www.gothictheatre.com/calendar/)                 | AEG feed 37 returned Airborne dated June 8, 2027.                                                                             | [AEG](../adapters/0002-aeg-json-feeds.md); future-month retrieval works.                                                            | Verified retrieval; feed cap and completeness unverified.                                                   |
| mission — [Mission Ballroom](https://www.missionballroom.com/upcoming-events/)     | AEG feed 89 returned FKJ dated April 29, 2027.                                                                                | [AEG](../adapters/0002-aeg-json-feeds.md).                                                                                          | Verified retrieval; shared cap/completeness limit.                                                          |
| bluebird — [Bluebird Theater](https://www.bluebirdtheater.net/calendar/)           | AEG feed 2 returned Dan and Peggy Reeder dated April 1, 2027.                                                                 | [AEG](../adapters/0002-aeg-json-feeds.md).                                                                                          | Verified retrieval; shared cap/completeness limit.                                                          |
| red-rocks — [Red Rocks](https://www.redrocksonline.com/events/?view=grid)          | Upcoming route returned a September 3, 2027 event; range route also returned records.                                         | [Clique](../adapters/0003-wordpress-clique-api.md); upcoming and range operations.                                                  | Verified retrieval; range boundaries and limits need tests.                                                 |
| marquis — [Marquis](https://www.marquisdenver.com/shows)                           | Initial HTML contained MusicEvent data, including BLÜ EYES dated October 18, 2026.                                            | [JSON-LD](../adapters/0007-event-json-ld.md).                                                                                       | Partial; further-page coverage unverified.                                                                  |
| ogden — [Ogden Theatre](https://www.ogdentheatre.com/calendar/)                    | AEG feed 7 returned Sylvan Esso dated March 6, 2027.                                                                          | [AEG](../adapters/0002-aeg-json-feeds.md).                                                                                          | Verified retrieval; shared cap/completeness limit.                                                          |
| black-box — [Black Box](https://blackboxdenver.co/events)                          | Public REST event query returned a published anniversary event dated November 21, 2026.                                       | [Supabase REST](../adapters/0004-supabase-rest-api.md).                                                                             | Verified retrieval; exact venue scope, pagination, and public credential lifecycle need checks.             |
| hi-dive — [Hi-Dive](https://hi-dive.com/events/)                                   | Plot pages included Holy Wave dated November 20, 2026 on the reported final page.                                             | [Plot](../adapters/0005-plot-listings-api.md).                                                                                      | Verified retrieval; doors/show and overnight fields need validation.                                        |
| oriental — [Oriental Theater](https://www.theorientaltheater.com/calendar_list)    | ICS 801 included Steve Hofstetter dated July 17, 2027.                                                                        | [iCalendar](../adapters/0006-icalendar-feeds.md).                                                                                   | Verified retrieval; sparse detail fields and provider timezone convention.                                  |
| larimer — [Larimer Lounge](https://larimerlounge.com/events/?view=month)           | RHP response included Milk & Bone dated December 15, 2026 and multiple room/venue names.                                      | [RHP](../adapters/0008-rhp-calendar-api.md).                                                                                        | Verified retrieval; source must not imply a single venue.                                                   |
| summit — [Summit](https://www.summitdenver.com/shows)                              | Initial MusicEvent data included Quadeca dated November 2, 2026.                                                              | [JSON-LD](../adapters/0007-event-json-ld.md).                                                                                       | Partial; further-page and Moonroom coverage unverified.                                                     |
| ball-arena — [Ball Arena](https://www.ballarena.com/events-tickets/calendar/)      | September–October request returned an Avalanche entry dated January 20, 2027.                                                 | [KSE calendar variant](../adapters/0009-kse-event-apis.md).                                                                         | Verified retrieval; requested bounds were not reliable filters. Includes sports and other categories.       |
| meow-wolf-denver — [Meow Wolf Denver](https://tickets.meowwolf.com/events/denver/) | Embedded application JSON included AdultiVerse dated December 19, 2026.                                                       | [Embedded application JSON](../adapters/0011-embedded-application-json.md).                                                         | Verified retrieval; startDateTime represented doors in samples. Includes non-concert categories.            |
| ophelias — [Ophelia’s](https://opheliasdenver.com/calendar/)                       | HTML event rows included Reprise dated December 12, 2026 at 9 PM.                                                             | [HTML, Ophelia’s profile](../adapters/0012-html-event-listings.md).                                                                 | Verified retrieval; no standalone event API verified.                                                       |
| black-buzzard — [Black Buzzard](https://www.theblackbuzzard.com/event-calendar)    | HTML cards included two Tim Butterly performances dated December 5, 2026.                                                     | [HTML, Black Buzzard profile](../adapters/0012-html-event-listings.md).                                                             | Verified retrieval; listing cards lack show times.                                                          |
| herbs — [Herb’s](https://www.herbsbar.com/live-music-calendar-1)                   | JSON upcoming records reached September 30, 2026; next-page records were past events.                                         | [Squarespace](../adapters/0010-squarespace-calendar-json.md).                                                                       | Partial; later-month access not verified. Trial October query failed JSON parsing.                          |
| paramount — [Paramount](https://www.paramountdenver.com/event-calendar/)           | KSE venue response included Elías Medina dated April 10, 2027.                                                                | [KSE venue-events variant](../adapters/0009-kse-event-apis.md).                                                                     | Verified retrieval; sampled local-time date components were inconsistent.                                   |
| cervantes — [Cervantes: original Etix listing](https://www.etix.com/ticket/o/6122) | Original URL returned HTTP 202 with a bot challenge. Official-site RHP response included promoted shows through June 5, 2027. | [RHP](../adapters/0008-rhp-calendar-api.md) via [official Cervantes calendar](https://cervantesmasterpiece.com/events/?view=month). | Verified alternative retrieval; direct Etix unresolved. Includes external venues, not just Cervantes rooms. |
| roxy — [Roxy](https://www.theroxydenver.com/calendar)                              | Wix Events integration was present; inspected HTML and embedded data exposed no usable event records.                         | No adapter assigned.                                                                                                                | Unresolved; needs browser network inspection or another authorized source.                                  |
| globe-hall — [Globe Hall](https://globehall.com/events/)                           | RHP response included Genevieve Artadi dated December 11, 2026.                                                               | [RHP](../adapters/0008-rhp-calendar-api.md).                                                                                        | Verified retrieval; sampled venue fields were empty.                                                        |
| fiddlers-green — [Fiddler’s Green](https://www.fiddlersgreenamp.com/calendar/)     | AEG feed 44 included Stick Figure dated June 5, 2027.                                                                         | [AEG](../adapters/0002-aeg-json-feeds.md).                                                                                          | Verified retrieval; publication completeness unverified.                                                    |
| hq — [HQ](https://www.hqdenver.com/page/calendar)                                  | ICS 6457 included Nervosa dated March 8, 2027.                                                                                | [iCalendar](../adapters/0006-icalendar-feeds.md).                                                                                   | Verified retrieval; sparse details and provider timezone convention.                                        |
| 19hz-denver — [19hz Denver](https://19hz.info/eventlisting_Denver.php)             | HTML table included FKJ dated April 29, 2027 and the publisher’s unreviewed-automation warning.                               | [HTML, 19hz profile](../adapters/0012-html-event-listings.md); discovery/enrichment role.                                           | Verified retrieval; aggregator observations require reconciliation with direct publishers.                  |

## September 10, 2026 candidate expansion

Approved scope: documentation only. Preserve the historical inventory above and
use this section for the new evidence. No importer, source configuration, or
published artifact changes are authorized by these assignments. Marquis,
Paramount, and Roxy retain their existing source identities.

| Source key and supplied URL                                                                   | Raw observation                                                                                                                                                                                                                                                                                     | Assignment and supported finding                                                                                                                                                 | Material limit / evidence state                                                                                                                          |
| --------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| fillmore — [Fillmore supplied listing](https://www.fillmoreauditorium.org/events/)            | Page identifies itself as an independent guide, unaffiliated with Live Nation, and advertises resale tickets. The [official listing](https://www.fillmoredenver.com/shows) identifies Live Nation and the Denver venue but its extracted calendar says Loading.                                     | Prefer the official venue source; [Event JSON-LD](../adapters/0007-event-json-ld.md) is a candidate, not a verified assignment. Preserve the supplied URL as discovery evidence. | Official event payload and enumeration unverified. Do not treat the resale guide as venue-authoritative.                                                 |
| levitt — [Levitt Pavilion Denver](https://www.levittdenver.org/summer-concert-series#/events) | Raw HTML embeds VenuePilot, with an event-grid widget and a loader at `widget.staging.venuepilot.com/2.0.0-0fc7f2c0/widget-loader.js`.                                                                                                                                                              | [Proposed VenuePilot family](../adapters/0020-venuepilot-widgets.md); the event widget is distinct from its Squarespace host.                                                    | Provider identified; data endpoint, response schema, access requirements, and coverage unverified.                                                       |
| lost-lake — [Lost Lake](https://lost-lake.com/events/?view=month)                             | HTML loads `rhp-events` calendar assets, including `monthMain.min.js`; extracted month page says Loading Events.                                                                                                                                                                                    | Candidate for [RHP](../adapters/0008-rhp-calendar-api.md), alongside Larimer, Globe Hall, and Cervantes.                                                                         | Plugin evidence only; Lost Lake's action response and enumeration have not been validated.                                                               |
| marquis — [Marquis month route](https://www.marquisdenver.com/shows/calendar/2026-09)         | New URL names the existing venue and a specific month; research browser retrieval failed.                                                                                                                                                                                                           | Additional URL for the existing [JSON-LD](../adapters/0007-event-json-ld.md) candidate.                                                                                          | Failure does not establish that the route is broken or that it supplies month-bounded data. Existing assignment rests on September 8 `/shows` evidence.  |
| paramount — [Paramount](https://www.paramountdenver.com/event-calendar/)                      | Exact source URL already appears in the historical inventory with a KSE venue response.                                                                                                                                                                                                             | Retain [KSE venue-events](../adapters/0009-kse-event-apis.md); do not add a duplicate source.                                                                                    | KSE response was not revalidated in this expansion.                                                                                                      |
| roxy — [The Roxy Theatre](https://www.theroxydenver.com/calendar)                             | Extracted page contains Wix assets and an Afton Tickets website credit, but no event records.                                                                                                                                                                                                       | Keep existing source unresolved; [HTML ADR](../adapters/0012-html-event-listings.md) does not assign a parser.                                                                   | Website credit is not proof of an event API. Calendar network or alternate public listing inspection remains necessary.                                  |
| federal — [The Federal Theatre](https://thefederaltheatre.com/calendar_list)                  | Calendar links [subscription 8693](https://holdmyticket.com/ics_user/8693). Retrieved feed declares The Federal Theatre and America/Denver; sampled entries include Extortionist on September 10, Federal Nights on September 12, Mad Caddies on September 13, and The Black Queen on September 17. | [HoldMyTicket iCalendar](../adapters/0006-icalendar-feeds.md), using the observed `/ics_user/8693` route, not an invented `/ics/8693` route.                                     | Feed retrieval verified; detail enrichment, complete venue scope, adapter compatibility, and full horizon remain unverified. No event count is asserted. |

### Admission-policy review — September 10, 2026

These are research findings, not implemented With adult rules. Apply the
[required admission review](0001-data-source-evaluation.md#required-venue-admission-policy-review)
before onboarding. Preserve event restrictions and configured overrides. Do not
generalize a parent exception to another restriction, venue, or off-site event.

| Evidence source                                                                                                               | Raw observation                                                                                                                           | Supported finding                                                                                                                    | Material limit / remaining work                                                                                                                                               |
| ----------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [Fillmore official visit policy](https://www.fillmoredenver.com/visit)                                                        | Explicitly prohibits under-16 guests at 16+ shows even with an adult; All Ages permits all ages and requires each guest to have a ticket. | A 14-year-old does not qualify for 16+ shows with an adult; explicit All Ages shows can support admission with the ticket condition. | Encode and test the supported age boundaries and event overrides before onboarding. No exception for other restricted categories was established.                             |
| [Lost Lake FAQ](https://lost-lake.com/faq/)                                                                                   | Under-16 guests at shows labeled 16+ must attend with a ticketed parent or guardian.                                                      | A 14-year-old can qualify for those shows under that condition.                                                                      | Test the under-16 boundary and preserve event-specific overrides. Do not extend the rule to 18+/21+ or copy it to other venues merely because the FAQ mentions sister venues. |
| [Levitt FAQ](https://www.levittdenver.org/faq) and [rules](https://www.levittdenver.org/rules-regulations)                    | Extracted text did not establish an age or accompaniment rule; the rules page includes an image.                                          | Admission review remains incomplete.                                                                                                 | Image content and event-specific terms require inspection. Free admission does not establish All Ages permission.                                                             |
| [Marquis visit page](https://www.marquisdenver.com/visit)                                                                     | Official visit/FAQ page was retrieved; no supported accompaniment rule was recorded in this investigation.                                | With adult permission remains unresolved.                                                                                            | Complete the policy and representative event review; do not infer permission from the shared Live Nation platform.                                                            |
| [Paramount calendar](https://www.paramountdenver.com/event-calendar/)                                                         | Existing calendar evidence establishes a data-source candidate, not accompaniment permission.                                             | Admission review remains incomplete.                                                                                                 | Verify official policy and event-specific restrictions separately from KSE retrieval.                                                                                         |
| [Roxy rules](https://www.theroxydenver.com/rules) and [box office](https://www.theroxydenver.com/box-office-tickets)          | Extracted rules did not establish an age rule; box-office text describes ticket purchase and fees.                                        | No supported accompaniment exception recorded.                                                                                       | Absence from extracted text is not a prohibition. Review event restrictions and any non-text policy content.                                                                  |
| [Federal about page](https://thefederaltheatre.com/page/about-us) and [calendar](https://thefederaltheatre.com/calendar_list) | Retrieved about text did not establish an age rule; calendar/feed evidence supplies scheduling data.                                      | No venue-wide accompaniment exception established.                                                                                   | Review official event detail restrictions and policy evidence; do not inherit HQ or Oriental policy from HoldMyTicket.                                                        |

At the time of the expansion review, all candidates remained onboarding-incomplete. Known policy
evidence is useful for implementation planning, but no new rule has been tested
through artifact publication and the With adult filter at that point.

Federal implementation update: the HoldMyTicket profile now normalizes the unused
malformed description blocks before standard iCalendar parsing. Explicit All Ages
events receive event-linked With adult metadata; no restricted-show exception is
inferred. This supersedes Federal's earlier retrieval/admission research gaps only
to the extent verified in [ADR 0019](0019-implementation-and-verification-plan.md).
Other candidates and HQ/Oriental policy status are unchanged. Recurring retrieval
and complete upstream coverage remain unverified.

Federal local onboarding is complete for the September 10 snapshot: 35 events
published, including 32 explicitly All Ages events with reviewed With adult
metadata. Staged and main-container desktop/phone checks passed. This is a
one-time import, not scheduled ingestion or a guarantee of future policy coverage.

## Retrieval configuration references

These are observed retrieval interfaces, not production-ready configuration. Parameters, methods, and semantics are specified in the linked adapter ADRs.

### AEG feeds

- Gothic: [feed 37](https://aegwebprod.blob.core.windows.net/json/events/37/events.json).
- Mission: [feed 89](https://aegwebprod.blob.core.windows.net/json/events/89/events.json).
- Bluebird: [feed 2](https://aegwebprod.blob.core.windows.net/json/events/2/events.json).
- Ogden: [feed 7](https://aegwebprod.blob.core.windows.net/json/events/7/events.json).
- Fiddler’s Green: [feed 44](https://aegwebprod.blob.core.windows.net/json/events/44/events.json).

### Other structured endpoints

- Red Rocks: [upcoming JSON](https://www.redrocksonline.com/wp-json/clique/v1/get_upcoming_events) and [tested range request](https://www.redrocksonline.com/wp-json/clique/v1/get_calendar_events_range?start=2026-09-08&end=2026-10-01).
- Black Box: [events REST endpoint](https://yotaohjqtxebyhlzezjl.supabase.co/rest/v1/events), with the public client access requirements and source filters described in ADR 0004. Bare-link access without headers is not the verified query.
- Hi-Dive: [tested Plot page](https://hi-dive.com/api/plot/v1/listings?currentpage=1&listingsPerPage=5).
- Oriental: [ICS 801](https://holdmyticket.com/ics/801).
- HQ: [ICS 6457](https://holdmyticket.com/ics/6457).
- Federal: [user subscription 8693](https://holdmyticket.com/ics_user/8693), observed September 10; adapter compatibility remains unverified.
- Ball Arena: [tested KSE calendar request](https://alttix.ksehq.com/api/tm/Calendar?Id=1&start=2026-09-01&end=2026-11-01).
- Paramount: [KSE venue response](https://alttix.ksehq.com/api/tm/venue/KovZpZAFa1nA).
- Herb’s: [ordinary calendar HTML](https://www.herbsbar.com/live-music-calendar-1); this supersedes the historical JSON route above.

RHP uses the read-only POST form in ADR 0008 at these targets; a browser GET to the link does not reproduce the verified request:

- [Larimer AJAX target](https://larimerlounge.com/wp-admin/admin-ajax.php).
- [Globe Hall AJAX target](https://globehall.com/wp-admin/admin-ajax.php).
- [Cervantes AJAX target](https://cervantesmasterpiece.com/wp-admin/admin-ajax.php).

Marquis and Summit use JSON-LD in their original pages. Meow Wolf uses the original page’s `__NEXT_DATA__` script at `props.pageProps.events.events`. Ophelia’s, Black Buzzard, and 19hz use original-page HTML. Roxy now uses the public Afton endpoint documented in [ADR 0022](../adapters/0022-afton-venue-events.md).

## Alternatives and consequences

An inventory organized only by ticket seller would not represent the verified access paths. Cervantes is the clearest example: Etix ticket links remain useful identifiers, while the official site’s RHP response is the verified calendar source.

A venue-only inventory would also misrepresent multi-venue publishers. Keep the original source and physical venue separate, then reconcile overlapping observations outside adapters.

The original candidates and the September 10 additions remain in scope. Roxy's earlier retrieval deferral is resolved by ADR 0022. Partial sources can support experiments, but must not drive completeness claims or automatic deletion based on absence.

## Acceptance and follow-up

- Preserve the original user-supplied URLs, the September 10 expansion URLs, and the requested Cervantes label. Additional routes for an existing venue do not create duplicate source identities.
- Link every assigned source to an adapter ADR; do not silently classify Roxy.
- Verify access/reuse conditions, exact source scope, pagination/caps, and time semantics before enabling ingestion.
- Apply the fixture, integration, and consumer checks in ADR 0001.
- Recheck endpoints at implementation time and record the new observation date.
- Keep historical evidence separate from current health and ingestion status.

No ingestion jobs, credentials, fixtures, or source configuration files are created by this ADR.
