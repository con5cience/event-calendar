# ADR 0017: Versioned artifacts and catalog publication

- Status: Version-one contract, local reader, and local publication command implemented; object-store publication proposed.
- Date: 2026-09-08
- Scope: Implementation design; no executable behavior is implemented by this ADR.
- Authority: [Product contract](0014-product-and-delivery-contract.md).
- Related: [Ingestion](0016-ingestion-execution-and-reconciliation.md), [storage](0018-storage-and-container-deployment.md), [verification](0019-implementation-and-verification-plan.md)

## Context

The product calls for application-ready source files, top-level venue metadata, retained last-known events, and automatic discovery. An ingestion run is not a database migration or a separate publishing service.

## Decision

Use YAML for operator configuration and JSON for published source artifacts and the generated catalog. Use one versioned JSON Schema contract, with Go and TypeScript types checked against shared fixtures.

Each source owns its prior and next artifact. The ingestion coordinator publishes the catalog last. The app consumes the contract without provider-specific parsing or a manually maintained venue list.

The version-one schemas and manually maintained Go/TypeScript types are implemented.
The local publication command and reader boundary are implemented as described below.
Source-job coordination, object storage, and shared event routes remain deferred.

## Evidence and alternatives

| Evidence source                                                                                                                     | Observation                                                                   | Supported finding                                           | Material limit                                                                               |
| ----------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ----------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| [JSON Schema 2020-12](https://json-schema.org/draft/2020-12)                                                                        | Defines a language-independent validation vocabulary.                         | A published format can be checked across Go and JavaScript. | Domain rules still need application tests.                                                   |
| [Go JSON Schema validator](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6), [Ajv](https://ajv.js.org/json-schema.html) | Both support modern JSON Schema; format enforcement depends on configuration. | Candidate producer/consumer validators can share a schema.  | Pin the draft and explicitly verify date/URI validation; defaults are insufficient evidence. |
| [Go YAML decoder](https://pkg.go.dev/go.yaml.in/yaml/v3)                                                                            | KnownFields can reject unmapped configuration keys.                           | Misspelled operator settings can fail early.                | Explicit override and value validation are still required.                                   |

YAML artifacts would be readable but require a browser-facing decoding step. JSON configuration would also work but lacks YAML comments. A database is unnecessary for the proposed snapshot contract. Type definitions alone do not validate incoming bytes.

## Source artifact outline

The contract envelope contains:

- schema_version.
- source: stable internal source key and adapter reference; venue holds default attribution.
- venue: name, timezone, address, phone, website, and optional admission policy.
- generated_at and inclusive coverage metadata; failed runs do not produce an artifact.
- events: normalized retained event records.

Each event contains:

- A stable internal identity and provider identity evidence where available.
- Its assigned public path.
- Required date, title, and effective venue.
- Optional actual off-site venue attribution and off_site flag.
- Optional performers, doors time, and show time. Field-level time provenance is deferred.
- Optional event URL, purchase URL, and admission-policy override. Legacy price fields remain readable only.
- A simple listed indicator distinguishing active-calendar records from removed-but-retained details.
- An expiration value derived from the event date and ninety-day rule.

An ordinary event inherits top-level venue metadata. A rare off-site record needs its actual venue name and flag, not a new venue directory. Do not inherit an on-site venue's admission policy or timezone blindly when the actual location differs; retain unknown data unless supported.

Publication-required effective venue does not require repeating the default venue object on every event.

### Policy and optional metadata

Use an admission policy object with human-readable text, optional URL, an optional explicitly mapped category, and optional reviewed `with_adult` metadata. Event omission inherits the venue policy; event presence replaces it. No null-suppression state is added. The reader does not interpret venue policy prose.

Accepted September 10, 2026: price is retired from active ingestion and consumption.
Retain its v1 schema for reading valid older artifacts and configuration. Reconciliation
strips prices after overrides and from retained events. The publisher strips prices from
new candidates before computing checksums and writing immutable files; already price-free
candidate bytes remain unchanged. Existing files and untouched source references are not
rewritten. The application ignores legacy prices and emits no price or cost category in
its APIs or exports. Cost classification and its configuration are removed.

Unknown optional fields remain absent. Version one rejects unspecified fields; it does
not include field-level provenance, tags, or classification logic. Operator adapter
options and overrides do not appear in published artifacts.

## Implemented version-one boundary

Schemas live in [internal/artifact/schema/v1](../../internal/artifact/schema/v1/).
[source.json](../../tests/contracts/source.json) and
[config.yaml](../../tests/contracts/config.yaml) are synthetic examples, not live
source configurations.

- `schema_version` is `1`. JSON documents and YAML configuration are limited to 4 MiB.
- Source metadata is `{id, adapter}`. The top-level venue requires `key`, `name`, and
  an IANA `timezone`; address, phone, website, and admission policy are optional.
- Each event requires `id`, `upstream_id`, `public_path`, `date`, `title`, `off_site`,
  `listed`, and `expires_on`. An optional `venue` replaces default venue attribution
  and requires `off_site: true`. Timed off-site events require their actual timezone.
- Admission policy is `{text, url?, category?, with_adult?}`. Categories are explicit data only:
  `All ages`, `13+`, `16+`, `18+`, `21+`.
  `with_adult` is `{url, reviewed_on, ranges}`. Each range contains integer
  `min_age` and `max_age` (inclusive, 0–17) and nonempty `condition` text.
  Ranges must be ordered, disjoint, and not reversed. The URL must use HTTP(S);
  `reviewed_on` is a date. An empty range list means reviewed but no minor is
  permitted; an absent object means unknown. Both fail an active With adult filter.
  Publish only manually reviewed evidence, not inferred eligibility. Deploy the
  updated reader first: older strict version-one readers reject this new field.
- Optional `status` is literal display text (1–2048 characters, not whitespace-only).
  It passes unchanged through the artifact and HTTP response. The UI maps it to
  `Scheduled` or `Cancelled` using the product ADR's presentation rule; only the
  cancellation label appears on cards, in bold red.
  Configured overrides can set or remove it. It does not change `listed`, identity,
  retention, or ticket links. Rebuild the reader before publishing this field:
  older strict version-one readers reject the added property.
- Price requires display `text`. Optional numeric data is either `amount_minor` or
  paired `min_minor`/`max_minor`, with `currency`. Values are nonnegative safe integers;
  a range cannot be reversed. These validations apply only to legacy compatibility;
  newly reconciled and published artifacts omit price, and no cost classification runs.
- Catalog entries contain `source_id`, a relative `sources/<source>/<generation>.json`
  artifact path, and its SHA-256 hex checksum. Decoding validates the reference shape;
  file existence, checksums, source identity, and cross-source path uniqueness are
  also checked by the local application reader before snapshot replacement.
- YAML configuration contains `schema_version`, `source`, `venue`, `state`
  (`new` or `established`), optional string-valued `adapter_options`, optional
  `admission_rules` keyed by the five exact age categories, and `overrides`.
  Each admission-rule value has the same shape as `with_adult`. Ingestion attaches
  a matching rule after policy overrides. Existing explicit `with_adult` and
  configured policy overrides take precedence. Off-site records do not receive
  source-venue rules. The HTTP projection passes the effective `with_adult` object
  through without venue-specific logic.
  Overrides match `upstream_id`, `date`, and `venue_key`. `set` changes content fields;
  `remove` removes optional content. Neither changes venue, date, or generated identity.
  Removing an event policy restores inheritance. Conflicting operations, duplicate
  selectors, YAML aliases/anchors, and duplicate YAML keys are rejected.

Go uses `jsonschema/v6` 6.0.3 and `go.yaml.in/yaml/v3` 3.0.5. TypeScript uses Ajv
8.20.0 and ajv-formats 3.0.1. Formats and domain checks are explicit. Schema references
resolve only against bundled resources. Types are checked through shared fixtures,
not generated from schemas.

The TypeScript module is not imported by the running UI. Its runtime Ajv compilation
is not compatible with the app's strict CSP. Application integration uses Go validation
and a compact HTTP display projection; the CSP remains unchanged. The prototype
`events.json` reader has been replaced, with no legacy-format fallback.

## Validation boundaries

1. Validate operator configuration and reject forbidden overrides.
2. Parse provider input into observations with identifiable rejects.
3. Reconcile observations with the previous source artifact.
4. Validate each final event and the complete envelope.
5. Validate the catalog and every referenced staged artifact before publication.
6. Validate at the generic consumer boundary before replacing its loaded snapshot.

Use a pinned schema version, explicit format checks, and domain assertions for dates, timezones, URL schemes, identity uniqueness, retention, and policy inheritance. Unknown schema versions fail visibly for operators, not as empty calendars.

Do not fetch arbitrary remote schema references at runtime. Bundle the approved schema. Never execute source HTML or scripts.

## Catalog and publication protocol

The catalog records its format version, generation identifier, and source artifact references with integrity information. It is generated from source configuration and job outcomes, never hand-maintained in the app.

Proposed single-writer flow:

1. Read the current catalog.
2. Run one or all selected source jobs.
3. Write successful validated artifacts under new immutable object/file names.
4. Keep the old artifact reference for each failed source.
5. Verify new references and checksums.
6. Replace the catalog last.

A source job stages its output; the coordinator performs publication inside the same ingestion workflow. There is no separately deployed publisher.

Before the final catalog replacement, failure leaves the prior catalog usable. After replacement, some sources may be newer than others if jobs failed. This is acceptable operationally; the app simply reads the published catalog.

Only one publishing coordinator may run at a time. Source children can run independently, but scheduled and manual coordinators must not overwrite one another. Enforcing that restriction is a deployment/integration check, not a reason to introduce Redis by default.

The filesystem implementation should stage on the same filesystem and use the platform's tested replacement behavior. S3-compatible storage has different semantics: do not assume a multi-object transaction or a filesystem rename operation.

## Reading and retention

### Implemented local publication boundary

`cmd/ingest` provides `publish --store DIR --expect GENERATION|none ARTIFACT.json [...]`.
Inputs are already reconciled artifacts. One invocation validates all supplied candidates
and the complete prior generation, merges references for untouched sources, and publishes
one generation. Invalid inputs abort the invocation; future job coordination must select
successful candidates and retain failed-source references. The command does not itself
fetch sources, reconcile observations, remove sources, or schedule processes.

Both writer and reader use `internal/store` for rooted, bounded reads and complete-generation
checks. This extracts the established reader implementation instead of duplicating it.
The publisher validates candidate bytes before writing and checks actual stored bytes again
before catalog replacement. Existing source/adapter and default-venue identities cannot
change through publication. Cross-source path collisions also fail the generation.

The writer holds a nonblocking advisory `flock` on the store directory inode for the
whole operation. No lock file is created or removed. Process termination releases the
lock. All writers must cooperate and the store directory must remain stable; this is
not a distributed lock or protection against an operator bypassing the protocol.
An expected catalog generation prevents stale prepared candidates from replacing newer
state. `none` means an intentionally absent catalog and must not substitute for missing
established state.

Generations use `g-` plus sixteen random bytes encoded as lowercase hex. Source files
and temporary catalog files are created exclusively. Files and affected directories
are synced; the catalog is renamed into place last and the store directory is synced
again. Prior source files are never overwritten. A pre-rename failure preserves the
old catalog but may leave unreferenced files. The next invocation uses new names.
Physical cleanup, catalog backups, and automated rollback remain deferred.

The report distinguishes publication from durability confirmation. Exit 0 indicates
successful publication and sync calls; exit 1 indicates no publication; exit 2 indicates
usage errors. Exit 3 indicates publication occurred but the final sync or report output
failed. A killed process may emit no report, so inspect the catalog before retrying.
Tests cover process death and injected failures; no hardware power-loss guarantee is claimed.

Linux Docker publication is verified, including the host bind mount. A macOS locking
implementation is present but native execution is unverified; unsupported OS targets
reject publication. Network filesystems and S3 require their own implementation and tests.

### Implemented local reader

`internal/web/catalog.go` reads `catalog.json` on each calendar API request. A mutex
serializes refresh and projection so concurrent requests cannot install generations
out of order. All source bytes are validated before replacing the complete in-memory
snapshot. Files are opened beneath `os.Root`; outside-root symlinks and path traversal
cannot supply source bytes. Regular-file checks and 4 MiB per-document/64 MiB per-generation
input limits bound local reads. These limits are not measured capacity guarantees.

An initially empty existing directory produces an empty calendar without establishing
a fallback snapshot. Missing/unreadable storage, orphan files without a catalog, and
invalid publications are errors. A validated empty catalog is a real empty snapshot.
After observing publication state, a missing catalog is never treated as empty startup.
On failure, operators receive a log entry and visitors receive the last valid snapshot,
or HTTP 503 if none exists. In-memory fallback is lost on restart.

The HTTP projection resolves venue and admission policy through shared helpers. It
joins performer names for display, preserves price text and event/ticket links, and
adds policy links and the off-site flag. It excludes unlisted and expired records on
every response, including fallback responses. Expiry starts at actual-venue midnight
on `expires_on`; untimed off-site events without a known timezone use the default venue
timezone for this lifecycle calculation only. Their public timezone remains unknown.

There is no background refresh. Reloading the SPA makes another API request. Reading
and validating every source on each request favors a simple implementation; a larger
dataset needs measurement before choosing caching or a refresh interval. Shared routes,
cross-source duplicate selection, and physical cleanup remain deferred.

### Remaining target behavior

Load references from one catalog generation, validate them, then replace the in-memory snapshot as a unit. Do not silently mix arbitrary directory listings or expose a partially read generation.

The reader remains source-independent. It can apply policy inheritance, duplicate display selection, filters, and read-time expiration from common fields. It must not scrape, reinterpret provider schemas, or write source artifacts.

Removed events remain route-addressable until expiration but are excluded from the calendar. Expired routes return 404 even when an old file still exists. Retained duplicate routes must remain resolvable while only the preferred event appears in the calendar.

Storage cleanup must remove obsolete unreferenced objects; immutable names are a publication mechanism, not permission for indefinite archives. Distinguish public ninety-day expiry from physical cleanup scheduling. Cleanup behavior for long-failing sources and referenced artifacts requires a testable policy before production; it must not damage active references or hide future events.

A genuinely empty configured artifact store supports the product's empty calendar. An unavailable store or established missing artifact is an operational failure, not a new empty dataset.

## Verification and consequences

Test interrupted writes, missing references, checksum mismatches, catalog replacement failure, old/new schema versions, deterministic serialization, and a reader refreshing during publication. Use the same contract tests for local and object storage.

This format keeps ingestion and presentation decoupled and allows file inspection. Durable state still exists in the artifacts. It is not an event-sourcing system, an infinite version archive, or a promise of transactional S3 behavior.
