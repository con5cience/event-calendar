# ADR 0001: Evaluate event sources and assign adapters

- Status: Proposed
- Date: 2026-09-08
- Scope: Ingestion architecture; no importer is implemented by this decision.
- Related: [Source registry](0013-source-adapter-registry.md)

Product behavior is governed by [ADR 0014](0014-product-and-delivery-contract.md). Its accepted interview decisions take precedence over conflicting proposals below, including source visibility, successful-refresh removal, event identity, and retention. The evaluation and adapter evidence remain applicable.

## Context

The candidate sources expose JSON APIs, static JSON feeds, iCalendar, structured data embedded in HTML, and HTML event listings. A calendar's visible month does not establish the retrieval limit of its underlying data.

A **source** is a configured publication of event information. It is not necessarily a venue. A promoter source can publish several venues; one venue can appear in several sources.

An **adapter** implements a stable provider contract: how to retrieve records, enumerate them, interpret their fields, and report limits. Transport and parsing helpers can be shared without making two provider contracts identical.

## Decision

Evaluate each source before assigning it an adapter. Reuse an adapter only when its retrieval, enumeration, and field semantics fit. Otherwise add an explicit provider variant, use a site-specific parser, or leave the source unresolved.

Prefer, in order, a suitable supported API, a verified public website feed, structured data embedded in a page, and HTML extraction. This order is conditional on coverage and data quality. A sparse API must not displace a more complete source without recording the tradeoff.

These are proposed contracts. They do not prescribe a programming language, HTTP library, database, scheduler, or class hierarchy.

## Evaluation record

Record the following for each source:

| Area                   | Questions and evidence required                                                                                                                                            |
| ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Identity and ownership | What is the original URL? Who publishes it? Is it a venue, room, promoter, or aggregator?                                                                                  |
| Access                 | Which exact URL, method, parameters, and headers work? Are credentials required? Are terms, robots guidance, and reuse restrictions understood?                            |
| Interface stability    | Is this a supported API or a website-internal interface? How was it discovered? Does access depend on a session, build ID, or changing script?                             |
| Raw data               | Inspect 5–10 representative records, or all records when fewer exist. Verify titles, dates, links, identifiers, and intended units before reporting counts or date bounds. |
| Enumeration            | Does it return a snapshot, page, cursor, or date range? What is the termination condition? Are filters honored? Could a response cap truncate it?                          |
| Coverage               | Are past, future, sold-out, canceled, and rescheduled events included? Which rooms and event categories are included?                                                      |
| Identity               | Is there a stable provider event ID? Are IDs namespaced? Can a canonical detail URL be a fallback?                                                                         |
| Time                   | Are dates local or UTC? Is an offset or IANA timezone supplied? Does a timestamp mean doors, show start, or a display boundary?                                            |
| Venue                  | Is the venue explicit, missing, or inferred? Does the source include events at other venues?                                                                               |
| Fields                 | Are lineup, age restrictions, status, prices, currency, images, and ticket links structured or embedded in HTML?                                                           |
| Enrichment             | Which fields need a detail request? Does enrichment introduce a different provider or access requirement?                                                                  |
| Quality and provenance | Are there conflicts, duplicates, placeholders, stale records, or warnings from the publisher?                                                                              |
| Operation              | What rate limits, retry signals, caching headers, error modes, schema changes, and credential lifecycles apply?                                                            |
| Fit                    | Which adapter contract matches? Which differences require configuration, an explicit variant, or a separate parser?                                                        |

Public accessibility is not a finding that bulk ingestion or redistribution is permitted. Review access and reuse conditions before enabling a source. Do not bypass bot challenges or use private credentials discovered in site content.

## Shared contracts

### Source configuration

Keep a stable source key, display name, original URL, selected retrieval URL, adapter reference, optional variant, publisher role, expected venue scope, timezone policy, enumeration settings, enrichment policy, and access requirements. Keep secrets outside documentation and committed configuration.

The registry uses three evidence states:

- **Verified retrieval:** representative records were retrieved during exploration. This does not mean complete or production-ready.
- **Partial:** a usable listing was inspected, but a material retrieval or coverage boundary remains unresolved.
- **Unresolved:** no usable event retrieval was verified.

### Adapter output

Return event observations, not an assertion that every observation is a unique real-world event. Preserve:

- Source key, provider ID or documented fallback, source URL, retrieval time, and raw field provenance.
- Title, optional lineup, venue and room, detail URL, ticket URL, image URL, age restrictions, and event category.
- Separate doors, start, and end values, timezone, precision, and the evidence for any inference.
- Provider status and normalized status, including unknown values.
- Optional price amounts and currency; absence is not zero.
- Parse warnings and unrecognized values.

Retain sufficient raw evidence under an explicit retention policy to reproduce normalization errors. Do not execute scripts or render untrusted source HTML as application content.

### Retrieval report

Return requested bounds, observed bounds, pages or ranges visited, completion signal, known caps, warnings, failures, and retrieval time. Distinguish:

- Enumeration completed according to the provider's verified contract.
- Retrieval succeeded but completeness is unknown.
- Retrieval was partial or failed.

Observed minimum and maximum dates are not proof of uninterrupted or complete coverage. Report aggregates only after validating raw records.

## Normalization and reconciliation

Use explicit IANA timezones, such as America/Denver, when supported by source evidence. Never apply a fixed Mountain Time offset throughout the year. Preserve unknown show times instead of converting doors into show starts. Reject placeholder end times as evidence of event duration.

Separate source identity from venue identity. Preserve the original venue name and any mapping evidence. Do not assign every Cervantes publication to the Cervantes building.

Namespace provider IDs. Reconcile cross-source observations outside the parser using stable IDs, ticket/detail links, venue, time, and title evidence. Do not merge on title alone. Prefer direct publisher evidence when resolving a conflict, but retain competing observations and the decision provenance.

Absence from a partial feed must not automatically cancel or delete an event. Only interpret removal after establishing the provider's coverage and removal semantics.

## Adapter assignment

See the [registry](0013-source-adapter-registry.md) for all candidate sources and links to the adapter ADRs, including families still under evaluation.

A generic HTTP client, JSON decoder, HTML parser, timezone library, and pagination primitives can be shared. Provider-specific selection, pagination rules, and time semantics remain in adapters or named variants.

## Alternatives

- **One importer per venue:** simple initially, but duplicates AEG, RHP, and HoldMyTicket behavior. Use per-source configuration instead where contracts match.
- **One universal scraper:** cannot reliably express all pagination and time semantics. Share infrastructure, not assumptions.
- **Ticket vendor as the only source:** potentially useful, but access, venue coverage, and completeness were not verified for every candidate.
- **Browser automation for everything:** adds operational dependencies. Reserve it for an approved source whose verified retrieval requires it.

## Acceptance and verification

### Required venue admission-policy review

Accepted September 10, 2026: `With adult` policy review is a required part of venue
onboarding, not optional enrichment after declaring ingestion complete. The
primary use case includes finding events a parent can attend with a 14-year-old.
Successful retrieval and exact age-category mapping alone do not establish venue
readiness. Apply this requirement to each physical venue, not merely each adapter.

Before marking a venue ready:

1. Review its official admission policy and applicable event-specific restrictions.
   Record the evidence URL, review date, supported show categories, child-age
   ranges, and accompaniment conditions in the venue's source configuration and
   linked ADR record. Do not assume venues using one provider share a policy.
2. Record explicit prohibitions as well as permissions. Keep missing or ambiguous
   rules unknown; document unresolved categories and the evidence needed to resolve
   them. Do not invent an exception to make a venue match the filter.
3. Publish supported rules through the existing policy/override structure. Preserve
   advertised restrictions, configured override precedence, and off-site isolation.
4. Test age boundaries, the age-14 use case, permitted and prohibited categories,
   unknown data, overrides, and the artifact-to-filter/detail path. A reviewed
   prohibition is a valid result; a venue need not offer an eligible show to pass.
5. Record the review outcome and material limits in the source registry. If the
   review or required validation remains unresolved, mark onboarding incomplete.

Existing published data may remain visible while this work is pending. This gate
does not authorize deletion or change the unknown-policy filtering behavior. See
[the product contract](0014-product-and-delivery-contract.md#age-accessibility)
and [the source registry](0013-source-adapter-registry.md) for current decisions.

### Importer checks

Before enabling an importer:

1. Capture representative fixtures with source URL and retrieval date. Include exceptional records where available.
2. Test required fields, namespaced identity, venue mapping, timezone conversion, missing values, status, and malformed responses.
3. Verify pagination or snapshot completion separately from parsing. Test empty, capped, overlapping, and failed responses.
4. Compare representative normalized observations with public source details.
5. Test retry, rate limiting, schema drift, and partial-failure reporting.
6. Verify deduplication and removal policies through the consumer interface.
7. Complete the access/reuse review and record operating limits.

These are future implementation requirements, not checks completed by this documentation change.

## Consequences and limits

We can reuse provider behavior while retaining source-specific evidence and uncertainty. More than one adapter can contribute observations for a venue.

The September 8 exploration verified particular responses, not service guarantees. Roxy remains unresolved. No production cadence, historical completeness, load tolerance, or redistribution permission has been established.
