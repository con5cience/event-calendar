# ADR 0026: SpotHopper event pages

- Status: Implemented for Black Sky Brewery
- Date: 2026-10-03
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md), [locale isolation](../adr/0023-locale-isolation.md)

## Context

Black Sky Brewery runs its site on SpotHopper (static.spotapps.co assets). The
events page is the venue's whole public interface: every announced event is a
server-rendered section with the platform's own event id, a recurring-series
marker, a weekday-and-month date without a year, a time window, and a free-text
description carrying doors/show times, ages, and HoldMyTicket ticket links. No
JSON API is exposed by the page; robots.txt sets no crawl rules beyond a
sitemap. An October 3, 2026 enumeration found 16 events: 12 one-offs through
January 16, 2027, and 4 recurring series.

## Decision

Capture the page twice and store the raw HTML; `replay-spothopper` parses and
validates in Go, which stays authoritative.

- Sections: exactly one title, date, time window, and info block per section;
  the platform's `data-event-id` must match the section id; ids unique; 1–200
  events; dates non-decreasing.
- Dates: the page prints weekday and month without a year. The year starts at
  the capture year and rolls over when the month goes backward; the printed
  weekday cross-checks the inferred year, so a wrong inference fails loudly.
- Times: the platform's window is the venue's own published event window and
  a listed show starts with it. `Doors open at` publishes doors and
  `Show`/`Bands start at` overrides the show time, both validated against the
  window with Denver DST-ambiguity checks. Door charges (`Door Charge of $X
  starts at`) and band set lists are deliberately left unparsed.
- Ages: per-event text only — `All Ages` and `Ages 18+` map to the schema
  categories; conflicts or unknown labels fail the record. No venue-level
  policy exists: the site publishes no minors statement, so unlabeled events
  publish without a policy rather than an inferred one.
- Tickets: HoldMyTicket links (`holdmyticket.com/tickets/<id>` and
  `tickets.holdmyticket.com/tickets/<id>`) from the description text; distinct
  conflicting links fail the record. Prices are retired metadata and are not
  parsed.
- Recurring series: the reviewed operator decision for this venue is to
  exclude them, recorded as `adapter_options.recurring: exclude`. The four
  series the platform marks `data-is-recurring` on October 3 — Monday Night
  Magic, Arts & Drafts, Heavy Metal Karaoke, and First Friday — are excluded
  as visible rejected observations, so every refresh report shows the
  exclusion instead of hiding it. Weekly instances carry new event ids, so the
  marker, not a title list, keeps the filter correct.

## Verification and operating limits

| Evidence | Observation | Supported finding | Limit |
| --- | --- | --- | --- |
| Live page read twice, all 16 sections | 12 one-offs publish with dates, windows, doors/show, ages, and ticket links; the 4 marked series reject with `recurring series excluded` | The full currently-announced calendar maps through capture and Go publication | One live capture, October 3, 2026; SpotHopper can change the markup |
| Focused Docker refresh | Seed, live capture, `replay-spothopper` ingest, and `--snapshot` export published durably into the tracked catalog | The venue integrates with the publication and export contracts | Local machine, not a GitHub runner |
| `npm run test:blacksky`, `go test ./internal/spothopper`, full `go test -race ./...`, gofmt/vet, lint, Prettier | All passed | Extraction, year rollover, weekday cross-checks, midnight windows, and failure modes are regression-covered | Fixtures, not live responses |
