# ADR 0027: Public Google Calendar iCal feeds

- Status: Implemented for D3 Arts
- Date: 2026-10-03
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md), [iCalendar feeds](0006-icalendar-feeds.md), [locale isolation](../adr/0023-locale-isolation.md)

## Context

D3 Arts embeds its events calendar on its site as a public Google Calendar
(`mkjl322tg56ecg0mf7qjvtllcc@group.calendar.google.com`, ctz America/Denver).
Google publishes that calendar as an iCal feed at the standard public
`basic.ics` URL — the venue's whole event interface in RFC 5545 form. The
WordPress site exposes no event API and the calendar page is an iframe.

An October 3, 2026 read found 715 VEVENTs: 26 recurring-series masters
carrying RRULE (most long ended; eight still open-ended), 25
RECURRENCE-ID overrides, and one-off shows through October 2027. The
recurring series are community meetings and classes — 12-step, AA, and
recovery meetings, kiki-house and self-defense classes, skate sessions —
and the operator decision is to exclude them all. Google repeats one UID
across a series' master and its overrides, some UIDs are UUIDs from an
imported calendar, and one carries Google's `_R<datetime>` rewrite suffix.
EXDATE legitimately repeats (one series carries 31 exclusion dates).

## Decision

Capture the feed once and store the raw iCal text; Google re-stamps every
event per fetch, so paired reads would never match. `replay-google-ics`
parses and validates in Go, which stays authoritative.

- Envelope: one `BEGIN:VCALENDAR`, balanced VEVENT counts, `X-WR-TIMEZONE`
  equal to the venue timezone, `X-WR-CALNAME: D3 Arts Events Calendar`,
  `VERSION: 2.0`. Repeated properties fail the capture except RFC 5545
  multi-instance EXDATE and RDATE.
- Identity: Google UIDs (`@google.com`, including the `_R` rewrite suffix)
  and imported UUIDs are both accepted. Uniqueness applies to one-offs only,
  because a recurring series repeats its UID across the master and every
  override.
- Recurring series: the reviewed operator decision, recorded as
  `adapter_options.recurring: exclude`, drops them by marker, not by title.
  A master is live without `UNTIL` or with `UNTIL` at or after the window
  start; an override is live when its `RECURRENCE-ID` instance date is
  inside the window. Live series reject visibly as `recurring series
  excluded` — masters keep the series UID, overrides carry the instance
  digits on theirs, and the records stay undated so the window filter keeps
  them in the report. Ended series and past overrides drop entirely.
- Dates and times: Google emits UTC (`Z`), TZID-qualified, floating, and
  `VALUE=DATE` forms. Timed events convert to Denver with DST-ambiguity
  checks; all-day events publish date-only. The description's own labeled
  times verify the stored start: `Doors @ 7PM`-style text (with or without
  a meridiem; a bare hour is an evening clock) must equal the DTSTART when
  it is the only label, or DTSTART must equal the labeled doors or show
  clock when both appear, and doors never follow the show. Band set lists,
  door charges, and donations are deliberately left unparsed.
- Ages: per-event text only (`ALL AGES`, bare `18+`), mapped to the schema
  categories with conflict rejection. The venue-level policy carries the
  venue's own statement — "All Ages Welcome: 90% of our shows are open to
  all ages" — recorded from its 3632 page.
- Event URL: the venue's calendar page, the only public page the venue
  gives each event. No ticket URLs are parsed; the venue is donation-based.

## Verification and operating limits

| Evidence | Observation | Supported finding | Limit |
| --- | --- | --- | --- |
| Live feed read, all 715 VEVENTs | 51 one-offs published with dates, windows, labeled doors/show times, and venue ages; 9 live series and one in-window override excluded visibly; one untitled event rejected; ended series and past noise dropped | The full currently-announced calendar maps through capture and Go publication | One live capture, October 3, 2026; Google can change the feed shape |
| Focused Docker refresh | Seed, live capture, `replay-google-ics` ingest, and `--snapshot` export published durably into the tracked catalog | The venue integrates with the publication and export contracts | Local machine, not a GitHub runner |
| `npm run test:d3`, `go test ./internal/googleics`, full `go test -race ./...`, gofmt/vet, lint, Prettier | All passed | Envelope, identity shapes, recurring liveness, labeled-time verification, and failure modes are regression-covered | Fixtures, not live responses |

Holds the venue lists on its public calendar ("165 Hold", "BOTL hold") are
published as one-offs: they are the venue's own public listings, and no
policy decision excludes them. GitHub-runner access to the Google feed is
not expected to differ from local access.
