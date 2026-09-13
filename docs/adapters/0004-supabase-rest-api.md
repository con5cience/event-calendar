# ADR 0004: Public Supabase REST event data

- Status: Implemented and published locally for Black Box
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

Black Box’s frontend uses a Supabase REST events table. Access was verified using the anonymous client credential published by that frontend.

## Decision

Use a configured Supabase/PostgREST adapter restricted to the intended public event query. Do not infer authorization to read other tables or use privileged keys.

## Retrieval and mapping contract

The observed endpoint is https://yotaohjqtxebyhlzezjl.supabase.co/rest/v1/events. The exploratory query used the frontend’s public anonymous credential with apikey and Authorization headers and selected published Black Box records. Fields include id, slug, title, date, time, end_date, end_time, venue, location, ticket_url, and status. Record exact filters and ordering in implementation configuration; use an explicit venue mapping rather than relying only on a broad text match.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

On September 8, the [Black Box frontend](https://blackboxdenver.co/events) and its public REST response were inspected. A published anniversary event dated November 21, 2026 was returned. This supports retrieval beyond the current month, but not completeness or unrestricted database access.

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test exact venue scope, published-status filtering, deterministic ordering, page/range limits, credential rotation, time composition, and permission errors. Verify pagination against observed server behavior before claiming enumeration completion.

## Black Box implementation — September 11, 2026

The operator command `node tests/blackbox/capture.mjs` discovers the anonymous
client credential from the normal public events request. The credential remains
in memory and is not written to artifacts, reports, or documentation. Only the
reviewed events endpoint is queried; no joined or unrelated tables are requested.
The app has no new database or browser dependency.

Select only `id,slug,title,date,time,end_date,end_time,venue,location,address,ticket_url,status,summary_html`.
Filter `status=eq.published`, the exact room names The Black Box and The Lounge,
and dates from the current Denver date through twelve months ahead. Order by
`date.asc,id.asc`. Use 25-row pages with `Prefer: count=exact`; verify each
Content-Range and a stable total, stopping when that total is satisfied.
A request past the total returned HTTP 416, so an extra empty page is not
required. Cap at 500 records and 4 MiB snapshots. Fail incomplete, changed,
duplicate, oversized, or unsuccessful captures.

The public page's observed date filter used the UTC day while Denver was still
on the previous date. Our query uses the configured venue day instead. Both room
names map to The Black Box only at 314 East 13th Avenue, Denver, 80203.
Distinct upstream event UUIDs stay distinct; room membership does not merge events.

Fetch each event's own ticket URL with HTTP redirects disabled. Parse its HTML
in an inert DOM without executing it. Extract only
the labeled Age Restriction section and ticket title. A 21+ Basscouch offer is
not the general event restriction. Repeat the selected event query and admission
enrichment; require identical normalized capture evidence. Prices and changing
counters are excluded from the query, not removed after comparison.

`replay-blackbox` reuses configuration validation, reconciliation, override
precedence, retention, and generation-guarded publication. It parses labeled Doors
and Music times from summary HTML into America/Denver. The listing time is not
assumed to mean doors: sampled Lounge listings say 9:01 PM while the explicit
doors and music labels say 9:00 PM. Missing labels stay omitted; conflicting
labels and music-before-doors reject. Raw HTML is never rendered in the app.

## Required admission review

The [official About page](https://blackboxdenver.co/about), inspected September 11
in a browser, describes the dual-room venue as 18+. Sampled official ticket terms
for [5AM](https://events.blackboxdenver.co/e/5am-sep11/tickets) and
[Girls Luv Groove](https://events.blackboxdenver.co/e/lakris-sep11/tickets) also
state 18+. No parent, guardian, or older-friend waiver was found. Missing or
unrecognized event terms remain unknown rather than receiving inferred clearance.
Explicit All Ages terms can support clearance, but the venue's 18+ default does
not. Configured event overrides remain authoritative.

Tests cover count/range validation, duplicate identity, changing captures,
room/address scope, explicit times, VIP-versus-event restrictions, missing terms,
overrides, CLI serialization, retention, API, and calendar export.
Run `npm run test:blackbox`, `npm run test:contracts`, and
`tests/browser/blackbox.spec.ts` with `BLACKBOX_BASE_URL` set to the chosen app.
The initial capture stopped on Night / Shift's ticket link, which redirects to
Dice and returned 403 there. A separate request with redirects disabled confirmed
the venue's HTTP 302 to Dice. A browser-navigation guard did not produce a valid
capture and was replaced with redirect-disabled HTTP retrieval. Capture marks
external redirects as an unsupported provider without requesting their destination. It does not solve or
bypass access checks. Other HTTP failures still stop the complete capture.
The successful capture enumerated 44 records twice and published 43 through
November 21, 2026. All published event terms are 18+; none receive age-14 clearance.
Night / Shift, UUID `6a8f4f60-7b4c-4e02-b892-34aa0a1e62ff`, is rejected as an
unsupported ticket provider. See ADR 0019 for runtime verification.

## Alternatives and consequences

Rendered-page scraping would duplicate data already available to the client. A generic JSON snapshot adapter would miss PostgREST query and authorization semantics. This approach depends on the site’s public access policy and may stop working if that policy changes. No credential value belongs in this ADR.
