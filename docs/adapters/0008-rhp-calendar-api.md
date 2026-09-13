# ADR 0008: RHP calendar JSON

Operational profiles now follow [ADR 0023](../adr/0023-locale-isolation.md).
The venue name, website origin and timezone come from the source configuration,
not a compiled Denver registry. Capture requires `SITE_DIR` and a source CLI
argument listed in that locale's `capture.json`. Existing origin, detail and
admission checks remain in place.

- Status: Accepted for Lost Lake, Larimer Lounge, Globe Hall, and on-site Cervantes snapshot ingestion
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

Larimer Lounge, Globe Hall, and the official Cervantes site use the same RHP Events calendar action. The response mixes structured fields and HTML fragments.

## Decision

Use a shared RHP adapter configured by site origin and verified calendar options. Retrieve event data through the read-only action used by the public calendar.

## Retrieval and mapping contract

POST to /wp-admin/admin-ajax.php on the configured origin with the form below. Read data.events after validating success and the response shape. This is a retrieval operation despite its POST method.

```text
action=loadEtixMonthViewEventPageFn
data[limit]=10000
data[display]=false
data[target]=calendar-1
data[archive]=true
```

Map plaintitle, url, start, imageurl, and available venue information. Decode entities in plain titles. Parse ticket links and CTA state from strctaHtml as untrusted HTML. The archive option was used by the public calendar; it is not evidence that historical events are returned. The requested limit is not a completeness guarantee. Local start values lack offsets. Repeated end values of 23:59:59 are display boundaries, not verified show end times.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source                                                                                          | Raw observation                                                                              | Supported finding                                                     | Material limit                                                |
| -------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- | ------------------------------------------------------------- |
| [Larimer calendar](https://larimerlounge.com/events/?view=month) and its AJAX response                   | Returned Milk & Bone on December 15, 2026, plus entries for other rooms.                     | The action is not limited to the visible month.                       | Source is not a single-room identity.                         |
| [Globe Hall](https://globehall.com/events/) and its AJAX response                                        | Returned Genevieve Artadi on December 11, 2026; sample venue fields were empty.              | Same contract fits Globe Hall.                                        | Venue configuration or detail evidence is needed.             |
| [Cervantes official calendar](https://cervantesmasterpiece.com/events/?view=month) and its AJAX response | Returned events at Cervantes, Mission Ballroom, and Fiddler’s Green, including June 5, 2027. | Official site provides an alternative to the challenged Etix listing. | Do not assign every event to Cervantes as its physical venue. |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test all three site fixtures, form serialization, failure envelopes, entity decoding, CTA extraction, venue/room mapping, timezone policy, and placeholder end times. Investigate any count equal to the requested or server cap. Do not use similartitle or eventtimestamp as canonical identity/time without validation.

The September 10 implementation below supersedes the initial unimplemented status.

## Lost Lake candidate — September 10, 2026

The supplied [Lost Lake month calendar](https://lost-lake.com/events/?view=month)
loads `rhp-events` assets, including `monthMain.min.js`. The extracted calendar
text says Loading Events. This supports investigating the existing RHP family,
but the Lost Lake action response has not been verified. Do not claim the form
above works for Lost Lake or that its response covers more than a month yet.

Inspect the page's actual action and options, retrieve representative records,
and verify schema, enumeration, venue scope, detail enrichment, and time semantics
before assigning a verified source profile. Add Lost Lake fixtures to the existing
site-specific verification set rather than assuming plugin identity proves fit.

The [official FAQ](https://lost-lake.com/faq/), reviewed September 10, allows
under-16s at shows labeled 16+ with a ticketed parent or guardian. Record that
condition through the existing policy/override structure, test the age-14 case
and the age-16 boundary, and preserve event overrides and off-site isolation.
No permission for 18+/21+ shows is established by this finding. The
[registry](../adr/0013-source-adapter-registry.md#september-10-2026-candidate-expansion)
keeps onboarding incomplete until retrieval and admission behavior are verified.

## Alternatives and consequences

Direct Etix retrieval was challenged for Cervantes, so it is not the selected path. HTML calendar scraping would repeat the AJAX data. RHP shares WordPress transport conventions with other plugins but not their record contract.

## Implemented contract — September 10, 2026

`internal/rhp` implements one snapshot adapter for `lost-lake`, `larimer`, and
`globe-hall`. `ingest replay-rhp` reuses the existing reconciliation and atomic
publication workflow. The subsequent on-site Cervantes extension is described below.
No app configuration or artifact-schema change is required.

The public calendar action and form above were verified for all three sites.
Each robots.txt explicitly allows `/wp-admin/admin-ajax.php`. This is access
guidance, not a legal license or a promise that future access remains permitted.

The snapshot contains the raw response object in `calendar` and a `details` map
from event URL to captured HTML. The operator capture helper retains original
calendar/page responses separately. It removes non-evidence scripts, styles,
images, metadata, and comments from replay HTML to fit the existing 4 MiB bound.
Event JSON-LD is preserved unchanged. The Go HTML parser never executes scripts.

Event URLs supply upstream identity. Event JSON-LD verifies title, date, venue,
and ticket URL against the calendar. Visible `eventDoorStartDate` markup supplies
separate doors/show times. Ambiguous or nonexistent Denver local times are rejected.
Feed end-of-day placeholders and all price fields are ignored. Ticket URLs are
restricted to the observed HTTPS Etix product route. Unknown mappings are rejected
with an operator report; failed calendar envelopes cannot delete published events.

Actual detail venues override the source venue. Off-site records remain distinct,
with no default admission-policy inheritance. Shared calendar fragments do not
prove one ticket covers multiple separately listed performances or rooms.

Coverage means all records returned by the verified unpaginated action within
the next 12 months. Reaching the requested 10,000-row cap fails the run. The
provider publishes no independently verified total, so an undocumented lower
server cap remains a limitation. Do not claim a full year of announced events
merely because the requested horizon is a year. Historical records follow the
existing 90-day retention rules; these feeds do not backfill past dates.

### Required venue admission review

| Evidence source                                                                                                                                                         | Raw observation                                                                                                                         | Supported rule                                                                    | Material limit                                                                                      |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| [Lost Lake FAQ](https://lost-lake.com/faq/) and [FOREVER DOLLY detail](https://lost-lake.com/event/forever-dolly-a-dolly-parton-celebration/lost-lake/denver-colorado/) | FAQ permits under-16 attendance at 16+ shows with a ticketed parent/guardian; event specifies a guardian aged 21+.                      | Exact event condition yields “Ticketed guardian aged 21+ required” for ages 0–15. | Does not establish an exception for 18+/21+ shows or an ordinary older friend.                      |
| [Larimer FAQ](https://larimerlounge.com/faq/) and [K Motionz detail](https://larimerlounge.com/event/k-motionz-w-nvoke-scottsbrandnew/larimer-lounge/denver-colorado/)  | Same FAQ exception and more specific event condition.                                                                                   | Same reviewed event rule for on-site listings.                                    | Campground and Treehouse do not inherit Larimer's policy.                                           |
| [Globe Hall FAQ](https://globehall.com/faq/) and [Center Mid detail](https://globehall.com/event/center-mid-w-rosewood-bitters/globe-hall/denver-colorado/)             | FAQ did not establish a blanket show exception; event description explicitly requires a ticketed guardian aged 21+ for under-16 guests. | Match the exact event-description condition, not a venue-wide assumption.         | Missing or unsupported evidence stays unknown. Restaurant access is not concert admission evidence. |

Reviewed September 10, 2026. The exact guardian condition must occur in an event
description list item, not unrelated page metadata or navigation. It enriches
known 16+ or All Ages policies only. When the exact condition exists,
ages 16–17 retain the existing “Permitted at this age” rule;
ages 0–15 carry the ticketed-guardian condition. If the header omits a label but the
exact condition exists, classify unaccompanied access as 16+ while preserving
the full original condition as the policy text. Explicit on-site All Ages events
without that condition use the existing all-ages accompaniment wording for ages
0–17. Configured event overrides still win. Unknown restrictions and off-site
events receive no inferred accompaniment clearance.

Tests cover each profile, envelopes, caps, HTML/entity handling, time conflicts,
off-site attribution, policy overrides, unrelated text, cancellation, all-ages
admission, CLI-to-artifact-to-HTTP/export behavior, failure retention, and successful
absence. See the [fixture instructions](../../internal/rhp/testdata/README.md) and
[verification record](../adr/0019-implementation-and-verification-plan.md).

## Cervantes on-site extension — September 10, 2026

Approved scope: add both Cervantes rooms and one-ticket dual-room events. Defer
promoted shows at external venues. Cross-source deduplication is not implemented;
this scope avoids adding duplicate Mission/Ogden listings. The original
[Etix organizer URL](https://www.etix.com/ticket/o/6122) remains the supplied source
reference; retrieval uses the official site's public RHP action.

Use source and venue key `cervantes`, display name `Cervantes`, and origin
`https://cervantesmasterpiece.com`. Verify the route and decoded JSON-LD location:

| Evidence source                                                                                                                                                                                                                                   | Raw observation                                                                                                                                         | Supported mapping                                                                                                                                       | Material limit                                                                                                |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| [The Gringos detail](https://cervantesmasterpiece.com/event/the-gringos-w-the-john-wood-band/cervantes-other-side/denver-colorado/)                                                                                                               | `cervantes-other-side` / `Cervantes’ Other Side`                                                                                                        | On-site Cervantes event                                                                                                                                 | No external-venue inference.                                                                                  |
| [Mala detail](https://cervantesmasterpiece.com/event/mala-w-machinedrum-chelsmosis-funktion-one-sound-by-lost-horizons/cervantes-masterpiece-ballroom/denver-colorado/)                                                                           | `cervantes-masterpiece-ballroom` / `Cervantes’ Masterpiece Ballroom`                                                                                    | On-site Cervantes event                                                                                                                                 | Separate ticketed performances remain separate records.                                                       |
| [Desert Dwellers / Great Dane detail](https://cervantesmasterpiece.com/event/desert-dwellers-w-entangled-mind-m%c4%81h-ze-t%c4%81r-living-light-ballroom-great-dane-w-kilo-nyrus-other-side/cervantes-and-other-side-dual-venue/denver-colorado/) | `cervantes-and-other-side-dual-venue` / `Cervantes’ and Other Side – Dual Venue`                                                                        | One calendar entry remains one event under Cervantes                                                                                                    | Do not split or merge based on title prose.                                                                   |
| [Official FAQ](https://cervantesmasterpiece.com/faq/)                                                                                                                                                                                             | At 16+ shows, ages 14–15 require a parent/guardian; under-14 guests cannot attend. All Ages shows admit all ages; escorts are encouraged for under-15s. | Configure 16+ ranges 14–15 with parent/guardian, 16–17 with ordinary admission; All Ages ranges 0–14 with encouragement, 15–17 with ordinary admission. | No inferred exception for 18+, 21+, or unknown restrictions. An older friend is not automatically a guardian. |

Admission review completed September 10, 2026. Rules use the existing
`admission_rules` configuration and link to the FAQ. Configured event overrides
still win. Cervantes must not inherit the other RHP venues' under-16/ticketed-
guardian-21+ rule. Missing or unrecognized restrictions remain unknown. Interpret
the FAQ's “16+” boundary consistently with event headers saying “Ages 16 and up.”

The shared time parser accepts `Doors: 7 pm | Show: 8 pm`; calendar and JSON-LD
show times must still agree. Both rooms use one public venue selector. Event
titles and links remain intact. Prices remain excluded.

External routes appear in the operator rejection report with reason
`deferred external venue: Cervantes on-site scope only`. These are deliberate
scope exclusions, not fetch failures. Original calendar/detail responses remain
available for later cross-source work. Unknown room routes require review before
allowlisting. No scheduled jobs or direct Etix scraping were added.
