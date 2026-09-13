# ADR 0016: Go ingestion jobs and source reconciliation

- Status: Reconciliation, local publication, and fixture-only AEG replay implemented; live fetching and batch job execution proposed.
- Date: 2026-09-08
- Scope: Implementation design; no executable behavior is implemented by this ADR.
- Authority: [Product contract](0014-product-and-delivery-contract.md).
- Related: [Adapter registry](0013-source-adapter-registry.md), [artifact contract](0017-artifact-contract-and-publication.md), [deployment](0018-storage-and-container-deployment.md)

## Context

Sources have different retrieval formats but share history, identity, override, validation, and publication rules. The user wants one independently built ingestion image with isolated source processes and commands to run one source or all sources.

## Decision

Use Go for ingestion and the generic application host, with shared packages for artifact types, validation, time interpretation, retention, and ICS encoding.

Use one ingestion executable/image with a batch coordinator and per-source child processes. Each child runs a source workflow; the coordinator collects outcomes and publishes the catalog. Process isolation must not be replaced silently by goroutines that share failure state.

Start with Gothic and Mission through the shared AEG adapter, followed by HQ through the iCalendar adapter.

The fixture-first pilot adds `internal/aeg` and the single-source `ingest replay-aeg`
command. It reads a local snapshot, captures the current catalog generation and prior
source artifact, normalizes, reconciles, validates, and publishes through `internal/store`.
A competing generation causes publication to fail; rerun from the new prior state.
No source-specific logic is added to the application. No network client, batch
coordinator, or schedule is enabled. Normalized observations can carry an adapter
failure reason so malformed known updates retain their last valid record.

## Package boundaries

The following are responsibilities. Reconciliation, validation, and artifact types
exist in `internal/artifact`; local generation reading/publication exists in
`internal/store`, exposed by `cmd/ingest publish`. Adapters and job coordination remain proposed:

- Adapter: provider-specific HTTP requests, pagination, raw identity extraction, and record parsing.
- Fetch helpers: timeouts, bounded responses, redirect policy, request errors, retry rules, and rate limiting.
- Reconciliation: prior records, stable URLs, user overrides, absent versus invalid records, and history.
- Validation: record checks followed by validation of the complete candidate artifact.
- Storage: read prior artifacts and stage validated new artifacts.
- Coordinator: child-process lifecycle, per-source outcomes, and catalog publication.
- Shared domain: types and source-independent product rules used by ingestion and the app host.

Do not create another importer per venue when an established adapter contract fits. Source configuration selects the adapter and provider-specific settings.

## Evidence and implementation candidates

| Evidence source | Observation | Supported finding | Material limit |
|---|---|---|---|
| [Go HTTP test support](https://pkg.go.dev/net/http/httptest), [Go fuzzing](https://go.dev/doc/security/fuzz/) | Controlled HTTP servers and native fuzz tests are available. | Provider parsing and failure paths can be tested without live websites. | No project tests have been created or run. |
| [Go HTML parser](https://pkg.go.dev/golang.org/x/net/html) | Provides HTML tokenization and parsing. | Reuse a parser instead of regex-only document extraction. | Source-specific selectors still require fixtures. |
| [go-ical](https://pkg.go.dev/github.com/emersion/go-ical) | Exposes decoder and encoder APIs. | Candidate for HQ ingestion and event export. | Floating times, malformed feeds, and export compatibility require tests. |
| [Go timezone data](https://pkg.go.dev/time/tzdata) | An embedded timezone database is available. | Minimal images need not depend on accidentally installed zone files. | Timezone behavior must be tested in the final image. |

TypeScript ingestion was considered because it could share a language with React. Go is selected in this proposal for the user's preferred execution model and shared ingestion/serving packages. This is not a measured performance advantage over TypeScript.

## Source lifecycle

1. Read validated source configuration, including its explicit new/established state.
2. Read its prior artifact when established. Unreadable prior state fails the job rather than starting fresh.
3. Request the next twelve calendar months using the adapter's verified enumeration contract.
4. Extract stable raw identities before optional field normalization wherever possible.
5. Apply configured overrides, excluding venue and date changes.
6. Validate records and reconcile them with the previous publication.
7. Validate the resulting artifact and stage it.
8. Return a structured job outcome and exit.

The coordinator can publish successful sources while keeping failed sources' previous references. Batch failure reporting must identify partial success; a nonzero exit is not a claim that no files were published.

An empty successful listing is different from a request, parsing, or pagination failure. An HTTP 200 alone does not establish a successful refresh.

## Reconciliation rules

- A valid new record creates an event with an assigned public path.
- A known valid update keeps the assigned path when only title, time, or optional metadata changes.
- A changed venue or date creates another event; do not add show-lifecycle tracking.
- A new invalid record is rejected and reported.
- A known present-but-invalid record retains its last valid version.
- An absent future record in a successful refresh leaves the active calendar but retains details until expiration.
- Retained past records are not removed just because the fetched response is upcoming-only.
- Records expire ninety days after their event date; exact calendar-day boundary behavior must be tested.
- A failed source preserves its publication. Read-time expiry remains enforced by the application.

Keep enough raw identity to distinguish an invalid update from disappearance. If a record cannot be identified and this makes safe absence reconciliation impossible, fail reconciliation rather than silently deleting potentially matching records.

The requested twelve-month window is not proof that the source actually publishes that far ahead. Detect known caps and failed pages before permitting absence-based removal. Do not restore a visitor-facing staleness policy.

## Identity and overrides

Use provider-scoped stable identifiers where available, together with the agreed occurrence date/venue rule. Retain public paths from prior records. Provider IDs alone must not collapse different occurrences after a date/venue move.

Do not include event time in the public path. The short suffix is assigned once, checked for collision, and preserved. Matching when a provider offers no stable ID requires explicit fixtures; a hash of mutable title/time is not automatically a stable identity.

Overrides always win until removed. Apply only allowed, schema-valid fields. Validation must reject forbidden venue/date overrides. Never use an override to expose credentials, scripts, or invalid URLs.

## Implemented reconciliation boundary

`internal/artifact.Reconciler` is a pure candidate builder, not a publisher. It accepts
normalized observations with a stable `upstream_id` outside the possibly invalid
event body. Identity matches source, provider ID, date, and effective venue key.
Absent or ambiguous identity fails the refresh when safe reconciliation is impossible.
An identifiable invalid update retains its prior record; an invalid venue/date move
does not count as the old occurrence being present.

New IDs use `<source>-<12 lowercase hex characters>` from six random bytes. Paths use
`/events/<venue>/<date>-<title-slug>-<same suffix>`. Assignment checks collisions against
prior and newly assigned identities. Title/time corrections keep the original path.
Suffix generation is injectable for deterministic tests; repeated collisions fail.

Coverage is inclusive and must start on the supplied current civil date. Only a complete
refresh permits absence reconciliation inside that coverage. Earlier history and records
outside coverage remain unchanged unless expired. `expires_on` is event date plus 90
calendar days; a supplied date on or after it expires the record. The future coordinator
must supply the correct lifecycle date. The local web reader now enforces read-time
expiry using the server clock, including during failed refreshes; see ADR 0017 for
timezone handling. File cleanup and shared-route HTTP 404 integration remain deferred.

The library checks explicit new/established state, prior artifact validity and source
ownership before reconciliation. Failure returns no candidate. Successful results contain
validated events and rejection reasons without mutating inputs. Configured overrides run
before record validation and persist until configuration changes.

## Duplicate handling boundary

Source processes produce their own normalized observations; they do not fetch or mutate other sources' artifacts to reconcile duplicates.

The application's generic artifact reader chooses a display winner across confidently matching observations, with the actual venue's own listing preferred. Keep all retained original routes resolvable; suppressing a duplicate card must not break a previously published link.

Use shared provider/ticket identifiers first, then the conservative venue/date/title/time evidence specified in ADR 0014. Do not remove identifying ticket URL parameters while normalizing tracking parameters. Conflicting venue/date values must not be merged solely because a ticket URL matches.

Do not combine fields across observations unless that additional rule is approved. Fallback ordering when neither source is the venue's own listing remains open.

## Pilot data checks

The preceding research inspected five records per pilot source. These observations are not new live checks during ADR authoring.

| Evidence source | Raw observation | Supported finding | Material limit |
|---|---|---|---|
| [Gothic AEG feed](https://aegwebprod.blob.core.windows.net/json/events/37/events.json) | POND supplied doors at 19:00, show at 20:00, America/Denver, and 16 & Over. | Separate times and age categories are available. | Requires tested field mapping. |
| [Mission AEG feed](https://aegwebprod.blob.core.windows.net/json/events/89/events.json) | Samples supplied eventId and separate doors/show fields. Both AEG pilot feeds paired null ticketPrice with $0 low/high values. | IDs support reconciliation; zero-price semantics require corroboration. | Do not label these records free solely from those low/high fields. |
| [HQ ICS](https://holdmyticket.com/ics/6457) | Samples supplied UID, floating DTSTART, title, event link, and empty DESCRIPTION; calendar declared Denver timezone. | Core records can be ingested with a provider timezone policy. | Sampled records did not provide price/age data or distinguish doors from show time. |

Unknown price remains unknown. A ticket URL is not a verified venue-homepage detail URL. Source-provided detail enrichment may be needed to satisfy the separate links requirement.

## Verification and consequences

Use fixture-first tests, controlled HTTP servers, and fixed clocks. Check process isolation, cancellation, bounded retries, partial pagination, prior-state failures, overrides, and every reconciliation transition. No live source is required for the ordinary unit test suite.

Capture representative public fixtures with provenance when implementation is approved. Pin and verify candidate dependencies then. Live access/reuse review remains required before scheduled production ingestion.

This design shares lifecycle behavior without hiding provider differences. It requires durable artifacts but not a database, queue, or Redis.
