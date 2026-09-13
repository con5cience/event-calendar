# ADR 0011: Embedded application JSON

Meow Wolf operational profiles now follow [ADR 0023](../adr/0023-locale-isolation.md).
`adapter_options.seller_id`, `rooms` (a JSON object encoded as a string), and
`default_room` scope accepted detail identities. Venue website and timezone are
configured. Capture requires `SITE_DIR`; `CAPTURE_SOURCE` selects a different
configured source. Its origin, city slug and seller ID come from `capture.json`.

- Status: Implemented — Meow Wolf profile; one-time local publication verified
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

Meow Wolf’s Next.js page includes an application-specific event dataset. It is structured data, but not a standardized event schema or stable standalone API.

## Decision

Use an embedded application-data adapter with a versioned Meow Wolf extraction profile. Share safe script extraction with other HTML-based adapters without assuming a universal Next.js event contract.

## September 12 implementation profile

The operator capture uses an ordinary Chromium session. The supplied URL can
return the legacy `/events/denver/` calendar or redirect to `/denver/events/`.
The latter uses a different Next.js serialization. Both contain the same scoped
event record fields, but the former `__NEXT_DATA__`-only extraction is insufficient.
No alternate deployment host, privileged endpoint, credentials, or access-control
bypass is used. Browser requests stay on the venue's normal public routes.

The versioned reader supports legacy pageProps and JSON model records embedded in
`self.__next_f.push` payloads. It does not execute those strings, import modules,
resolve executable references, or implement a general React client. It skips
length-framed text by its hexadecimal UTF-8 byte count and ignores recognized
non-model records. Unnumbered stylesheet hints are recognized. The framing was
checked against [React's Flight client](https://github.com/facebook/react/blob/main/packages/react-client/src/ReactFlightClient.js).
Unknown framing or absent/ambiguous data fails the capture.

The new listing profile selects `eventsData` from the component props and the
Denver Events seller `017a7f54-e443-a261-3c55-46ef4d921efb`, timezone America/Denver.
It compares the embedded records with every rendered event-card link. The
inspected calendar renders all 50 supplied rows with client-side search and
category filtering; it exposes no additional page control. This proves coverage
of that public listing, not unpublished inventory or a full year of events.

Each event detail must match the template ID and timeslot ID in the listing's
composite ID. The new detail envelope contains `event`, `eventVenue`, and an
`associatedEvents` object. Only event identity, venue identity, admission, labeled
times, cancellation/ticket metadata, and timeslots are retained. Prices, capacity,
cart data, images, descriptions and unrelated metadata are not stored in the
capture artifact. Description text is not used to grant admission.

Two independent full captures must agree on selected fields. The snapshot uses
`from`, `through`, `total`, `events`, and `check`; limits are 500 records and 4 MiB.
HTTP failures, incomplete captures, schema changes and identity errors are not
successful empty results. Recognized redirects to the listing or general
exhibition can reject an individual record. No rejected record is silently
reclassified as an event at the exhibit.

`internal/meowwolf` supplies `replay-meowwolf` through the established publication
workflow. Only reviewed room IDs and matching room names map to Meow Wolf Denver:
The Perplexiplex (`14f4d967-6acc-4d09-2e5b-f2707768c20d`), Sips (with a Z)
(`d1bdc498-735e-781d-a390-f4c7f234c482`), and Convergence Station
(`017a7f54-ebc3-c5e1-1499-0887afa464fc`). The latter's detail venue object
confirms the Denver address; its event-level room label may be blank.
The timestamp converts to a Denver-local date and must match the explicit doors
clock and selected timeslot. If detail doors metadata is absent, the listing's
explicit date and `Doors @` clock must agree with the timestamp. A separately
labeled show clock is preserved.
Cancellation metadata maps to Cancelled; Sold Out does not. Cancelled events have
no ticket link. Scheduled events link to the event ticket page unless an explicit
safe HTTPS ticket URL is supplied. Synthetic JSON fixtures must never be published.

### Admission review

The [official event FAQ](https://faq.meowwolf.com/are-there-age-restrictions-for-shows-and-events)
requires tickets for every guest. At All Ages shows, guests younger than 16 need
an adult over 18. Event detail metadata additionally states a ticketed guardian
requirement. Ages 16–17 can attend All Ages shows subject to tickets and valid ID.
Restricted shows have no underage adult exception. The venue's general exhibition
ticket policy must not replace these event-specific requirements.

Use the explicit `eventAge` metadata, never a missing field or general listing
tag, to grant All Ages clearance. Unknown text remains visible without inferred
clearance. Configured overrides retain their existing precedence. The default
venue policy points to the FAQ, and reviewed All Ages ranges use the existing
admission-rule configuration. No prices or new public filters are introduced.

The paired capture and staged replay accepted all 50 records. Main publication
and desktop/phone checks passed; results are recorded in ADR 0019. Trick or
Treating in the Multi-Verse has no explicit detail admission metadata and keeps
unknown eligibility despite its listing tag. No recurring retrieval is enabled.

## Retrieval and mapping contract

Read the public listing and linked details using the versioned profile above.
The legacy listing path is `props.pageProps.events.events`; the newer format
uses embedded JSON model records. Map identity, title, labeled times, safe links,
admission and cancellation metadata. Do not ingest prices, images or descriptions.
Do not invent a performance start.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

On September 8, the [Denver page](https://tickets.meowwolf.com/events/denver/) embedded records including AdultiVerse on December 19, 2026. This verifies later-month access through page data, not a supported standalone API or complete event inventory.

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test the exact data path, absent or changed scripts, arrays, composite IDs, URL resolution, UTC-to-Denver date conversion, doors semantics, sold-out banners, and event categories. Treat a schema-path change as an explicit failure rather than silently returning zero events.

The importer, fixtures, capture tests and runtime checks now implement this profile.

## Alternatives and consequences

Rendered-card scraping would discard structured data already present. A build-ID-specific Next.js data route is not selected: standalone access was not verified and deployments change build IDs. The profile must be maintained if the app changes its data serialization.
