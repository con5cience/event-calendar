# ADR 0022: Afton venue event listings

- Status: Implemented; local verification recorded in ADR 0019
- Date: 2026-09-12
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [registry](../adr/0013-source-adapter-registry.md), [verification](../adr/0019-implementation-and-verification-plan.md)

## Context and decision

The [Roxy calendar](https://www.theroxydenver.com/calendar) embeds an Afton widget
inside a Wix iframe. The public widget is scoped to Roxy, not an aggregator.
Use its anonymous paginated JSON listing, enriched with the linked Afton event
pages. AEG, VenuePilot and HoldMyTicket adapters do not fit this pagination or
record contract. Reuse the existing replay, artifact, retention and admission
interfaces; no schema or application change is needed. Do not scrape calendar
cells or depend on browser execution during capture.

The browser's public listing request established this endpoint:

```text
https://aftontickets.com/api/get-events?key=732e49e3d992ab7ff998f13c3a2d3f08&per_page=12&page=1
```

The key is the public venue-widget identifier, not a secret. Direct requests work
without cookies, authorization or the generated `aftclid` tracking parameter.
The Roxy and Afton robots files allow the inspected public paths. The embed host's
robots URL returns application HTML, not usable robots directives. This review
does not establish redistribution rights or approval for recurring ingestion.

## Retrieval and coverage

`tests/afton/capture.mjs` enumerates all pages twice and reads each Afton-hosted
event page in each pass. Each page must agree on total, page number, page size,
remaining length and next-page state. IDs must be valid and unique. Requests use
canonical URLs, never arbitrary returned next-page URLs. A capture has a
conservative 500-record guard, 1 MiB per response, 30-second request timeout,
redirect rejection and a 4 MiB snapshot bound. Unverified zero-event responses
fail safely rather than removing all prior events.

Listing output retains only reviewed fields and omits prices. Detail HTML is
compacted with the established RHP helper; JSON-LD and visible admission markup
remain intact. The helper now correctly strips executable scripts at the start of
a fragment, with a regression test. Go parses inert HTML and compares normalized
detail results across passes. Different templates, offer prices and unrelated
markup cannot establish a changed event. Material field disagreement fails the
whole refresh. No checkout, login, cart or purchase action runs.

The reviewed capture contains 26 upcoming listings on three pages through March
4, 2027. Six raw records were inspected before reporting counts. Twenty-one have
Afton details and five link to external ticket providers. The adapter filters the
available listing to the next twelve months; it does not claim twelve populated
months or backfill past pages. Normal retention preserves previously published
past records for 90 days after the event date.

## Mapping and identity

Native events use Afton's opaque event ID. External numeric IDs get an `external-`
prefix to prevent collisions. Preserve title, date and the exact reviewed ticket
link as both the event and ticket link. Only known Afton, Skeletix and Strong
Survive URL forms are accepted. External links are not fetched in this profile.

For native events, cross-check JSON-LD title, venue/address, link identity, start
instant and status. Offer URLs establish identity; the organizer's matching URL
is the verified fallback for free events with no offers. Prices are never mapped.
Sold-out inventory does not mean cancelled. Cancellation comes from event status.
One upstream listing remains one event, including multi-day passes; do not split
ticket tiers or infer extra performances. Separate upstream performances remain
separate records.

Convert the structured start instant to America/Denver and require agreement with
the listing's local clock. The source labels summer dates MST, but its structured
instant confirms Denver's seasonal offset. Respect time visibility flags. Doors
use the explicitly labeled local field; reject invalid, ambiguous, later-than-show
or different-date doors. External listings retain their dates but omit ambiguous
times, since inspected external details disagreed with the widget's clock meaning.

Malformed identified records retain their last valid version and are reported.
Missing identity, incomplete capture or changed paired results stop publication.
Successful absence unlists prior events through the existing reconciler, while
their public URLs retain the last version until expiry. Generation guards protect
unrelated sources during publication.

## Roxy admission review

The [venue rules](https://www.theroxydenver.com/rules) and
[box-office page](https://www.theroxydenver.com/box-office-tickets) do not establish
a blanket admission age or parent exception. Do not infer either from the rules
on underage drinking. The Roxy configuration therefore has no default age category
and no venue-wide adult-admission rule.

Read the explicit event restriction from either inspected Afton template:
`modal-event-info-label` with its associated value, or `.age-restriction` with the
Age Restriction label. Repeated values must agree. Exact All Ages labels qualify
ages 0–17 for the existing child filter with
`All ages; event ticket requirements apply`. This does not invent an accompaniment
exception. Recognized restricted categories retain their restriction; unknown
wording remains unclassified and receives no clearance. Cancelled events receive
no clearance. Configured event-policy overrides win.

Five externally ticketed listings remain age-unknown and do not match the child
filter. Their distinct Skeletix and HoldMyTicket enrichment paths are follow-up
work, not an inferred Roxy policy. Afton restrictions must not be copied onto them.

## Operation and verification

Build the ingestion image and run `replay-afton` with a real captured snapshot,
`internal/afton/testdata/roxy.yaml` and an empty staging store. Never publish the
synthetic HTML fixture. Subsequent refreshes use an operator configuration with
`state: established`. Validate staging before generation-guarded publication.

Run `npm run test:afton`, `npm run test:rhp`, the full `npm run test:contracts`,
host checks, and `tests/browser/afton.spec.ts` with `ROXY_BASE_URL`. These cover
pagination, identities, external unknowns, templates, clocks, admission, overrides,
failed refreshes, retention, app details, filtering and ICS. The app image and
Compose startup remain unchanged; ingestion is operator-triggered only.
