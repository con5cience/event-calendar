# ADR 0015: React SPA with FullCalendar

- Status: FullCalendar selection accepted; first-milestone integration verified. Remaining production design proposed.
- Date: 2026-09-08
- Scope: Implementation design and the first-milestone integration result below.
- Authority: [Product contract](0014-product-and-delivery-contract.md).
- Related: [Artifact contract](0017-artifact-contract-and-publication.md), [verification plan](0019-implementation-and-verification-plan.md)

## Context

The product requires day, week, and month views, compact sorted event cards, configurable per-day limits, mobile vertical scrolling, filters, and event detail panels. The user explicitly selected a community calendar rather than custom calendar components.

Indexing and basic link previews were added on September 12, 2026, as described
below. Accounts and analytics remain out of scope. Event deep links and real
expired-link HTTP 404 responses remain required.

## SEO and preview foundation — September 12, 2026

Use the existing Go host to render escaped title, description, canonical, Open
Graph, and summary-card metadata into the compiled Vite shell. Render basic
event information or homepage event links inside the root; React replaces this
content on startup. The response is the same for visitors and crawlers. There
is no separate crawler route or user-agent detection. The calendar stays a SPA.

Canonical and preview URLs use the fixed public origin
`https://denver.withadult.com`, never request headers. The existing approved
554x554 purple Denver PNG is the shared square preview image. Reuse the validated
event projection; do not introduce price data, inferred eligibility, invented
show times, or new artifact fields. Structured event rich-result markup is
deferred until its required fields can be supported accurately.

The homepage has crawlable links to listed, unexpired events. The XML sitemap
includes those URLs and the homepage. The robots file allows crawling and points
to the sitemap. Removed-but-retained event pages remain HTTP 200 with `noindex`,
but are absent from the sitemap. Expired and unknown event URLs remain HTTP 404.
An unavailable catalog without a last valid snapshot returns sitemap HTTP 503,
not a misleading empty sitemap. Existing last-valid-snapshot behavior is reused.

Metadata is supplied on document requests; this does not add client-side head
management during calendar navigation. Copy Event Link continues to copy the
venue URL. Pasting a withAdult event URL shares this site's preview instead.

Selected over client-only metadata because some preview crawlers do not execute
JavaScript. Selected over a new SSR framework because Go already resolves event
routes and has the required validated data. HTML templates escape upstream text;
XML encoding escapes sitemap URLs. The built shell must contain exactly one
title and one empty root; invalid shells return 503 rather than incomplete HTML.
Vite development mode alone does not provide these server features.

Verification covers initial HTML without JavaScript, escaped hostile titles,
canonical host isolation, lifecycle and sitemap behavior, and React startup in
the built container. Actual search indexing and external service preview caches
cannot be verified before public deployment.

References: [Google JavaScript SEO](https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics)
and [Open Graph](https://ogp.me/).

## Decision

Use React and TypeScript, built with Vite, with React Router for client routing. Use FullCalendar's standard React integration as the calendar engine. The FullCalendar selection is accepted; package versions and integration details require the checks below.

Use a small Go application host to serve the built assets and published artifacts. Node is needed for the frontend build and development tools, not necessarily the production runtime.

Do not implement a parallel calendar engine, custom date-grid view, or private fork of FullCalendar to satisfy an integration gap. A material library limitation returns to design review.

## Evidence and alternatives

| Evidence source                                                                                                                                                                 | Documented observation                                                                        | Supported decision                                               | Material limit                                                 |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- | -------------------------------------------------------------- |
| [React integration](https://fullcalendar.io/docs/react)                                                                                                                         | Standard React integration is MIT-licensed and offers event-content hooks.                    | Customize event text and styling within the community component. | Actual dependency compatibility and performance are untested.  |
| [DayGrid](https://fullcalendar.io/docs/daygrid-view), [event limits](https://fullcalendar.io/docs/dayMaxEvents), [more-link action](https://fullcalendar.io/docs/moreLinkClick) | Day/week/month cells, per-day limits, and day-view drill-in are supported.                    | Use built-in desktop views for the agreed 10/5 limits.           | These limits are documented for DayGrid, not List views.       |
| [List views](https://fullcalendar.io/docs/list-view)                                                                                                                            | Built-in listWeek displays a week vertically.                                                 | Candidate mobile representation of the same week.                | Ten-per-day truncation and Show All require a spike.           |
| [Ordering](https://fullcalendar.io/docs/eventOrder)                                                                                                                             | Custom ordering is available; compactness can change order unless strict ordering is enabled. | Supply the product comparator and strict ordering.               | Untimed and equal-time fixtures need verification.             |
| [Vite deployment](https://vite.dev/guide/static-deploy), [React Router modes](https://reactrouter.com/start/modes)                                                              | Vite emits static assets; basic router mode handles client URL matching.                      | A client-rendered app does not require an SSR framework.         | Vite preview is not a production server.                       |
| [Vike](https://vike.dev/ssr)                                                                                                                                                    | SSR can be disabled per page.                                                                 | A feasible future alternative, not a current requirement.        | Disabling SSR does not itself remove hosting responsibilities. |

The sources above were inspected during the preceding research. These are documented capabilities, not results from a running prototype.

React Big Calendar was also considered in [its upstream documentation](https://github.com/bigcalendar/react-big-calendar). FullCalendar's documented DayGrid limits and drill-in are a closer match. Custom calendar components were rejected by the user, not selected as a performance optimization.

## Calendar integration

- Desktop: built-in DayGrid views for day, week, and month.
- Phone: evaluate built-in listWeek for the default week interval; day/month presentation must preserve the selected date interval and vertical navigation.
- Set Sunday as the first day of the week.
- Read display limits from generic application configuration, initially 10 for week and 5 for month. The first milestone defaults day to unlimited; a configured day limit can be expanded with Show All.
- Render compact content as time, artist/title, and venue, using FullCalendar's supported hooks.
- Untimed records come first; then time, venue, and artist/title. Preserve null time instead of inventing midnight.
- Calendar-only mappings such as treating an untimed row as date-only must not change domain metadata or falsely label it an all-day performance.
- Prefer doors, then show time, as defined by ADR 0014. Do not synthesize an end time for the domain record.
- Keep click handling, modal/detail presentation, and filters in React application components.

FullCalendar owns calendar layout, visible date ranges, and date navigation. Application code owns product data and filtering. Do not let the calendar widget become the only copy of event data.

### Mobile integration gate

Before committing to production calendar integration, demonstrate more than ten same-day events in listWeek, per-day limits, a Show All action, untimed rows, and preserved product ordering.

Use only supported configuration, rendering hooks, or a small presentation projection. Do not rely on DOM surgery, a maintained library fork, or a hand-written replacement view. Any synthetic presentation row must remain separate from real events, search, detail routes, and ICS export.

If this cannot meet the product contract, report the exact limitation and request a decision about another community component or a product adjustment. Do not silently drop the phone limit or reintroduce custom calendar components.

## Search, filters, and state

Load the available published dataset independently of the visible week. Apply case-insensitive, all-query-word search across available searchable metadata and combine it with the selected venue and age filters. Price and cost metadata are retired and are not searchable, displayed, or returned by the API.

Keep normal calendar views. Searching does not switch to another results interface or automatically select another date.

Use a versioned localStorage entry for visitor filter preferences as the proposed browser persistence mechanism. Handle unavailable or malformed storage without blocking the calendar. Do not put filters into URLs. A direct event visit uses default filters for that visit; whether those defaults overwrite stored preferences remains open.

Use the product ADR's exact age categories and reviewed With Adult rules. Price classification and its configuration are removed as of September 10, 2026.

## Application host and routes

The Go host has generic responsibilities only:

- Serve compiled assets and the SPA entry point.
- Read the artifact catalog through a filesystem or object-store implementation.
- Serve published data to the browser without disclosing credentials or operator configuration.
- Resolve event paths against retained records before returning the SPA shell.
- Return HTTP 404 for expired or unknown event paths.
- Generate a single-event ICS download using shared event/time logic.

A storage failure is not proof that an event is unknown. Do not turn an unreadable catalog into mass 404 responses or a false empty calendar. Use a previously loaded valid snapshot where available, or a generic unavailable response without exposing source health.

On direct event navigation, React selects that event's week and opens the detail panel with default filters. Closing retains that week. Removed-but-retained events can still open their details. Retention must also be enforced at read time so expiry does not depend on the next successful ingestion run.

## Styling, accessibility, and performance

Use a dark, minimal theme with jewel/earth colors through supported styling hooks. Verify keyboard access, focus restoration, Escape handling, contrast, and touch controls. The detail panel should follow the [WAI modal dialog pattern](https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/).

Use date-only values for calendar dates and explicit IANA timezones for event times. A library such as [Internationalized Date](https://react-aria.adobe.com/internationalized/date/) is a candidate if additional date arithmetic is needed; avoid adding overlapping date libraries without a need.

Measure built asset size, startup, filtering, and calendar navigation with representative fixtures. No bundle-size or response-time claim has been measured. Do not add virtualization, workers, or a client state framework solely on anticipated scale.

## Verification and consequences

Apply the mobile spike and browser scenarios in [ADR 0019](0019-implementation-and-verification-plan.md). Pin one compatible dependency set after verifying its documentation and behavior; the preceding research inspected FullCalendar v7 documentation, not a tested lockfile.

This design reuses the calendar engine while retaining a small amount of product integration code. It avoids SSR for now without sacrificing HTTP route correctness. FullCalendar maintenance replaces ownership of a custom calendar implementation.

## First-milestone result — 2026-09-08

The mobile integration gate passes for the fixture-driven milestone. Use the locked
React 19.2.8, FullCalendar 7.1.0, Vite 8.2.2, and TypeScript dependency set in
[package.json](../../package.json) and its lockfile. FullCalendar's installed package
declares the MIT license. Only community plugins are used.

FullCalendar owns all DayGrid and List date layouts. Phones use listDay/listWeek/listMonth.
The app uses supported content/class hooks to place the mobile date heading above
full-width cards. It does not inspect or modify FullCalendar's generated DOM.
The app's keyboard-focus logic is confined to its own detail dialog.

For List views, `projectEvents` caps each date and creates a separate `kind: more`
presentation row. Domain records remain complete and unchanged. For DayGrid, native
`dayMaxEvents` and a view-name return from `moreLinkClick` perform the drill-in.
Returning no action would leave FullCalendar's default popover enabled.

Date cards use date-only calendar placement, including timed records. This is a
presentation coordinate, not an all-day duration in the domain model. Time labels use
the original instant and venue timezone. The UI omits the widget's all-day/time label.
Explicit title formatting shows the actual week interval rather than only its month.

| Evidence source                                                                           | Observation or test result                                                                                                                                                 | Supported finding                                                            | Material limit                                                                      |
| ----------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| [Browser tests](../../tests/browser/calendar.spec.ts), built Go containers                | Desktop week has seven aligned date columns; phone List cards occupy over 80% of calendar width with no page-level horizontal overflow.                                    | The community views satisfy the milestone layout gate.                       | Chromium desktop and emulated phone, not physical-device or cross-browser coverage. |
| Same browser tests                                                                        | Fourteen same-day fixtures show ten week cards and five month cards; Show All opens all fourteen on the correct day. Custom 4/2/3 limits also work.                        | Supported APIs plus a small List projection preserve caps and complete data. | Small synthetic dataset; filtering and export are not implemented.                  |
| [Projection tests](../../web/calendar.test.ts) and browser tests with Asia/Tokyo timezone | Untimed/venue/title order, doors priority, show fallback, Denver summer/winter and DST-transition labels pass.                                                             | Calendar placement does not depend on the visitor's timezone.                | No ingestion parser or production schema has been tested.                           |
| Browser tests and inspected week/detail screenshots                                       | Click/tap, date navigation, Escape, Tab containment, focus restoration, sparse metadata, and resize with an open event pass. Mobile date-label contrast is at least 4.5:1. | The tested interactions work in the running interface.                       | This is not a full accessibility audit or screen-reader certification.              |

The first milestone does not implement routing, filters, ICS, publication lifecycle,
or age-policy inheritance. Its small JSON input is explicitly temporary; it does not
replace ADR 0017. See the [verification record](0019-implementation-and-verification-plan.md#first-milestone-verification--2026-09-08)
and [run instructions](../../README.md).
