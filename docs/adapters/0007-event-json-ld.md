# ADR 0007: Embedded Event JSON-LD

- Status: Proposed
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

September 11 implementation update: Marquis now uses the richer responses from
its normal browser scroll flow, defined in [ADR 0021](0021-livenation-venue-events.md).
The JSON-LD first-page inventory below remains historical exploration; it is not
the selected complete Marquis importer. Summit remains unimplemented.

## Context

Marquis and Summit return structured MusicEvent records inside their HTML. An HTML request does not necessarily require scraping visual event cards.

## Decision

Use a JSON-LD extraction adapter with explicit source selection rules for Event and MusicEvent nodes.

## Retrieval and mapping contract

Fetch the source page and parse script elements of type application/ld+json as data, without executing scripts. Support arrays and graph containers in the parser contract. Extract provider identifiers where available, title, start date, location, offers, status, images, and URLs. Resolve relative links against the source URL. Preserve source values when optional schema fields are missing.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source                                | Raw observation                                                                   | Supported finding                                    | Material limit                                   |
| ---------------------------------------------- | --------------------------------------------------------------------------------- | ---------------------------------------------------- | ------------------------------------------------ |
| [Marquis](https://www.marquisdenver.com/shows) | Initial HTML contained MusicEvent records including BLÜ EYES on October 18, 2026. | Structured page extraction reaches beyond September. | Further-page coverage was not established.       |
| [Summit](https://www.summitdenver.com/shows)   | Initial HTML contained MusicEvent records including Quadeca on November 2, 2026.  | Reuse the same extraction family.                    | Summit and Moonroom coverage needs verification. |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test arrays, graphs, malformed unrelated scripts, duplicate nodes, timezone-bearing dates, missing location/offer fields, and sold-out or canceled statuses. Independently verify how additional listings are loaded; initial-page success is not full enumeration.

No importer, fixtures, or runtime checks are implemented by this ADR.

## Additional candidates and routes — September 10, 2026

Preserve the new [Marquis September route](https://www.marquisdenver.com/shows/calendar/2026-09)
under the existing source identity. Research browser retrieval failed; that failure
does not prove the route is broken. Its month component does not establish a range
API or complete monthly enumeration. The verified `/shows` JSON-LD evidence above
is historical and does not establish the payload of this new route.

The supplied [Fillmore listing](https://www.fillmoreauditorium.org/events/)
identifies itself as an independent, unaffiliated resale guide. Prefer the
[official Denver venue listing](https://www.fillmoredenver.com/shows) for further
evaluation and preserve the supplied URL in the registry. The official extracted
calendar says Loading; no Event JSON-LD payload was verified in this review.
This family is only a candidate for Fillmore. Sharing Live Nation branding with
Marquis and Summit does not prove an identical data contract. Inspect the official
payload before selecting JSON-LD, an API, or another retrieval model.

The [official Fillmore visit policy](https://www.fillmoredenver.com/visit), reviewed
September 10, prohibits under-16 guests at 16+ events even with an adult. Explicit
All Ages events permit all ages and require a ticket for each guest. These are
policy findings, not implemented filter rules. Complete the venue-specific
review and boundary/override tests described in the
[registry](../adr/0013-source-adapter-registry.md#september-10-2026-candidate-expansion)
before onboarding either candidate; do not copy policy between Live Nation venues.

## Alternatives and consequences

Card scraping is more coupled to layout. Ticketmaster Discovery may be an alternative with authorized API access, but keyed access and complete coverage were not tested. JSON-LD is the verified path, with page-level coverage limits.
