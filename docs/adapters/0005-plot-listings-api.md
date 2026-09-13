# ADR 0005: Plot listings API

- Status: Implemented for Hi-Dive; operator-triggered capture
- Date: 2026-09-08
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [source registry](../adr/0013-source-adapter-registry.md)

## Context

Hi-Dive exposes a Plot listings endpoint with explicit page parameters. It is a distinct contract from the RHP and Clique plugins.

## Decision

Use a Plot adapter with page-based enumeration and provider-specific time interpretation.

## Retrieval and mapping contract

The verified endpoint is https://hi-dive.com/api/plot/v1/listings?currentpage=1&listingsPerPage=5. Enumerate the returned arrays using each record's `maxPages`. Use the numeric Plot/WordPress `id` as the source-scoped upstream identity: DICE IDs can be absent or false, including on valid unticketed listings. Preserve unnormalized responses in the operator capture for audit. Do not equate `startTime` with performance time without corroboration. Ignore all price fields under the current product contract.

All output, provenance, coverage, reconciliation, and access-review requirements in ADR 0001 apply. An unknown or malformed response must not be reported as a successful empty calendar.

## Inspected evidence

| Evidence source                                                                               | Raw observation                                                                                              | Supported finding                     | Material limit                                               |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ | ------------------------------------- | ------------------------------------------------------------ |
| [Listings endpoint](https://hi-dive.com/api/plot/v1/listings?currentpage=1&listingsPerPage=5) | Page 2 and the reported final page 7 were retrieved; the final page included Holy Wave on November 20, 2026. | Page traversal reaches later months.  | Results can change during traversal.                         |
| [Hi-Dive calendar](https://hi-dive.com/events/) and inspected records                         | Descriptions sometimes distinguished later show times; an overnight end-day value was inconsistent.          | Time fields need semantic validation. | Calendar fields alone may not determine the actual duration. |

These observations come from the September 8, 2026 exploration, not a repeat verification during ADR authoring. They establish retrieval feasibility only.

## Required implementation verification

Test page advancement and final-page handling, duplicate IDs across pages, changing pagination metadata, YYYYMMDD parsing, price omission, doors versus show time, and overnight events. A blocked API root must not be treated as proof that the specific working route is unavailable.

The September 11 implementation below supersedes the original feasibility-only status.

## Hi-Dive implementation and admission review — September 11, 2026

`tests/plot/capture.mjs` makes sequential unauthenticated GET requests, with a
30-second timeout, no automatic retries, no redirects, five records per page,
200-page safety cap, and 4 MiB response/snapshot caps. It fetches every reported
page, requires an empty page after the last page, and repeats page one to detect
movement. Duplicate identities, short intermediate pages, changing metadata,
HTTP errors, and inconsistent repeated pages abort the capture. A successful
empty result also requires repeated empty responses. Snapshots and original
responses remain in a new operator-owned temporary directory, not public output.

`internal/plot` revalidates enumeration before marking coverage complete, then
normalizes records for today through the same date next year in America/Denver.
Completeness means all currently exposed listings were enumerated, not that the
venue has announced a year's schedule. Changes confined to middle pages without
changing IDs/page counts or page one can escape the consistency check; the API
does not provide a versioned snapshot. Subsequent captures refresh those records.

Use explicit `doors`, not `startTime`, as doors time. Ignore unreliable end-day
and end-time fields; no inferred show time or duration. Map title, optional lineup,
own-site event URL, optional ticket URL, and explicit cancellation wording.
Descriptions are only inspected for the provider's `This is a 21+ event`-style
restriction statement; truncated/absent statements leave the restriction unknown.
Do not publish descriptions, images, or prices. Non-null venue fields and multi-day
records require review and are rejected rather than assigned to Hi-Dive by guess.
The observed capture contained neither case.

### Required venue admission review

The [official FAQ](https://hi-dive.com/faqs/) states that most shows are 21+, some
are 18+, and underage patrons can attend with a parent/legal guardian. This is
not permission to attend with any older friend. The configuration records ages
0–17, the app's supported child range, and the condition `Parent or legal guardian
required`. It preserves known 18+/21+ event restrictions and adds that clearance
through `admission_rules`. When no restriction is available, the reviewed venue
policy remains the effective fallback; it does not invent a 21+ category.
Configured event overrides continue to win and can remove the exception. No
event-specific prohibition was found in the captured restriction statements;
full detail pages are not exhaustively enriched, so new exceptions need operator
review and a configured override. The FAQ also requires valid ID and rejects
vertical IDs; the policy link exposes that detail rather than modeling ID types.

| Evidence source                                            | Raw observation                                                                                                | Supported finding                                                 | Material limit                                                                     |
| ---------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| September 11 capture, eight sampled raw records            | Numeric IDs, YYYYMMDD dates, explicit 7pm/8pm doors, own-site URLs; some ticket links and descriptions absent. | Required event mapping and optional-field handling are supported. | Descriptions can truncate the age statement.                                       |
| All captured pages and terminal probe                      | Page lengths 5,5,5,5,5,5,3; page eight empty; repeated first page unchanged.                                   | 33 available records enumerated, September 11–November 20.        | No evidence of later announced events in this capture.                             |
| Official FAQ and `hi-dive.yaml`                            | Explicit parent/legal-guardian exception, reviewed September 11.                                               | With Adult clearance is supported even for known 21+ listings.    | Not an older-friend exception; unknown restrictions remain unknown.                |
| [robots.txt](https://hi-dive.com/robots.txt), September 11 | Disallows wp-admin and wpforms uploads; no listings-route exclusion.                                           | No robots conflict identified for this route.                     | Not a grant of reuse rights; no access controls bypassed or legal conclusion made. |

Run `npm run test:plot` for capture tests and `npm run test:contracts` for decoder,
override, retained-record, publication, API, and calendar-export checks. Run
`PLOT_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/plot.spec.ts`
against a staged preview; this checks known/unknown restrictions, ages 0/14/17,
policy and ticket links, detail URLs, and ICS through the real app.

Use `replay-plot --store DIR --config YAML --snapshot JSON --now RFC3339` for local
replay. The supplied YAML declares `state: new`; use an operator copy with
`state: established` when replaying into a store that already contains Hi-Dive.
Stage first, then use generation-guarded `publish` to promote the source. Capture
does not publish, schedule jobs, or run during app startup. Synthetic fixtures
must never be promoted to the main catalog.

## Alternatives and consequences

A page scraper is unnecessary for core records while this endpoint works. Generic WordPress handling would not capture Plot pagination and time issues. Detail enrichment may remain necessary.
