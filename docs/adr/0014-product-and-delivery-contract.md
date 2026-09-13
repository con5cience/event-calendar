# ADR 0014: Product and delivery contract

Locale configuration and independent deployment now follow [ADR 0023](0023-locale-isolation.md).

- Status: Accepted for the product decisions below; implementation choices marked open remain undecided.
- Date: 2026-09-08
- Basis: Product interview, rounds 1–8, and the final cost-bracket correction.
- Scope: MVP behavior, data contract, ingestion workflow, and delivery constraints. This ADR does not claim implementation.
- Related: [Source evaluation](0001-data-source-evaluation.md), [source and adapter registry](0013-source-adapter-registry.md)

Implementation follow-up: [frontend and FullCalendar](0015-frontend-and-calendar.md), [Go ingestion](0016-ingestion-execution-and-reconciliation.md), [artifact contract](0017-artifact-contract-and-publication.md), [storage and containers](0018-storage-and-container-deployment.md), and [implementation/verification plan](0019-implementation-and-verification-plan.md). FullCalendar is the accepted community calendar selection. The remaining implementation details in those ADRs are proposals, not implemented behavior.

## Context

Build a public event-calendar single-page application (SPA) that displays events collected from configured websites. Users should be able to explore dates, follow venues, and find events worth attending without visiting each publisher separately.

The product is a lightweight calendar that links to upstream event and ticket pages. It is not an intelligent scheduler, ticketing system, admission-rule engine, or show-lifecycle tracker.

The user wants low operational overhead. Scheduled ingestion processes should publish application-ready files. The application should display what it is fed without source-specific configuration or parsing logic.

## Authority and precedence

This ADR records the interview decisions. It takes precedence over conflicting product assumptions in earlier proposed ADRs. Their source research and adapter-specific evidence remain useful.

In particular:

- Source identity is internal schema metadata, not a user-visible field or filter.
- Date or venue changes create a new event. Do not track a show across those changes.
- A successful source refresh can remove absent future events from the calendar. A failed job cannot.
- An invalid update to a known event retains its last valid version.
- Keep records only until 90 days after the event date. Old URLs then return 404; indefinite retention is not required.
- Prefer doors time over show time.
- Aggregator-specific handling is deferred, but ordinary-source duplicates must still display once.
- Price data and cost classification are retired as of September 10, 2026.

Do not treat an interview suggestion as a decision unless the user accepted it. Open choices are listed at the end of this ADR.

## 1. Audience, scope, and first delivery

### Audience and content

- The application is for the public internet.
- Ingest broadly, rather than restricting ingestion to live music.
- Geographic scope is defined by configured data sources, not a separate city boundary or radius.
- Event classification is deferred. The MVP can display music, comedy, sports, trivia, and other titled events present in the configured sources.
- The MVP must support venue and age-accessibility filtering, plus text search.
- Enrichment initially uses only information already available from the sources. External enrichment services are not part of this phase.

### Initial sources

Start with Gothic Theatre and Mission Ballroom using the AEG adapter, then HQ using the iCalendar adapter. This is the accepted initial working subset, not a requirement to enable all sources before the application is useful.

The [registry](0013-source-adapter-registry.md) remains the inventory of intended sources and their adapter assignments. Expand source coverage after the product can be exercised with the initial subset.

Aggregator-specific ingestion and attribution work can follow later. Do not require Roxy or another unresolved source to launch the first usable version.

## 2. Calendar interface

### Views and navigation

Event details close on a background click/tap, Escape, or the close button.
Interior clicks and drags starting inside the popup must not dismiss it. Use the
same close behavior for URL cleanup, focus restoration, and calendar context.

- Provide day, week, and month views.
- Brand the site as `withAdult(denver): Bring your people.` for `denver.withadult.com`. Use a 36px header with the existing system font uniformly at 20px and weight 700. Keep original purple `withAdult()` and light-gray city, colon, and tagline. Hide the colon and tagline below 768px. No footer or header border. Keep the header in document flow with proportional side gutters; reserve only its 36px in calendar sizing and preserve scrolling on short screens. Halve the gap below it to `clamp(6px, 1dvh, 12px)` on desktop and 6px on phones. Domain deployment is separate work.
- Default to Week on desktop and phones when no valid saved view exists. Persist explicit view choices, including date drill-in, in `event-calendar.view.v1` local storage. Reload restores the saved view; unavailable storage leaves view switching functional. Direct event URLs open the event's week without replacing the saved preference. Keep today's existing highlight.
- Weeks start on Sunday.
- Desktop week view uses seven date columns containing event cards.
- Phones use vertical scrolling rather than horizontal navigation through seven columns.
- Clicking a date number opens day view for that date.
- A day's `Show All` control opens day view for that date.
- Display limits for all views must be configurable.
- Desktop event cards size to their text and existing padding, without the shared button minimum height. Phone event cards retain a 44px minimum touch height. Keep fonts, toolbar controls, and Show All sizing unchanged.
- Center `Show All` horizontally within its date cell (or mobile date section), without changing its vertical position or day drill-in behavior.
- Phone week-view limit: 10 events per day by default. Desktop Week has no count cap; show `Show All` only when events overflow the available day-cell height.
- Phone month-view limit: 5 events per date by default. Desktop Month has no count cap; show `Show All` only when events overflow the available day-cell height.
- Desktop Month and Week fit the available viewport height and delegate overflow entirely to FullCalendar, without applying configured count caps first. Keep event font sizes, compact Month card padding, phone scrolling and limits, and desktop Day scrolling.
- Disable event slicing in fitted desktop grids so FullCalendar reserves the measured overflow-link height, not a whole event row. Calendar entries occupy individual dates; they do not need multi-day segment slicing.
- Desktop Week marks today's full weekday-and-date label (for example, `Fri 11`) with a purple pill and tints today's column faint plum (`#15121c`). Other dates, phone lists, and Day retain their existing appearance and navigation.
- `Show All` uses a muted plum background (`#292238`), bold lavender text (`#c4b5fd`), and a brighter plum hover (`#382e4c`). Keep its existing height and centering, with a visible inset keyboard-focus outline. Apply the same treatment to phone controls.
- Side gutters are equal: 5% of window width each, at least 12px. Remove the fixed 1600px maximum calendar width. Month and Week have minimum grid heights of 36rem and 16rem; short windows scroll rather than compressing these minima. Shared layout spacing scales within 12–24px, the date heading within 1.15–1.35rem,. Keep existing 44px control targets and native browser zoom. Resizing must preserve date, view, filters, open event, and focus. The tested minimum width is 320 CSS pixels.
- Month view shows only the selected month's dates and events. Use only the required week rows, with blank cells for weekday alignment; do not force six rows.
- No numeric day-view limit has been selected.
- When there are no available events, show an empty calendar with `No events available`.

### Ordering and compact cards

Within a date:

1. Put events without a time first.
2. Sort timed events by their selected event time.
3. Break ties by venue.
4. Then sort by artist name, using the event title when no separate artist is available.

The Month/Week card format is a single line, visually truncated with an ellipsis
when it overflows. The full title remains in the accessible name and event modal:

```text
<event title>
```

Day cards show `<event title> @ <venue>` and allow wrapping. This applies to both
desktop and phone layouts. Show All controls are not truncated by this rule.

Do not invent a time for an untimed event. The exact untimed-card presentation is an implementation detail.

### Time semantics

- Doors time is the preferred event time.
- If doors time is absent, use a separately supplied show time.
- If neither is present, keep the event untimed.
- Where both doors and show times are available, retain their distinction rather than discarding one.
- Normalize displayed times to the actual venue's timezone and clearly indicate the timezone.
- Use an IANA timezone internally so the displayed abbreviation follows the event date, such as MST or MDT for America/Denver.
- Do not invent an event duration or infer it from a calendar display boundary.

### Detail panel

Selecting an event opens a concise detail panel in calendar context. Initially display the following when available:

- Venue.
- Event title as the panel heading; omit a separate Artist row. Artist metadata remains available for search and sorting. Keep Status after Venue.
- Doors time.
- Applicable admission/age policy, including its policy link when available.
- Direct link to the event on the venue's site.
- Direct link to buy tickets.

The selected event time can use show time when doors time is absent; do not mislabel it as doors time. Missing optional metadata is omitted, not shown as a series of unknown-value placeholders.

Retain an off-site indicator where applicable. Images, long descriptions, and general tags are not required for the initial panel.

UI follow-up, accepted September 9, 2026: preserve raw upstream status in stored
data, but show only `Scheduled` or `Cancelled` in the popup. An exact cancellation
label (case-insensitive, surrounding whitespace ignored; `Cancelled` or `Canceled`)
maps to `Cancelled`; all other values, including absence, display as `Scheduled`.
This is a display rule, not independent confirmation that the event will occur.
Do not infer cancellation from absence in a refresh or alter ticket links.

Month and Week cards show only the event title on one line with overflow ellipsis.
Day cards show the event title followed by `@` and the venue, with wrapping.
Cards do not show times. Venues also remain in filters and event details.

Venue accents use the approved revision-2 palette in `web/venueColors.ts`, keyed
by the canonical venue names in the calendar projection. Assign each integrated
venue a unique fixed color. Use a 2px left card edge and the same dark surface
background on all cards, without venue tints; keep title text white and cancellation strikethroughs red. Repeat
the color as a decorative, accessibility-hidden marker beside the venue name in
the picker and event details. Do not color the modal heading or Show All buttons.
Filtering, ordering, and reloads must not change assignments. Unknown venues use
the existing purple accent until assigned a color during integration. Visual
separation takes priority over branding. Color is supplementary, not a substitute
for venue names, and the palette is not claimed to be universally distinguishable.
Cancelled events use only a red strikethrough across their card text, without a dot or
`Cancelled` suffix. The popup's `Cancelled` status remains bold red;
`Scheduled` styling is unchanged. Event ordering, detail times, and exports retain
their existing time semantics. Off-site information remains in the
popup. Popup links are labeled `View Event` and `Buy Tickets`; their destinations are unchanged.

The popup heading combines a purple date and white event title at the same 1.6rem size
as `YYYY-MM-DD : Event title`. Align them on the text baseline and allow natural
wrapping on narrow screens. Preserve the full title and semantic heading.

Popup actions use Font Awesome Free SVGs bundled locally: Regular `copy` for
`Copy Event Link`, Solid `download` for `Download Calendar Entry`, Solid
`arrow-up-right-from-square` for `View Event`, and Solid `ticket` for `Buy Tickets`.
Show icons only. Preserve the action names with `aria-label` and provide matching
custom tooltip text after 300ms of mouse hover or immediately on keyboard focus.
Escape dismisses a visible tooltip before closing the modal. Omit native `title`
tooltips to avoid duplicates. Use equal-width slots for the actions that exist;
do not reserve gaps for absent actions. Inline age-policy links remain text links. Mark the decorative SVGs
`aria-hidden="true"` and `focusable="false"` to avoid duplicate announcements.
All four actions have matching button styles, visible keyboard focus, and targets
at least 44px wide and tall. Downloads and external links remain semantic anchors; copying
remains a button. No icon font, CDN, or new runtime dependency is required.
`Download Calendar Entry` retains the existing single-event `.ics` file and download behavior.
`Copy Event Link` copies the venue event URL used by `View Event`, not the app URL.
On clipboard failure, display that URL for manual copying. Omit the copy action
when no venue event URL exists, without hiding the independently available export.

### Direct event links

- A direct event URL opens week view for the event's date.
- Open the event's detail panel immediately.
- Use default filters and event visibility for this direct-link visit, even if saved filters would otherwise hide the event.
- Closing the panel leaves the user on that week.
- A retained event removed from the active calendar can still open through its direct URL, with a brief `No longer listed` notice.
- After retention expires, the event URL returns 404.

### Appearance and performance

- Prefer dark mode.
- Use a simple, minimal presentation matching music-finder's charcoal-and-purple
  palette: background `#0d0d0d`, surfaces `#1a1a1a`, hover surfaces `#252525`,
  borders `#333333`, text `#e0e0e0`, muted text `#888888`, and accent `#6c5ce7`.
  Use white text on selected purple controls and lighter purple `#a99cff` for
  small links and focus outlines. Keep cancellation red. This changes colors,
  not layout, typography, or interaction behavior.
- Prioritize user-experience performance over visual ornament.
- No numerical performance budget or benchmark has been selected yet.

## 3. Filtering, search, and export

### Common filter behavior

- The search field has no visible placeholder text; retain its Search events accessible name and existing icon behavior. Center the venue and With Adult controls as one group in the space between the date range and search when they share a row, including the child-age picker when enabled. Preserve narrow-screen wrapping.

- Put search to the left of Day/Week/Month, wrapping below on phones. Show a right-aligned decorative Font Awesome magnifying glass only when the field is empty and unfocused. Hide it on focus or while text is present; restore it on blur only if empty. Keep the Search events accessible label and do not intercept input clicks with the icon.
- Put the venue dropdown, With Adult checkbox, and conditional child's-age picker between the date range and search. Keep the checkbox and age picker together when wrapping. Preserve the input's accessible label and age-policy help. Start date navigation and the range at the left content edge. Remove the drawer, hamburger, Reset filters, and age-category dropdown.

- Expose venue filtering, not source filtering.
- Venue selection uses multiple checkboxes inside a compact dropdown. Its button shows `All venues`, the single selected venue's name, or `N venues selected`.
- Keep the dropdown open while selecting venues. Escape closes it and returns focus to the button; an outside click or tabbing away also closes it. Reload starts with the dropdown closed but preserves the selections.
- In the Venue dropdown, clicking or tapping entry text or row padding activates the native checkbox and keeps the dropdown open. The All row clears selections. Keyboard checkbox activation and dismissal remain unchanged.
- Default venue selection is `All`.
- Selecting one venue narrows results to that venue; multiple selected venues match any selected venue.
- Text search and other active filter dimensions further narrow the results.
- Filters are positive: an event must have data that matches an active filter.
- Missing filter metadata does not hide an event when that filter is inactive.
- Persist venue selections, search text, With Adult, and child age in `localStorage` under `event-calendar.filters.v1`. Ignore retired age-category, cost, and past-visibility selections. Invalid settings fall back to defaults; unavailable storage leaves filtering functional for the current visit.
- Clear controls individually: select All venues, clear search, or uncheck With Adult. Deselecting the final venue removes that restriction. Keep saved venues absent from current data available for deselection.
- Apply filters before view display limits and Show All counts. Filter changes do not navigate the calendar.
- Do not encode filters, search, or calendar view state in URLs.
- Direct event URLs remain shareable.

### Search

- Search across available event metadata where possible.
- Update matching as the user types.
- Match without regard to capitalization.
- Multiple query words must all match somewhere in the searchable event metadata.
- Search the available date range, not only the currently displayed week.
- Do not introduce a separate search-results interface, automatic view switching, or other search-specific navigation.
- Ordinary calendar views continue to show matching events for their dates.
- Show all retained, listed events by default, including past events. Do not provide an `Include past events` toggle. Ignore legacy saved past-visibility settings while preserving venue, With Adult, child age, and search selections. The 90-day retention rule is unchanged.
- Show times and timezone labels in event details, not calendar entries; do not display a separate timezone footer.

### Age accessibility

Place a small information button beside With Adult, accessible as `About With
Adult`. Support hover, keyboard focus and tap, plus Escape and outside-tap
dismissal. Opening help must not toggle the checkbox. Use this text:
“Shows events whose reviewed venue policies allow someone of the selected age to
attend with an adult. Conditions vary—check the event details and venue policy
before buying tickets.”

Use a native age dropdown with values 0–17 only, default 14. Display options as
`0` through `17` beside a visible `With Adult · Age:` label group. The separator
is decorative and hidden from assistive technology. Keep matching inherited
typography and vertically centered controls, retaining numeric values and the
Child’s age accessible label. Keep the select 64px wide
with a minimum 44px touch-target height. Preserve saved age and accessible help.
The open venue picker supports case-insensitive name-prefix focus and scrolling,
with repeated-letter cycling and a 750ms prefix reset. Typing does not toggle
selection; Space toggles the focused checkbox and Escape closes the picker.

The retained metadata categories are (not a separate UI filter):

- All ages.
- 13+.
- 16+.
- 18+.
- 21+.

Use venue age-policy defaults with event-level overrides. Keep the human-readable policy and optional published-policy link. Structured categories support admission-rule mapping; adapters can map clear source restrictions.

Approved September 9, 2026, refined September 12: `With Adult` is off by default,
with a saved child age from 0–17, default 14. Checking it reveals the age picker;
unchecking it removes the admission restriction. Retired exact-category selections
are ignored. Venue and search filters still apply. Unknown or ambiguous permissions
do not match while With Adult is active.

Age Policy text is never a hyperlink; append the shared `Venue admission
policy` icon when that policy has its own URL. With Adult keeps its separate icon
and URL. External links use `target="_blank"` and `rel="noopener noreferrer"`;
internal navigation and calendar downloads do not change. The browser chooses
whether the new context is a tab or window. Display-only normalization changes
the exact reviewed Globe Hall guardian string to sentence-style capitalization
and `RECOMMENDED AGE 18+` to `Recommended age 18+`. Preserve unfamiliar strings
and abbreviations; do not alter source records, age values, or eligibility rules.

Use curated source-config rules to publish reviewed age ranges and accompaniment
conditions. Do not interpret ambiguous prose in the application or add a general
eligibility engine. Explicit event metadata and configured policy overrides win;
do not apply source-venue rules to off-site records. Preserve the advertised
restriction. Show a `With Adult` metadata row below `Age Policy`, aligned with the
other labels and values. Its value contains the child age and applicable condition,
followed by a 14px hyperlinked external-link icon with accessible name and tooltip
`Venue admission policy`, using the shared tooltip behavior and a 44px touch target.
Align the label and value text baselines for both single-line and wrapped content.
Keep the policy link's touch target outside text flow so single-line metadata rows
retain equal spacing without shrinking the touch target.
Left-align the icon within its touch target to keep it close to the policy text.
Do not display the review date in the modal;
retain it in the artifact and review records. Unknown/no-permission wording and
filter semantics remain unchanged. The filter is not a guarantee of admission.

Accepted September 10, 2026: venue onboarding is incomplete until its `With Adult`
policy review and ingestion-to-filter verification are complete. This review is
required even when event ingestion and exact age filtering already work. Record
official evidence, review date, supported age ranges, accompaniment conditions,
prohibitions, and unresolved cases. Do not infer permission from an unknown policy.
An explicit prohibition can complete a review; positive eligibility is not required.
Use the [source-evaluation checklist](0001-data-source-evaluation.md#required-venue-admission-policy-review)
and track pending work in the source registry. Already-published events remain
available; this decision does not remove them or change filter semantics.

Bluebird is the first reviewed venue, using its
[official FAQ](https://www.bluebirdtheater.net/frequently-asked-questions/), reviewed
September 9, 2026. For 16+ shows, ages 11–15 require a ticketed adult; ages 0–10
require a parent/legal guardian. Ages 16–17 are permitted. All-ages shows require a
parent/legal guardian for ages 0–10 and permit ages 11–17. The 18+/21+ columns do
not permit minors. No 13+ rule or missing-restriction fallback is inferred.

Expanded September 9, 2026: equivalent tables were independently verified for
[Gothic](https://www.gothictheatre.com/venue/),
[Mission](https://www.missionballroom.com/faq/), and
[Ogden](https://www.ogdentheatre.com/faq-policies/). Each configuration cites its
own page. [Fiddler's Green](https://www.fiddlersgreenamp.com/policies/) permits all
ages unless noted otherwise and generally requires tickets from age 2. Configure
only its All ages category, retaining the ticket condition in event details.
Do not infer exceptions for other categories or fill missing restrictions during
this enrichment. The existing source-independent reader and UI consume these
rules without a schema or application-code change.

An event-level age policy replaces the venue policy. An omitted event policy inherits the venue policy. No explicit-null suppression state is required.

If eligibility is unknown, the event remains visible without an age filter and is excluded when it cannot match an active age filter.

Use a multi-select dropdown with exact category matching. Selecting `16+` matches
the `16+` category, not every category that might admit a particular person. Multiple
selected categories match any selected category. Do not derive a category from
ambiguous policy prose in the application reader.

### Cost

Accepted September 10, 2026: retire price data across all venues. Do not extract,
publish, display, search, export, or classify prices. Reconciliation strips observation,
override, and retained-record prices; publication strips prices from legacy candidates.
Existing immutable artifacts and raw captures remain unchanged. Keep v1 price fields
readable for compatibility, but omit `price` and `cost_category` from all public APIs.
Remove `COST_BRACKET_LIMITS` configuration. Existing local events lose public price data
when the app is rebuilt, without re-ingestion. Preserve ticket links and age filtering.
This decision supersedes all price-display and classification decisions below.

Superseded September 9, 2026: remove the Cost dropdown and cost filtering from the MVP.
Ignore previously saved cost selections without changing other preferences. Keep
optional price metadata and display it in event details when supplied. The current
Gothic/Mission feeds omit usable prices, and ticket checkout retrieval was blocked.
The bracket decisions below remain historical context and describe retained API
classification only; they do not require a user-facing filter.

Use these configured categories exactly as requested:

| Category | Advertised dollar bracket |
| -------- | ------------------------- |
| Free     | $0                        |
| $        | $1–20                     |
| $$       | $21–75                    |
| $$$      | $76–200                   |
| $$$$     | $201+                     |

The last bracket explicitly starts at $201. Do not change it to $200+.

Free requires explicit zero-price/free information. Missing price is not free. Events without a known matching price are excluded when a cost filter is active.

Use the lowest advertised ticket price and round up to a whole dollar before
bracket matching. Thus $20.55 matches `$$`, $75–150 matches `$$`, and $200.01 matches
`$$$$`. Keep original price text for display. Do not add fees absent from the source.
Adapters represent known exact prices or the advertised minimum/maximum in the
existing structured price fields; source-specific ticket-tier extraction remains
adapter work, not application scraping.

Keep the bracket definitions configurable through `COST_BRACKET_LIMITS`, default
`20,75,200`: the upper whole-dollar limits for `$`, `$$`, and `$$$`. Free is always
zero and `$$$$` begins above the last configured limit. Reject non-increasing,
non-positive, non-integer, or incorrectly sized configurations.

Use a multi-select Cost dropdown. Classify structured USD prices, including zero,
or explicit standalone `Free` text with USD/unspecified currency. Do not infer
numeric prices from ambiguous prose, convert foreign currencies, or treat provider
placeholder zeros as structured free prices. Missing structured amounts remain
unknown unless the price explicitly says Free.

### Export

Provide one event per downloadable `.ics` file for import into third-party calendars. Bulk export and continuously updating calendar subscriptions are not required for the MVP.

## 4. Event and venue model

### Publication requirements

An event needs:

- A venue.
- A date.
- An event title.

An artist is not separately required. Other fields are optional, subject to valid data types and the artifact schema.

### Sources and venues

- Include source identity in the internal schema for provenance and job ownership.
- Do not expose source identity as a visitor-facing field or filter.
- A normal source is configured around one default venue.
- Keep venue-level metadata such as name, address, phone number, timezone, and age policy at the artifact's top level.
- A rare off-site event should carry an off-site flag/tag and its actual venue name, rather than being mislabeled as occurring at the source's default venue.
- Do not build a separate venue dictionary or full off-site venue directory for the MVP. Users can follow the event link for further information.
- Detailed multi-venue aggregator handling remains deferred.

### Occurrences and rooms

- One ticketed event spanning multiple rooms is one event.
- Independently ticketed performances in different rooms are separate events.
- Matinee and evening performances are separate events.
- Do not add special modeling for weekend passes versus single-day passes.
- Rooms alone do not determine event identity.

### Identity and URLs

Use a readable URL with a venue, event date, event name, and short opaque suffix. Illustrative shape:

```text
/events/<venue>/<YYYY-MM-DD>-<event-name>-<suffix>
```

- Assign the URL once and retain it in later artifacts.
- Do not include event time in the URL.
- Changes to title spelling or time preserve the original URL.
- A change of venue or date creates a new event and URL.
- Do not redirect or connect a moved show to its earlier occurrence as a show-lifecycle feature.
- The suffix distinguishes otherwise identical readable paths.
- Exact suffix generation and upstream-record matching are implementation choices, not settled algorithms.

### Duplicate suppression

Display one event when several sources clearly describe the same occurrence.

Accepted matching approach:

1. Prefer a shared ticket/event identifier or normalized ticket URL.
2. Otherwise consider actual venue, date, and normalized title, using time to distinguish separate performances.
3. Leave uncertain matches separate rather than hiding a potentially distinct event.

The venue's own listing wins over an off-site or other duplicate listing. This is not approval to merge every field across conflicting records. Fallback priority when no direct venue listing exists and optional-field merging remain open.

### Configuration overrides

- Operator-maintained configuration files are sufficient; no admin UI is required.
- Configured overrides always win over incoming source values until removed.
- Event overrides can modify any event field except venue and date.
- Venue defaults can define age policy and structured filter metadata.
- Overrides must not defeat schema validity or URL identity rules.
- General curated tags, if introduced later, belong to individual events and use a vocabulary maintained by the user.

## 5. Artifact contract and lifecycle

### Published format

Each source workflow produces an application-ready artifact containing top-level metadata and an event list. JSON or YAML is acceptable at the requirements level; the serialization format has not been chosen.

The workflow is:

```text
consume source -> produce artifact -> validate artifact -> publish artifact
```

A job reads its previous artifact to preserve retained history, assigned URLs, and last-valid records. The new publication contains the current events and retained history needed by the application.

The application consumes the published format without source-specific extraction or venue configuration.

### Retrieval horizon and retention

- On each run, fetch what the source makes available for the next 12 months.
- This is a requested horizon, not a guarantee that every source publishes 12 months of data.
- Users can browse retained past events.
- Retain only the last known version, not a history of every field change.
- Expire records 90 days after their event date, including removed records retained for direct links.
- At expiration, remove the details and let the event URL return 404.
- There is no requirement to preserve raw responses or event versions forever.
- There is no requirement to backfill historical events that were never ingested.

### Refresh outcomes

| Condition                                                              | Required result                                                                                    |
| ---------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| New source with no previous artifact                                   | Allow an explicit first run starting empty.                                                        |
| Established source cannot read its previous artifact                   | Fail the job; do not silently reset history.                                                       |
| Job fails, including an incomplete retrieval treated as failure        | Leave that source's existing publication unchanged.                                                |
| Successful refresh contains valid records                              | Publish those records, subject to overrides and identity rules.                                    |
| New record fails validation                                            | Reject and report it; publish the other valid records.                                             |
| Previously published record is present but invalid                     | Retain its last valid version and report the rejected update.                                      |
| Previously published future record is absent from a successful refresh | Remove it from the active calendar, retaining last-known detail data until its retention deadline. |
| Record has passed and is still inside retention                        | Keep its last known version for past browsing, unless already removed from the active calendar.    |
| Record is more than 90 days past its event date                        | Expire it; direct URL returns 404.                                                                 |

The next-12-month refresh does not erase retained past events simply because the upstream response is upcoming-only.

A successful refresh, not an HTTP 200 alone, is the authority for absence-based removal. Adapter retrieval validation must distinguish a completed job from errors or incomplete pagination. The implementation must preserve the difference between an absent record and an identifiable rejected update.

Do not add show rescheduling state, visitor-facing staleness rules, or a complex historical event model.

### Discovery and publication

- Placing a valid published artifact in the configured storage/catalog system should make it available without adding venue-specific application configuration.
- An automatically generated artifact catalog is acceptable.
- The application requires a generic artifact/catalog location, not a manually maintained list of sources.
- Normal operation will likely run all source jobs in a batch and generate their artifacts together.
- Do not require application logic that coordinates ingestion schedules.
- Preserve prior data for failed sources even when other sources complete in the same batch.
- Atomic publication mechanism, storage location, and whole-batch versus per-artifact release mechanics remain implementation choices.

## 6. Execution, deployment, and maintenance

### Ingestion

- Use one ingestion container image containing the ingestion jobs.
- Build and trigger it independently of the application image.
- Provide commands to run one configured source or all configured sources.
- Each source runs as an isolated process with its own output and failure result.
- A failed source must not prevent successful sources from producing their data.
- Jobs are scheduled, but scheduler choice and cadence are not yet decided.
- Go is the leading ingestion-language candidate because the workload is requests, parsing, and file production, with testability a priority. It is not a final stack selection.

### Application

- The frontend should support a rich, flexible, performant calendar experience.
- A Node/React-based approach is the leading candidate, not a selected framework.
- Use a community calendar component: FullCalendar's standard React integration is selected. Do not hand-roll calendar views; its mobile event-limit integration remains an explicit verification gate in ADR 0015.
- Vike is a possible future option if SSR/indexable URLs become a requirement; it is not an MVP dependency.
- The app must not require specialized per-source configuration.
- Accounts, analytics, and advertising remain out of scope. On September 12, 2026,
  basic SEO and link previews were approved: Go-rendered page metadata and basic
  content, a sitemap, robots.txt, and a shared preview image. See ADR 0015.
- Event deep links and expired-link behavior are still functional requirements even without SEO.

### Local development

- Use Docker Compose.
- Default `docker compose up` starts the application, not ingestion.
- Include ingestion service definitions in the Compose file commented out, as requested.
- Do not automatically contact live sources on application startup.
- Without artifacts, show the empty calendar and `No events available`, not a committed demonstration dataset.

### Deployment and storage

- Keep deployment compatible with Railway and Docker containers.
- Prefer not to operate a stateful database or Redis.
- The reason is low maintenance and low operational complexity, not a ban on durable files or all state.
- A database remains permissible if investigation establishes an unavoidable need or clear benefit.
- Durable artifacts are required for retention and stable URLs across runs.
- No storage provider, volume strategy, runtime server, hosting budget, or backup policy has been selected.

### Operator visibility

- Source health is for operators, never visitors.
- Logs and job exit status are sufficient as the initial operator interface; no admin panel is implied.
- Report rejected records and failed jobs.
- Staleness policy, alerts, and health dashboards are deferred.

## 7. Alternatives set aside

- **Immediate support for every source:** start with the selected subset and expand adapter coverage later.
- **A database or Redis by default:** prefer an artifact-based approach unless implementation evidence justifies additional services.
- **Source-specific logic in the SPA:** normalize inside ingestion and publish a common application-ready format.
- **A general-purpose admission rules engine:** use configured defaults, overrides, categories, and explicitly reviewed child-age ranges only.
- **A complete venue directory for off-site records:** keep a simple off-site indicator and actual venue attribution.
- **Show-lifecycle tracking and indefinite URLs:** create a new event on venue/date changes and expire records after 90 days.
- **Automatic search view changes:** filter the existing calendar.
- **External metadata enrichment:** use source-provided information first.
- **An admin panel:** use configuration files and operator logs initially.

These are scope choices, not claims that the alternatives are technically impossible.

## 8. Open implementation choices

Resolve these through focused design and testing without expanding product scope:

- Frontend framework, serving strategy, and ingestion language implementation details are proposed in ADRs 0015–0016. FullCalendar is selected; mobile limits and dependency compatibility still require verification.
- JSON versus YAML for published artifacts and operator configuration.
- Exact artifact schema, schema versioning, catalog format, and removed-record representation.
- Durable storage, safe publication, backup/recovery, scheduling, and Railway deployment mechanics.
- Stable upstream matching, suffix generation, deduplication implementation, and fallback precedence.
- Source-specific extraction of multiple ticket prices; range classification and decimal rounding now follow the agreed lowest-price rule. Non-USD prices remain unclassified without currency conversion.
- Precise age-category mapping without a guardian/eligibility engine.
- Day-view limit, default time formatting, and small presentation details.
- Filter-reset persistence behavior after a direct-link visit.
- Performance acceptance targets and representative dataset sizes.

A choice in this list is not authorization to omit the associated accepted product behavior.

## 9. Acceptance scenarios for implementation

These are required future checks, not tests already run:

1. Compose starts only the app. With no artifacts, the public interface shows `No events available`.
2. One ingestion image can run one source or all configured sources without starting the app.
3. Gothic, Mission, and HQ artifacts load through the same application contract.
4. A new valid artifact becomes discoverable without source-specific app changes.
5. Day/week/month navigation, Sunday week starts, configurable limits, and `Show All` behave as specified on desktop and phone layouts.
6. Cards sort untimed first, then by time, venue, and artist/title; doors take precedence over show time.
7. Venue, age, and text filters combine correctly, including unknown metadata and retained past events.
8. Filters survive reloads without appearing in the URL. Search does not change the interface mode.
9. Event links open the correct week and detail panel with default filters; closing leaves that week visible.
10. One-event ICS export represents the known date/time accurately without inventing missing facts.
11. Duplicate observations display once when confidently matched, with the venue's own listing preferred.
12. Failed refreshes preserve data; valid records publish despite rejected peers; known invalid updates retain last-valid data.
13. Successful omission removes a future event from the calendar while retaining its detail link until expiration.
14. Title/time corrections preserve the URL. Venue/date changes create a new event.
15. Past events remain browsable inside the 90-day window; expired links return 404.
16. Venue age policies inherit correctly; event policies and configured overrides take precedence.
17. Exact bracket boundaries include $200 in `$$$` and $201 in `$$$$`.
18. Off-site records are marked without introducing a venue-directory requirement or exposing source filtering.

## Consequences

The product stays small at the presentation layer while ingestion owns extraction, normalization, validation, retained records, and publication. The artifact system still needs durable storage and stable record matching, but it does not require a database by default.

This ADR is the development reference for agreed product behavior. Implementation ADRs may choose mechanisms; they must not silently broaden the MVP or reverse these decisions.
