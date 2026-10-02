# ADR 0025: ReelWorks Denver events through the public WordPress listing

- Status: Implemented for ReelWorks
- Date: 2026-10-02
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md), [JSON-LD exploration](0007-event-json-ld.md), [locale isolation](../adr/0023-locale-isolation.md)

## Context

ReelWorks Denver is a WordPress site with mixed ticketing: an October 2, 2026
enumeration found AXS on 13 of 18 announced shows, Afton on two, and TicketSauce,
kyd.to, and Shotgun on one each. No single ticketing API enumerates the venue,
so the venue's own site is the aggregation point.

The WordPress core REST API is open (`robots.txt` allows everything) and exposes
the `event` custom post type at `/wp-json/wp/v2/event` (330 posts, 100 per page),
but the list response carries no event date: no `content` or `meta`, and ACF
fields are empty. Posts are ordered by publish date and interleave past and
upcoming shows, and the `past_event_archive` type holds a single post, so archiving
is not systematic. The REST list cannot enumerate upcoming events. Its embedded
Yoast `og_description` prose carries dates and ages in inconsistent formats.

Each event page instead serves a complete `MusicEvent` JSON-LD node, and the
listing page is the venue's own upcoming view: server-rendered, one dated block
per announced show, October 2, 2026 through February 6, 2027. The transport is
UA-sensitive (`Python-urllib` receives HTTP 406; browser and Node fetch do not).

## Decision

Enumerate through the listing, not the REST API. The operator capture reads the
listing and the venue FAQ, then every linked event page, twice, and stores the
raw listing and FAQ HTML plus, per event, the single schema-graph script and the
labeled sidebar blocks (Date, Time, and Age when present). `replay-reelworks`
normalizes and validates: Go remains authoritative for parsing and publication.

- Listing: exactly one date heading, one title heading, at most one support
  heading, and consistent event links per item; unique slugs; non-decreasing
  dates; the listing set must equal the captured event-page set; 1–200 items.
  Each item's subtree is parsed alone so page content after the last event
  cannot leak into its fields.
- Schema: exactly one `MusicEvent` node per page; `location.name` must be
  ReelWorks Denver; offline attendance mode; scheduled or cancelled status only.
- Times: `startDate`/`endDate` must carry the Denver offset in effect and agree
  with the sidebar Date and Time text and the listing date. The labeled Time
  window's start is the doors time; the venue's own prose calls it doors and
  no separate show time is published, so `show_at` stays absent.
- Ages: the sidebar Age text is the admission policy, per the venue FAQ that the
  promoter sets it per show. `16`, `18`, and `21` with or without `+`, and
  `All Ages`, map to the schema categories. `Under 16 w/ adult guardian` is the
  one reviewed parenthetical and publishes `with_adult` ranges 0–15; any other
  parenthetical fails the record. A missing or unparseable age fails the record.
- Tickets: the structured offer URL or the listing's buy button is the ticket
  link; when both exist they must agree. Hosts are reviewed per provider
  (AXS, Afton, TicketSauce, kyd.to, Shotgun); an unreviewed host fails the
  record. ReelWorks editors have been observed leaving leading spaces inside
  `href` and `offers.url` values; href whitespace is trimmed the way browsers
  resolve it.

The venue has no venue-level admission policy: ages are per-event, so the source
configuration carries none and the FAQ page anchors the per-event reading with
its `depends on the promoter` and `event page for the show` statements.

## Verification and operating limits

| Evidence | Observation | Supported finding | Limit |
| --- | --- | --- | --- |
| Live listing and all 18 event pages, twice | Every announced show parses: date, doors, age, performer, and ticket links cross-check across listing, sidebar, and JSON-LD with zero rejections | The full currently-announced calendar maps through capture and Go publication | One live capture, October 2, 2026; listing form can change |
| Focused Docker refresh | `package-snapshot` seed, live capture, `replay-reelworks` ingest, and `--snapshot` export published 18 events durably into the tracked catalog | The venue addition integrates with the existing publication and export contracts | Local machine, not a GitHub runner |
| `npm run test:reelworks`, `go test ./internal/reelworks`, full `go test -race ./...`, gofmt/vet, lint, Prettier | All passed | Extraction, normalization, cross-checks, and failure modes are regression-covered | Fixtures, not live venue responses |

GitHub runner access to reelworksdenver.com remains unverified until the next
deploy dry run; the WAF's datacenter behavior is unknown. No scheduling or
browser dependency is added to the app.
