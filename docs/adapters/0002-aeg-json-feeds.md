# ADR 0002: AEG JSON feeds

Operational profiles now live in `locales/<id>/sources/` under
[ADR 0023](../adr/0023-locale-isolation.md). `adapter_options.feed_id` and
`venue_id`, venue website and timezone replace the compiled Denver feed registry.
Returned venue identities and timestamps must still match the selected profile.

- Status: Five AEG venues supported and imported locally; scheduled fetching not implemented.
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

September 9 implementation update: the supported feed/venue bindings now include
Bluebird `2`/`100811`, Ogden `7`/`101141`, and Fiddler's Green `44`/`100869`.
Synthetic mapping tests cover all five venues, including rejecting foreign venue
IDs. Live snapshots for the three additions normalized without rejections and
were published locally after staging. No parsing or completeness safeguards were
relaxed. See the [expansion verification](../adr/0019-implementation-and-verification-plan.md#aeg-source-expansion--2026-09-09).

AEG venue calendars expose JSON blobs on a shared host. Gothic, Mission Ballroom, Bluebird, Ogden, and Fiddler’s Green use venue-specific feed identifiers.

## Decision

Use one AEG adapter configured by feed URL and venue identity. Treat each response as a published snapshot, not an arbitrary date-range API.

## Retrieval and mapping contract

Fetch the configured JSON URL and validate the observed envelope before reading event records. Preserve provider identifiers, event and ticket URLs, status, venue fields, and supplied time fields. Interpret each time field from fixtures rather than its name alone. Do not assume a blob is a complete database export.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source                                                                             | Raw observation                                                                   | Supported finding                                            | Material limit                                                                            |
| ------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ------------------------------------------------------------ | ----------------------------------------------------------------------------------------- |
| [Gothic feed](https://aegwebprod.blob.core.windows.net/json/events/37/events.json)          | Exploration returned an Airborne event dated June 8, 2027.                        | One response can include later months.                       | Publication horizon is not guaranteed.                                                    |
| [Fiddler’s Green feed](https://aegwebprod.blob.core.windows.net/json/events/44/events.json) | All four returned records were inspected, including Stick Figure on June 5, 2027. | Same feed family can support another venue by configuration. | Prior AEG responses advertised a 100-row setting; behavior beyond a cap was not verified. |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test each venue configuration, field availability, time interpretation, malformed envelopes, and responses at any advertised cap. Mark completeness unknown until cap and enumeration behavior are established.

The fixture-only implementation and verification boundary are described below.

## Fixture-only pilot — September 9, 2026

`internal/aeg.Decode` accepts local JSON bytes and an explicit clock. Source configs
and synthetic snapshots live in `internal/aeg/testdata`. The two configurations bind
feed 37 to venue 2274 (Gothic), and feed 89 to venue 127278 (Mission). IDs, titles,
ticket URLs, and cancellations in those fixtures are synthetic. They are not live
event artifacts and must not be used as public listings.

The adapter rejects missing/malformed metadata, duplicate JSON keys, duplicate event
IDs, unexpected venue identity/timezone, unsupported publication states, additional
dates, and responses at the advertised row cap. It requires page 1, an explicit event
array, a matching total, and a total below the cap. An explicit zero-total empty array
is a successful empty snapshot. These are conservative snapshot checks, not proof
that the provider has announced every event during the next twelve months.

Coverage runs from the supplied clock's Denver civil date through its one-year
anniversary, inclusive. Valid observations outside that range are omitted. The
existing reconciler retains past events for ninety days and handles future absence.
Invalid observations are passed to reconciliation for rejection/last-valid retention;
they are not silently filtered as missing. Envelope or scope failures publish nothing.

Mapping:

- Use `eventId` as the provider identity. Use `eventTitleText`, falling back to
  `headlinersText`. Keep the latter as one performer display string; do not split
  comma-containing artist names or parse HTML titles/bios.
- Use `eventDateTimeISO` for show time/date. Cross-check supplied local/UTC show
  fields and the venue timezone. Normalize `doorDateTimeUTC` into the venue timezone
  and cross-check a supplied `doorDateTime`. Local-only doors time is rejected rather
  than guessing during a DST overlap. Separate doors/show times remain separate.
- Keep `age` as policy text. Map only exact, clear `All Ages` and 13/16/18/21
  `& Over` values into filter categories. Other text has no inferred category.
- Ignore `ticketPrice` and all numeric price fields. Price metadata is retired across
  all venues; raw captures remain unchanged.
- Build the venue detail route from the configured venue origin and
  `/events/detail?event_id=<id>`, as found in the venue's shared calendar script.
  Use `ticketing.eventUrl` for the ticket link, matching the calendars' configuration.
- Preserve `ticketing.status` as literal optional status. Do not hide a cancelled
  event or interpret the status as a reconciliation instruction.

No network client or scheduled job exists. `ingest replay-aeg` reads local fixtures
and publishes into an explicitly selected local test store. Its JSON report includes
publication/durability results and rejected records. See the README's replay workflow.

Bluebird's configuration also contains manually reviewed `admission_rules` from
its [official FAQ](https://www.bluebirdtheater.net/frequently-asked-questions/),
reviewed September 9, 2026. Shared ingestion enrichment selects a rule using the
normalized exact category; it does not change the provider's age text. Unknown
restrictions (including 13+ without a reviewed rule) remain unknown. Explicit event
metadata and configured policy overrides win. Off-site records do not inherit these
rules. Gothic, Mission, and Ogden now have equivalent ranges verified against
their own official policy tables. Fiddler's Green has an All ages rule only, with
the general ticket requirement from age 2. Its restricted and missing categories
have no inferred exception. All five configurations record their own policy URL
and September 9, 2026 review date. See ADR 0017 for the
range schema and the README for non-refreshing `enrich-admission` publication.

## Access and live-validation gate

The inspected calendar pages identify these blobs as their data source. A repeat
five-record Gothic sample confirmed provider IDs, Denver local/UTC/ISO times, venue
identity, plain-text titles, and null price text with `$0` low/high placeholders.
The Gothic detail route returned its event-detail component and the expected feed.
This does not establish live Mission detail rendering or downstream ticket availability.

The venue footer links to `https://www.aegpresents.com/terms/`, which redirects through
HTML to `https://aegworldwide.com/terms-of-use`. That page loads a OneTrust terms JSON
document; the referenced document returned HTTP 404 during investigation. The
`robots.txt` routes returned site HTML, not usable robots directives. Access review
is therefore incomplete; public availability is not permission to reuse data.
Resolve access conditions and validate representative live snapshots, detail pages,
and completeness before adding any fetching command or enabling schedules.

### One-time local import clarification — September 9, 2026

The user approved a one-time local-development import despite the unavailable terms
document. A broken terms link does not establish an access prohibition. The unresolved
review remains a public-deployment/recurring-ingestion consideration, not a technical
blocker for this approved exercise. This is not a finding of redistribution permission.

Both public blobs were downloaded once with a 35-second timeout and 4 MiB limit.
The existing local replay command accepted the real snapshots with no rejected rows:
57 Gothic events and 61 Mission events. Both totals matched feed metadata and were
below the advertised cap. Mission's event-detail route resolved through its normal
redirect to the detail component and feed 89. Two browser sizes displayed real cards
and POND's metadata/ticket link from the main app. No network client or schedule was
added. See the [verification record](../adr/0019-implementation-and-verification-plan.md#one-time-live-local-import--2026-09-09).

## Alternatives and consequences

Separate venue scrapers would duplicate a shared contract. A generic JSON decoder is useful infrastructure but cannot supply AEG semantics. This adapter reduces duplication while retaining a dependency on an undocumented website feed.
