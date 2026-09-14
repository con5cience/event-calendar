# ADR 0019: Implementation sequence and verification gates

## Sorted fallback and first-paint loading shell — September 12, 2026

Approved: sort the initial HTML event list and remove its visible flash during
normal startup. Keep no-JavaScript event content, SEO metadata, locale isolation
and the existing security policy. This is a display-only change; catalog bytes,
event IDs, API ordering and ingestion behavior do not change.

Test-first: the new Go eight-row sorting test failed on the existing artifact
order. The initial desktop/phone startup tests failed before the loading shell
existed. Server rendering now sorts a copy using date, venue-local doors/show
clock, venue, artist/title and ID, with untimed events first. A small synchronous
same-origin head script and critical styles select the loading shell before body
rendering. React signals replacement from its first layout effect. Script/config
errors and a 15-second startup deadline restore the fallback. JavaScript-disabled
visitors see the sorted list without the loader.

The fallback remains in the same initial HTML for visitors and crawlers. No
user-agent detection, inline-script permission, new framework or schema was added.
README and ADR 0015 describe this behavior. Markdown source is inspected because
the repository has no documentation render workflow.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| `go test ./internal/web` in the Go container | Pass, including eight deliberately unordered records and unchanged caller order | Fallback sorting follows date/time/venue/title rules without mutating input | Synthetic sorting cases |
| `npm run test:e2e -- tests/browser/startup.spec.ts tests/browser/seo.spec.ts` | 14 pass at fixture port 8091; 14 pass with `CALENDAR_BASE_URL=http://127.0.0.1:8090` | Delayed bundle/configuration, module failure, 503, timeout and no-JavaScript paths behave as specified | Chromium desktop and emulated phone |
| Full `npm run test:e2e` with dedicated-venue variables at port 8090 and empty app at 8096 | 206 pass, eight skip | Existing calendar and new startup checks pass | Four locale and two replay tests run separately; two layout-specific exclusions |
| `npm run test:locales`; `npm run test:aeg` | Both exit 0; independent locale rebuild checks pass; replay's two browser checks pass | Standalone images, locale isolation and replay consumption remain compatible | Local Docker, not a remote deployment |
| `npm run test:contracts` | Go formatting, vet and full race suite pass; producer handoff and 91 frontend tests pass | Shared code and contracts pass regression checks | Local containers and fixtures |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check` | Exit 0 | Applicable repository checks pass | Markdown inspected as source |

The updated app and fixture are running on ports 8090 and 8091. The temporary
empty test container was stopped after verification. No event data was refreshed,
and no commit, push or remote deployment was performed.

## Independent locale instances — September 12, 2026

Implemented the approved locale-isolation refactor in
[ADR 0023](0023-locale-isolation.md). One selected locale owns public settings,
source profiles, capture endpoints, reviewed admission configuration, assets and
catalog storage. Exported Railway build contexts contain only that locale's
operational configuration and active generation. Shared code and test fixtures
remain shared. No new live sources, translations or remote deployment were added.

Test-first checks covered explicit startup selection, runtime presentation,
non-Denver provider profiles and packaging. The first isolated frontend build
exposed a test import of operational Denver configuration; it was changed to a
contract fixture. The first full browser run exposed synthetic fixture adapter
IDs rejected by the real Denver registry. Fixture Compose now uses its own
registry; the runtime ownership check was not weakened. An empty-startup check
also required an actual empty directory, not a nonexistent path.
A later isolation rerun exposed Docker's asynchronous `--rm` name release after
stop. The rebuilt fixture now uses a distinct container name; the full isolation
workflow then passed again.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| `npm run test:locales` | Separate Denver/coastal contexts build and serve; four desktop/phone tests pass; coastal-only rebuild leaves Denver image ID and API bytes unchanged | Build and runtime isolation work through HTTP and the browser | Synthetic second locale; local Docker, not Railway |
| `npm run test:contracts` | Go formatting, vet, full race suite and producer handoff pass; 91 frontend tests pass | Shared contracts and server regression checks pass | Local containers and fixtures |
| Full `npm run test:e2e` with dedicated-venue URL variables set to 8090 and empty URL to 8096 | 196 pass, eight skip | Existing Denver desktop and phone behavior remains intact | Four locale tests run separately; two replay tests run separately; two layout-specific exclusions |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8096 npm run test:publication`; `npm run test:aeg` | Both exit 0; AEG replay's two browser tests pass | Snapshot normalization, reconciliation, guarded publication and application consumption work | Isolated fixture stores; no upstream fetch |
| `test:rhp`, `test:clique`, `test:plot`, `test:kse`, `test:marquis`, `test:blackbox`, `test:venuepilot`, `test:meowwolf`, `test:ophelias`, `test:buzzard`, `test:herbs`, `test:afton`, `test:locale-tools`, `test:capture-profiles` via npm | All pass; VenuePilot includes a mocked non-Denver account selection | Existing capture safeguards and explicit locale selection pass | Offline tests; specialist layouts still require individual review |
| Added Go non-Denver AEG normalization and cross-locale HTTP tests | Auckland event serializes with `+12:00`, Harbor path and venue URL; foreign catalog returns 503 on fresh startup and cannot replace last-good data | Configured timezone/identity survives normalization; registry protects catalog consumption | Representative fixtures, not every provider in a new city |
| Byte/checksum comparison of original and migrated stores | Five inspected rows identify Ball Arena, Black Box, Black Buzzard, Bluebird and Cervantes; every referenced source and catalog matches original bytes | Denver event IDs, public URLs and reviewed artifact data are preserved | Existing generation only; no refresh |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check` | Exit 0 | Applicable repository checks pass | No Markdown renderer is configured |
| Documented snapshot staging command | Builds, packages a new temporary store, and passes catalog `cmp` | Established-source refresh instructions preserve the current generation | No live ingestion performed |

The local app now runs as `withadult-denver-app-1` on port 8090 and reads
`.artifacts/denver`. Its fixture uses port 8091. Original app/fixture containers
remain stopped for rollback; original `.artifacts/catalog.json` and source files
remain untouched. Stop the new Compose project before restarting the old
containers on those ports. Temporary verification containers are stopped after
checks; their exported contexts and fixture stores remain available for inspection.

README, product/frontend/deployment ADRs and affected adapter ADRs were updated.
Their Markdown source was inspected; no documentation render workflow exists.
At verification, this refactor had not yet been committed or pushed. No remote deployment was performed.

## SEO and link-preview foundation — September 12, 2026

Approved: extend the Go host with initial page-specific metadata, basic readable
HTML, crawlable homepage event links, sitemap.xml, and robots.txt. Use the
approved purple Denver PNG as the shared preview image. Preserve React calendar
behavior, artifact interfaces, expiry, and Copy Event Link's venue destination.
The design and deployment limits are recorded in ADR 0015 and README.

Test-first: `go test ./internal/web -run TestSEO -count=1` in the read-only Go
container failed on missing metadata, links, and discovery endpoints before
production changes. The implementation uses escaped HTML templates and XML
encoding, fixes canonical URLs to the public origin, and excludes unlisted
records from the sitemap while keeping their retained pages accessible/noindex.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| `go test ./internal/web -count=1` in the Go container | Pass | Escaping, canonical host isolation, sitemap, retention and existing web checks pass | Synthetic catalog |
| `npm run test:contracts` | Go formatting, vet and all race tests pass; 90 frontend tests pass including handoff | Shared producer/consumer and server checks pass | Local container |
| `npm run test:e2e -- tests/browser/seo.spec.ts` | Four pass against fixture; four pass with CALENDAR_BASE_URL=http://127.0.0.1:8090 | Initial metadata/content work without JavaScript; React opens the modal; preview PNG is served | External preview services not exercised |
| Full `npm run test:e2e` with local venue URLs on 8090, fixture on 8091 and empty app on 8096 | 196 pass, four existing skips | Desktop and phone regression suite passes | Chromium and emulated phone; AEG replay and layout-specific skips |
| `npm test` | 89 pass, one handoff skip | Standalone frontend suite passes | Handoff covered by test:contracts |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check` | Exit 0 | Repository checks pass | Git diff does not include untracked files |
| `docker compose -f compose.yaml -f compose.test.yaml up -d --build app fixture-app` and browser requests | Rebuilt services serve HTML metadata and unchanged calendar UI | Container behavior observed | No remote Railway deployment |

An initial full browser run had 188 passes and eight missing-trace-file errors.
A second browser command had been started concurrently with the same output
directory. Inspection of Playwright's installed clear-output task confirmed it
removes that directory at startup. Running the full suite alone passed without
application changes. Do not run independent Playwright commands concurrently
with a shared output directory.

Documentation source was inspected; no Markdown render workflow exists.
The temporary empty app is stopped after verification. No ingestion, catalog
mutation, commit, or remote deployment occurs. Search indexing and actual
external preview rendering remain post-deployment checks.

## With Adult policy helper — September 12, 2026

Approved: add an information button named `About With Adult` beside the checkbox,
with the agreed explanation of reviewed venue policies and advice to check event
details and venue policy before buying tickets. Reuse ActionTooltip with opt-in
help behavior: hover delay remains 300ms, focus and tap open immediately, Escape
and outside taps dismiss. The decorative information glyph is hidden from screen
readers; the button receives the open tooltip through aria-describedby. Keep it
outside the checkbox label so opening help cannot toggle filtering. Existing
action tooltips retain their behavior. No new dependency or admission-rule change.

The new test failed on the absent button before implementation. Initial rendering
checks found toolbar wrapping and phone tooltip overflow. A compact 24x44px
button restored the desktop row. Reducing tooltip width alone did not prevent
phone overflow when the filter group wrapped differently; anchoring the phone
panel to the filter group resolved the same reproduction. Desktop retains its
icon-relative panel. The focused suite passed 39 tests with one existing skip,
including existing action tooltips and keyboard controls. Desktop and phone
screenshots were inspected. README and the product contract include the helper.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e` against local containers | 191 passed, four skipped, one empty-fixture startup failure | Helper and shared UI checks pass | Empty container start initially blocked by sandbox; suite reached it before the approved retry completed |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8096 npm run test:e2e -- tests/browser/calendar.spec.ts -g 'normal startup is empty'` | Two passed after fixture start | Previously failed check passes without application changes | Full suite not repeated after infrastructure recovery |
| `npm test` | 89 passed, one skipped | Unit suite passes | Existing producer handoff skip |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check` | Exit 0 | Applicable checks pass | Local environment; git diff excludes untracked files |
| Compose rebuild and inspected screenshots | Healthy services; help fits desktop and phone | UI observed locally | Chromium/emulated phone, not physical device or screen-reader verification |

Documentation source inspected; no Markdown render workflow exists. The temporary
empty fixture was returned to its stopped state. No commit or remote deployment.

## Install the Denver favicon — September 12, 2026

Approved: install the exact user-approved transparent purple PNG as the site
favicon. Copy `docs/denver-purple.png` to `web/public/assets/favicon.png` and add
the PNG favicon link in `web/index.html`. Keep the source artwork unchanged; no
generated replacement or resizing. Vite includes the public asset in its build;
the existing Go static handler serves it. No server, dependency, Docker or data
changes are required. README records the asset and served path.

The test first failed because no favicon link existed. The initial root-level
favicon returned 404: inspection confirmed the Go handler allows only `/assets/`
for static files, despite the PNG being present in the build. Moving the asset
and link under `/assets/` resolved the same browser check without expanding server
routing. Four focused branding checks pass on desktop and phone. Browser decoding
verifies 554x554 dimensions, transparent background, and exact purple artwork.
Byte comparisons confirm the source, public and built images are identical.
Browser chrome itself is not captured by these tests. No remote deployment occurs.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e` against local containers | 190 passed, four skipped | Browser regression suite passes | Existing skips unchanged |
| `npm test` | 89 passed, one skipped | Unit suite passes | Existing producer handoff skip |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check` | Exit 0 | Applicable checks pass | Git diff excludes untracked files |
| Compose build/start and favicon browser checks | Healthy services; HTTP 200 image/png; decoded expected pixels | Installed favicon served by the built app | Chromium/emulated phone; tab icon not manually inspected |

Documentation source inspected; no Markdown render workflow exists. The temporary
empty fixture was stopped after verification. No commit was made.

## Restore the admission-label separator — September 12, 2026

Approved: restore `With Adult · Age:` and verify matching label/select typography
and alignment without changing keyboard behavior. The pre-edit browser assertion
confirmed equal computed font family, size, weight and vertical centers on desktop
and phone, then failed on the missing separator. No font change was needed. Add
one decorative, aria-hidden span using the existing flex gap and inherited font.
The separator disappears with the age picker when With Adult is unchecked.
README and the product contract now record the separator. No CSS, filtering,
ingestion, dependency, data or remote deployment changes are included.

The focused container suite passed 19 tests with the existing desktop arrow-key
skip. Screenshots show the dot, matching labels and compact native picker in both
layouts. Number-key selection still passes. Initial browser launch was blocked
by sandbox permissions; the approved retry reproduced the expected test failure.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e` against local containers | 188 passed, four skipped | Shared UI regression suite passes | Existing skips unchanged |
| `npm test` | 89 passed, one skipped | Unit suite passes | Existing producer handoff skip |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check` | Exit 0 | Applicable checks pass | Local environment; git diff excludes untracked files |
| Compose rebuild, computed-style assertions and screenshots | Healthy services; matching typography/centers and visible dot | Approved appearance observed | Chromium and emulated phone; user's exact browser not inspected |

Documentation source inspected; no Markdown rendering workflow exists. Temporary
empty fixture stopped after verification. No commit or remote deployment performed.

## Numeric age options and venue keyboard matching — September 12, 2026

Approved: put a visible `Age:` caption beside numeric-only native options 0–17,
with a 64px select and unchanged numeric storage, default 14, accessible name,
and admission rules. This supersedes the repeated `Age N` option labels below.
Extend the existing checkbox dropdown rather than replacing it with an ARIA menu
or adding a dependency. Case-insensitive typing focuses and scrolls to the first
name-prefix match; repeated letters cycle matches. Reset after 750ms or reopening.
Typing never toggles a checkbox. Space, Tab, Escape, pointer selection and saved
preferences retain their existing behavior. Confidence before edits was 9/10;
native keyboard behavior required browser verification.

Before production edits, both layouts failed the numeric-label assertion and the
venue letter-focus assertion. Inspection found only Escape handling in the custom
dropdown, and the browser test confirmed absent matching. It did not reproduce
the user's exact jump-to-top symptom, so no browser-specific cause is claimed.
No reusable name-matching handler existed in web/. README and the product contract
now describe these controls. No schema, ingestion, dependency or remote deployment
change is included.

Focused container checks passed (35 passed, one existing skip). Opening the native
age picker and typing 5 selects 5 in both layouts. Separate checks against populated
8090 passed for venue cycling, full-prefix focus, Space selection and persistence.
Desktop and phone screenshots show the visible Age caption and compact numeric
select without overflow. Docker inspection and the populated browser invocation
initially hit sandbox restrictions; approved retries succeeded.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e` against local containers | 188 passed, four skipped | Shared UI regression suite passes | Existing desktop arrow-key, optional AEG replay and desktop-only skips unchanged |
| `npm test` | 89 passed, one skipped | Unit suite passes | Existing producer handoff skip |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check` | Exit 0 | Applicable checks pass | Local environment; untracked files are outside git diff checks |
| Compose rebuild and browser screenshots | Both services healthy; desktop and phone controls fit | Updated UI observed locally | Chromium/emulated phone, not physical-device or remote verification |

Documentation source inspected; the repository has no Markdown render workflow.
The temporary empty fixture was returned to its stopped state. No commit was made.

## Explicit age option labels — September 12, 2026

Approved: retain With Adult and label the native picker options `Age 0` through
`Age 17`. Keep numeric option values, default 14, numeric saved preferences,
admission filtering, and the Child’s age accessible label unchanged. Increase the
fixed width from 64px to 96px for the longer label and native arrow. Reuse the
existing option rendering and select; no custom dropdown or schema change.
The test failed on numeric-only labels before implementation. README and product
contract now describe labeled options and the new width. No data refresh or remote
deployment is included.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e`, fixture 8091, sources 8090, empty 8096 | 184 passed, four skipped | Age labels and shared-layout regressions pass | Existing desktop age-keyboard, optional AEG replay, and desktop-only skips unchanged |
| `npm test` | 89 passed, one skipped | Frontend unit suite passes | Existing producer handoff skip |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable frontend checks pass | Local environment |
| Compose build/start, focused browser checks, desktop and phone screenshots | Healthy services; all 18 labels and numeric values checked; 96px picker fits both layouts and retains selection after reload | Changed UI observed in containers | Emulated devices, not physical hardware |

Documentation source inspected; no Markdown render workflow is supplied. The
temporary empty fixture was stopped after verification. No commit was made.

## Unified header typography; footer removed — September 12, 2026

Approved: remove the empty footer and use `withAdult(denver): Bring your people.`
in the current wordmark's system font, 20px, weight 700 throughout. Keep colors
unchanged; colon/tagline still hide on phones. The user explicitly confirmed the
existing font family and weight. Remove the footer CSS and reserve only 36px for
the header, returning 18px to the calendar. Update the document title to match.
Approved during implementation: halve the header-to-toolbar padding to
`clamp(6px, 1dvh, 12px)` on desktop and 6px on phones, and remove the header border.
The additional browser test failed against the former 1px border before the edit.
This supersedes the footer and mixed-size typography decisions below.

The revised browser test failed against the existing footer before production
edits. Tests check matching family/weight/size, exact text, retained colors, no
footer, and calendar containment after view transitions. README and product ADR
are updated. No ingestion, domain configuration, or remote deployment is included.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e`, fixture 8091, sources 8090, empty 8096 | 184 passed, four skipped | Header, spacing, and shared-layout regressions pass | Existing desktop age-keyboard, optional AEG replay, and desktop-only skips unchanged |
| `npm test` | 89 passed, one skipped | Frontend unit suite passes | Existing producer handoff skip |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable checks pass | Local environment |
| Compose build/start, focused browser checks, and desktop screenshot | Healthy services; uniform 20px/700 header, no border/footer, halved top padding | Changed UI observed in containers | Phone checks are emulated; no physical-device verification |

Documentation source inspected; no Markdown render workflow is supplied. The
temporary empty fixture was stopped after verification. No commit was made.

## withAdult Denver brand bars — September 12, 2026

Approved: 36px header, 18px empty footer, original purple `withAdult()` with a
light-gray `denver` argument and `Bring your people.` tagline. Hide the tagline
on phones, retain proportional gutters, and leave future navigation unpopulated.
Future links must allow adequate phone touch targets. Update the browser title
and theme color; no DNS, TLS, domain purchase, or deployment is performed.

The app shell keeps semantic header/footer outside main. Desktop grid height
reserves the 54px bars while intrinsic minimum size allows short screens to scroll.
Do not use overlays or fixed bars that cover calendar content. Confidence was
9/10 before implementation, pending rendered sizing verification.

The new browser test failed before the header existed. Initial follow-up checks
found old height budgets, an empty-state check aimed at the populated instance,
and a footer measurement taken before the calendar view transition settled.
Waiting for the settled geometry disproved persistent overlap without a production
change. Tests now include the bars in their budgets and use the empty fixture on
8096. All 38 focused branding/calendar/responsive tests pass. Screenshots show
the desktop tagline and phone wordmark-only header. README and the product
contract replace the former collapsed-banner decision. Documentation source is
inspected; no Markdown render workflow is supplied.

The first full suite had two failures from a legacy assertion forbidding any footer.
That assertion now requires the new footer to be empty, preserving its original
intent of preventing a timezone notice. Both focused tests pass after the update.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e`, fixture 8091, sources 8090, empty 8096 | 184 passed, four skipped | Brand bars and shared-layout regressions pass | Existing desktop age-keyboard, optional AEG replay, and desktop-only skips unchanged |
| `npm test` | 89 passed, one skipped | Frontend unit suite passes | Existing producer handoff skip |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable frontend checks pass | Local environment |
| Compose build/start and inspected branding screenshots | Both services healthy; 36px header and 18px footer; desktop tagline hidden on phone | Changed UI observed in containers | Emulated devices, not physical hardware |

The temporary empty-calendar container was returned to its stopped state. No
commit or remote deployment is included.

## Plain-text age policies and external icons — September 12, 2026

Approved: Age Policy uses plain text with a trailing icon when its own URL exists.
Reuse the existing With Adult icon, tooltip, accessible name, 14px artwork, and
44px target through a shared local VenuePolicyLink component. Both policy icons,
View Event, and Buy Tickets use `_blank` with `noopener noreferrer`. App links and
calendar downloads stay unchanged. The browser chooses a tab or window.

Normalize only the two reviewed full policy strings from Globe Hall and Paramount
in the frontend display helper. Do not lowercase arbitrary policies or alter ID,
age values, categories, rules, stored artifacts, search, or exports. The approved
plan had 9/10 confidence; unfamiliar text is deliberately unchanged. No producer
or schema change, data refresh, remote deployment, or commit is included.

The unit test failed before the display helper existed. Browser fixture setup
initially did not target the selected event; it was corrected to a direct event
URL and then failed on the uppercase wording before implementation. After the
change, a text assertion included hidden SVG license metadata; the test now checks
rendered text, consistent with existing icon tests. A formatter failure was fixed.
Focused policy, theme, and admission tests pass on desktop and phone, including
an observed popup using an intercepted policy-page fixture while the modal remains
open. Screenshots show plain wrapped policy text and the shared icon/tooltip.
README and the product contract were updated; Markdown source is inspected because
the repository supplies no documentation render workflow.

The first full suite had 28 failures: existing source tests selected the shared
policy-link name without a row scope, and the catalog test still expected a linked
`16+` label. Source assertions now target the With Adult row; the catalog assertion
targets the icon. No production change was needed for these test updates.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e`, fixture 8091, sources 8090, empty 8096 | 182 passed, four skipped | Modal, source, and admission regressions pass | Existing desktop age-keyboard, optional AEG replay, and desktop-only skips unchanged |
| `npm test` | 89 passed, one skipped | Reviewed text normalization and frontend unit checks pass | Existing producer handoff skip; unfamiliar text deliberately unchanged |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable frontend checks pass | Local environment |
| Compose build/start, focused popup test, desktop/phone screenshots | Both services healthy; popup fixture opened while modal remained; plain text and icon visible | Container UI and separate browsing context observed | Emulated devices and intercepted destination; not a physical-device check |

The empty-calendar fixture was returned to its stopped state after verification.

## Neutral card backgrounds — September 12, 2026

Approved follow-up: remove venue background tints to reduce visual clutter. Keep
the fixed colored edges, picker/modal markers, white text, and cancellation red.
This supersedes the tinted-background part of the venue-accent decision below.
Reuse the existing surface background by removing only the color-mix override.
Confidence was 10/10. The updated theme test failed on desktop and phone against
the tinted cards before the CSS change. README and the product contract now
describe neutral backgrounds. No ingestion, schema, or palette assignments change.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Focused theme, venue-color, and event-text browser tests | Six passed | Neutral background, retained accents, and cancellation styling pass | Desktop and emulated phone |
| Full `npm run test:e2e`, sources 8090, fixture 8091, empty 8096 | 180 passed, four skipped | Shared-style regressions pass | Existing age-keyboard, optional AEG replay, and desktop-only skips unchanged |
| `npm test` | 88 passed, one skipped | Frontend unit suite passes | Existing producer handoff skip |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable checks pass | Local environment |
| Compose build/start and inspected theme screenshots | Both services healthy; plain dark cards with colored edges on desktop and phone | Changed UI observed in containers | No physical-device verification |

Documentation source inspected; no Markdown render workflow is supplied. The
temporary empty-calendar test container was returned to its stopped state. No
remote deployment or commit was performed.

## Fixed venue accents — September 12, 2026

Approved revision-2 palette: use a fixed color per integrated venue on calendar
card edges and faint tints, with matching decorative markers in the venue picker
and event-detail Venue row. Keep titles white, cancellation strikethroughs and
status red, and Show All styling unchanged. The canonical names were checked
against all 26 referenced catalog artifacts, including the full Fiddler's Green
and Red Rocks names and straight apostrophes in Herb's and Ophelia's.

Reuse the existing calendar projection and dropdown checkbox behavior. The
frontend palette map avoids a new ingestion/schema contract; dynamically deriving
colors or assigning by list position would break stable assignments. Unknown names
use the existing purple accent until explicitly assigned during integration.
The dropdown accepts an optional marker renderer; the shared VenueMarker is
decorative and hidden from accessibility APIs. Confidence was 9/10 before edits,
with rendered alignment and browser behavior still to verify. No data refresh,
remote deployment, or commit is included.

Test-first checks failed because the palette module and modal marker did not
exist. After implementation, the focused color and cancellation checks passed.
The initial phone test incorrectly tried to click a search field covered by the
open dropdown; it now uses the dropdown trigger to close it. The first full suite
also found two obsolete theme assertions expecting plain gray/purple cards; these
now assert the approved Mission tint and cyan edge. No production changes were
needed for either test correction.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Full `npm run test:e2e`, fixture 8091, sources 8090, empty 8096 | 180 passed, four skipped | Venue accents, filters, responsive layouts, cancellation, and source regressions pass | Existing skips: desktop native age keyboard selection, two optional AEG replay tests, and one desktop-only test on phone |
| `npm test` | 88 passed, one skipped | Palette uniqueness, unknown-name fallback, and frontend unit checks pass | Existing producer handoff skip; schema and producer unchanged |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable frontend checks pass | Local environment |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait` and fixture screenshots | Both services healthy; matching accents visible in desktop and phone cards, picker, and modal | Container UI observed | Emulated devices, not physical hardware |

Desktop and phone screenshots from the fixture container show the card accents,
picker markers, and aligned modal marker. README and the product contract define
the palette behavior. Markdown source was inspected; no documentation render
workflow is supplied. The repository remains untracked, so git diff --check does
not validate its file contents.

## Compact native age picker — September 12, 2026

Approved: replace the typed age field with a native select containing exactly the
integers 0–17. Default remains 14; saved values and admission rules are unchanged.
The control is fixed at 64px wide with a minimum 44px height. Preserve the accessible
Child’s age label, policy help, centered filter group, and conditional visibility.
Reuse existing input styles and focus outlines. Native selection avoids a custom
menu implementation and invalid typed values. Read-time age validation remains.
Confidence before implementation was 9/10; browser rendering needed verification.

The first browser test failed on both layouts because no combobox existed. After
replacement, all 18 values, option labels/values, fixed width, saved selection,
admission behavior, and responsive alignment passed. Browser source tests now use
combobox/selectOption rather than spinbutton/fill. README and the product contract
describe the picker.

Desktop keyboard verification remains incomplete. Home, ArrowUp/Enter, and an
explicit Space/ArrowUp/Enter sequence left the value unchanged. A standalone HTML
select outside the app also retained 17 through those keys while focus remained
on the select, then lost focus on Tab. This reproduces the limitation independently
of React, FullCalendar, and application handlers; no custom keyboard code was added.
The desktop keyboard-selection test is explicitly skipped pending manual checking.
Phone-emulated selection and desktop focus/Tab checks remain in the suite.

The first full run had 176 passes, one desktop keyboard failure, and three existing
skips. No source/schema change,
ingestion run, remote deployment, or commit is included.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Final full `npm run test:e2e`, sources 8090, fixture 8091, empty 8096 | 178 passed, four skipped | Picker selection, persistence, sizing, responsive layout, and source admission regressions pass | Desktop keyboard selection remains unverified; three existing skips are two optional AEG replay tests and one desktop-only layout check |
| `npm test` | 86 passed, one handoff skipped | Frontend unit checks pass | Producer/schema unchanged; Go handoff not rerun |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable frontend checks pass | Local environment |
| Compose build/startup and inspected desktop/phone screenshots | App and fixture healthy; compact two-digit picker and arrow visible | Changed UI observed in containers | Emulated devices, not physical hardware |

Documentation source inspected; no Markdown render workflow is defined. The
repository is untracked, so `git diff --check` does not validate its files.

## With Adult label capitalization — September 12, 2026

Approved label-only change: the toolbar checkbox now reads `With Adult`, matching
the event-detail label. Native labeling changes its accessible name as well.
Browser selectors, README, and the product contract use the same capitalization.
No state, admission rules, layout, or source data changed. The exact-label test
failed on both profiles before the production change. Confidence: 10/10 for the
single text change and corresponding exact-name selectors.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Focused toolbar browser tests | Six passed | New visible/accessibility label and existing controls work | Local fixture container, desktop/phone emulation |
| Full browser suite, source URLs 8090, fixture 8091, empty 8096 | 175 passed, three skipped | Updated selectors and source regressions pass | Two optional AEG replay tests need a separate URL; one desktop-only check skips on phone |
| `npm test` | 86 passed, one handoff skipped | Unit suite passes | Producer/schema unchanged; Go handoff not rerun |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Frontend checks pass | Local only |
| Compose rebuild/startup | App and fixture healthy | Updated label served by containers | No remote deployment |

Markdown source inspected; no repository Markdown render workflow. No commit made.

## Centered toolbar filters and blank search — September 12, 2026

Approved: remove visible search placeholder text and center the venue/With adult
group between the date range and search. Include the optional child's-age input
in that group. Preserve mobile wrapping, accessible names, and the search icon's
empty/unfocused behavior. Confidence before implementation: 9/10, with responsive
alignment awaiting browser verification.

Reuse the flex layout: a leading auto margin on the filter group shares free space
equally with the existing leading auto margin on the search/view group at desktop
widths. This avoids a new grid structure or JavaScript measurement. A whitespace
placeholder preserves the existing CSS `:placeholder-shown` icon rule without
visible text or new focus state. No schema, ingestion, storage, or admission changes.

Tests first failed on both browser profiles: side gaps differed by 174.875px and
the placeholder still contained Search events. Updated tests check equal gaps at
1440, 1920, and 2560px with adult filtering on/off, blank placeholder text, and
existing icon behavior. Existing responsive checks cover widths down to 320px.
README and the product contract reflect the change. Verification results follow.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Focused toolbar, search, and responsive browser run | 26 passed | Equal spacing, blank field, icon behavior, and wrapping pass | Chromium desktop/phone emulation |
| Full `npm run test:e2e`, source URLs on 8090, fixture 8091, empty 8096 | 173 passed; three skipped | Existing calendar and source regressions pass | Two optional AEG replay tests lack their separate URL; one desktop-only test skips on phone |
| `npm test` | 86 passed; one skipped | Frontend unit tests pass | Separate Go handoff not run; producer/schema unchanged |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable frontend checks pass | Local environment |
| Compose rebuild and startup; inspected desktop/phone screenshots | Both services healthy; centered desktop group and blank fields visible | Changes observed in running containers | No remote deployment or physical-device test |

Updated Markdown source was inspected; no repository Markdown render workflow is
defined. `git diff --check` exited 0 but does not cover this untracked repository.
The empty test container was stopped after verification; the app remains running.

## Simplified toolbar — September 12, 2026

Approved: remove the hamburger, filter drawer, Reset filters, and the remaining
age-category dropdown. Navigation and the range begin at the left content edge;
proportional page gutters stay. Venue and With adult controls retain their toolbar
section, and the child's-age input stays beside With adult when enabled.

Reuse the existing toolbar and native controls. Remove the drawer component and
its CSS rather than keeping a hidden dialog. Ignore legacy saved age-category
selections instead of retaining an invisible restriction. Preserve venue, search,
With adult, and child-age preferences. Source artifacts, admission rules, event
metadata, and exports are unchanged. Clear filters individually through their
controls. This section supersedes earlier drawer and reset decisions below.

Plan confidence was 9/10: component paths, state persistence, and existing tests
were inspected; responsive wrapping remained to be observed. No new dependencies
or ingestion changes were required. Existing drawer-only tests were removed and
filter tests now use visible controls. README and the product contract describe
the new behavior.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Initial toolbar browser test | Failed on both layouts: expected no Filters button, found one | New removal assertion detects the old UI | Fixture container |
| Initial filter unit tests | Two failures for retained age preferences; all 12 passed after removal | Retired selections cannot hide events | In-memory and simulated storage |
| Focused browser run | 40 passed | Placement, adult admission, search, persistence, and responsive controls pass | Chromium desktop and phone emulation |
| First full browser run | 169 passed, two obsolete Filters-button assertions failed, three skipped | Remaining test references needed migration | Height assertion now targets Previous |
| `npm test` | 86 passed, one skipped | Frontend unit regressions pass | Separate Go handoff not rerun; producer/schema unchanged |
| Compose build and startup | App and fixture healthy | Updated app runs locally | No remote deployment |
| Inspected desktop and phone screenshots | Navigation at calendar left edge; drawer absent; remaining controls visible | Requested layout observed | Emulated devices |
| Final full `npm run test:e2e` (source URLs on 8090, fixture 8091, empty 8096) | 171 passed, three skipped | Toolbar and existing source/browser regressions pass | Two optional AEG replay checks need a separate URL; one desktop-only test skips on phone |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Applicable frontend checks pass | Local build, not remote deployment |

A read-only Node command had a syntax error before any write and was corrected.
The repository is untracked, so `git diff --check` does not validate these files.
Markdown has no repository render workflow; updated source is inspected directly.

## Venue and adult controls in the toolbar — September 12, 2026

Approved refinement: move the venue dropdown and With adult checkbox out of the
drawer, between the date range and search. Show the child's-age input only when
With adult is checked. Keep that checkbox and input together on wrapped rows.
Age-category filtering remains in the drawer while With adult is off; Reset
filters remains available in the drawer in both modes. No filtering, admission,
storage, source-artifact, or view-selection rules change.

The existing FilterDropdown and native inputs retain their handlers and accessible
names. Toolbar-specific CSS provides compact controls and a bounded overlay menu.
The age input retains its accessible label and policy help through visually hidden
text. No new dropdown implementation, dependency, or additional filter state was
added. Medium-width toolbars can wrap to another row; phone controls stay within
the viewport. The desktop calendar continues to use remaining available height.

Tests first failed because venue controls were unavailable with the drawer closed.
A second test reproduced the age input wrapping 52px below its checkbox before
the pair was grouped. Tests now exercise placement, persistence, reset, keyboard
dismissal, and widths 320–1920 with adult filtering enabled. Source browser tests
use the toolbar directly. Drawer tests still verify focus containment and now use
Age for dropdown checks. The helper waits for Reset filters instead of the moved
venue control.

Early test failures included old phone-height limits and clicks behind an open
venue popup. Outside-dismissal tests now click the unobstructed range heading;
other tests explicitly close the popup before using covered controls. One pair of
overlapping test runs conflicted while writing traces; subsequent browser runs
were sequential. A focused calendar run omitted the empty-container URL, causing
two empty-state failures; the full run supplies that URL. A read-only test-migration
script had a syntax error on its first attempt and made no edits before correction.

README and the product ADR document the new control locations.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Initial `npm run test:e2e -- tests/browser/toolbar-filters.spec.ts` | Both layouts failed before production edits | Closed-drawer control access is tested | Fixture data |
| Added checkbox/input alignment assertion | Both tests failed with a 52px center offset; passed after grouping | Adult checkbox and age input stay together when wrapping | Widths 320–1920 |
| Focused toolbar, filters, drawer, calendar run | 46 passed; two empty-state failures with omitted empty URL | Moved controls and retained drawer behavior pass | Empty state required the corrected full-run environment |
| Full `npm run test:e2e` with source URLs on 8090, fixture 8091, empty 8096 | 173 passed; three existing skips | Toolbar, calendar, and dedicated-venue regressions pass | Two optional AEG replay checks need a separate replay URL; one desktop-only check skips on phone |
| `npm test` | 86 passed; one handoff skipped | Existing frontend tests pass | Producer/schema unchanged; separate Go handoff not rerun |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Frontend checks pass | Local build |
| Compose rebuild and inspected desktop/phone screenshots | App and fixture healthy; new toolbar order and grouped age controls visible | Behavior observed in local containers | Chromium emulation, not physical devices or remote deployment |

The temporary empty-calendar test container was stopped after verification. No
source artifacts were refreshed. Markdown source was inspected; no documented
Markdown render workflow exists. `git diff --check` exits 0 but cannot inspect the
untracked worktree files.

## Search placement and empty-field icon — September 12, 2026

Approved refinement: search precedes Day/Week/Month on desktop and remains below
the controls on phones. Its right-aligned magnifying glass is visible only when
the input is empty and unfocused. Focus or any text hides it; leaving an empty
field restores it. The accessible Search events name, query persistence, filtering,
and view persistence are unchanged.

The icon uses the official Font Awesome Free 7.3.1
[magnifying-glass SVG](https://github.com/FortAwesome/Font-Awesome/blob/7.3.1/svgs/solid/magnifying-glass.svg),
with unchanged path geometry and the existing local attribution and decorative
accessibility attributes. CSS `:focus` and `:placeholder-shown` supply the state
without React focus state or a new dependency. Padding reserves text space, and
`pointer-events: none` lets clicks on the icon focus the input.

Tests first failed for desktop placement and the missing icon on both layouts;
the unchanged phone wrapping test passed. The new checks exercise icon alignment,
accessible decoration, click-through focus, typing, saved text after reload,
clearing while focused, and restoring the icon on blur. The first broader run
found a test-helper race: it focused search after the dialog was hidden but before
the asynchronous close handler restored focus to Filters. Waiting for that existing
focus restoration initially passed the focused suite, but one phone failure
recurred in the full suite. The helper now also waits for `aria-expanded=false`,
which confirms the close handler has updated state. Five repetitions per layout
then passed without changing application logic.

README and the product ADR describe this refinement. Source artifacts, ingestion,
calendar sizing rules, and modal action icons are unchanged.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Initial placement/icon browser tests | Three expected failures; unchanged phone placement passed | Tests detect the requested differences | Before production changes |
| Focused view-preference and responsive-sizing suite | 20 passed | Icon states, placement, persistence, and resizing pass | Chromium desktop and emulated phone |
| Resize-focus test, five repetitions per layout | 10 passed | Helper waits for completed close handling | Repeated local runs, not proof against every timing condition |
| Full `npm run test:e2e` with source URLs on 8090, fixture 8091, empty 8096 | 171 passed; three existing skips | App and published-source regressions pass | Two optional AEG replay checks lack their separate URL; one desktop-only check skips on phone |
| `npm test` | 86 passed; one Go handoff test skipped | Existing frontend tests pass | Schema and producer unchanged; separate handoff not rerun |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build` | Exit 0 | Frontend checks pass | Local build |
| Compose rebuild and desktop/phone screenshot inspection | App and fixture healthy; icon right-aligned and desktop search before view buttons | Changed UI observed in running containers | No remote deployment or physical-device test |

The temporary empty-calendar verification container was stopped after the run.
README and ADR Markdown source were inspected; no documented Markdown render
workflow exists. `git diff --check` exits 0 but does not inspect untracked files.

## Roxy / Afton integration — September 12, 2026

Approved scope: complete public Afton pagination, paired native event-detail
enrichment, reviewed admission mapping, tests, container staging and one-time local
publication. External listings retain unknown ages and ambiguous times. No app
behavior, schema, prices, schedules, remote deployment or earlier source data
changed. The registry, README and [Afton ADR](../adapters/0022-afton-venue-events.md)
describe the implemented flow. All dedicated venue candidates are now integrated.
The search relocation and Close-button alignment are completed in the UI record below;
Roxy external-ticket admission enrichment remains separate follow-up work.

Read-only research traced the official Wix calendar into its Afton iframe and
public GET endpoint. Three pages with exact totals enumerate 26 available events.
The existing API adapters have different contracts, so this source uses a new
provider decoder while reusing replaySnapshot, reconciliation, immutable artifact
publication, bounded HTTP and inert DOM traversal. The existing RHP compactor
preserves JSON-LD and admission markup within the snapshot bound. Calendar-cell
scraping and browser-dependent capture were rejected because anonymous HTTP works.
External provider enrichment, checkout, older-page backfill and remote systems
are not part of this implementation. No credentials were needed or saved.

Capture and decoder tests first failed on missing implementations; the replay
test failed on the absent command before wiring. Focused tests then passed.
The initial live staging replay failed on paired detail disagreement and published
nothing. The same captured event had identical structured identity and start data
but alternated between `.age-restriction` and modal admission markup. Regression
tests reproduced the unsupported layout, conflicting repeated values, free-event
identity without offers and the `21+ Only` category. All passed after the decoder
supported the verified variants. Replaying the same capture then published all
26 records in staging with zero rejects.

A new capture test also exposed a pre-existing compactor edge case: a fragment
starting with an executable script was retained. The six prior RHP capture tests
passed before the change; the new seventh test failed as expected. The helper now
preserves only the captured JSON-LD split segments, not any segment that starts
with a script. Both Afton and RHP capture suites pass. The source response is
always parsed inertly, so the failure did not execute the retained script.

The first combined browser run passed 158 tests and failed the Herb's desktop
admission check. The same failure reproduced in isolation. The failure snapshot
showed a full Week cell and Show All. A read-only browser check found zero visible
Mike Maurer Band cards in Week and one in Day under the same age-14 filter.
The artifact references were unchanged. This isolated a test assumption about
Week capacity, not an admission regression. The shared test helper now supports
opening Day on either layout; Herb's and Roxy use it. Their four focused checks
passed afterward. App filtering and calendar layout were not changed.

Six raw listing records, six staged artifact records and six records from each
staged/main API were inspected before counts. They contain source IDs, venue,
dates, separate clocks where supplied, exact event links and event-level age
evidence. Twenty records explicitly allow All Ages, Threshing Day is 21+, and
five external listings have unknown ages. No blanket venue exception is inferred.
All previous source references and all current artifact hashes were checked.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Captured API pages and staged replay | Three pages; 26 events; zero rejects | Full current widget listing is accepted | Available dates end March 4, 2027; unknown zero-event responses fail safely |
| `npm run test:afton`, `npm run test:rhp` | 11 and seven passed | Pagination, allowed URLs, compaction and HTTP failure handling pass | Semantic detail comparison runs in Go |
| Container `go test ./internal/afton ./cmd/ingest` | Passed | Mapping, clocks, both admission layouts, overrides and retention pass | Fixtures are synthetic and never published |
| `docker build --target ingestion -t event-calendar-ingestion .` | Passed; rebuilt image replayed the original snapshot | Container producer works | One-time local run |
| `npm run test:contracts` | Go formatting, vet and full race suite passed; 87 consumer checks passed | Producer/consumer contract remains valid | No schema change |
| Staged `ROXY_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/afton.spec.ts` | Two passed | Desktop/phone details, links, filtering and ICS pass | Chromium |
| Main API/catalog on 8090 | 26 Roxy events; 20 All Ages, one 21+, five unknown; 25 prior references unchanged; 26 valid hashes | Main app consumes Roxy without replacing earlier sources | Unknown external ages do not match the child filter |
| Focused main `npm run test:e2e -- tests/browser/herbs.spec.ts tests/browser/afton.spec.ts` with both source URLs | Four passed | Admission tests work independently of Week overflow | No app behavior changed |
| Full main `npm run test:e2e` with all live source URLs including ROXY_BASE_URL | 159 passed, three skipped | Combined desktop/phone regression checks pass | Two optional AEG replay checks and one desktop-only check on phone skipped |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; 86 unit tests passed, one skipped | Applicable host checks pass | Consumer handoff ran separately; worktree remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-roxy-hqQp3x`,
at `2026-09-12T20:17:10.761Z`.
Stage: `/private/tmp/event-calendar-roxy-stage.ImSLrl`, generation
`g-a44b4a2b0919a609b5d4f8b06e8a3093`.
The guarded main publication advanced `g-37d1bb1df3d84f9cc8470dfe17a8bd40` to
`g-d0bda5a8978d6d2ef93d28aaffee22ad`. The prior catalog remains in staging as
`catalog-before.json` for recovery. Earlier immutable artifacts remain on disk.
The unchanged app image consumed staging on 8097 and main publication on 8090.
Markdown sources and local link targets were inspected. The repository has no
documentation renderer or documentation build command. Temporary test containers
are stopped after verification; capture, staged artifacts and catalog backup remain.

## Herb's integration — September 12, 2026

Approved scope: ordinary HTML capture, explicit Denver-local interpretation of
printed clocks, reviewed parent cutoff, decoder and replay CLI, tests, staging and
one-time local publication. No UI behavior, schema, prices, schedules, remote
deployment or earlier source data changed. Search relocation and Close-button X
alignment remain queued until dedicated source integration is complete.

The implementation reuses bounded HTTP transport, inert DOM traversal conventions,
replaySnapshot, reconciliation, admission fields and generation-guarded publication.
Source-specific selectors and the time-dependent admission condition remain in
the Herb's profile. JSON, month and incoming ICS routes were rejected under the
inspected robots guidance. Browser capture is unnecessary for the ordinary HTML.
Epoch conversion was rejected because the site is configured to New York and
disagrees with printed clocks. The user approved Denver-local printed clocks;
venue intent remains unverified. Event details, ticket systems, older pagination,
remote deployment and unrelated adapters were not changed or needed for this flow.

Tests first failed on the absent capture module and decoder. The CLI integration
test failed on the missing replay command before wiring. A later regression test
for the inspected description layout exposed missed `21+` text; matching now
covers descriptions and numeric-plus restrictions. The same regression passed
after the change. Six raw listing records, six staged artifact records and six
records from each staged/main API were inspected before reporting counts. These
include IDs, dates, printed start clocks, titles and event links, including old
slug years and the same-day trivia layout. Policy evidence came from the venue
footer; no guardian exception was inferred from third-party listings.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Paired ordinary HTML and staged replay | 22 in-window events; zero rejects | Available upcoming records normalize consistently | Dates stop September 30; unknown empty/pagination fails safely |
| `npm run test:herbs`, `npm run test:ophelias`, `npm run test:buzzard` | Four passed in each suite | Capture restrictions and existing shared HTTP behavior pass | Semantic validation runs during replay |
| Container `go test ./internal/herbs ./cmd/ingest` | Passed | Cutoff, clocks, restrictions, overrides, failed refresh and retention pass | Synthetic fixtures are not published |
| `docker build --target ingestion -t event-calendar-ingestion .` and staged replay | Build and replay passed | Ingestion container produces application-ready artifacts | One-time local run |
| `npm run test:contracts` | Go formatting, vet and full race suite passed; 87 consumer tests passed | Existing producer/consumer interfaces remain valid | No schema change |
| `HERBS_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/herbs.spec.ts` | Two passed | Desktop/phone details, age-14 filter, policy link and ICS work | Late/missing clocks are covered by fixtures, not current live listings |
| Main catalog and API on 8090 | 22 Herb's events with parent condition; all 24 prior references unchanged; all 25 hashes match | Main app consumes the new source without replacing prior sources | Printed Denver clock semantics remain the approved assumption |
| Full `npm run test:e2e` with all live source URLs including HERBS_BASE_URL | 157 passed, three skipped | Combined desktop/phone regression checks pass | Two optional AEG replay checks and one desktop-only check on phone skipped |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; 86 unit tests passed, one skipped | Host checks pass | Handoff ran separately; worktree remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-herbs-ZciTDk`,
at `2026-09-12T18:03:02.410Z`.
Stage: `/private/tmp/event-calendar-herbs-stage.uj4Nhh`, generation
`g-26fe74eb3478c8d446a50b676bf7e299`.
The guarded main publication advanced `g-f237986e3c93ebfd09378037290b2bb7` to
`g-37d1bb1df3d84f9cc8470dfe17a8bd40`. Staging retains the prior catalog at
`catalog-before.json` for recovery. Immutable earlier artifacts remain on disk.
The unchanged app image consumed data on staging port 8097 and main port 8090.
The README, HTML profile, historical Squarespace proposal and source registry
describe the implemented flow. Markdown source and local references were
inspected; the repository has no documentation renderer or build command.

## Black Buzzard integration — September 12, 2026

Approved scope: paired HTTP calendar/homepage capture, scoped HTML and JSON-LD
cross-checking, reviewed venue policy, replay CLI, tests, staging and one-time
local publication. No app behavior, schema, prices, schedules, remote deployment
or earlier source data changed. README, HTML adapter ADR and registry are updated.
Search relocation and Close-button alignment remain queued UI work.

Capture and decoder tests first failed on missing implementations. The CLI test
failed on the missing replay command before wiring. Focused tests then passed.
The established replay, reconciliation, publication and inert HTML traversal
patterns were reused. Ophelia's four transport tests passed before extracting its
unchanged bounded HTTP reader into `tests/html/http.mjs`; both source capture
suites passed afterward. Source-specific selectors remain separate. Browser
automation and standalone JSON-LD ingestion were rejected because ordinary HTML
supplies the required cross-page identity and age evidence.

Six raw homepage records were inspected, followed by six staged artifact records
and six records each from staged and main APIs. These contain dates, titles,
Denver venue/address, Tixr identities and status. The thirteen age cards all have
the known placeholder, so records use the reviewed 18+ default with no guardian
clearance. No price or ambiguous time is published. The two Tim Butterly shows
retain separate IDs, links and public paths. The earlier research web request to
a Tixr link failed; ticket enrichment is not part of this implementation. Detail
paths excluded by robots guidance, other venues and remote environments were not
accessed for this integration.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Paired public pages and staged replay | Thirteen equal records; zero rejects | Inspected calendar, homepage JSON-LD and age cards agree | Available dates end December 5; unknown empty/paginated responses fail safely |
| `npm run test:buzzard`, `npm run test:ophelias` | Four passed in each suite | Shared transport preserves behavior, limits and HTTP failure handling | Semantic comparison occurs in Go replay |
| Focused `go test ./internal/buzzard ./cmd/ingest` in container | Passed | Mapping, identity, policies, overrides, failed refresh, invalid updates and successful absence retention pass | Synthetic fixtures are never published |
| `npm run test:contracts` | Image build, Go formatting, vet and full race suite passed; 87 handoff tests passed | Producer/consumer interfaces remain valid | No new app interface |
| `BUZZARD_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/buzzard.spec.ts` | Two passed | Desktop/phone separate shows, links, ICS and age-14 exclusion work | Chromium |
| Main catalog and API on 8090 | Thirteen events, no adult clearance; all 23 previous references unchanged; all 24 artifact hashes match | Local app consumes the added source | One-time local publication |
| Full `npm run test:e2e` with all live source URLs including BUZZARD_BASE_URL on 8090 | 155 passed, three skipped | Combined desktop/phone regression checks pass | Two optional AEG replay checks and one desktop-only check on phone skipped |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; 86 unit tests passed, one handoff skipped | Applicable host checks pass | Handoff ran separately; worktree remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-buzzard-qNCVSq`,
at `2026-09-12T17:41:05.843Z`.
Stage: `/private/tmp/event-calendar-buzzard-stage.IV00bY`, generation
`g-5c62a0d859162eb509f2f26518c350b3`.
Guarded main publication advanced `g-aeceb4dd5d5e15e972e9e907d4b57443` to
`g-f237986e3c93ebfd09378037290b2bb7`. The prior catalog is retained in staging
as `catalog-before.json` for recovery. The rebuilt ingestion image performed
staging and publication; the unchanged app image consumed artifacts on 8097 and
8090. Markdown sources and local references were inspected; the repository has
no documentation renderer or documentation build command.

## Ophelia's integration — September 12, 2026

Approved scope: paired public HTTP capture, scoped HTML decoder and replay CLI,
reviewed admission configuration, tests, staging and one-time local publication.
No app behavior, schema, prices, schedules, other source data or remote deployment
changed. The README, HTML adapter ADR and registry describe the implemented flow.
The queued search relocation and Close-button X alignment remain separate UI work.

The capture test first failed because capture.mjs did not exist. Go mapping tests
failed on the absent Decode implementation; the CLI retention test failed on the
missing replay command. A malformed-clock regression reproduced a slice-bounds
panic; format validation now rejects it before slicing. Focused checks then passed.
The implementation reuses replaySnapshot, reconciliation, generation-guarded
publication, admission rules, x/net/html and established DOM traversal conventions.
There is no shared HTML listing contract that fits these selectors. Browser
capture and a universal selector configuration were rejected as unnecessary here.
Ticketmaster checkout, unrelated sources and remote deployment were not inspected
because they are not changed or required by this read-only listing profile.

Eight captured records, eight normalized artifact records and six records each
from the staged and main API were inspected. They contain event titles, dates,
MT show clocks, optional labeled doors, ticket identities and restrictions. HTML
entities decode correctly, and winter offsets use -07:00. Seven 16+ events include
explicit guardian exceptions; four 18+ events use the reviewed FAQ rule. All 27
21+ events have no adult exception. No published record has price data.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Paired HTTP pages and staged replay | 38 equal selected records; zero rejected | Complete inspected listing consumed | Available dates end December 12, 2026; unpublished inventory not proven |
| `npm run test:ophelias` | Four passed | Paired reads, HTTP/content-type failure and size cap work | HTML semantic equality is checked by Go replay |
| Focused `go test ./internal/ophelias ./cmd/ingest` in Go container | Passed | Mapping, age boundaries, overrides, cancellation, invalid records, failed refresh and successful absence retention work | Empty-calendar markup remains unverified and intentionally fails |
| `npm run test:contracts` | Image build, Go formatting, vet and full race suite passed; 87 handoff tests passed | Existing producer/consumer interfaces remain valid | Synthetic fixture is never publication data |
| `OPHELIAS_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/ophelias.spec.ts` | Two passed | Staged desktop/phone links, ICS, age-14 inclusion and age-12 exclusion work | Chromium |
| Main catalog hashes and API on 8090 | 38 events, 11 adult-clearance records; all 22 prior references unchanged; all 23 artifact hashes match | Main app consumes the new source only | One-time local publication |
| Full `npm run test:e2e` with all live source URLs including OPHELIAS_BASE_URL on 8090 | 153 passed, three skipped | Combined catalog passes desktop/phone regression checks | Two optional AEG replay checks and one desktop-only check on phone skipped |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; 86 unit tests passed, one handoff skipped | Applicable host checks pass | Handoff ran separately; worktree remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-ophelias-es57aN`,
at `2026-09-12T16:56:56.294Z`.
Stage: `/private/tmp/event-calendar-ophelias-stage.zwOEBg`, generation
`g-d62bfaa24f81461bd460309c042e965b`.
Guarded main publication advanced `g-a2eb1f9d5ebb275efe9ac3547c10c46a` to
`g-aeceb4dd5d5e15e972e9e907d4b57443`. The prior catalog remains in staging as
`catalog-before.json` for recovery. The ingestion image was rebuilt and used for
staging and publication; the unchanged app image consumed staging on port 8097
and main artifacts on 8090. Markdown sources and local references were inspected;
the repository supplies no documentation renderer or documentation build command.

## Meow Wolf integration — September 12, 2026

Approved scope: paired public-page capture, scoped decoder and CLI replay,
reviewed admission configuration, staged verification and one-time publication.
The existing snapshot, reconciliation, publication, retention-test and browser-test
patterns were reused. No app behavior, schema, prices, schedules or remote deployment
changed. The adapter ADR, source registry and README describe the implemented flow.

Capture, decoder and CLI tests failed on missing implementations before code was
added. Live capture exposed legacy and replacement Next.js formats. After three
failed capture iterations, implementation paused for raw-document inspection and
review of React's framing implementation. Test-first fixes cover object-shaped
associatedEvents, UTF-8 byte-framed text, unnumbered stylesheet hints and the
non-model X marker. Unknown framing still fails; no failed capture was published.
The reader does not execute embedded code or retain description text.

Eight raw and eight staged API records were inspected before aggregate reporting.
Six main API records were inspected after publication: titles, dates, summer and
winter offsets, venue, links, restrictions and adult conditions matched the contract.
An initial main API diagnostic incorrectly filtered by source_id, which the public
projection does not expose; filtering by its venue field confirmed publication.
The sandbox blocked the first API connection; the approved retry succeeded.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Paired capture and replay | Both passes agree on 50 records; zero replay rejects | Public listing consumed completely | Available dates end January 23, 2027, not a full year |
| `npm run test:meowwolf` | 13 passed | Capture framing, scope and consistency checks pass | Upstream serialization remains undocumented |
| Focused Go tests and `npm run test:contracts` | Passed, including formatting, vet, full race suite and 87 handoff tests | Decoder, retention, publication and consumer boundaries pass | Fixtures are synthetic and never published |
| Staged Meow Wolf browser spec on 8097 | Two passed | Desktop/phone admission, links, cancellation and ICS work | Chromium |
| Main catalog and API on 8090 | 50 events: 24 All Ages, 14 18+, 11 21+, one unknown; one cancelled; 24 adult-clearance records; no prices | Main app consumes the source | Unknown restriction grants no clearance; cancelled record remains cancelled |
| Artifact verification against prior catalog | All 21 prior references unchanged; all 22 files exist and hashes match | Publication only added Meow Wolf | Local publication, not a remote deployment |
| Full `npm run test:e2e`, all live source URLs including MEOWWOLF_BASE_URL on 8090 | 151 passed, three skipped | Combined desktop/phone regression suite passes | Two optional AEG replay checks and one desktop-only check on phone skipped |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; 86 unit tests passed, one handoff skipped | Applicable host checks pass | Handoff tested separately; repository remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-meowwolf-hkT3e8`,
at `2026-09-12T15:37:15.215Z`.
Stage: `/private/tmp/event-calendar-meowwolf-stage.2zwo02`, final generation
`g-7bf4600422fcc2bd8caff5ea50d3cb7b`. The initial stage rejected eight main-venue
records until the detail venue ID and Denver address were reviewed and tested.
Guarded publication advanced `g-43c5023cd9fe71378ffe6c3a35ed77a4` to
`g-a2eb1f9d5ebb275efe9ac3547c10c46a`. The previous catalog is retained in staging
as `catalog-before.json`. Markdown sources and referenced paths were inspected;
the repository has no documentation renderer or documentation build command.

## Toolbar, Close buttons, and remembered view — September 12, 2026

Approved scope: move search beside Day/Week/Month (below on phones), retain the
other controls in the drawer, center both Close buttons, default to Week, and
restore explicit view choices on reload. Direct event links retain their Week
context without replacing the saved preference. Today keeps its existing marker
and tint. Source artifacts and ingestion are outside this change.

View storage is separate from filters: `event-calendar.view.v1`, accepting only
`day`, `week`, and `month`, with Week as the fallback. Explicit button choices and
date drill-in save the preference. Reset filters does not change it. This follows
the existing optional-local-storage pattern without coupling view restoration to
filter resets or automatic event-link navigation.

The new browser tests first failed on both layouts for the existing Month default,
unavailable closed-drawer search, and absent grid centering. Updated tests exercise
visible toolbar search, persistence, invalid/blocked storage, shared-link behavior,
accessible Close controls, and responsive placement. Existing Month-specific tests
now select Month explicitly. Drawer tests wait for drawer controls, not search;
search outside a modal drawer is correctly inert while that drawer is open.

Verification exposed a reset-click defect in the shorter drawer. At 640×450,
pointer-down collapsed the venue dropdown and moved Reset from y=392.5 to y=289
before click. Pointer activation left Mission Ballroom selected; keyboard Enter
cleared it. Dropdown outside dismissal now waits for click completion and defers
pointer-induced blur closure. Keyboard blur, Escape, and touch row toggling pass
the regression checks. Pointer cancellation has cleanup handling but no dedicated
automated regression in this change.

Initial regression failures also included obsolete Month-startup assertions and
a test reloading before the modal close event updated the URL. Tests now wait for
the public URL transition. A sandbox Chromium launch failure required an escalated
rerun. Lint caught and removed unused imports after moving search test actions.

| Evidence source | Raw result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Initial `npm run test:e2e -- tests/browser/view-preference.spec.ts` | Six expected failures before production edits | Tests distinguish the old default, hidden search, and uncentered layout | Chromium desktop and emulated phone |
| Focused browser run: responsive-sizing, filters, event-links, month-fit, theme, view-preference | 57 passed; one desktop-only phone skip | View persistence, toolbar layout, reset, focus, and responsive behavior pass | Fixture data |
| Full `npm run test:e2e` with dedicated-source URLs on 8090, fixture 8091, empty 8096 | 169 passed; three skipped | Existing app and published venue regressions pass | Two optional AEG replay checks need a separate replay URL; one desktop-only test skips on phone |
| `npm test` | 86 passed; one Go handoff test skipped | Existing frontend unit and contract tests pass | Separate Go handoff not rerun; schema and producer unchanged |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check` | All exit 0 | Frontend checks pass | Worktree is untracked, so git diff does not inspect those files |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait` | App and fixture rebuilt and healthy | Changed code runs in local containers | No remote deployment |
| Inspected desktop/phone toolbar and Close-button screenshots | Search beside desktop view controls, below on phone; centered Close layout | Final local layout inspected | Chromium screenshots, not physical-device testing |

README and product ADR now describe Week fallback and independent view persistence.
Markdown source was inspected; this repository has no documented Markdown render
workflow. The temporary empty-calendar test container is stopped after verification.
No source artifacts were refreshed.

### Original queue (implemented by this change)

Move the search field out of the filter tray and place it beside the Day/Week/Month
controls. Keep the other filters in the tray. This is the user's September 12
request, queued after the remaining dedicated venue sources; aggregators remain
out of scope. Preserve search behavior and persistence. Verify desktop and phone
layout, keyboard access, filtering and available calendar space before completion.
No UI change is included in the Meow Wolf integration.

Also queued on September 12: vertically center the X in Close buttons. The user
reported that it is off-center. Reproduce the alignment issue before changing
shared button/icon styling; verify all affected Close buttons on desktop and phone.
This remains UI follow-up, not part of the Ophelia's source integration.

## Levitt VenuePilot integration — September 12, 2026

Approved scope: public GraphQL capture, separate VenuePilot decoder, reviewed
admission configuration, tests, staging, and one-time local publication. See the
[VenuePilot ADR](../adapters/0020-venuepilot-widgets.md) and README workflow.
No app behavior, schema, prices, schedules, remote deployment, or earlier source
data changed. The existing replay, reconciliation, publication, admission-rule,
CLI retention-test and browser-test patterns were reused. No shared provider
client exists for this contract; the scoped capture is a separate implementation.

Capture tests first failed on the absent module, decoder tests on missing Decode,
and CLI tests on missing replay-venuepilot. A later biography regression test
failed because broad All Ages matching granted clearance; standalone admission
line matching fixed that failure. Focused tests then passed. Tests also cover
conflicting restrictions, null ages, generic footers, overrides, unsafe links,
local DST offsets, missing times, changed counts/records, duplicate IDs, coverage,
last-valid retention, failed-job preservation and successful-empty unlisting.

All ten raw records and six normalized API records were inspected. The September
12 capture excludes yesterday's event, so it contains ten rather than the eleven
seen during September 11 research. Both passes agree. Eight events explicitly say
All Ages, including two yoga events; both Beer Festival dates explicitly say 21+.
All numeric age fields are null. The FAQ requires an adult for ages 0–16; no
adult exception is inferred for restricted or unknown events. Rez Metal keeps its
October 1 date and unchanged September 25 ticket slug. Prices are absent.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| `npm run test:venuepilot` | Ten passed | Pagination, empty results, scope, counts, identity, HTTP/GraphQL failures and paired consistency pass | Public API remains undocumented |
| Focused Go tests and `npm run test:contracts` | Go formatting, vet and full race suite passed; 87 consumer-handoff tests passed | Decoder, CLI, retention, API and ICS contracts pass | Synthetic fixtures are not publication data |
| Staged `LEVITT_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/venuepilot.spec.ts` | Two passed | Desktop/phone links, downloads and age-14 inclusion/exclusion work | Chromium |
| Main catalog and `http://127.0.0.1:8090/api/calendar` | Ten Levitt events; eight All Ages with adult clearance, two 21+ without it; 20 prior references unchanged; all 21 hashes match | New source is consumed without refreshing earlier sources | Upcoming data only through October 11 |
| Full `npm run test:e2e` with all live source URLs, including LEVITT_BASE_URL on 8090 | 149 passed, three skipped | Combined catalog passes desktop/phone regression checks | Two optional AEG replay checks and one desktop-only layout check on phone skipped |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; unit run 86 passed, one handoff skipped | Applicable host checks pass | Handoff ran separately in contracts; worktree remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-levitt-lIPMBZ`,
at `2026-09-12T14:52:48.850Z`.
Stage: `/private/tmp/event-calendar-levitt-stage.F1zXSk`, generation
`g-fdc2cee556a8e9bc8393fdfccaa7c4f0`.
Generation-guarded publication advanced the main catalog from
`g-fcc68fc7561fe03f60eba48bcde981d1` to
`g-43c5023cd9fe71378ffe6c3a35ed77a4`.
The prior catalog remains in staging as `catalog-before.json` for recovery.
The Markdown sources and referenced local paths were inspected; this repository
has no documentation renderer or documentation build command.

## Black Box Supabase integration — September 11, 2026

Approved scope: scoped exact-count retrieval, two-room mapping, ticket-level
admission review, explicit doors/music times, staging, and one-time local
publication. See the [Supabase ADR](../adapters/0004-supabase-rest-api.md).
README and registry describe the operator workflow. No app behavior, schema,
prices, database service, schedule, remote deployment, or other source data changed.

Capture and decoder tests failed on missing implementations before code was
added; the CLI test failed on the absent replay-blackbox command before wiring.
Existing reconciliation/publication helpers and x/net/html were reused.
The CLI test helper now supports Black Box title and exact-count fields.
Tests cover admission, explicit All Ages event-policy links, overrides,
VIP-versus-event restrictions, room identity, clocks, and retention.

The first live capture stopped on a venue ticket link redirecting to Dice, where
HTTP 403 was returned. A redirect-disabled request confirmed the original HTTP
302 destination. A browser-navigation guard did not yield a valid capture and
was replaced with redirect-disabled HTTP requests and inert HTML parsing.
The completed capture never follows external ticket redirects. Such records
reject; ordinary HTTP failures still fail the capture. No partial capture was
published and no access control was bypassed.

Six raw and six API records were inspected. Two complete captures agreed on 44
records; 43 were valid and one external-provider record rejected. All published
event terms are 18+. Sampled Lounge listing times of 9:01 p.m. are not treated as
doors: the explicit 9 p.m. doors/music labels supply the normalized times.

| Evidence source | Raw observation or result | Supported finding | Material limit |
| --- | --- | --- | --- |
| `npm run test:blackbox` | 12 passed | Count/range, identity, consistency, redirects and HTTP checks pass | Credentials discovered at runtime, not stored |
| Focused Go and `npm run test:contracts` | Go format/vet/race checks passed; 87 handoff tests passed | Decoder, CLI, retention, API and export contracts pass | Synthetic fixtures never published |
| Staged browser spec with BLACKBOX_BASE_URL on 8097 | Two passed | Desktop/phone links, export and age-14 exclusion pass | Chromium |
| Main catalog hashes and API | 43 events through November 21; all 18+; 19 prior references unchanged; 20 hashes match | App consumes new source without refreshing earlier sources | One unsupported ticket provider rejected; no adult waiver |
| Full browser suite with all live source URLs | 147 passed, three skipped | Combined catalog passes regression suite | Two optional AEG replay checks and one desktop-only layout check on phone skipped |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; standalone unit run 86 passed, one handoff skipped | Applicable host checks pass | Handoff ran in contracts; worktree remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-blackbox-ET3Mkw`,
at `2026-09-12T01:47:59.468Z` (September 11 in Denver).
Stage: `/private/tmp/event-calendar-blackbox-stage.VJwpib`, generation
`g-fa2d6139be3ca14806bc4368487b6628`.
Generation-guarded publication advanced the main catalog from
`g-32588e4b29203c983bde37221ee29ecd` to
`g-fcc68fc7561fe03f60eba48bcde981d1`.
The prior catalog remains in staging as `catalog-before.json`.

## Fillmore Live Nation integration — September 11, 2026

Approved scope: add the official Fillmore profile to the existing Live Nation
capture and decoder, configure separately reviewed admission, test boundaries
and rejection, stage, and publish valid records locally. See the
[Fillmore adapter record](../adapters/0021-livenation-venue-events.md#fillmore-profile-and-admission-review--september-11-2026).
README and registry describe the operator workflow. No app behavior, schema,
prices, scheduling, remote deployment, or other source data changed.

Before implementation, the capture test failed with Unsupported venue; Go tests
for admission/rejection and CLI replay failed with unsupported config. They passed
after the profile was added. Configured All Ages eligibility requires a ticket at
every age; 16+ eligibility starts at 16 and requires valid ID. Missing/unknown
restrictions have no inferred default. Overrides remain authoritative.
The shared browser test now checks ages 0, 2, 3, 14, 15, 16 and 17; Fillmore
adds 16+ and 21+ cases without changing Marquis or Summit policy.

Six raw and six normalized API records were inspected before counts were reported.
Two captures matched pages 36, 13, 0. Of 49 records, three multi-day passes reject
under existing time-consistency checks: Shpongle's midnight timestamp conflicts
with its doors note, and both Decibel passes have multiple labeled times.
The latter includes a three-day offer with a Ratio Beerworks pre-fest; no off-site
event is assigned to Fillmore. Separate daily listings remain.

The first full browser run passed Fillmore but failed the Hi-Dive phone test:
Holy Wave + Queen Serene was outside the capped Week preview. A direct check with
unchanged admission filters found zero visible cards in Week and one in Day.
The test now uses the established phone day-view helper, as the other admission
tests do. No calendar code changed. The focused Hi-Dive desktop/phone run passed
both tests, and host formatting, lint, type/build checks passed again.
The final full browser suite passed 145 tests with three skips: two optional AEG
replay checks and one desktop-only layout check on the phone project.

| Evidence source | Raw observation or test result | Supported finding | Material limit |
| --- | --- | --- | --- |
| `npm run test:marquis` | 10 passed | Shared capture profile and pagination validation pass | Operator capture remains separate |
| Focused container `go test ./internal/livenation ./cmd/ingest` | Passed | Admission, overrides, off-site/pass rejection, retention and replay pass | Synthetic fixtures never published |
| `npm run test:contracts` | gofmt, vet and full Go race tests passed; 87 handoff tests passed | Shared producer/consumer checks pass | Local containers only |
| Shared browser spec with FILLMORE_BASE_URL on 8097 and Summit/Marquis on 8090 | Six passed | Desktop/phone admission boundaries, links and export pass | Chromium |
| Main catalog hashes and API | 46 Fillmore events through April 3, 2027; 26 age-14 eligible; 18 previous references unchanged; all 19 artifact hashes match | App consumes new data without refreshing prior sources | Three multi-day passes rejected; no under-16 exception |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; standalone Vitest 86 passed, one handoff skipped | Applicable host checks pass | Handoff ran in contracts; repository remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-fillmore-OFiZgr`,
at `2026-09-12T01:10:22.678Z` (September 11 in Denver).
Stage: `/private/tmp/event-calendar-fillmore-stage.LrKPwN`, generation
`g-c18483d786329045bf49ff06807aa388`.
Generation-guarded publication advanced the main catalog from
`g-64d7822b22499cdfcfae832270c2d818` to
`g-32588e4b29203c983bde37221ee29ecd`.
The prior catalog is retained as `catalog-before.json` in staging.

## Summit Live Nation integration — September 11, 2026

Approved scope: extend the reviewed Marquis capture/decoder with Summit's own
profile, review admission, test both rooms, stage, and publish once locally.
The [Live Nation ADR](../adapters/0021-livenation-venue-events.md#summit-profile-and-admission-review--september-11-2026)
records retrieval, room IDs, admission evidence, and limitations. README and the
source registry describe the shared operator workflow.

The new capture test failed on the absent profile export; Go room/admission and
CLI replay tests failed on unsupported Summit configuration. Both passed after
adding the profile. Existing Marquis behavior remains tested. The shared browser
test now uses the established phone day-view helper to avoid preview event caps.
No app code, schema, prices, schedules, or other source data changed.

The operator capture ran two fresh public browser enumerations and required
identical pages. Six raw and six app records were inspected before counts were
reported. One two-day pass has midnight as its listing timestamp and 7 p.m. doors
in notes; it was rejected by the existing consistency check rather than given
special pass-time semantics. Separate Itchy-O daily events remain published.
Two user interruptions occurred during verification. Checks were resumed and
completed. A sandbox-blocked localhost API read succeeded with approved access.

| Evidence source | Raw observation or test result | Supported finding | Material limit |
| --- | --- | --- | --- |
| `npm run test:marquis` | 10 passed | Shared capture validation and reviewed profiles pass | Live capture is a separate operator action |
| Focused Go container tests | `go test ./internal/livenation ./cmd/ingest` passed | Both rooms, admission, overrides, rejection and replay contracts pass | Synthetic tests are not published |
| `npm run test:contracts` | Container gofmt/vet/race checks passed; 87 consumer tests passed | All Go and artifact handoff checks pass | No remote deployment tested |
| Staged browser spec with Summit on 8097 and Marquis on 8090 | Four passed | Desktop and phone admission, links, times and export pass | Chromium |
| Main catalog and API | 53 Summit events, 48 with clearance; both Moonroom events present; 17 prior references unchanged; 18 files match hashes | App consumes the published artifact and preserves previous sources | One two-day pass rejected; four 18+ and one 21+ have no adult waiver |
| Full browser suite with all live source URLs, including SUMMIT_BASE_URL | 143 passed, three skipped | Combined local catalog passes regression suite | Two optional AEG replay checks and one desktop-only phone layout check skipped |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; standalone unit run 86 passed, one handoff skipped | Applicable host checks pass | Handoff ran in contracts; worktree remains untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-summit-WcqcIq`,
at `2026-09-12T00:33:38.734Z` (September 11 in Denver).
Stage: `/private/tmp/event-calendar-summit-stage.HaNOeT`, generation
`g-22a1595dab4357d6b9b880e7ead017c4`.
Generation-guarded publication advanced the main catalog from
`g-d3388f1093d26bd65f100cb1ab474958` to
`g-64d7822b22499cdfcfae832270c2d818`.
The previous catalog is retained as `catalog-before.json` in staging.
Raw capture and staging remain available for inspection; no automatic refresh
or remote deployment was added.

## Ball Arena KSE calendar integration — September 11, 2026

Approved scope: KSE calendar feed plus official listing enrichment, venue admission
review, staged tests, and one-time local publication. The
[KSE ADR](../adapters/0009-kse-event-apis.md#ball-arena-implementation-and-admission-review--september-11-2026)
documents the contract and limits. README and registry now describe the operator
workflow. No app behavior, prices, schedules, remote deployment, or other source
data changed.

Capture and decoder tests failed on missing implementations before production
code was added. The replay test failed on the missing command before wiring it.
The existing replay test helper was extended for Ball's title field and paired
HTML snapshot; it still tests Paramount and Marquis. Tests cover semantic listing
rechecks, identity reconciliation, unknown dates, invalid records, game-policy
boundaries, configured overrides, retained URLs, failed-job protection, and removal.

The first full browser run passed Ball but reproduced the previously recorded
Show All test failure: an immediate `boundingBox()` read returned null. The same
failure reproduced in isolation against the fixture app, which does not consume
Ball data. The test now retries the height measurement with `expect.poll` while
retaining its expected height and hover-size assertions. No calendar code changed.
Focused Show All/month/week checks passed seven tests with one inapplicable phone
layout skip. This supersedes the earlier unresolved immediate-measurement failure.

The final full browser run with `BALL_BASE_URL` and all established live/empty
source URLs passed 141 tests. Three checks skipped: two optional AEG replay checks
and one desktop-only layout check on the phone project.

| Evidence source                                                                         | Raw observation or result                                                                                                      | Supported finding                                                       | Material limit                                                       |
| --------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Raw capture, HTML cards, and eight normalized samples                                   | 119 dated events; 120 listing cards; one extra card marked TBA                                                                 | Available dated inventory reconciles and publishes through May 16, 2027 | The undated NBA Cup record is rejected                               |
| `npm run test:kse`                                                                      | 13 passed                                                                                                                      | Both KSE capture workflows pass validation tests                        | Ball's HTML semantic checks run in Go                                |
| `npm run test:contracts`                                                                | Container gofmt/vet/race checks and 87 consumer tests passed                                                                   | Decoder, CLI, store, API and export contracts pass                      | Operator-only workflow                                               |
| `BALL_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/ball.spec.ts`    | Two passed                                                                                                                     | Staged desktop/phone links, exports, and admission boundaries pass      | Chromium                                                             |
| Main API and catalog hashes                                                             | 119 Ball events; 85 game clearances and 34 unknown policies; no prices; 16 old references unchanged; all 17 files match hashes | Main app consumes the new data without refreshing other venues          | Unknown concert admission remains excluded from With adult           |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check` | Passed; build includes typecheck; 86 unit tests passed and standalone handoff skipped                                          | Applicable host checks pass                                             | Handoff ran separately in contracts; worktree files remain untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-ball-k4WvMR`,
at `2026-09-11T23:32:03.666Z`. Stage:
`/private/tmp/event-calendar-ball-stage.dPNckR`, source generation
`g-913e8f1e86fc01c3b568573d55fde3f9`. Main catalog advanced from
`g-dccbc076d9d28d60b70621655873686e` to
`g-d3388f1093d26bd65f100cb1ab474958` by generation-guarded publication.
The stage preserves `catalog-before.json`; prior immutable source files remain
available for recovery. The ingestion image was built and run locally, and staging
and main app containers consumed its output. No app-code change required an app
image rebuild. Markdown source was inspected; there is no documentation renderer.
No commit or remote deployment was made.

## Marquis Live Nation integration — September 11, 2026

Approved scope: keep Marquis ahead of Ball, capture its normal public browser
scroll flow, normalize event responses in Go, review admission rules, and publish
only after staging validation. [ADR 0021](../adapters/0021-livenation-venue-events.md)
records retrieval, mapping, policy, and operating limits. README, JSON-LD ADR,
and registry now identify this selected adapter. No Ball work, UI behavior change,
prices, schedule, or remote deployment is included.

Test-first capture and decoder tests failed on missing implementations. The first
decoder run then failed because the fixture used `All Ages` instead of the schema's
`All ages` category; mapping and fixtures were corrected. The new replay test failed
on the missing command before wiring it. The existing KSE retention/API test was
parameterized to exercise the same publication boundary for Marquis without
duplicating the workflow assertions.

The first full browser run passed Marquis but failed Ogden and Lost Lake phone
visibility assertions. Both tests assumed eligible events always fit in the Week
preview. With the additional events, they fell behind Show All. A read-only
reproduction observed zero matching visible cards before expansion and one after
expansion for each event, with admission filtering unchanged. The tests now open
the event's day through the existing UI before asserting admission visibility.
All eight focused phone admission tests passed. No app code changed.

A subsequent full run hit a separate Show All test failure: `boundingBox()` returned
null at its immediate height read. That test passed unchanged in isolation. This
is an observed intermittent test failure; its underlying timing cause was not
isolated or repaired as part of ingestion.

The final unchanged full-suite rerun passed 139 tests with three skips (two
optional AEG replay checks and one desktop-only layout check on the phone project).
This verifies Marquis against the populated main app while retaining the earlier
intermittent Show All failure as a known test limit.

| Evidence source                                                                                    | Raw observation or result                                                                                      | Supported finding                                                                 | Material limit                                                         |
| -------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| Operator capture and staged artifact                                                               | Two identical enumerations, pages 36/36/2/0; eight normalized samples inspected; 74 valid, zero rejected       | Available Marquis listings ingested through February 10, 2027                     | No future announcement or recurring retrieval guarantee                |
| `npm run test:marquis`                                                                             | Nine passed                                                                                                    | Complete/empty, changed, partial, duplicate, and malformed-page validation passes | Browser transport also checked through the real capture                |
| `npm run test:contracts`                                                                           | Container gofmt, vet, race tests and CLI tests passed; 87 consumer tests passed                                | Normalization, overrides, retention, publication, API and export contracts pass   | No remote deployment                                                   |
| Staging `MARQUIS_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/marquis.spec.ts` | Two passed                                                                                                     | Desktop and phone age boundaries, modal links and exports work                    | Chromium                                                               |
| Main API and catalog/source digest checks                                                          | 74 events: 73 All Ages and one 18+; no prices; 15 prior source references unchanged; all 16 files match hashes | Main app consumes the new artifact without changing other venues                  | Unknown/restricted admission receives no inferred waiver               |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check`            | Passed; build includes typecheck; 86 unit tests passed and standalone handoff skipped                          | Applicable host checks pass                                                       | Handoff ran separately in contracts; repository files remain untracked |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-marquis-L9NIpf`,
at `2026-09-11T21:30:28.278Z`. Stage:
`/private/tmp/event-calendar-marquis-stage.CByFle`, source generation
`g-37e67f8a91c76adff2d7213c70799186`. Main catalog advanced from
`g-ae23d52cfeed0bd459d15f6e4dc657d7` to
`g-dccbc076d9d28d60b70621655873686e` by generation-guarded publication.
`catalog-before.json` in the staging directory preserves the prior catalog;
immutable source files remain available for recovery. The ingestion image was
built and run locally; staging and main app containers consumed its output.
Browser capture runs on the operator host using existing Playwright dependencies.
The unchanged app image did not require rebuilding. Markdown source was inspected;
no documentation renderer is configured. No commit was created.

## Paramount KSE integration — September 11, 2026

Continued the approved dedicated-venue scope with the public array used by
Paramount's own calendar. The selected approach reuses snapshot replay, admission
overrides, reconciliation, and atomic publication. Direct Ticketmaster scraping
and a guessed paginated API were not needed. Ball's different KSE schema, other
venues, UI changes, remote deployment, and scheduled ingestion remain out of scope.
The [KSE ADR](../adapters/0009-kse-event-apis.md) records mapping and admission review.
README and the source registry describe the new workflow and local publication.

Test-first checks failed on the missing decoder, capture module, and replay command.
A conflict test then exposed accepted contradictory age statements; the final
decoder rejects them and retains the fixed policy-review date across captures.
Staging rejected two rescheduled shows whose doors timestamps still pointed to
their original May dates. They were not published or silently assigned new times.

| Evidence source                                                                            | Raw observation or result                                                                                                     | Supported finding                                                            | Material limit                                                           |
| ------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Captured array and staged artifact                                                         | Eight raw and normalized rows inspected; 74 input rows, 72 valid and two rejected; latest April 10, 2027                      | Current public array ingested with date and venue checks                     | Available listings, not a promise of a full year of future announcements |
| `npm run test:kse`                                                                         | Eight passed                                                                                                                  | Capture rejects malformed, changing, duplicate, capped, and failed responses | Synthetic transport tests plus the one-time real capture                 |
| `npm run test:contracts`                                                                   | Container gofmt/vet/race build passed; handoff and 87 consumer tests passed                                                   | Decoder, replay retention, publication, and consumer checks pass             | No recurring retrieval                                                   |
| Staging `KSE_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/kse.spec.ts` | Two passed                                                                                                                    | Phone and desktop admission boundaries, modal links, and exports work        | Chromium                                                                 |
| Full `npm run test:e2e` with KSE and established live/empty source URLs                    | 137 passed, three skipped                                                                                                     | Existing app and new source browser checks pass                              | Two optional AEG replay checks and one desktop-only phone check skipped  |
| Main `/api/calendar` and catalog/source files                                              | 72 Paramount events; no price fields; 63 unknown policies; 14 previous references unchanged; all 15 files match their digests | Local publication preserves other sources and does not invent clearance      | Unknown/recommended policies do not qualify for With adult               |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check`    | Passed; build includes typecheck; 86 unit tests passed, standalone handoff skipped                                            | Applicable host checks pass                                                  | Handoff verified separately by contracts; files are untracked            |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-kse-paramount-yef66m`,
at `2026-09-11T20:03:52.900Z`. Stage:
`/private/tmp/event-calendar-kse-stage.V0YB28`; source generation
`g-50ae4003211dc3e4e67898c30dd28e7c`. Main catalog advanced from
`g-b6a925b6116b1dccca2875d2b454351d` to
`g-ae23d52cfeed0bd459d15f6e4dc657d7` through generation-guarded publication.
The stage contains `catalog-before.json`; prior immutable source artifacts remain
available for recovery. The ingestion image was built locally and its actual
output was consumed by staging and main app containers. No application-code change
required an app image rebuild. Markdown source was inspected; no documentation
renderer is configured. No commit or remote deployment was made.

## Hi-Dive Plot integration — September 11, 2026

Resumed the approved dedicated-venue integration scope. Hi-Dive now has an
operator-only capture, offline Go decoder, `replay-plot` CLI, synthetic fixtures,
and a reviewed parent/legal-guardian policy. Its mapping, enumeration, access
review, and limits are in [ADR 0005](../adapters/0005-plot-listings-api.md).
The source registry and README describe the current workflow. No aggregator,
scheduled job, price enrichment, frontend production change, or remote deployment
was added.

Test-first work initially failed for the absent decoder and capture module, then
for the missing CLI dispatch. The unknown-age test initially dereferenced an
absent event-level policy. Inspection confirmed the established consumer uses
`artifact.EffectivePolicy`, which falls back to the venue policy. The test now
uses that helper, and the CLI/API and browser tests verify that boundary rather
than duplicating the fallback onto every event. No shared policy logic changed.

| Evidence source                                                                                     | Raw observation or result                                                                              | Supported finding                                                                                                       | Material limit                                                                                |
| --------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| Eight raw capture samples, all page identities, terminal probe, repeated first page                 | 33 records across seven pages; dates September 11–November 20; page eight empty; page one stable.      | Current exposed listings were enumerated.                                                                               | API is not a versioned snapshot and has no date-range selector.                               |
| Staged artifact and eight normalized sample records                                                 | 33 valid events, zero rejections, zero prices; all effective policies contain guardian clearance.      | Mapping and venue-policy fallback are valid.                                                                            | Unknown event age categories remain unknown; only explicit restriction statements are mapped. |
| `npm run test:plot`                                                                                 | Ten capture tests passed.                                                                              | Pagination failures, identity changes, HTTP errors, and empty confirmation are checked.                                 | Mock transport, supplemented by live capture.                                                 |
| `npm run test:contracts`                                                                            | Go formatting, vet, full race tests, handoff producer, and all 87 frontend contract/unit tests passed. | Decoder, configured override precedence, malformed records, retention, and real API/ICS boundaries pass.                | No live provider mutation simulated.                                                          |
| `PLOT_BASE_URL=http://127.0.0.1:8097 npm run test:e2e -- tests/browser/plot.spec.ts`                | Both browser projects passed against isolated staged data.                                             | Known/unknown restrictions, guardian clearance at ages 0/14/17, policy/event/ticket links and ICS work through the app. | A parent/legal guardian is required, not merely an older friend.                              |
| Full browser suite, including `PLOT_BASE_URL=http://127.0.0.1:8090` and established live/empty URLs | 135 passed, three skipped.                                                                             | Main-catalog ingestion and existing UI workflows pass.                                                                  | Two optional AEG replay checks and one desktop-only phone check skipped.                      |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check`             | Passed; build includes typecheck; 86 tests passed and standalone handoff skipped.                      | Applicable host checks pass.                                                                                            | The handoff ran separately through `test:contracts`.                                          |
| Ingestion image build, generation-guarded publication, all source referents checked                 | Durable publication; all 13 previous source references unchanged; 14 referenced source files exist.    | Hi-Dive was added without refreshing other venues.                                                                      | Local publication only.                                                                       |

Capture: `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-plot-hi-dive-TyOa9Z`,
captured at `2026-09-11T19:05:49.025Z`. Stage:
`/private/tmp/event-calendar-plot-stage.nU5hxv`; staged source generation
`g-0aa2cdc7636d3a3d9c483fe70f568294`. Main catalog advanced from
`g-f3b832a793813de69d8cc492900db391` to
`g-b6a925b6116b1dccca2875d2b454351d`. The prior catalog is preserved as
`catalog-before.json` in the staging directory; prior immutable source files
remain available for rollback. Temporary preview containers can be stopped without
removing these artifacts. Markdown source was inspected and formatting checked;
no documentation renderer is configured.

## Week today pill includes the weekday — September 11, 2026

Approved refinement: today's full Week header text (for example `Fri 11`) sits
inside a purple pill instead of highlighting only the date number. The shared
Week label remains 24px high; the column tint and click-to-day behavior are
unchanged. README and product ADR supersede the earlier number-only circle.
The updated test first failed with expected `Wed 9`, received `9`. It now checks
the full label, pill dimensions, navigation, and phone exclusion. The rebuilt
fixture screenshot was inspected.

| Evidence source                                                                                                           | Raw observation or result                                                            | Supported finding                                 | Material limit                                                           |
| ------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------- | ------------------------------------------------------------------------ |
| `npm run test:e2e -- tests/browser/week-today.spec.ts tests/browser/week-fit.spec.ts tests/browser/grid-capacity.spec.ts` | Six passed.                                                                          | Full-label marker, navigation, and capacity pass. | Desktop sizing in both browser projects.                                 |
| Full browser suite with established live-source and empty-calendar environment                                            | 133 passed, three skipped.                                                           | Existing UI workflows pass.                       | Two optional AEG replay checks and one desktop-only phone check skipped. |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check`                                   | Passed; build includes typecheck; 86 unit tests passed and one handoff test skipped. | Applicable frontend checks pass.                  | Paused Hi-Dive Go work remains unverified and unpublished.               |
| Compose rebuild and inspected Week screenshot                                                                             | App and fixture healthy; full label highlighted.                                     | Change available locally.                         | No remote deployment.                                                    |

Markdown source inspected and formatting checked; no documentation renderer is
configured. No ingestion behavior changed for this UI refinement.

## Week today marker and column tint — September 11, 2026

Approved change: retain the custom clickable Week headers, but render weekday
and date number separately. Today's number gets a 24px purple circle with white
text. Only today's desktop Week cell receives the faint plum `#15121c` background.
Month, Day, phone lists, event cards, and date drill-in keep their existing behavior.
README and product ADR describe the presentation.

Read-only browser comparison isolated the missing marker: the custom header had
no colored background; disabling only that renderer restored FullCalendar's
purple date circle. The new browser test failed before implementation because
the marker was absent. It now checks circle geometry/colors, today's cell tint,
other dates, day drill-in, navigation away/back, and switching to phone. The
Week screenshot was inspected. White-on-purple contrast uses the existing tested
theme pair; event text remains on unchanged card surfaces.

| Evidence source                                                                          | Raw observation or result                                                           | Supported finding                                       | Material limit                                                                                                                                                 |
| ---------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Focused Week today, capacity, calendar, and theme browser run                            | 34 passed; two empty-calendar checks failed because the empty URL was not supplied. | Marker, navigation, and capacity checks passed.         | Empty-state checks required corrected environment.                                                                                                             |
| Full suite with fixture 8091, live app 8090, empty app 8096                              | 133 passed, three skipped.                                                          | UI regression checks pass with the correct environment. | Two optional AEG replay checks and one desktop-only phone check skipped.                                                                                       |
| `npm run format:check`, `npm run lint`, `npm run build`, `npm test`, `git diff --check`  | Passed; build includes typecheck; 86 unit tests passed, one handoff test skipped.   | Applicable frontend checks pass.                        | Initial formatting check identified the two paused Plot capture files; these were formatted without behavior changes. Hi-Dive decoder tests remain unfinished. |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait`; Week screenshot | App and fixture healthy; circle and tint visible.                                   | Change is available locally.                            | No remote deployment or new venue publication.                                                                                                                 |

Full browser command: `npm run test:e2e` with the established live-source and empty
environment variables. Markdown source was inspected and formatting checked;
no documentation renderer is configured. Ingestion work remains paused at the
unknown-age test failure; it is not part of this UI completion claim.

## Show All visual emphasis — September 11, 2026

Approved trial: muted plum `#292238`, bold lavender `#c4b5fd`, brighter plum
`#382e4c` on hover. The existing desktop overflow and phone synthetic controls
share the treatment. No padding, font size, or height changed. An inset 3px
keyboard-focus outline stays inside the compact control. README and product ADR
record the treatment. Only presentation CSS and browser tests changed.

The extended Show All tests failed before implementation because both controls
had transparent backgrounds. They now check normal/hover colors, weight, focus,
unchanged height (approximately 22px desktop and 44px phone), centering, and day
navigation. The existing contrast test covers both new color pairs at 4.5:1 or
better. Desktop and phone screenshots were inspected after removing test focus
to show the normal background.

| Evidence source                                                                                 | Raw observation or result                                              | Supported finding                                                | Material limit                                                               |
| ----------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- | ---------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| Focused Show All, grid-capacity, Month, Week, and theme tests against rebuilt fixture container | 11 passed, one desktop-only phone check skipped.                       | Styling, contrast, focus, height, overflow, and navigation pass. | Chromium and emulated phone; visual preference remains a user judgment.      |
| Full browser suite against fixture 8091, app 8090, empty app 8096                               | 131 passed, three skipped.                                             | Existing UI workflows pass.                                      | Two optional AEG replay checks and one desktop-only phone check skipped.     |
| Formatting, lint, typecheck, build, unit tests, and diff check                                  | Passed; 86 unit tests passed, one handoff test skipped.                | Applicable frontend checks pass.                                 | No ingestion/schema changes; unfinished Hi-Dive Go work was not revalidated. |
| Compose rebuild; inspected desktop/phone Show All screenshots                                   | App and fixture healthy; muted background and centered labels visible. | Trial is available locally.                                      | No remote deployment.                                                        |

Commands: `npm run test:e2e -- tests/browser/show-all.spec.ts tests/browser/grid-capacity.spec.ts tests/browser/month-fit.spec.ts tests/browser/week-fit.spec.ts tests/browser/theme.spec.ts`,
`npm run test:e2e` with the established live-source environment variables,
`npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`,
`npm test`, `git diff --check`, and
`docker compose -f compose.yaml -f compose.test.yaml up --build --wait`.
Markdown source was inspected and formatting checked; no documentation renderer
is configured.

## Desktop grids reserve only the overflow-link height — September 11, 2026

The user's screenshot exposed a gap in the earlier overflow verification: those
tests proved that counts changed with height, but did not prove that another event
row could not fit below Show All. The approved fix sets `eventSlicing={!fitGrid}`
on FullCalendar. Desktop Month and Week disable slicing; phone and Day retain the
default. This uses the existing calendar component rather than custom placement
or smaller fonts/padding. The projection places each event on one date, so it does
not require slicing multi-day segments.

Inspection of the installed FullCalendar 7.1.0 pixel-placement implementation
showed that slicing reserves a whole event level before measured pruning. A
browser-only override at the actual application call site isolated that setting:
at 2048×1300, September 15 changed from four cards and 38.625px unused below Show
All to five cards and 5.078px unused. Event rows occupied 33.546875px. At smaller
heights, two or three cards remained correct; disabling slicing does not guarantee
an extra card at every height. An initial diagnostic targeted the option definition
rather than the application call site and had no effect; it was corrected before
drawing this conclusion.

`grid-capacity.spec.ts` failed in both browser projects before production changed.
It checks that the space below Show All is less than another event row, allowing
one pixel for borders and rounding, at two Month and two Week heights. Existing
tests cover all-fit, five/six-week layouts, drill-in, resize, and phone caps.
README and the product ADR now describe the measured-height reservation.

| Evidence source                                                                                              | Raw observation or result                                                                                                | Supported finding                                             | Material limit                                                                    |
| ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| Rebuilt app at 8090, September 15 at 2048×1300                                                               | Five visible entries and 5.078125px below Show All, without a browser override.                                          | The original unused-row reproduction is fixed locally.        | Smaller windows can still correctly fit only two or three entries.                |
| Focused grid-capacity, Month, Week, and Show All browser tests                                               | Nine passed, one desktop-only phone check skipped; revised atomic capacity check also passed both projects.              | Capacity, resize, all-fit, drill-in, and phone behavior pass. | Chromium and emulated phone.                                                      |
| Full browser suite against fixture 8091, live app 8090, empty app 8096                                       | Final run: 131 passed, three skipped.                                                                                    | UI regression checks pass.                                    | Two optional isolated AEG replay checks and one desktop-only phone check skipped. |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm test`, `git diff --check` | Passed; 86 unit tests passed and one environment-dependent handoff test skipped.                                         | Applicable frontend checks pass.                              | Unfinished Hi-Dive Go tests were not revalidated; no ingestion code changed.      |
| Compose rebuild and catalog SHA-256                                                                          | App and fixture rebuilt and healthy; catalog remains `ce77d76994b6209ba679162abbfafdc60196302df83bbf8f0d5accc911652039`. | Local behavior changed without a data refresh.                | No remote deployment.                                                             |

The first full suite found a test measurement race: a card disappeared between
separate bounding-box reads during the Week transition. The test now waits for
the target view and measures all geometry in one browser evaluation, returning
false while layout elements are unavailable. No second production change was
needed. Markdown source was inspected and formatting checked; no documentation
renderer is configured.

## Desktop Month uses actual overflow — September 11, 2026

User-approved extension of the Week change: desktop Month also uses no count cap.
Both fitted desktop grids pass all matching events to FullCalendar and use its
automatic day-cell overflow control. Phone Month remains capped at five by default;
phone Week and all Day behavior are unchanged. This supersedes earlier references
to a configured desktop Month cap or a synthetic capped row on tall desktop grids.

Before implementation, the 1440×2000 fixture showed five cards. A browser-only
increase of the cap allowed eight cards to fit; genuine overflow still required
Show All. Initial immediate measurements occurred before layout settled; the
diagnostic was repeated with an explicit wait and only that result was accepted.
New tests reproduced the cap before changing production code: a very tall Month
could not display all 14 records and the 2000px test could not exceed five.

The extended Month tests cover five- and six-week layouts, more than five cards
when space permits, no Show All when all 14 fit, actual overflow on shrink,
day drill-in, restored visibility on expansion, and the retained phone cap.
Existing Month assertions now distinguish the phone cap from desktop capacity.
The centering test covers automatic overflow at both desktop test heights.
README and the product ADR document this desktop-only behavior.

| Evidence source                                                                                              | Raw observation or result                                                                                                | Supported finding                                                                               | Material limit                                                                                           |
| ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `npm run test:e2e -- tests/browser/month-fit.spec.ts tests/browser/show-all.spec.ts`                         | Five passed, one desktop-only check skipped on phone.                                                                    | Month capacity, resize, drill-in, and centered overflow controls work through the container UI. | Chromium desktop and emulated phone.                                                                     |
| Full browser suite, fixture at 8091, live sources at 8090, empty calendar at 8096                            | 129 passed, three skipped.                                                                                               | Existing UI and published-source workflows pass.                                                | Two optional isolated AEG replay checks and one desktop-only phone check skipped.                        |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm test`, `git diff --check` | Passed; 86 unit tests passed and one handoff test skipped.                                                               | Applicable frontend checks pass.                                                                | Handoff environment was not configured; unfinished Hi-Dive Go tests remain outside this UI verification. |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait`; catalog SHA-256                     | App and fixture rebuilt and healthy; catalog remains `ce77d76994b6209ba679162abbfafdc60196302df83bbf8f0d5accc911652039`. | Change is available locally without changing event data.                                        | No remote deployment.                                                                                    |

Markdown source was inspected and formatting checked. No documentation renderer
is configured. Ingestion internals were not changed or revalidated for this UI task.

## Desktop Week uses actual overflow — September 11, 2026

User-approved change: desktop Week no longer applies the configured count before
FullCalendar fits the day cell. `limit` is null for desktop Week; percentage-height
layout and `dayMaxEvents: true` control overflow. Phone Week, Month, Day, and API
configuration validation remain unchanged. This supersedes the earlier responsive
sizing statement that both desktop grids apply a configured count first.

The original 1440×1200 fixture rendered ten cards and Show All in a 1058px-high
day cell, with the last card ending around 542px. Temporarily raising the count
in a browser-only response rendered all 14 cards without Show All. The new test
then reproduced the ten-versus-fourteen failure before implementation.

`week-fit.spec.ts` verifies 14 cards without Show All at 1440×1200, real overflow
at 1024×420, the overflow control within the day boundary, all 14 events after
day drill-in, expansion after resize, and the unchanged ten-event phone list.
Both browser projects passed this test against rebuilt Compose containers.
Existing calendar tests were updated for the new desktop behavior; their first
run exposed a phone-only date selector used on desktop. It was replaced with the
existing desktop day-header selector, without another production-code change.

Source integration was paused for this UI request. Hi-Dive's test-first adapter
work remains in progress and has not been published. No source artifacts changed.

| Evidence source                                                                                              | Raw observation or result                                                                                                           | Supported finding                                                      | Material limit                                                                                         |
| ------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Full browser suite with live venue URLs at 8090, fixture at 8091, empty at 8096                              | Final run: 127 passed, three skipped.                                                                                               | Overflow, resize, navigation, existing source, and filter checks pass. | Two optional AEG replay checks and one desktop-only phone check skipped.                               |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm test`, `git diff --check` | Passed; 86 unit tests passed, one environment-dependent handoff test skipped.                                                       | Applicable frontend checks pass.                                       | Go adapter work is still in its separate test-first phase; no ingestion behavior changed for this fix. |
| Compose rebuild and catalog SHA-256                                                                          | App and fixture containers rebuilt and running; catalog remains `ce77d76994b6209ba679162abbfafdc60196302df83bbf8f0d5accc911652039`. | Fix is available locally without a data refresh.                       | No remote deployment.                                                                                  |

The first full browser run exposed a timing issue in the responsive test: it
recorded the Month title before the Week transition completed. The test now waits
for the expected Week title before recording the range and resizing. The corrected
full run passed. README and product ADR describe the new desktop-only rule;
Markdown source was inspected and formatting checked. No documentation renderer
is configured. The temporary empty test container was stopped after checks.

## Dropdown label activation — September 11, 2026

Approved fix: clicking or tapping text or padding in the Venue and Age dropdowns
activates the existing checkbox without closing the dropdown. All clears selected
values. Escape, outside-click dismissal, Tab dismissal, and native keyboard
activation remain unchanged. The product contract records the row behavior.

Browser tracing showed label mousedown moved focus from the trigger to the drawer
dialog. The dropdown's blur handler then removed the label before click. A temporary
diagnostic suppression of blur allowed the native label to toggle the checkbox.
The shared component now focuses the label's checkbox during primary mousedown
and prevents that mousedown's default focus transfer. It does not intercept
pointerdown or manually toggle state. This preserves native label activation and
avoids pointer-state tracking, delayed blur timers, or duplicate click handlers.

| Evidence source                                                                          | Raw observation or test result                                                                          | Supported finding                                         | Material limit                                                                              |
| ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| New browser regressions before implementation                                            | Four tests failed because clicking or tapping the label removed its checkbox before selection.          | Both dropdowns reproduce the reported defect.             | First attempt had an invalid nested selector; it was corrected before these failures.       |
| `npm run test:e2e -- tests/browser/filters.spec.ts` after rebuild                        | 16 passed, including text/padding activation, All, Space, Escape, Tab, outside clicks, and persistence. | Fixed behavior works through the container UI.            | Chromium desktop and emulated phone touch.                                                  |
| Full browser suite with all live source URLs at 8090 and empty URL at 8096               | 117 passed, three skipped.                                                                              | Existing calendar, modal, and live-source workflows pass. | Two optional isolated AEG replay checks and the desktop-only layout check on phone skipped. |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `npm test` | Passed; 86 unit tests passed and the handoff test skipped without its environment.                      | Host checks pass.                                         | Initial format check required a second formatting pass on the new tests.                    |
| `npm run test:contracts`                                                                 | Go check image built from unchanged cached Go layers; handoff and all 87 frontend tests passed.         | Cross-language handoff is verified separately.            | No ingestion or schema code changed.                                                        |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait`                  | Both images rebuilt and app/fixture services healthy at 8090/8091.                                      | Fix is available locally.                                 | No remote deployment.                                                                       |
| Catalog SHA-256 before and after rebuild; `git diff --check`                             | Catalog remains `ce77d76994b6209ba679162abbfafdc60196302df83bbf8f0d5accc911652039`; diff check passed.  | Local catalog is unchanged.                               | Untracked files mean diff check alone is not a formatting or scope check.                   |

Verification used the existing fixture/public API interfaces. Ingestion internals
and provider APIs were not reinvestigated because the change affects only the
shared dropdown interaction. Documentation Markdown source was inspected; no
documentation renderer is configured. No commit was created.

## Red Rocks Clique integration — September 11, 2026

Approved scope: one additional venue, operator capture, snapshot normalization,
admission review, tests, and one-time local publication. No frontend production
change, scheduled job, price field, or Railway deployment was added.
The implementation and upstream limitations are in [ADR 0003](../adapters/0003-wordpress-clique-api.md).

| Evidence source                                                                                                    | Raw observation or test result                                                                                                                            | Supported finding                                                                                | Material limit                                                                                                 |
| ------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------- |
| Capture `event-calendar-clique-red-rocks-SVb2GY`, September 11 at 14:44:59.912Z                                    | Upcoming, padded full-year range, and two populated partitions agree; replay reports no rejections.                                                       | 71 records can be published for September 11, 2026 through September 11, 2027.                   | Covers currently announced records, including non-music events; no provider total guarantee.                   |
| Five published records inspected: Hasan Minhaj & Ronny Chieng, stair climb, Clipse & J.I.D, Bleachers, Sammy Virji | Actual titles, dates, links, summer/winter offsets, optional doors, and admission policies are present.                                                   | The records represent events and survive serialization and HTTP consumption.                     | Sample of mapping; unit tests cover malformed inputs.                                                          |
| Local catalog and source checksums                                                                                 | New generation `g-f3b832a793813de69d8cc492900db391`; all 12 prior references and file checksums unchanged.                                                | Red Rocks adds the thirteenth source without replacing existing venue data.                      | Ignored local artifacts only.                                                                                  |
| Running localhost:8090 API                                                                                         | 71 Red Rocks events, all with an age-14 permission range; one cancelled event.                                                                            | Reviewed admission metadata reaches the consumer.                                                | Permission does not make a cancelled event attendable; event conditions remain authoritative.                  |
| Staged desktop and phone browser checks                                                                            | Six passed: All Ages boundaries, 13+ excludes age 12 and includes 13/14, policy and ticket links, modal and ICS cancellation. Phone screenshot inspected. | Source data works through the existing UI and export interfaces.                                 | Chromium only; no changes to application UI.                                                                   |
| `npm run test:clique`                                                                                              | Seven capture tests passed.                                                                                                                               | HTTP failures, limits, overlap requests, partitions, and explicit empty results are covered.     | Stubbed requests; live behavior separately captured above.                                                     |
| `docker build --target ingestion -t event-calendar-ingestion .`; `npm run test:contracts`                          | Image build, Go formatting, vet, all Go race tests, handoff, and 87 frontend tests passed.                                                                | Mapping, admission overrides, failure retention, disappearance, publication, HTTP, and ICS pass. | Test-first failures were observed before adding the decoder and 13+ permission handling.                       |
| Full `npm run test:e2e` with live venue URLs at 8090 and empty URL at 8096                                         | 113 passed, three skipped.                                                                                                                                | Fixture UI and all integrated live venue checks pass.                                            | Two AEG replay checks need their isolated workflow; one desktop-only layout test intentionally skips on phone. |
| `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; `git diff --check`                   | Passed.                                                                                                                                                   | Applicable host checks pass.                                                                     | Files are untracked; diff check alone cannot validate formatting.                                              |

The first capture aborted on HTTP 500 for empty-month probes. The next staged
replay rejected a missing morning boundary event. Neither attempt touched the
main catalog. Populated partition checks and one-day overlap resolved the observed
inputs without treating failures as empty results. The first general browser run
had two empty-startup failures because the temporary container lacked `/data`;
logs confirmed the missing directory. A fresh container with an empty directory
passed the full rerun. No application change was needed for that test setup error.

`npm run test:aeg` subsequently built its isolated publisher and application,
replayed fixtures, and passed both desktop/phone replay checks. The only remaining
inapplicable test is the desktop-only layout check on the phone project.

Final stage: `/private/tmp/event-calendar-clique-final.XvKzcZ`. The previous local
catalog is retained there as `catalog-before-local.json`; old source files remain
on disk. The approved source was promoted with `publish --expect` after staged
browser checks. Refresh localhost:8090 to load it. A later Railway snapshot build
can include the new source; no upload was performed. Documentation source was
inspected and formatted; this repository has no documentation rendering workflow.

- Status: Local calendar, publication, event links, and ICS export verified; scheduled ingestion and deployment remain proposed.
- Date: 2026-09-08
- Scope: Implementation sequence and verification records.
- Authority: [Product contract](0014-product-and-delivery-contract.md).
- Related: [Frontend](0015-frontend-and-calendar.md), [ingestion](0016-ingestion-execution-and-reconciliation.md), [artifacts](0017-artifact-contract-and-publication.md), [deployment](0018-storage-and-container-deployment.md)

## Railway image-baked snapshot — 2026-09-10

Approved scope: replace the Pages frontend reference with `Dockerfile.railway`,
package the validated local catalog and referenced artifacts, and provide a local
upload context that includes ignored data without committing it. The default
Dockerfile, Compose configuration, application routes, and local artifact files
are unchanged. See the [deployment decision](0018-storage-and-container-deployment.md#current-decision--september-10-2026)
and [operator commands](../../README.md#railway-snapshot-deployment).

| Evidence source                                                                                  | Observation/result                                                                                                                                                                            | Supported finding                                                           | Material limit                                                                                                                                                                        |
| ------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cmd/package-snapshot` tests                                                                     | Tests first failed for the missing implementation, then passed for byte preservation, exclusion of unused files, missing/corrupt catalog or source rejection, and refusal to overwrite output | Packaging reuses the existing validation contract                           | Build packaging, not concurrent live publication                                                                                                                                      |
| Exported `upload-context`                                                                        | Five catalog references inspected; all 12 source files match original bytes and checksums; only those files and the catalog are under `.artifacts/`                                           | Local ignored data is present in the upload context without old generations | Source-code fixtures remain build inputs; no Railway upload performed                                                                                                                 |
| Build from `/private/tmp/event-calendar-railway-final.3vQX4E` alone                              | `docker build -f .../Dockerfile.railway -t event-calendar-railway ...` succeeds                                                                                                               | Exported context is sufficient to build the complete image                  | Local Docker engine; some build layers cached                                                                                                                                         |
| Running `event-calendar-railway-check` image on port 8095                                        | No mounts, read-only filesystem, UID/GID 65532, `PORT=8087`; calendar API equals the existing local app                                                                                       | Baked data works without a volume and the port override works               | Not a remote Railway deployment                                                                                                                                                       |
| HTTP checks against that container                                                               | Health and calendar return 200; five event pages, JSON details, and ICS exports pass; unknown event returns 404                                                                               | Existing public interfaces work from the packaged snapshot                  | Read-time expiry is covered by existing Go tests rather than waiting 90 days                                                                                                          |
| `npm run test:contracts`                                                                         | Formatting, vet, all Go race tests, artifact handoff, and 87 frontend tests pass                                                                                                              | Existing code and new helper pass the container checks                      | Initial new tests used unavailable preview fixtures; switched to existing contract fixtures before the successful rerun                                                               |
| Browser regression command below                                                                 | 105 pass, three skip                                                                                                                                                                          | Desktop/phone behavior and live-source checks pass                          | Synthetic UI checks use port 8091; live-source checks use the Railway image; two empty-store tests excluded; two optional AEG replay tests and phone-inapplicable layout test skipped |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `git diff --check` | Exit 0                                                                                                                                                                                        | Static checks pass                                                          | Repository remains untracked; diff check alone does not establish scope                                                                                                               |

```sh
LIVE_AEG_BASE_URL=http://127.0.0.1:8095 FEDERAL_BASE_URL=http://127.0.0.1:8095 HMT_BASE_URL=http://127.0.0.1:8095 LIVE_ADMISSION_BASE_URL=http://127.0.0.1:8095 RHP_BASE_URL=http://127.0.0.1:8095 CERVANTES_BASE_URL=http://127.0.0.1:8095 npm run test:e2e -- --grep-invert 'normal startup is empty'
```

The packaged generation is `g-602e55079e66415b7944d673b28b45b3`. The temporary
upload context remains available locally; it does not refresh automatically.
`railway up --help` confirms `--path-as-root`, `--no-gitignore`, and explicit
project/service/environment selection in the installed CLI. No authenticated
upload, remote build, public deployment, or commit was performed. Markdown source
and formatting were inspected; the repository has no documentation renderer.

## Responsive calendar and controls

- Status: Closed September 11, 2026; implementation verified and user accepted sizing.
- Requested: September 10, 2026.
- Scope: Calendar, toolbar, filter drawer, and shared control sizing.

Approved implementation: make the layout adapt to the available window dimensions. Remove the fixed
1,600px calendar-width cap, fit desktop month and week views to available height,
and use bounded responsive sizes for spacing, typography, and filter-drawer width.
Keep the existing FullCalendar component and mobile layout. Do not use whole-page
zoom or transforms to shrink the interface.

Acceptance conditions:

- Resizing updates the layout without reloading or losing date, view, filters,
  selected event, or keyboard focus.
- Desktop month grids with five or six rows fit the available height at supported
  desktop sizes; week view uses available space without exceeding configured
  event limits. Show All still opens the complete day.
- Filter controls remain reachable in narrow or short windows. Preserve readable
  text, 44px control targets, browser zoom, and vertical scrolling where fitting
  everything would violate those minimums. No horizontal page overflow.
- Large windows use available width without making controls excessively large.

User approved equal 5vw side gutters with a 12px minimum. Month and Week use the
existing FullCalendar percentage-height and automatic-overflow behavior; the
existing projection enforces the configured cap before fitting. CSS owns the
layout, without a new resize listener, whole-page scaling, or calendar component.
The minimum grid heights are 36rem for Month and 16rem for Week. A short viewport
scrolls vertically. Day and mobile list views retain natural content height.
Spacing uses 12–24px bounds, the date heading 1.15–1.35rem, and the drawer
`min(clamp(280px, 25vw, 400px), calc(100vw - 40px))`. Event font sizes and existing
44px control targets remain unchanged. Minimum tested width: 320 CSS pixels.

| Evidence source                                       | Raw observation or test result                                                                                                                 | Supported finding                                                               | Material limit                                                                           |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| New responsive tests before implementation            | Six failures: fixed width/gutters, fixed drawer width, compressed short Month grid.                                                            | Tests reproduce the accepted sizing gaps.                                       | Synthetic fixture app.                                                                   |
| Sizing, Month fit, drawer, and navigation browser run | 35 passed; one desktop-only check skipped on phone. Five- and six-week Month layouts and Show All passed.                                      | Existing navigation and visibility limits survive the change.                   | Chromium desktop and emulated phone.                                                     |
| Eight responsive checks                               | Passed at widths 320–2560; retained search/focus, week/date, selected event URL, drawer reachability, and short-window scrolling.              | Layout responds without reload or lost state.                                   | Representative viewport sizes, not all devices.                                          |
| Text/reflow checks                                    | 640×450 CSS viewport, with default and 200% root text sizes, passed control reachability and no-horizontal-overflow checks.                    | Enlarged text and the reflow equivalent of a 1280×900 window at 200% zoom work. | Native browser toolbar zoom was not exercised; this is not a browser-zoom certification. |
| Six populated-app measurements                        | At widths 2560/1440/1024/844/390/320, left gutters were 128/72/51.1875/42.1875/19.5/16px; page widths equal viewport widths.                   | Proportional gutters work without horizontal page overflow.                     | Current September listing.                                                               |
| Populated app desktop and short window                | Month bottoms at heights 1440/900/768 were 1428/888/756px; at 844×390 the grid bottom was 640px. Large-screen and 320px screenshots inspected. | Normal desktop sizes fit; short windows scroll as designed.                     | Phone lists intentionally extend below the viewport.                                     |

The first build caught an unrenamed `fitMonth` reference in the synthetic Show All
row. It was corrected to `fitGrid`; type checking and the rebuilt containers then
passed. A browser run started against the old image was not counted as verification
of the new layout. The succeeding run used the rebuilt app and fixture containers.
No ingestion, schema, provider, or deployment workflow was changed.

Final checks: the full browser command used the existing fixture URL at 8091,
all live venue URLs at 8090, and `CALENDAR_EMPTY_URL=http://127.0.0.1:8096`.
It passed 125 tests and skipped three (two optional isolated AEG replay checks,
and the desktop-only layout test on phone). `npm run format:check`, `npm run lint`,
`npm run typecheck`, `npm run build`, and `git diff --check` passed. `npm test`
passed 86 tests with its environment-dependent handoff test skipped.

`npm run test:contracts` built the unchanged Go check image from cached layers,
but Docker twice rejected the existing host `test-results/contracts` bind path.
The same `TestConsumerHandoff` container test and `npm test` were rerun with
`CONTRACT_HANDOFF_DIR` using `/private/tmp/event-calendar-sizing-handoff.01RywN`;
the Go handoff and all 87 frontend tests passed. This is a verified alternate
output location, not a change to the repository test workflow. The Docker mount
failure's underlying cause was not investigated further.

The local app and fixture containers were rebuilt with
`docker compose -f compose.yaml -f compose.test.yaml up --build --wait` and remain
running. The temporary empty test container was stopped after verification.
Catalog SHA-256 remains `ce77d76994b6209ba679162abbfafdc60196302df83bbf8f0d5accc911652039`.
README and product ADR were updated. Markdown source was inspected and formatted;
no documentation rendering workflow is configured. No commit or remote deployment
was made. The user subsequently confirmed that sizing verification looks good
and explicitly requested closure on September 11, 2026. The remaining manual
sizing check is closed by user acceptance. The automated browser-zoom limitation
above remains an accurate description of test coverage, not an open work item.

## Cervantes on-site expansion — 2026-09-10

Approved scope: reuse the RHP adapter for Cervantes’ Masterpiece Ballroom, Other
Side, and dual-room tickets. Separate ticketed performances remain separate
events. External promotions are deferred. The
[venue and admission review](../adapters/0008-rhp-calendar-api.md#cervantes-on-site-extension--september-10-2026)
records the room mapping and official FAQ evidence. Ages 14–15 may attend a
published 16+ event with a parent or guardian. This does not establish permission
for an unrelated older friend, under-14 admission, or exceptions to 18+/21+ shows.

The existing configured admission rules supply this policy. No schema, frontend,
price extraction, or scheduled-ingestion changes were needed. New capture and Go
tests first failed because Cervantes was unsupported, then passed after the
adapter extension. Both plain and pipe-separated doors/show times are supported.

Representative raw records were inspected before counts were reported. Five
published records were then compared with the local API for title, venue, links,
admission rules, and absence of price data.

| Evidence source                                                                                             | Observation/result                                                                                                                    | Supported finding                                                                                               | Material limit                                                                                                                                 |
| ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| Capture at `2026-09-11T01:39:13.751Z` (September 10 in Denver), staged artifact, and local API on port 8090 | 131 captured records; 119 published on-site events; 12 external promotions rejected with the explicit scope reason; no fetch failures | Cervantes is populated locally under one venue                                                                  | One-time capture; external promotions remain deferred; latest on-site date is April 23, 2027                                                   |
| Published admission ranges and API                                                                          | 108 events match With Adult at age 14                                                                                                 | Reviewed FAQ rules reach the application                                                                        | Other records have no supported age-14 permission; admission remains subject to venue conditions                                               |
| Catalog comparison and artifact checksum                                                                    | All 11 prior source references unchanged; Cervantes SHA-256 matches; final generation `g-602e55079e66415b7944d673b28b45b3`            | Publication adds Cervantes without replacing other sources                                                      | Existing immutable artifacts remain on disk                                                                                                    |
| `npm run test:contracts` and `npm run test:rhp`                                                             | Go formatting, vet, race tests, CLI artifact handoff, 87 frontend tests, and six capture tests pass                                   | Adapter, reconciliation, publication interfaces, and capture checks pass                                        | Automated fixtures do not establish future upstream stability                                                                                  |
| `TestCapturedHTMLMatchesRaw` with the retained Cervantes capture                                            | Original and compacted normalization agree for all 131 records, including expected external exclusions                                | Capture compaction preserves consumed data                                                                      | This capture only                                                                                                                              |
| Staged Cervantes browser tests and full live browser suite                                                  | Two staged tests pass; full suite has 107 passes and three skips; age 13 excludes and ages 14–16 include the selected 16+ event       | Desktop/phone filtering, modal policy links, event/ticket links, and ICS export work through running containers | Two dedicated AEG replay tests and one phone-inapplicable layout test skip; separate AEG/publication scripts were not rerun for this extension |
| Format, lint, typecheck, build, and ingestion Docker build                                                  | Checks pass                                                                                                                           | Static checks and container build succeed                                                                       | No public deployment or scheduled refresh                                                                                                      |

Full live browser command:

```sh
CALENDAR_EMPTY_URL=http://127.0.0.1:8093 LIVE_AEG_BASE_URL=http://127.0.0.1:8090 FEDERAL_BASE_URL=http://127.0.0.1:8090 HMT_BASE_URL=http://127.0.0.1:8090 LIVE_ADMISSION_BASE_URL=http://127.0.0.1:8090 RHP_BASE_URL=http://127.0.0.1:8090 CERVANTES_BASE_URL=http://127.0.0.1:8090 npm run test:e2e
```

Original responses, compacted snapshot, and capture report remain in the operator
temporary directory `event-calendar-rhp-cervantes-c5WV7S`. Staging and the prior
catalog backup remain at `/private/tmp/event-calendar-cervantes-stage.cPtiRC`.
These are local evidence, not portable test fixtures. The source registry, RHP
ADR, README, and fixture instructions describe the implemented scope. Markdown
source and formatting are checked; no documentation renderer is configured.

## RHP source expansion — 2026-09-10

Approved scope: implement one RHP adapter and add Lost Lake, Larimer Lounge, and
Globe Hall, including admission-policy review before local publication. The
[RHP ADR](../adapters/0008-rhp-calendar-api.md#implemented-contract--september-10-2026)
records the mapping and venue-policy evidence. Capture remains an explicit operator
action; no scheduled ingestion or frontend configuration was added.

The adapter uses `golang.org/x/net/html` v0.59.0 rather than a handwritten HTML
parser. Its module requirement also updates `golang.org/x/text` to v0.42.0.
`replay-rhp` reuses the existing reconciliation and publication entry point.
New artifacts contain no prices. Unknown admission permissions stay unknown;
on-site event conditions do not carry over to off-site venues.

Five raw records per source were inspected before reporting counts. Titles,
event URLs, local dates/times, ticket links, and detail-page evidence were checked.
Five normalized records per source were then compared with the running local API.

| Evidence source                                                       | Raw observation/result                                                                                                         | Supported finding                                                                                            | Material limit                                                                                                  |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------- |
| Lost Lake capture, artifact, and localhost:8090 API                   | 46 published events; 46 have reviewed age-14 accompaniment; latest returned date March 7, 2027.                                | Lost Lake is populated with event-specific guardian conditions.                                              | One-time capture, not a complete year of announced events.                                                      |
| Larimer capture, artifact, and local API                              | 106 published records; 34 have reviewed age-14 accompaniment; 34 records identify other venues; latest date December 15, 2026. | Broad ingestion preserves Campground/Treehouse attribution without inheriting Larimer's policy.              | Counts describe the source feed, not 106 on-site Larimer shows. Off-site accompaniment review remains deferred. |
| Globe Hall capture, artifact, and local API                           | 61 published events; 60 have reviewed age-14 accompaniment; latest date December 11, 2026.                                     | Explicit guardian conditions and explicit on-site All Ages evidence reach filtering.                         | One event has no supported accompaniment evidence.                                                              |
| CLI staging and local promotion                                       | All three final replays report zero rejections. Final catalog generation is `g-21946c127919621f9500b080036e7edc`.              | 213 records added; 140 match With adult at age 14.                                                           | Source completeness is limited to the returned unpaginated action; no provider total was available.             |
| Catalog comparison and update checks                                  | Original eight source references unchanged; all RHP IDs and paths preserved through the age-boundary update.                   | Existing data and shared URLs remain intact.                                                                 | Old immutable artifact files are retained.                                                                      |
| `npm run test:contracts`                                              | Go formatting, vet, all race tests, real artifact handoff, and 87 frontend tests pass.                                         | Adapter, configuration, reconciliation, HTTP/export, and shared dependency checks pass.                      | Synthetic suite; optional capture comparison requires explicit data.                                            |
| `npm run test:rhp`                                                    | Five capture tests pass.                                                                                                       | Form fields, failed envelopes, off-origin protection, missing details, and JSON-LD preservation are covered. | Does not prove future upstream markup stability.                                                                |
| `TestCapturedHTMLMatchesRaw` with each retained capture               | Original and compacted page normalization agrees for every captured record in all three sources.                               | Compaction preserves consumed event data.                                                                    | Tested September 10 captures only.                                                                              |
| Full `npm run test:e2e` with live source flags                        | 105 pass, three skip; RHP checks cover age 14 and age 16 on desktop and phone.                                                 | Local calendar, modal links, export, existing venues, and admission filters work with the added records.     | Two dedicated AEG replay checks run separately; one desktop-only layout test skips on phone.                    |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:publication`   | Real container publication passes; 81 browser tests pass, 27 optional source/layout checks skip.                               | Existing publication workflow still works.                                                                   | Isolated synthetic store; live-source checks run separately.                                                    |
| `npm run test:aeg`                                                    | AEG replay and both browser checks pass.                                                                                       | Existing adapter dispatch remains functional.                                                                | Synthetic replay, not a live AEG refresh.                                                                       |
| Format, lint, typecheck, build, Docker builds, and `git diff --check` | Commands exit 0.                                                                                                               | Applicable static and container checks pass.                                                                 | Repository remains untracked; diff check alone is not sufficient evidence.                                      |

Corrections made during verification:

- A capture test reproduced whitespace compaction changing a JSON-LD title.
  JSON-LD is now preserved unchanged; snapshots were regenerated from saved raw
  pages. The original responses were not overwritten.
- The larger dataset put a Federal test event behind the week card limit.
  Expanding its date revealed the event. The age-filter test now uses full day
  view. No production calendar behavior changed to satisfy this test.
- An explicit age-16 boundary test caught omission of ordinary admission after
  the under-16 exception ends. Reviewed guardian rules now include ages 16–17
  with “Permitted at this age,” matching the established range convention.

Live browser command used:

```sh
CALENDAR_EMPTY_URL=http://127.0.0.1:8093 LIVE_AEG_BASE_URL=http://127.0.0.1:8090 FEDERAL_BASE_URL=http://127.0.0.1:8090 HMT_BASE_URL=http://127.0.0.1:8090 LIVE_ADMISSION_BASE_URL=http://127.0.0.1:8090 RHP_BASE_URL=http://127.0.0.1:8090 npm run test:e2e
```

Retained capture directories under the operator's temporary directory:
`event-calendar-rhp-lost-lake-SBEEzn`, `event-calendar-rhp-larimer-TAbuiD`, and
`event-calendar-rhp-globe-hall-daSFPH`. Each contains original responses and reports;
the corrected replay file is `snapshot-v2.json`. Final update staging and copies
of the previous records remain at `/private/tmp/event-calendar-rhp-boundary.4BSwZb`.
The earlier catalog backup is at
`/private/tmp/event-calendar-rhp-final.MzKwxG/local-before-catalog.json`.
These are local operator evidence, not portable repository fixtures.

README, the source registry, RHP ADR, and fixture instructions were updated.
Markdown source and formatting were checked; this repository has no documentation
renderer. No public deployment, domain registration, or commit was performed.

## Event links and single-event export — 2026-09-09

> September 10, 2026 price-retirement verification: AEG and HoldMyTicket no longer
> extract prices. Reconciliation removes prices from observations, overrides, and
> retained records. Publication strips prices from legacy candidates before checksums
> are computed. APIs, search, modals, and exports omit price and cost metadata.
> The legacy v1 schema stays readable; existing files and raw captures are unchanged.
>
> | Evidence source                                                     | Raw result                                                                          | Supported finding                                                  | Material limit                                                                          |
> | ------------------------------------------------------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------ | --------------------------------------------------------------------------------------- |
> | `docker build --target go-test -t event-calendar-contract-tests .`  | Formatting, vet, and all package race tests passed                                  | Extraction, reconciliation, publication, and API tests pass        | Synthetic Go fixtures                                                                   |
> | `npm run test:contracts`                                            | 87 TypeScript tests passed with Go handoff                                          | Legacy artifact compatibility remains intact                       | Local contract corpus                                                                   |
> | Full browser suite against fixture and populated app containers     | 99 passed, 3 skipped                                                                | Public UI and source integration tests pass                        | Two optional AEG replay tests and one desktop-only check skipped                        |
> | `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:publication` | Passed; 81 browser tests passed, 21 optional tests skipped                          | Real CLI publication and container consumption work without prices | Isolated synthetic store                                                                |
> | Five previously priced records each from Federal, HQ, and Oriental  | API details and `.ics` omit prices; IDs and links match; stored checksums unchanged | Existing local data requires no re-ingestion or file rewrite       | Fifteen sampled exports; all calendar records also checked for absent price/cost fields |
>
> `npm run test:aeg` also passed the five-source replay/publication checks and both
> desktop/phone replay tests skipped by the main browser run. No live ingestion was
> triggered. The temporary publication and AEG stores are retained for inspection.
>
> Initial regression failures were an obsolete price-preservation assertion and an
> obsolete `$20` UI assertion. Concurrent browser runs also collided in their shared
> test output directory. Expectations were updated and both suites passed when run
> sequentially. One Markdown formatting warning was fixed and rechecked.

Event routes and downloads now use the stored public path and the shared validated
catalog reader. Calendar listings exclude unlisted records; detail/export routes
retain them until venue-local expiry. Cold storage failure returns 503, while a
warm reader uses its last valid snapshot. Unknown/expired events return real 404s.
Native browser history supplies Back/Forward without another routing dependency.
Direct visits use temporary defaults without overwriting saved filters; explicit
filter changes resume persistence. Card clicks reuse loaded data and keep their
view. Price support described in historical verification entries below was retired
September 10, 2026; the current contract omits it from publication and all public views.

The existing pinned golang-ical library serializes one event with CRLF, stable UID,
doors/show fallback, date-only unknown times, and no invented end. Export is a
passive download, not an invitation or subscription. Client imports have not been
tested in Apple Calendar, Google Calendar, or Outlook.

Drafted tests first failed on missing routes. Verification exposed an obsolete
missing-storage 404 expectation, an incorrect drafted venue-button label, a
duplicate detail fetch replacing card data, and a delayed dialog-close event
interfering with Forward navigation. These were corrected. The first broad run
lacked its empty-data test container; later full runs used an isolated empty store.

| Evidence source                                                     | Raw result                                           | Supported finding                                                               | Material limit                                      |
| ------------------------------------------------------------------- | ---------------------------------------------------- | ------------------------------------------------------------------------------- | --------------------------------------------------- |
| `npm run test:contracts`                                            | Go format/vet/race checks and 77 frontend tests pass | Shared API projection and export parser checks pass                             | Synthetic data                                      |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:publication` | Final run: 48 pass, two dedicated AEG checks skip    | Publication, routes, downloads, history, and existing UI verified in containers | Chromium desktop/phone; no external calendar import |
| `npm run test:aeg`                                                  | Two checks pass                                      | Replayed source artifacts still reach the UI                                    | Isolated fixture store                              |
| Repeated card-link browser test                                     | Ten checks pass                                      | Back/Forward race correction passes repeated desktop/phone checks               | Chromium, fixture data                              |
| Format, lint, typecheck, build                                      | Exit 0                                               | Applicable host checks pass                                                     | No public deployment                                |

Live fetching, HQ ingestion, scheduled execution, object storage, and Railway deployment
remain outstanding. Background-click dismissal is recorded below. HQ research during this
work confirmed floating America/Denver feed times; DAMAG3's ticket page distinguishes
20:00 show time from 19:00 doors. It also supplies a structured USD offer, unlike
the inspected AXS listings. No price filter is restored, and no HQ data was published.

## Background-click dismissal — 2026-09-09

The existing native dialog closes when a primary-pointer interaction starts and
ends on the backdrop outside its bounds. Interior clicks and drags starting inside
the popup do not close it. Pointer cancellation clears the pending dismissal.
The existing close handler owns URL cleanup, focus restoration, and calendar
context. This extends the existing dialog rather than introducing another modal
component or relying on a newer browser-only dismissal attribute.

The new desktop/phone browser test failed first because the popup stayed open
after background interaction. After implementation it passes for mouse clicks,
touch taps, inside clicks, inside-to-outside drags, card focus restoration, and
direct-link dismissal. README and the product ADR describe the behavior.

| Evidence source                                                          | Raw result                          | Supported finding                                              | Material limit                                                                        |
| ------------------------------------------------------------------------ | ----------------------------------- | -------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| Event-link, calendar, and filter browser suites, excluding empty startup | 44 passed                           | Dismissal and existing interactions pass in rebuilt containers | Chromium desktop/phone; empty-startup and ingestion workflows unchanged and not rerun |
| `npm test`                                                               | 76 passed, one handoff test skipped | Frontend regressions pass                                      | No shared schema or Go change                                                         |
| Format, lint, typecheck, build                                           | Exit 0                              | Applicable frontend checks pass                                | No public deployment                                                                  |

Both local app containers were rebuilt and restarted. No event artifacts were
edited. Markdown source was inspected; no documentation renderer is configured.

## Month boundaries — 2026-09-09

The desktop month view now configures FullCalendar with `fixedWeekCount: false`
and `showNonCurrentDates: false`, scoped to `dayGridMonth`. Blank cells preserve
weekday alignment. Day/week views and the phone list view keep their settings.
The test first reproduced an adjacent-month event in September on desktop; the
phone list already passed. After the change, September uses five rows and August
uses six; adjacent-month events disappear from September and remain accessible
by navigating to their own month.

| Evidence source                                                         | Raw result                          | Supported finding                                              | Material limit                                            |
| ----------------------------------------------------------------------- | ----------------------------------- | -------------------------------------------------------------- | --------------------------------------------------------- |
| Calendar, event-link, and filter browser suites excluding empty startup | 46 passed                           | Month boundaries and UI regressions pass in rebuilt containers | Desktop/phone Chromium; unchanged empty startup not rerun |
| `npm test`                                                              | 76 passed, one handoff test skipped | Frontend regressions pass                                      | Go and artifact interfaces unchanged                      |
| Format, lint, typecheck, build                                          | Exit 0                              | Applicable host checks pass                                    | Local deployment only                                     |

README and product ADR updated; Markdown source inspected. Both local app images
were rebuilt and services restarted. No event data was edited.

## Empty banner strip — 2026-09-09

Removed the visible header heading, eyebrow, and tagline. The existing header is
an empty, accessibility-hidden 150px placeholder. Removed desktop/phone top padding
and header margins so filters start at vertical coordinate 150px on both layouts.
Other page padding and the popup's eyebrow styling remain unchanged.

The new browser test first failed on the existing header content, then passed
against rebuilt local containers. README and product ADR updated; Markdown source
inspected. No source data or ingestion code changed. Existing dropdown tests were
updated to click the empty banner instead of the removed heading.

| Evidence source                                                         | Raw result                   | Supported finding                                    | Material limit                                            |
| ----------------------------------------------------------------------- | ---------------------------- | ---------------------------------------------------- | --------------------------------------------------------- |
| Calendar, event-link, and filter browser suites excluding empty startup | Final rerun: 48 passed       | Empty 150px strip and existing interactions verified | Chromium desktop/phone; unchanged empty startup not rerun |
| Unit tests; format, lint, types, build                                  | 76 tests pass; checks exit 0 | Applicable frontend checks pass                      | One Go handoff test skipped; schema unchanged             |

## Month startup default — 2026-09-09

Superseded by the September 12 remembered-view decision above.

Normal startup and reload now select Month on desktop and phone. Direct event links
continue to select the event's Week with temporary default filters. The initial
React view state changed; FullCalendar configuration, filtering, and stored data
remain unchanged. Week-specific browser cases explicitly select Week.

The new startup test first failed on the old Week value, then passed in rebuilt
containers for both layouts, including reload after switching to Week. README and
the product ADR now record the revised default.

| Evidence source                                                | Raw result                          | Supported finding                                                    | Material limit                                                                  |
| -------------------------------------------------------------- | ----------------------------------- | -------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Browser suite excluding empty startup and dedicated AEG replay | 54 passed                           | Month startup, reload, direct-link Week, and UI regressions verified | Desktop/phone Chromium; unchanged empty-store and ingestion workflows not rerun |
| `npm test`                                                     | 76 passed, one handoff test skipped | Frontend unit regressions pass                                       | Schema and Go unchanged                                                         |
| Format, lint, typecheck, build                                 | Exit 0                              | Applicable host checks pass                                          | Local deployment only                                                           |

Both local containers were rebuilt and restarted. No data artifacts were changed.
Markdown source was inspected; the repository has no documentation renderer.

## Left filter drawer — 2026-09-09

The left filter drawer supersedes the earlier banner and horizontal-filter layout.
The drawer starts closed, slides in from the viewport's left edge, and overlays
rather than resizes the calendar. The empty banner now has zero height. Search,
venue, age, past visibility, and reset reuse the same preference/filter logic.
An Active indicator remains visible on the toolbar button when filtering applies.
The native-dialog implementation shares backdrop hit testing and keyboard focus
containment with event details. No UI dependency was added.

The initial drawer test failed on the absent Filters button. After implementation,
the browser suite passed on desktop and phone, including drawer dismissal,
dropdown Escape precedence, focus cycling/restoration, filter persistence while
closed, reload, reset, compact calendar positioning, and existing event dialogs.
Desktop/phone drawer screenshots were inspected. Reduced-motion CSS disables the
entrance animation. README and the product ADR describe the new layout.

| Evidence source                                                | Raw result                          | Supported finding                                               | Material limit                                                                  |
| -------------------------------------------------------------- | ----------------------------------- | --------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Browser suite excluding empty startup and dedicated AEG replay | 56 passed                           | Drawer and existing interactions verified in rebuilt containers | Chromium desktop/phone; unchanged ingestion and empty-store workflows not rerun |
| `npm test`                                                     | 76 passed, one handoff test skipped | Frontend regression tests pass                                  | Go/schema unchanged                                                             |
| Lint, typecheck, build                                         | Exit 0                              | Applicable code checks pass                                     | Local deployment only                                                           |

The app and fixture containers were rebuilt and restarted. No event artifacts
were changed. Markdown source was inspected; no documentation renderer is configured.

## Viewport-fit desktop Month — 2026-09-09

Desktop Month now uses a viewport-height flex layout and FullCalendar's automatic
overflow placement. The existing capped-event projection supplies no more than
the configured month limit plus a presentation-only Show All row. FullCalendar
can hide additional cards as space decreases. Both overflow paths open the full
day. Desktop Month uses the label `Show All` without a count because an automatic
overflow group can include the presentation-only row. Day/Week and phone lists
retain their existing limits, count labels, and scrolling behavior.

The toolbar gap is 8px and footer gap is 12px in desktop Month. Card vertical
padding is 4px; font sizes and the 44px minimum button height remain unchanged.
CSS owns the available height; no resize listener or card-height estimator was
added. FullCalendar owns placement and overflow. Very short rows or long titles
can leave only Show All visible, which still opens all matching events.

The test first reproduced the old overflow: the fixture footer ended at
1220.83px in a 900px viewport. Subsequent test failures exposed selectors for
hidden FullCalendar measurement nodes and tests assuming the third month card
was always visible. Tests now use accessible overflow buttons and open Week when
they require a specific event for unrelated detail assertions.

| Evidence source                                                                                  | Raw result                                                                                                                                   | Supported finding                                                     | Material limit                                          |
| ------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- | ------------------------------------------------------- |
| `tests/browser/month-fit.spec.ts` against rebuilt fixture container                              | Five- and six-week months pass at 1440×900, 1280×800, 1024×768, 1440×1200, and 1440×2000; dense final week drills into all 14 fixture events | Grid fits on resize; configured cap and full-day access remain intact | Chromium; bounded synthetic fixtures                    |
| Populated app at localhost:8090, 1440×900                                                        | Five inspected row bottoms: 269, 417, 565, 713, 860px; footer bottom 888px                                                                   | Final row and footer fit the viewport                                 | September 2026; screenshot inspected                    |
| UI browser suite excluding empty startup and dedicated AEG replay                                | 57 passed, desktop-only sizing test skipped on phone                                                                                         | Existing UI interactions pass                                         | Unchanged ingestion and empty-store workflows not rerun |
| `npm test`                                                                                       | 76 passed, one contract-handoff test skipped                                                                                                 | Frontend regression tests pass                                        | No Go/schema change                                     |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `git diff --check` | Exit 0                                                                                                                                       | Applicable formatting, static, and build checks pass                  | Untracked files are covered by formatter, not Git diff  |
| `docker compose -f compose.yaml -f compose.test.yaml up -d --build app fixture-app`              | Images built; both containers started                                                                                                        | Changes available in local development                                | No external deployment                                  |

Chromium launch and Docker socket access required sandbox escalation. Both
checks ran successfully with approval. README and the product ADR were updated;
Markdown source was inspected because no documentation renderer is configured.
No ingestion jobs ran and no event artifacts were modified.

## Retained dates by default and footer removal — 2026-09-09

The timezone footer and Include past events control are removed. Filtering no
longer applies a date cutoff. The server still controls the retained, listed
record set and its 90-day expiry. The existing preference key remains valid;
reading it discards legacy `includePast` values without losing venue, age, or
search selections. Reset and direct-link defaults include all retained dates.
The Today date still refreshes; no clock value is needed by filtering.

Tests first failed on excluded past records, preserved obsolete preferences,
and the existing footer. After implementation, verification produced:

| Evidence source                                                                     | Raw result                                        | Supported finding                                                                                           | Material limit                                                                  |
| ----------------------------------------------------------------------------------- | ------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `npm test`                                                                          | 77 passed, one handoff test skipped               | Date filtering removed; legacy preferences migrate without losing other filters                             | Go/schema unchanged                                                             |
| `npm run test:e2e -- --grep-invert 'normal startup is empty\|AEG replay artifact'`  | 59 passed, one desktop-only test skipped on phone | Retained dates, removed controls, reload, reset, month fit, dialogs, and filters pass in rebuilt containers | Chromium desktop/phone; unchanged empty-store and ingestion workflows not rerun |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`        | Exit 0                                            | Applicable frontend checks pass                                                                             | Local build only                                                                |
| `docker compose -f compose.yaml -f compose.test.yaml up -d --build app fixture-app` | Images built and both containers started          | Approved changes available locally                                                                          | Event artifacts mounted read-only; no ingestion run                             |

README and product ADR describe the new defaults. Markdown source was inspected;
no documentation rendering workflow is configured. The following change resolves
the subsequent cancellation styling and calendar time-label requests.

## Calendar card simplification and cancellation styling — 2026-09-09

Calendar entries now show event name and venue without a time in Day, Week, and
Month on desktop and phone. This supersedes the intermediate request to remove
only timezone abbreviations. Sorting still uses the stored time. Detail times,
timezone labels, search metadata, and ICS exports remain unchanged.

Cancelled entries have a 2px red strikethrough across name and venue. The separate
bold red Cancelled label is not struck through. Popup Cancelled status reuses that
bold red class; Scheduled retains its existing appearance. The shared formatter
and status classification were not changed.

The focused browser tests first failed on the missing strikethrough and the old
time-prefixed card text. After the implementation and local container rebuild:

| Evidence source                                                                                  | Raw result                                                                 | Supported finding                                                                                               | Material limit                                                                    |
| ------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| UI browser suite excluding empty startup and dedicated AEG replay                                | 59 passed, one desktop-only test skipped on phone                          | New styles and no-time labels pass in all views; ordering, modal times, exports, and existing interactions pass | Chromium against local fixture container; unchanged ingestion workflows not rerun |
| Saved browser screenshots                                                                        | Phone card shows red strikethrough; desktop popup shows bold red Cancelled | Styling visually checked                                                                                        | Synthetic cancellation fixture                                                    |
| `npm test`                                                                                       | 77 passed, one handoff test skipped                                        | Frontend regressions pass                                                                                       | No schema or Go changes                                                           |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `git diff --check` | Exit 0                                                                     | Applicable static and build checks pass                                                                         | Git diff does not inspect untracked files; formatter does                         |
| `docker compose -f compose.yaml -f compose.test.yaml up -d --build app fixture-app`              | Both images built and containers started                                   | Local app updated                                                                                               | No external deployment or event artifact modification                             |

README and the product ADR were updated. Markdown source was inspected; the
repository has no documentation renderer.

## Remove redundant Artist row — 2026-09-09

The event title remains the modal heading. The separate Artist row is removed;
Status stays directly after Venue, with cancellation styling unchanged. Artist
metadata remains in the records and still participates in search and ordering.
The earlier proposed Status/Artist swap was superseded by this decision.

The browser regression first failed on the existing Artist row on desktop and
phone. It now verifies a supplied artist distinct from the title, preserves the
title heading, omits Artist, and keeps Venue then Status.

| Evidence source                                                              | Raw result                                        | Supported finding                                                 | Material limit                                          |
| ---------------------------------------------------------------------------- | ------------------------------------------------- | ----------------------------------------------------------------- | ------------------------------------------------------- |
| UI browser suite excluding empty startup and dedicated AEG replay            | 59 passed, one desktop-only test skipped on phone | Modal change and existing interactions pass in rebuilt containers | Local Chromium; unchanged ingestion workflows not rerun |
| `npm test`                                                                   | 77 passed, one handoff test skipped               | Frontend regressions pass                                         | No Go/schema change                                     |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build` | Exit 0                                            | Applicable frontend checks pass                                   | Local build                                             |
| Compose app and fixture rebuild                                              | Both containers started                           | Change available locally                                          | Event artifacts unchanged                               |

README and product ADR were updated and Markdown source inspected. No documentation
renderer is configured. The cancellation suffix change is recorded below.

## Strikethrough-only cancellation entries — 2026-09-09

Calendar entries no longer append `· Cancelled`. Red strikethrough remains across
name and venue; the modal still displays Cancelled in bold red. The browser test
first failed on the old suffix and now checks exact card text plus strikethrough
in Day, Week, and Month on desktop and phone.

| Evidence source                                                   | Raw result                                        | Supported finding                                                          | Material limit                          |
| ----------------------------------------------------------------- | ------------------------------------------------- | -------------------------------------------------------------------------- | --------------------------------------- |
| UI browser suite excluding empty startup and dedicated AEG replay | 59 passed, one desktop-only test skipped on phone | Entry text, strikethrough, and modal status verified                       | Local Chromium                          |
| `npm run test:aeg`                                                | 2 browser tests passed; isolated replay completed | Stored source fixtures publish and display cancellation without the suffix | Two synthetic events; no live ingestion |
| `npm test`                                                        | 77 passed, one handoff test skipped               | Frontend regressions pass                                                  | No schema/Go change                     |
| Format check, lint, typecheck, build, and Compose rebuild         | Exit 0; app and fixture containers started        | Change available locally                                                   | No external deployment                  |

The phone entry screenshot was inspected. README and product ADR were updated
and Markdown source inspected. Temporary replay containers were removed by the
test workflow; its isolated artifacts remain available. Real event artifacts
were not modified. The subsequent Show All centering change is recorded below.

## Center Show All — 2026-09-09

Show All is horizontally centered within the date cell or mobile date section.
FullCalendar's `moreLinkClass` hook centers native overflow with flex alignment;
the existing `.show-all` button centers its text. Vertical placement, display
limits, and full-day navigation are unchanged. No dependency or layout calculator
was added. README and product ADR were updated; Markdown source was inspected.

The new layout test first measured off-center labels on desktop and phone, then
passed after the CSS change and container rebuild. Chromium required escalation
after a sandbox launch failure.

| Evidence source                                                   | Raw result                                                                             | Supported finding                                                             | Material limit                                                          |
| ----------------------------------------------------------------- | -------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `tests/browser/show-all.spec.ts`                                  | 2 passed; text center within 3px of target center; full day contains 14 fixture events | Native overflow, capped desktop row, and phone row are centered and clickable | 1440×900, 1440×2000, and 390×844; synthetic fixture                     |
| UI browser suite excluding empty startup and dedicated AEG replay | 61 passed, one desktop-only test skipped on phone                                      | Existing UI interactions pass                                                 | Local Chromium; unchanged ingestion and empty-store workflows not rerun |
| `npm test`                                                        | 77 passed, one handoff test skipped                                                    | Frontend regressions pass                                                     | No Go/schema change                                                     |
| Format check, lint, typecheck, production build, Compose rebuild  | Exit 0; app and fixture containers started                                             | Change available locally                                                      | No external deployment                                                  |

Desktop and phone screenshots were inspected. No event artifacts were changed.

## Title-only calendar entries — 2026-09-09

Calendar entries now display only the event title. Venue names remain in filters
and event details. Stored venue/artist/time metadata, event ordering, title
strikethrough for cancellations, and Show All behavior are unchanged. The unused
venue-label CSS was removed. README and the product ADR describe this decision.

The browser tests first failed on the old title-plus-venue text, then passed after
the change and container rebuild. Tests cover title-only entries in Day, Week,
and Month on desktop and phone, including cancelled entries, modal venue display,
venue filtering, and the unchanged ordered event list.

| Evidence source                                                   | Raw result                                        | Supported finding                                   | Material limit                                          |
| ----------------------------------------------------------------- | ------------------------------------------------- | --------------------------------------------------- | ------------------------------------------------------- |
| UI browser suite excluding empty startup and dedicated AEG replay | 61 passed, one desktop-only test skipped on phone | Title-only cards and existing interactions verified | Local Chromium; unchanged ingestion workflows not rerun |
| `npm test`                                                        | 77 passed, one handoff test skipped               | Frontend regressions pass                           | No Go/schema change                                     |
| Format check, lint, typecheck, build, and Compose rebuild         | Exit 0; both containers started                   | Change available locally                            | No external deployment                                  |

The desktop month screenshot and Markdown source were inspected. No documentation
renderer is configured. Event artifacts were not modified.

## Content-height desktop event cards — 2026-09-09

Desktop `.event-card:not(.show-all)` overrides the shared button minimum height
with zero. Cards now follow their line count and existing padding. Phone cards
retain the 44px minimum; font sizes, padding, toolbar controls, and Show All
sizing are unchanged. This supersedes the earlier desktop-card minimum-height
decision without changing calendar placement or overflow logic.

The test first failed because a single-line desktop Month card had 17.44px of
height beyond its line height and padding. Phone sizing already passed. After
the CSS change, the test verifies content height in all views, one extra line
of height for a wrapped Month title, and preserved phone touch sizing. A full
suite run exposed a transient detached-card measurement during view changes;
the test now retries the measurement assertions until the rendered card settles.

| Evidence source                                                   | Raw result                                        | Supported finding                                        | Material limit                                          |
| ----------------------------------------------------------------- | ------------------------------------------------- | -------------------------------------------------------- | ------------------------------------------------------- |
| UI browser suite excluding empty startup and dedicated AEG replay | 63 passed, one desktop-only test skipped on phone | Card sizing, overflow, and existing UI interactions pass | Local Chromium; unchanged ingestion workflows not rerun |
| `npm test`                                                        | 77 passed, one handoff test skipped               | Frontend regressions pass                                | No Go/schema change                                     |
| Format check, lint, typecheck, build, and Compose rebuild         | Exit 0; both containers started                   | Change available locally                                 | No external deployment                                  |

The desktop Month screenshot was inspected. README and product ADR were updated
and Markdown source inspected. No documentation renderer is configured. Event
artifacts were not modified.

## Font Awesome popup actions — 2026-09-09

The four approved icons now appear beside their existing labels in matching
button styles. The local `ActionIcon` component contains the official Free 7.3.1
SVG geometry and attribution, including metadata retained in the rendered SVG.
This avoids adding an icon font, CDN, or package for four fixed assets. Inspection
found no existing icon component or library to extend. Existing dialog focus,
copy feedback, download handling, and link attributes remain unchanged. Ingestion
and Go internals were not changed or reinvestigated for this presentation change.

The new browser test first failed on both layouts because SVGs were absent.
It now checks each action's accessible name, decorative SVG attributes, focus,
button styling, and minimum target height. Visible labels provide the accessible
names; separate icon alt text would duplicate those names. The initial official
SVG fetch failed in the network sandbox; an approved retry succeeded.

| Evidence source                                                                     | Raw result                                                                        | Supported finding                                                                                           | Material limit                                                                                                  |
| ----------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `npm run test:e2e -- --grep-invert 'normal startup is empty\|AEG replay artifact'`  | 65 passed, one desktop-only check skipped on phone                                | Icons, accessible names, focus styles, copy, export, and existing UI pass against rebuilt fixture container | Chromium desktop/phone; no manual screen-reader session; unchanged empty-store and ingestion workflows excluded |
| `npm test`                                                                          | 77 passed, one handoff test skipped                                               | Frontend regression checks pass                                                                             | Container handoff not rerun; no schema changes                                                                  |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`        | Exit 0                                                                            | Frontend checks pass                                                                                        | No public deployment                                                                                            |
| `docker compose -f compose.yaml -f compose.test.yaml up -d --build app fixture-app` | Both images built and containers started                                          | Updated UI deployed locally                                                                                 | Event artifacts unchanged                                                                                       |
| `test-results/modal-actions-desktop.png`, `test-results/modal-actions-phone.png`    | Four labeled icons visible, wrapped within modal, focused Tickets outline visible | Both captured layouts display the actions without clipping                                                  | Synthetic event; screenshots are ignored test artifacts                                                         |

README and the product contract were updated. Markdown source was inspected;
the repository has no documentation render workflow. No data refresh or external
deployment was performed.

## View-specific event text — 2026-09-09

Day cards now show `Event @ Venue` with wrapping. Month and Week cards keep
title-only text on a single line with CSS overflow ellipsis. The complete text
remains in the DOM for accessible names and in the event modal. Red cancellation
strikethrough applies to the displayed summary, including the venue in Day.
Show All, time semantics, filters, data, and publication are unchanged.

The existing renderer and view attributes were extended instead of truncating
stored strings or adding a text measurement helper. The new browser test first
failed on both layouts because `white-space` was `normal`, not `nowrap`.
Existing card-height and Day-title assertions were updated for the new contract.

| Evidence source                                                                                                      | Raw result                                                                                         | Supported finding                                                                                  | Material limit                                         |
| -------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| `npm run test:e2e -- tests/browser/event-text.spec.ts`                                                               | Two passed after initial expected failures                                                         | Day venue, Month/Week overflow, complete accessible names, modal title, and strikethrough verified | Synthetic long title; Chromium desktop/phone           |
| `npm run test:e2e -- --grep-invert 'normal startup is empty\|AEG replay artifact'`                                   | 67 passed, one desktop-only check skipped on phone                                                 | Existing UI regression checks pass                                                                 | Unchanged empty-store and ingestion workflows excluded |
| `npm test`                                                                                                           | 77 passed, one handoff test skipped                                                                | Frontend checks pass                                                                               | No schema or producer changes; handoff not rerun       |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`                                         | Exit 0                                                                                             | Applicable frontend checks pass                                                                    | No public deployment                                   |
| `docker compose -f compose.yaml -f compose.test.yaml up -d --build app fixture-app`                                  | Both containers rebuilt and started                                                                | Changed UI served locally and tested on fixture container                                          | Artifacts unchanged                                    |
| Inspected `event-text-desktop-Month.png`, `event-text-phone-Week.png`, `event-text-phone-Day.png` in `test-results/` | Ellipsis visible in Month/Week; complete wrapped title and venue in Day; red strikethrough visible | Captured layouts match the display contract                                                        | Ignored screenshots; no manual screen-reader test      |

README and product contract updated; final Markdown source inspected. No supported
documentation renderer exists. Backend and ingestion internals were outside this
presentation change and were not reinvestigated.

## AEG source expansion — 2026-09-09

Approved scope: add Bluebird, Ogden, and Fiddler's Green using the existing AEG
snapshot adapter, validate in isolation, and publish a one-time local import.
Gothic and Mission must remain unchanged. No scheduler, network client, frontend
special cases, new schema, or inferred admission/price policy was added.

The supported-feed allowlist now binds Bluebird feed 2 to venue 100811, Ogden
feed 7 to venue 101141, and Fiddler's Green feed 44 to venue 100869. Tests first
failed because these feeds were unsupported, then passed with the added bindings.
All five synthetic venue fixtures pass mapping, reconciliation, foreign-venue
rejection, publication, and HTTP/UI consumption checks. Test configurations use
`state: new`; future refreshes require an established-source configuration.

Inspected five raw events each for Bluebird and Ogden, and all four Fiddler's
records. Samples include Joshua Ray Walker, The Yawpers, Ingrid Andress, Meltt,
Temples; Waylon Wyatt, Brandon Flowers, Peter Hook & The Light, Sleep,
Ninajirachi; Riley Green, Rob Zombie & Marilyn Manson, $UICIDEBOY$, and Stick
Figure. IDs, venue IDs, ISO show times, UTC doors, age text, and ticket URLs were
inspected. Venue detail routes returned HTTP 200 and referenced the expected feed;
this does not test downstream ticket checkout. Feed totals matched their event
arrays and remained below the 100-row cap. Future coverage is announced listings,
not proof that every event in the next year has been announced.

Downloaded snapshots and normalized candidates remain at
`/private/tmp/event-calendar-aeg-expansion-j7zVkE`. Each staged source had zero
rejections. Publication committed generation `g-afa96e2f9878576ebf2d8a694f34c9eb`
to `.artifacts` in one operation. The previous catalog is saved as
`catalog-before.json` in that temporary directory. Original Gothic/Mission source
references and SHA-256 checksums are unchanged. Old immutable artifacts remain
available, so the saved catalog provides a recovery point; no rollback was needed.

| Evidence source                                                                              | Raw result                                                                                                        | Supported finding                                                           | Material limit                                                                                                                                                                       |
| -------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Live snapshots, staged artifacts, and localhost:8090 `/api/calendar`                         | Bluebird 70, Ogden 56, Fiddler's Green 4; no staging rejections                                                   | 130 real events added                                                       | One-time snapshot; no automated refresh                                                                                                                                              |
| Served raw sample and catalog references                                                     | Five records inspected per venue, all four for Fiddler's; 248 total; Gothic 57 and Mission 61 unchanged           | Main app serves five real venues without replacing prior data               | Current local store only                                                                                                                                                             |
| `npm run test:aeg`                                                                           | Final run passes HTTP assertions for all five synthetic records and two browser tests                             | Real ingestion container to app/UI boundary passes                          | Initial desktop assertion hit Month display cap; corrected test selects Week; fixture store retained at `/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-aeg-h6g2o3` |
| `LIVE_AEG_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/live-aeg.spec.ts` | Two passed                                                                                                        | New live venue events open in modals with expected links and filter options | Chromium desktop/phone; external purchase flow not exercised                                                                                                                         |
| `npm run test:contracts`                                                                     | Initial expanded-adapter run: Go checks and 78 frontend checks passed; final Go image also passes format/vet/race | Adapter and artifact tests pass                                             | Later handoff reruns failed because Docker could not see `test-results/contracts`; repeated separately and unrestricted with same failure; cause not isolated                        |

Final host checks (`npm run format:check`, `npm run lint`, `npm run typecheck`,
`npm test`, `npm run build`) exited 0; Vitest reported 77 passed and one optional
handoff check skipped. `git diff --check` passed but does not cover untracked files;
the formatter covers the applicable source files.

The initial Docker build required sandbox escalation. A proposed shared-output
race did not explain the persistent contract mount failure, so that cause is not
claimed. No publication failure occurred. No app/data changes were made to work
around verification infrastructure. Other source families and public deployment
remain outside this change. README, fixture documentation, source registry, and
adapter ADR were updated; Markdown source inspected without a repository renderer.

## Music-finder palette — 2026-09-09

Replaced green/earth colors with tokens matching
`music-finder/client/src/styles/global.css`: background `#0d0d0d`, surface
`#1a1a1a`, hover `#252525`, border `#333333`, text `#e0e0e0`, muted `#888888`,
accent `#6c5ce7`, and accent hover `#7c6cf7`. Calendar, modal, drawer, links,
controls, and FullCalendar theme variables share these local CSS tokens. No
runtime dependency on music-finder was added, and that repository was not modified.

Two contrast adaptations: selected purple controls use white text, and small
links/focus outlines use lighter purple `#a99cff`. Purple hover is used on selected
control borders, not behind white small text. Existing red cancellation styles,
fonts, dimensions, and interactions remain unchanged. No neon effects or other
music-finder layout features were copied.

The new test first failed against the old green background. Screenshot inspection
then found a remaining teal current-day marker. A new assertion reproduced it as
`rgb(160, 208, 203)`; configuring FullCalendar's tertiary colors made that assertion
pass. FullCalendar theme code was inspected to identify the supported tokens.
The first Compose attempt was denied by the Docker socket sandbox; the approved
retry succeeded. Data and ingestion code were outside this change.

| Evidence source                                                                          | Raw result                                                                                          | Supported finding                                                   | Material limit                                                                                                                          |
| ---------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `npm run test:e2e -- --grep-invert 'normal startup is empty\|AEG replay artifact'`       | 69 passed, three skipped                                                                            | Calendar, modal, filters, theme, cancellation, and existing UI pass | Chromium desktop/phone; live-data opt-in and desktop-only-on-phone checks skipped; unchanged ingestion/empty-startup workflows excluded |
| `tests/browser/theme.spec.ts`                                                            | Seven specified text/background pairs meet 4.5:1; rendered key colors match expectations            | Checked palette text combinations remain readable                   | Not a full accessibility audit or manual screen-reader test                                                                             |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm test`, `npm run build` | Exit 0; 77 unit tests pass, one handoff skipped                                                     | Frontend checks pass                                                | No producer/schema changes; handoff not rerun                                                                                           |
| Compose build/restart and `theme-*.png` screenshots                                      | Both local containers started; charcoal calendar/modal/drawer and purple current-day marker visible | Theme observed through rebuilt container on desktop/phone           | Synthetic visual sample; no public deployment                                                                                           |

README and product contract updated. Final Markdown source inspected; no supported
documentation renderer exists in this repository. Event artifacts were not changed.

## Reviewed With adult filter — 2026-09-09

Implemented the approved Bluebird-first filter with saved age 0–17 (default 14).
Curated configuration produces optional `with_adult` artifact metadata. Shared
validation rejects invalid ranges and unsafe provenance URLs. Exact-category
selections are suspended, not erased. Event details retain the advertised age text
and show the applicable condition, official policy link, and review date.

Test-first failures covered the unknown config property, missing ingestion command,
missing filter control, and unmatched eligible event. Verification also found an
input accessible-name defect, which was fixed with `aria-labelledby`. Tests now
await the completed close navigation before reload and include the new control in
keyboard order. An ingestion test fixture's source website was corrected to match
the published source; the production identity guard was retained.

| Evidence source                                                                                          | Raw observation/result                                                                                             | Supported finding                                               | Material limit                                                                                                                               |
| -------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `npm run test:contracts`                                                                                 | Go formatting, vet, race tests and handoff pass; 86 TypeScript tests pass                                          | Shared contract and producer/consumer checks pass               | Initial Docker handoff mount failed; a read-only parent-mount check and subsequent normal rerun passed; underlying Docker cause not isolated |
| `npm run test:aeg`                                                                                       | Container publication and desktop/phone checks pass                                                                | Reviewed fixture configuration reaches stored JSON, HTTP and UI | Synthetic events; no network ingestion                                                                                                       |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:publication`                                      | 73 passed, 5 skipped                                                                                               | Publication, empty startup and browser regressions pass         | Dedicated AEG/live-data tests run separately; desktop-only test skips on phone                                                               |
| Host format, lint, typecheck, unit tests and build                                                       | Pass; unit tests 85 passed, one handoff test skipped                                                               | Host checks pass                                                | Handoff runs in the contract workflow                                                                                                        |
| Staged Bluebird artifact comparison                                                                      | Five representative records inspected; all 70 records preserve their prior content except added admission metadata | Non-refreshing enrichment preserves records and public URLs     | No upstream refresh or policy review for other venues                                                                                        |
| Live publication report and catalog comparison                                                           | Durable generation `g-d962e23cc098b142f25a7fc992ff12c9`; other four source references unchanged                    | Only Bluebird was published                                     | Prior catalog backed up at `/private/tmp/event-calendar-admission-v1H3dF/catalog-before.json`                                                |
| `LIVE_ADMISSION_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/live-admission.spec.ts` | Two passed; desktop filter and phone modal screenshots inspected                                                   | Local real-data filter and condition display work               | Chromium; Bluebird only; not a guarantee of admission                                                                                        |

Both application containers and the ingestion image were rebuilt. The reader was
deployed before enriched data. `enrich-admission` preserves existing enrichment and
already-applied overrides; ordinary replay is the route for recomputing rules from
fresh normalized records. No database, scheduler, or upstream HTTP client was added.

## Context

### HQ and Oriental HoldMyTicket import — 2026-09-10

Implemented the approved shared HoldMyTicket adapter and `ingest replay-hmt`.
Reused golang-ical, the artifact schema, reconciler, publisher, and HTTP/UI reader.
The AEG replay command now delegates to a shared snapshot-publication function;
its existing command tests passed before that refactor. New tests first failed on
the missing adapter and command, then passed after implementation. Live samples
revealed additional exact `N+ / Bar with ID` labels; tests failed before those
explicit category mappings were added and passed afterward.

The local capture read both public subscription feeds and one Event JSON-LD block
per detail page with Python's standard HTML parser. No page scripts were executed.
The capture helper and original responses remain outside the repository in
`/private/tmp/event-calendar-hmt-bB6vEl`. No network client, scheduled job, schema,
or venue-specific frontend logic was added. Fetch failures stop capture. The Go
adapter separately checks envelope completeness, event identities, venue, title,
show-time agreement, and ticket destinations. Unsupported recurrence fails the
snapshot; invalid individual records use the existing rejection contract.

| Evidence source                                                                                                                                                       | Raw observation/result                                                                                                        | Supported finding                                                             | Material limit                                                                                             |
| --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Saved feeds and Event JSON-LD                                                                                                                                         | Five raw records inspected per source with IDs, venue, show/doors, age and labeled offers; 49 HQ and 43 Oriental feed records | Both sources provide usable structured enrichment                             | Each feed retained one September 9 event, outside the new September 10–September 10, 2027 ingestion window |
| Staged replay reports and artifacts                                                                                                                                   | No rejected records; 48 HQ and 42 Oriental events published; HQ 47 priced, Oriental 42 priced                                 | Ninety events fit the requested horizon with preserved age and separate times | Offer prices remain labeled text, not per-person comparisons or a restored price filter                    |
| Sampled date bounds                                                                                                                                                   | HQ September 11, 2026–March 8, 2027; Oriental September 12, 2026–July 17, 2027                                                | Feeds extend beyond the current month                                         | Bounds do not establish complete venue listings; automatic absence refresh remains unapproved              |
| `npm run test:contracts`                                                                                                                                              | Go formatting, vet, race tests and handoff pass; 86 TypeScript tests pass                                                     | Shared contracts and both replay commands pass                                | Synthetic edge cases, not provider service guarantees                                                      |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm test`, `npm run build`                                                                              | Pass; 85 host unit tests pass, one handoff test skips                                                                         | Host checks pass                                                              | Handoff tested separately by contracts                                                                     |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:publication`                                                                                                   | 73 passed, 19 skipped                                                                                                         | Full publication and existing browser regressions pass                        | Opt-in live/HMT/AEG tests run separately; desktop-only test skips on phone                                 |
| Staging `HMT_BASE_URL=http://127.0.0.1:8094 npm run test:e2e -- tests/browser/hmt.spec.ts`                                                                            | Four passed; Oriental phone screenshot inspected                                                                              | Real staged artifacts render with price, age and links on both layouts        | Chromium; no checkout or external calendar-client test                                                     |
| Local publication report and catalog comparison                                                                                                                       | Durable generation `g-15c7da83c234d5a3e9019b5dd6e7e50e`; all five prior source references unchanged                           | Two sources added atomically without refreshing existing venues               | Backup `/private/tmp/event-calendar-hmt-bB6vEl/catalog-before.json`; original artifacts retained           |
| `HMT_BASE_URL=http://127.0.0.1:8090 LIVE_ADMISSION_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/hmt.spec.ts tests/browser/live-admission.spec.ts` | Fourteen passed                                                                                                               | New venue details and all prior admission policies work in the main local app | HQ and Oriental do not receive unreviewed With adult exceptions                                            |

The ingestion image and isolated application images were built. The unchanged
main application consumed the new publication without restart. New fixture files
are synthetic and must not be published as live data. Saved live responses are
operator artifacts, not committed fixtures. Public redistribution, recurring
retrieval permission, historical backfill, and complete provider enumeration
remain outside this one-time local import.

The final `npm run test:aeg` rerun also passed its container replay and two browser
checks, confirming that the shared command refactor preserves AEG publication.
The temporary HMT staging and empty-store containers were stopped and removed;
their data directories remain available for inspection. The main and fixture
application containers were left running.

### Four additional reviewed venue policies — 2026-09-09

Added curated configuration for Gothic, Mission, Ogden, and Fiddler's Green using
their official pages linked in ADR 0014. Reused the existing schema, enrichment,
publisher, HTTP projection and filter; no production-code change was necessary.
No automatic policy scraper or missing-restriction fallback was added. The new
four-venue test first failed because reviewed rules were absent, then passed after
configuration was added. Existing override and off-site protections were retained.

| Evidence source                                                                                          | Raw observation/result                                                                                                                   | Supported finding                                                                                 | Material limit                                                                                   |
| -------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `TestPublishedVenueAdmissionRules`                                                                       | Four venues pass; checks ages 0, 1, 2, 10, 11, 14, 15, 16, 17 and unknown restrictions                                                   | Configured ranges support the approved categories                                                 | No inferred 13+ rule or missing policy                                                           |
| `npm run test:contracts`                                                                                 | Go formatting, vet and race tests pass; 86 TypeScript tests pass                                                                         | Contracts and prior behavior remain valid                                                         | No schema change                                                                                 |
| `npm run test:aeg`                                                                                       | Isolated publication and both browser layouts pass                                                                                       | Configured data reaches artifacts, HTTP and filtering                                             | Fiddler's synthetic 16+ event deliberately does not match                                        |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm test`, `npm run build`                 | Pass; 85 unit tests pass, one handoff test skips                                                                                         | Host checks pass                                                                                  | Handoff covered separately by contracts                                                          |
| Staged artifact comparison                                                                               | Five records inspected per theatre; all four Fiddler's records inspected; enrichment counts Gothic 57, Mission 61, Ogden 56, Fiddler's 3 | Only admission metadata changed; all event identities, URLs and other fields retained             | Fiddler's fourth record, Stick Figure, lacks a restriction and remains unknown                   |
| Local publication                                                                                        | Durable generation `g-56b3dc03bdf3bea5dcaa062f845d6e5c`; Bluebird reference unchanged                                                    | Four sources published together without refreshing upstream                                       | Backup `/private/tmp/event-calendar-policies-W3T9TK/catalog-before.json`; staging store retained |
| `LIVE_ADMISSION_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/live-admission.spec.ts` | Ten tests pass                                                                                                                           | Each venue matches at age 14 and displays its condition and policy link in desktop/phone Chromium | Admission remains subject to venue conditions                                                    |

The ingestion image and isolated application image were built. The unchanged
running local reader consumed the new publication without a restart. Full unrelated
publication/browser suites were not repeated for this configuration-only change;
the affected ingestion, contract and live browser workflows were run.

At design authoring, the repository contained documentation only. The first milestone
has since added the app, tests, and local container workflow recorded below. CI and
the later ingestion/publication components are not implemented.

The product contract is the acceptance authority. Documentation research establishes possible mechanisms, not completed behavior or performance.

## Decision

Implement in bounded stages after implementation approval. Use test-first development for executable changes, fixture-driven source tests, and public-interface checks against the running containers.

Do not skip the FullCalendar mobile integration gate. Do not add a handwritten calendar as a shortcut if the selected community component fails a requirement.

## Phase 0: Resolve implementation gates

### FullCalendar spike

Build a small isolated integration using the proposed React dependency set and supported FullCalendar APIs.

Fixtures must cover:

- More than ten events on one day in week view and more than five in month view.
- Desktop seven-column week and phone vertical week presentation.
- Show All opening the correct day.
- Untimed records before timed records, then venue/title tie breaks.
- Doors priority, separate show time, and Denver DST.
- Event clicks, date-number clicks, keyboard focus, and panel close behavior.
- Resize between phone and desktop without losing selected week or event.
- Empty data and hidden events after filtering.

Record dependency versions and results. Compare behavior with the product ADR, not just whether the library renders. If supported APIs cannot preserve the mobile limit, stop and present the limitation for a product/community-library decision.

### Remaining product semantics

Age-category matching is exact. Cost uses the lowest advertised USD price rounded
up to a whole dollar; original text is preserved and unspecified fees are not added.
Default brackets put $200 in $$$ and $201 in $$$$. Unknown data must not be guessed.

Before production publication, resolve single-writer enforcement, physical cleanup, app refresh cadence, and recovery procedures. These decisions need not block the first fixture-driven UI.

## Phase 1: Schema and lifecycle fixtures

Create the versioned JSON Schema, shared artifact fixtures, operator configuration shape, and fixed-clock test helpers.

Define known-good and rejected examples for required fields, overrides, null/absent values, policy inheritance, source identity, off-site attribution, URLs, timezones, and price metadata.

Implement and test the reconciliation contract before adding live adapters. Confirm failure tests fail for the intended reason before production code changes.

## Phase 2: Application and local container

Implement the generic Go host and React/FullCalendar SPA against test fixtures.

Verify default Compose startup without ingestion or data. Fixtures used by tests must not become default app content.

Verify actual HTTP status codes, including HTTP 404 for expired links, as well as UI states. A screen saying Not Found while the server returns 200 does not satisfy expired-link acceptance.

## Phase 3: AEG pilot

Capture representative public Gothic and Mission fixtures after checking access conditions. Inspect raw IDs, venue, title, doors/show times, admission fields, and links before aggregates.

Use one AEG adapter with two configurations. Corroborate ambiguous zero-price fields before mapping Free. Verify venue-homepage links separately from ticket links.

Exercise success, empty response, provider failure, invalid records, retained invalid updates, missing records, known caps, and source-scope checks.

## Phase 4: HQ iCalendar

Add HQ using the same published artifact contract.

Test UID matching, floating time and provider timezone handling, sparse metadata, empty descriptions, valid line folding/escaping, and unknown doors versus show time. Verify exported ICS with a parser and representative third-party calendar imports where available.

Do not infer prices or age policies from their absence.

## Phase 5: Publication and Railway verification

Run the same storage contract tests against the local implementation and an explicitly approved Railway test bucket.

Test staging, catalog-last publication, failed-source carry-forward, process crashes, checksum failures, concurrent-reader refresh, and rejected competing writers.

Build both images and verify the app through the running container. Verify cron execution exits and produces operator-visible outcomes. Do not provision or modify external resources without approval.

## Phase 6: Expand source coverage

Add remaining adapters using the registry and existing fixture patterns. A new source must not require provider-specific application code. Apply source access, completeness, and normalization checks before enabling its schedule.

Aggregator-specific handling and unresolved sources remain separate work.

## Acceptance matrix

These are planned tests, not empirical results.

| Contract                    | Verification                                                       | Required observable result                                                                           |
| --------------------------- | ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------- |
| Empty startup               | Run default Compose with an empty local store.                     | Only app starts; calendar shows No events available; no upstream calls.                              |
| Shared artifact consumption | Load Gothic, Mission, and HQ fixtures.                             | Same reader/UI works without source-specific configuration.                                          |
| Views and limits            | Browser tests at desktop and phone sizes.                          | Sunday week; day/week/month controls; configured 10/5 caps and correct day drill-in.                 |
| Sorting and times           | Fixed-clock fixtures across DST, unknown times, and ties.          | Doors priority; correct venue timezone; untimed first; stable product sort.                          |
| Filters/search              | Component and browser tests over multiple dates and metadata gaps. | Positive matches, AND query words, no search-mode switch, upcoming default and past option.          |
| Persistence/deep links      | Reload with saved filters, then open a direct event URL.           | Reload restores preferences; direct link opens event week/panel with defaults; closing retains week. |
| Event details               | Sparse and full fixtures.                                          | Required display and links work; optional gaps omitted; policy override/inheritance correct.         |
| Cost boundaries             | Exact prices and unknown values.                                   | $200 maps to $$$, $201 to $$$$; unknown does not become Free.                                        |
| Stable identity             | Repeat ingestion with title/time corrections and venue/date moves. | Original paths preserved for corrections; moves create new events.                                   |
| Overrides                   | Configured allowed/forbidden changes.                              | Allowed override persists; venue/date override fails validation.                                     |
| Failed source               | Simulate network, parse, pagination, and prior-state failures.     | Previous source publication retained; explicit operator failure.                                     |
| Record rejects              | Mix valid new, invalid new, and invalid known records.             | Valid records publish; new rejects reported; known invalid retains last valid.                       |
| Successful absence          | Remove a future record from a completed response.                  | Calendar excludes it; retained URL opens No longer listed details.                                   |
| History and expiry          | Fixed clock before/after retention deadline.                       | Past browsing works within window; expired UI data excluded and HTTP route returns 404.              |
| Duplicate precedence        | Same occurrence from own-venue and off-site sources.               | One preferred calendar item; retained original links still resolve.                                  |
| Uncertain identity          | Similar names, separate performances, conflicting dates.           | Distinct or uncertain occurrences are not silently collapsed.                                        |
| Publication interruption    | Stop coordinator at each write boundary.                           | No catalog intentionally references incomplete artifacts; prior valid data remains usable.           |
| ICS                         | Export timed and date-only events, then parse/import.              | Correct known time/date and stable UID; no fabricated time or duration.                              |
| Credential isolation        | Inspect build artifacts, served JSON, logs, and routes.            | No secrets or operator config in public output; no arbitrary file/bucket proxy.                      |

## Tooling and commands

Proposed tools:

- Go testing, httptest, race detection, and parser fuzzing.
- [Vitest](https://vitest.dev/guide/) for frontend pure functions.
- [React Testing Library](https://testing-library.com/docs/react-testing-library/intro/) for component behavior.
- [Playwright](https://playwright.dev/docs/test-webserver) against the running application container.
- The same schema/fixture corpus at Go and TypeScript boundaries.

The first milestone installed Vitest and Playwright and uses Go's standard testing
tools. React Testing Library and parser fuzzing remain candidates for later work.
Current commands are documented in the root README; CI is not configured.

Expected Go checks include formatting, vet, ordinary tests, and race tests where the build environment supports them. Frontend checks must include formatting, lint, type checking, production build, component tests, and browser tests. Container builds and runtime smoke tests are required for both images.

Record command, environment, result, and unverified boundaries at each handoff. Integration with a simulated S3 server is not proof of Railway compatibility.

## Performance and access checks

Measure production asset size, initial data transfer/parse time, filter interaction, view navigation, and memory on a representative dataset. Include a larger synthetic fixture only as an explicitly labeled stress case.

Set budgets from the observed baseline and product priorities. Do not invent a throughput, event-volume, or hosting-cost estimate. Custom calendar components are not a fallback justified by an unmeasured speed claim.

Check keyboard/touch navigation and the detail-panel accessibility behavior in the actual rendered interface. Unit tests alone cannot verify the visual/mobile contract.

Before scheduled production ingestion, complete the source access/reuse review and configure bounded requests, timeouts, retries, and body sizes. Browser automation or external enrichment would require separate scope review.

## Completion and consequences

A stage is complete only when its relevant behavior has been observed through the implemented interface. A passing parser test does not prove catalog publication, browser behavior, or deployment.

The implementation plan authorizes no code changes by itself. This documentation approval does not authorize external resources, deployments, or live scheduled jobs.

Known open boundaries at authoring included FullCalendar mobile limits and dependency
compatibility. The first-milestone record below addresses those two gates. Production
schema integration, Railway integration, scale budgets, and filter/operational policies remain open.

## Second-milestone verification — 2026-09-09

Approved scope: version-one JSON schemas, YAML operator configuration, Go/TypeScript
types and validation, policy inheritance, overrides, stable URL assignment, reconciliation,
shared fixtures, and a serialized producer/consumer handoff. No live adapters, coordinator,
production storage, UI schema migration, filters, shared HTTP routes, ICS, or deployment
were added. The prototype UI still consumes its original `events.json` format.

The existing artifact ADR and prototype reader were inspected before implementation.
Bundled JSON Schema with standard validators was selected over handwritten structural
validation; domain rules remain explicit code. YAML nodes are checked before conversion
to JSON so duplicate keys, aliases, nulls, and unknown fields cannot disappear during
typed decoding. Existing Go tests, Vitest, Docker stages, and Playwright were extended
or reused; no separate task runner or database was added.

Tests first failed for missing implementations. Later red tests exposed identity/path
inconsistencies, invalid venue-move matching, and the maximum source-key/event-ID length
boundary. Those same tests pass after the fixes. The handoff output was inspected in
full: one catalog reference and one synthetic event, containing configured title and
18+ policy, separate offset-bearing doors/show times, USD 2000 minor units, stable path,
and December 9 expiry. The referenced file existed and the TypeScript test verified its
SHA-256 against the catalog. This is test-only transport/storage, not atomic publication.

| Evidence source / command                                                    | Raw observation or test result                                                                  | Supported finding                                                    | Material limit                                                         |
| ---------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build` | All exit 0.                                                                                     | Formatting, static checks, and frontend production build pass.       | New validators are not imported by the UI.                             |
| `npm test`                                                                   | 63 pass; one handoff test skips without generated output.                                       | Shared fixtures and existing projection tests pass.                  | Handoff requires the dedicated command below.                          |
| `npm run test:contracts`                                                     | Go formatting, vet, and race tests pass; generated-byte handoff runs; all 64 Vitest tests pass. | Go produces data accepted by the TypeScript contract boundary.       | Synthetic data; no live adapter or production storage.                 |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait`      | Both images build; app and fixture-app report healthy.                                          | The existing container workflow still runs.                          | Local containers only.                                                 |
| `npm run test:e2e`                                                           | All 20 desktop/phone checks pass against the rebuilt containers.                                | Existing HTTP, empty startup, calendar, and dialog regressions pass. | Prototype input; custom-limit and error cases use response overrides.  |
| `git diff --check`                                                           | Exit 0.                                                                                         | No tracked-diff whitespace errors.                                   | The worktree is untracked; formatter checks cover new supported files. |

The browser skill was inspected, but its required Node REPL was unavailable. The
existing Playwright suite is the regression fallback for the unchanged UI. CI, live
ingestion, production publication/cleanup, cross-source duplicate selection, real shared
route expiry, and Railway remain unverified. ADRs have no repository render/build
workflow; Markdown source and local links are checked instead.

## First-milestone verification — 2026-09-08

Approved scope: the FullCalendar integration spike, a minimal Go host, local JSON
consumption, empty default Compose startup, test-only fixtures, and browser checks.
This completes the UI spike and a small hosting scaffold, not every item in Phase 2
or the product acceptance matrix. No source jobs, live ingestion, DB, Railway resource,
public deployment, filters, shared routes, or ICS were added.

Environment: macOS ARM64 host; Node 26.4.0; Docker Desktop 29.5.2 and Compose 5.1.4;
Node 24 and Go 1.26 build containers. Runtime is a non-root, read-only scratch image.
The Go timezone database is embedded. Ports are loopback-only 8090 (empty normal app)
and 8091 (explicit synthetic fixture preview).

The fixture file and its first five records were inspected: Kestrel, Untimed Alpha,
Untimed Zulu, Early doors, and Amber contain IDs, titles, venues, date-only values,
IANA timezone names, and optional offset-bearing instants. Early doors includes the
full optional metadata and example.com links. The fixture endpoint returns all fifteen
expected IDs; fourteen records share September 8 and one uses September 9. These are
synthetic test events, not scraped venue listings.

| Evidence source / command                                               | Raw observation or test result                                                                              | Supported finding                                                         | Material limit                                                                                       |
| ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| `npm run format:check`                                                  | Exit 0.                                                                                                     | Frontend/config/README formatting passes.                                 | ADRs use their established Markdown style.                                                           |
| `npm run lint`                                                          | Exit 0.                                                                                                     | ESLint checks pass.                                                       | Static analysis only.                                                                                |
| `npm run typecheck`                                                     | Exit 0.                                                                                                     | TypeScript checks pass.                                                   | The production cross-language schema is deferred.                                                    |
| `npm test`                                                              | Five projection/time tests pass.                                                                            | Sort, limits, immutable projection, and DST cases pass.                   | Unit tests use synthetic data.                                                                       |
| `npm run build`                                                         | Exit 0; HTML, CSS, and JS files emitted and inspected on disk.                                              | The frontend builds with the locked dependencies.                         | Asset size is not an application-scale performance guarantee.                                        |
| `docker build --target go-test .`                                       | `gofmt` check, `go vet ./...`, and `go test -race ./...` exit 0.                                            | Go reader, validation, route, configuration, and size-limit tests pass.   | No live source or publication lifecycle exists.                                                      |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait` | App and fixture-app images build; both containers start.                                                    | The local container workflow runs.                                        | No Railway or public deployment test.                                                                |
| `npm run test:e2e`                                                      | Twenty checks pass: ten scenarios in each Chromium configuration.                                           | Real container HTTP data is rendered and the milestone interactions pass. | The custom-limit and 503 UI cases override HTTP responses; default data cases use real Go responses. |
| Week and detail screenshots in `test-results/`                          | Inspected desktop columns, vertical phone cards, and detail dialogs; date-label contrast regression passes. | The rendered interfaces match the checked layout.                         | Screenshots are regenerated test output, not tracked assets.                                         |

Tests first failed for missing projection and Go implementations and for the missing
calendar interface. Further regression tests exposed an oversized-JSON acceptance bug,
mobile width/contrast issues, and sparse-dialog focus containment. Each now passes.
An early fixture request returned unexpected EOF; host/container checksums subsequently
matched and repeated HTTP/browser checks passed without a server fix for that error.
Its transient cause was not isolated. Production atomic publication remains deferred.

### Small-fixture performance observations

The baseline test measures five page-load-to-visible-week samples and five subsequent
week-to-month interactions per viewport. Measurements include Playwright round trips
and assertion time. Later loads can use browser cache. Both workers run concurrently
against local Docker; no CPU/network throttling is applied.

| Evidence source                        | Raw observations, milliseconds                            | Supported finding                                                       | Material limit                                                             |
| -------------------------------------- | --------------------------------------------------------- | ----------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| Desktop baseline, five samples         | Ready: 81, 28, 55, 46, 45. Month: 42, 38, 23, 23, 21.     | The small fixture loads and switches views in these observed intervals. | Not a production budget or a prediction for larger datasets.               |
| Phone-emulation baseline, five samples | Ready: 78, 36, 22, 23, 23. Month: 27, 29, 41, 43, 44.     | The same fixture renders in the phone layout.                           | Emulation on a desktop CPU, not physical-phone performance.                |
| Vite production build                  | JS 455.41 kB, gzip 135.25 kB; CSS 22.71 kB, gzip 5.41 kB. | Establishes a build-size baseline.                                      | Gzip is Vite's calculated size; the Go server does not enable compression. |

No filtering, large-volume, memory, internet-latency, or Railway performance claim is
made. Those components and environments are outside this milestone.

### Tooling limits and next boundary

The browser skill's required Node REPL tool was unavailable. Verification used the
repository's Playwright Chromium runner and local screenshot inspection instead.
Safari, Firefox, physical devices, and screen readers were not tested. Markdown source
and relative links were checked; this repository has no documentation render pipeline.

Run commands and the deliberately small prototype input are documented in
[README.md](../../README.md). The next implementation boundary is Phase 1: establish
the production artifact schema and lifecycle fixtures before implementing live adapters.

## Third-milestone verification — 2026-09-09

Approved scope: replace prototype file loading with the local catalog reader, retain
the calendar endpoint and FullCalendar UI, validate whole generations, retain last-valid
state on read failures, apply venue/policy inheritance and read-time expiry, and migrate
the preview to multi-source artifacts. No live ingestion, publisher, shared event routes,
filters, ICS, database, or Railway resource was added.

The reader reuses `internal/artifact` rather than duplicating schema checks. Server-side
validation avoids introducing a browser schema compiler or relaxing CSP. Request-time
refresh was selected over a background scheduler for this bounded milestone; no claim
is made about large-dataset throughput. Snapshot replacement is serialized and uses one
validated catalog plus all its referenced source files. The prior snapshot remains usable
when any member fails. An explicitly valid empty catalog clears it.

Baseline Go web and Vitest tests passed. New tests first failed for missing clock/reader
interfaces; the new browser test failed because the prototype response lacked the policy
URL. Go tests then passed, including concurrent refresh, checksum/source/path/schema
failures, missing files and directories, outside-root symlinks, oversized inputs, empty
publication, recovery, and expiry at Denver midnight during failed refresh. A test-variable
declaration error was corrected. Formatting changed a fixture's bytes and its checksum
test failed; the catalog checksum was updated to match the final formatted file.

Six representative raw records were inspected across three synthetic source artifacts:
Kestrel, Dune, Untimed Alpha, Amber, Untimed Zulu, and Early doors. They contain source-
scoped IDs, dates, paths, expiry, actual time data where known, and optional price/policy
metadata. Mission has an inherited 16+ policy; Untimed Zulu overrides it with 21+.
The fixture additionally covers an untimed off-site event, an unlisted event, and an
expired event. The real HTTP response exposes sixteen listed, unexpired records.

| Evidence source / command                                               | Raw observation or test result                                                               | Supported finding                                                                                 | Material limit                                                                  |
| ----------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `npm run lint`, `npm run typecheck`, `npm run build`                    | Exit 0.                                                                                      | Static checks and production frontend build pass.                                                 | No scale benchmark.                                                             |
| `npm run format:check`, `git diff --check`                              | Exit 0.                                                                                      | Supported files pass formatting checks.                                                           | Git diff does not cover the untracked worktree; the formatter checks new files. |
| `npm test`                                                              | 64 pass; one generated-handoff test skips.                                                   | Shared contract, preview checksums, and projection checks pass.                                   | Dedicated command runs the skipped boundary.                                    |
| `npm run test:contracts`                                                | Go formatting, vet, and full race suite pass; all 65 Vitest checks pass with handoff output. | Go serialization/TypeScript consumption and concurrent local reader checks pass.                  | No production publisher or object-store transport.                              |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait` | Both images build; app and fixture-app report healthy.                                       | Container workflow runs with the catalog reader.                                                  | Local Docker only.                                                              |
| `npm run test:e2e`                                                      | 22 desktop/phone tests pass against rebuilt containers.                                      | Catalog files reach the actual HTTP/UI boundary, including policy links and off-site attribution. | Error/custom-limit UI scenarios still use response overrides.                   |
| `test-results/desktop-week.png`, `test-results/phone-week.png`          | Inspected seven-column desktop and vertical phone views, including the off-site card.        | New data preserves the checked layout.                                                            | Synthetic small fixture.                                                        |

The browser skill's required Node REPL remained unavailable; repository Playwright
provided browser verification. No new browser-control mechanism was installed. README
and ADRs document request-time refresh, in-memory-only fallback, supported input, and
the migration away from `events.json`. ADRs have no render/build workflow; their Markdown
source and local links are checked. Preview records start expiring December 7, 2026;
their dates and checksums must be refreshed together before that date for later browser
runs. Production expiry is not disabled for the preview. Shared-route HTTP expiry and
physical cleanup remain separate work.

## Fourth-milestone verification — 2026-09-09

Approved scope: local artifact publication through a command in the future ingestion
executable, an independently built container, catalog-last replacement, unchanged-source
carry-forward, competing-writer rejection, interruption/recovery checks, and published
data reaching the running calendar. No live adapter, schedule, existing app-data write,
cleanup job, or Railway resource was added.

The existing reader, schema, reconciliation interfaces, container stages, package scripts,
and publication ADR were inspected. Baseline Go and Vitest tests passed before extracting
the shared reader code to `internal/store`. Standard rooted-file operations and schema
validation were reused. No database, lock-file recovery protocol, or second schema
implementation was added. The installed Go documentation confirmed `syscall.Flock` and
`os.Root.OpenFile`; Compose help confirmed run and project cleanup options.

New tests first failed for the missing publisher and CLI. Tests now cover initial
publication, untouched references, stale expected generations, invalid/duplicate inputs,
source identity changes, corrupt/missing prior state, root confinement, exclusive names,
and failure checkpoints before and after catalog replacement. A real child process is
killed after staging a source file: the prior catalog remains unchanged and another writer
acquires the released lock. A failing stdout test exposed an exit-status reporting error;
the command now distinguishes a committed publication whose report cannot be delivered.

The container test published Gothic, HQ, and Mission synthetic artifacts into a unique
temporary store, rejected a stale update, and replaced Gothic while retaining the exact
HQ/Mission references. Six raw records were inspected in the published files: Untimed
Alpha, Amber, Kestrel, Dune, Untimed Zulu, and Early doors. They retain source-scoped IDs,
public paths, dates/expiry, supplied instants, and optional policy/price data. All three
catalog references point to files on disk. The real reader and browser tests check their
consumption, not a mock publication endpoint.

| Evidence source / command                                                                                                                    | Raw observation or test result                                                                                                     | Supported finding                                                               | Material limit                                                 |
| -------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`                                                                 | Exit 0.                                                                                                                            | Formatting, static checks, and frontend build pass.                             | No new UI feature or scale claim.                              |
| `npm test`                                                                                                                                   | 64 pass; one generated-handoff test skips.                                                                                         | Existing contract and UI projection checks pass.                                | Dedicated command runs the handoff.                            |
| `npm run test:contracts`                                                                                                                     | Go formatting, vet, and full race suite pass; all 65 Vitest checks pass.                                                           | Shared reader, publisher, CLI, and cross-language checks pass.                  | Synthetic data and local filesystems.                          |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait`                                                                      | Both app images build and services start.                                                                                          | Refactored reader works in the container workflow.                              | Local Docker only.                                             |
| `npm run test:publication`                                                                                                                   | Exit 0; independent ingestion/app images build; stale update rejected; untouched references preserved; all 22 browser checks pass. | Real container CLI → files → HTTP → calendar path works.                        | No live adapter or object store.                               |
| `go test ./internal/store -run TestCompetingProcessAndCrashRecovery -count=1` in the test image, with `TMPDIR` on the publication bind mount | Pass.                                                                                                                              | Writer contention and process-death recovery also work on this host bind mount. | Advisory locking; not a power-loss or network-filesystem test. |
| `docker compose config --services`                                                                                                           | Only `app`.                                                                                                                        | Default startup does not enable ingestion.                                      | Uncommenting the documented service is an operator action.     |
| Published-data desktop screenshot in `test-results/desktop-week.png`                                                                         | Inspected calendar columns, event cards, and off-site attribution.                                                                 | Published data reaches the checked layout.                                      | Small synthetic dataset.                                       |

The browser skill's Node REPL remained unavailable, so verification used repository
Playwright. The temporary publication test container and network were removed; its
artifact directory was retained and printed for inspection. Existing `.artifacts` and
source fixtures were not modified by the publication run. README and ADRs describe
permissions, expected generations, exit statuses, orphan files, and recovery limits.
Markdown source and relative links are checked; no documentation renderer exists.

Native macOS execution, Windows publication, network filesystems, S3, hardware power
loss, backups, automatic cleanup, and live-source completeness are not verified. The
next implementation boundary is the AEG pilot, including a new completeness/access
check before enabling live ingestion.

## Fifth-milestone verification — 2026-09-09

Approved scope: fixture-only AEG adapter for Gothic and Mission, literal status
metadata, and local replay through the existing reconciler/publisher. No live fetcher,
schedule, new data store, or batch coordinator was authorized or added.

The source evaluation, AEG contract, product decisions, source-config schema,
reconciler, publication command, reader, React projection, test scripts, and Docker
workflow were inspected before implementation. Confidence was 9/10 for fixture-only
work; live acceptance remained outside the approval because access review was incomplete.
One shared adapter with two configs was selected over separate venue scrapers.
The existing reconciler, confined file reader, publisher, HTTP reader, and browser
test patterns were reused. A duplicate-key check is adapter-local because the existing
artifact JSON decoder does not enforce that upstream-envelope requirement.

New tests first failed on the absent adapter/replay command, unsupported status schema,
missing HTTP status projection, and missing rendered label. Focused tests then passed.
The full browser run exposed a test that selected an event hidden by the configured
week limit. Inspecting the raw first records and calendar projection isolated that
test-input error. Selecting the existing visible fixture by stable ID made the test
pass without another production change.

Both synthetic replay records were inspected through HTTP. They contain source-scoped
IDs, actual venue names, September 12 dates, distinct doors/show instants, literal
Cancelled text, age policy, separate venue/ticket links, and no invented price.
The actual source files and catalog were written by the ingestion container, then
read by the application container. The test publisher ran with networking disabled.

| Evidence source                                                                         | Raw observation or result                                                                                                                 | Supported finding                                                                  | Material limit                                              |
| --------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| `npm run test:contracts`                                                                | Go formatting, vet, all race tests, and 66 TypeScript tests pass.                                                                         | Shared schema, adapter, reconciliation, CLI, store, and reader checks pass.        | Synthetic input only; no upstream service acceptance.       |
| `npm run lint`, `npm run typecheck`, `npm run build`, `npm run format:check`            | Exit 0.                                                                                                                                   | Applicable frontend checks pass.                                                   | Not a production load benchmark.                            |
| `npm test`                                                                              | 65 pass; Go handoff test skipped without its environment.                                                                                 | Ordinary unit suite passes.                                                        | The handoff passes separately in `test:contracts`.          |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait app fixture-app` | Images build and both services start.                                                                                                     | Changed application runs in its container.                                         | Local Docker only.                                          |
| `npm run test:e2e`                                                                      | 24 pass; two dedicated replay checks skip outside their workflow.                                                                         | Existing calendar behavior and literal status rendering pass on desktop and phone. | Chromium; status-escaping case replaces one HTTP field.     |
| `npm run test:aeg`                                                                      | Both source processes publish, repeated new-source replay fails without catalog changes, real HTTP values match, two browser checks pass. | Snapshot → reconciliation → validated files → HTTP → calendar path works.          | Authored snapshots; no live fetch or permission conclusion. |
| `npm run test:publication`                                                              | Exit 0; publication checks and 24 browser checks pass.                                                                                    | Existing publication workflow remains valid with the added command/schema.         | Two dedicated replay browser checks run separately.         |
| `docker compose config --services`; ingestion image `replay-aeg --help`                 | Only app; documented replay flags present.                                                                                                | Default startup remains app-only and command examples match the image.             | Operators can explicitly change configuration.              |

The browser skill required checking the in-app runtime first. Its Node REPL was not
available, so repository Playwright was used. The phone replay screenshot was inspected:
status, venue, doors time, policy, and links were readable in the detail panel.

Test projects removed only their own containers/networks and retained output stores
under temporary `event-calendar-aeg-*` and `event-calendar-publication-*` directories.
Normal `.artifacts` remains empty. No commit was made. The worktree was wholly
untracked on entry, so `git diff --check` does not establish a complete change review.
README and relevant ADRs were updated; Markdown source and links were inspected.
There is no repository documentation-rendering workflow.

Live access conditions, live normalization/completeness, Mission detail-page rendering,
and downstream ticket availability remain unverified. The source terms document linked
by AEG returned 404 during research. Do not enable live ingestion until that gate is
resolved. Native macOS execution, Railway, object storage, and hardware power-loss
behavior remain outside this milestone.

## One-time live local import — 2026-09-09

The user approved fetching real Gothic and Mission listings for the local app.
The unresolved terms-document review was explicitly retained as a public-deployment
consideration, not a blocker for this one-time local exercise. No executable code,
scheduled job, public deployment, or synthetic fixture was changed.

The main `.artifacts` directory was inspected and empty before publication. Both
downloads were limited to HTTPS, 35 seconds, and 4 MiB. Six raw records per source
were inspected before counts were reported. Samples included plain event titles,
provider/venue IDs, Denver local/UTC/offset-bearing instants, ticket links, literal
status, and absent price text. Five normalized records per source were inspected,
including winter MST instants, and artifact checksums matched the staging catalog.

The existing ingestion image was rebuilt. Each source ran through `replay-aeg` in an
isolated temporary store with networking disabled. The final `publish --expect none`
command installed both source artifacts together in the main `.artifacts` directory.
The unchanged application container read the new generation on its next HTTP request.

| Evidence source                                   | Raw observation or result                                                                                         | Supported finding                                 | Material limit                                              |
| ------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | ------------------------------------------------- | ----------------------------------------------------------- |
| Feed 37 and its staged artifact                   | Metadata total 57, rows 100; 57 normalized events; no rejections.                                                 | Gothic snapshot imports successfully.             | Announced listings only; no guarantee of all future events. |
| Feed 89 and its staged artifact                   | Metadata total 61, rows 100; 61 normalized events; no rejections.                                                 | Mission snapshot imports successfully.            | Same snapshot/completeness limit.                           |
| Main publication and `/api/calendar` at port 8090 | Publication committed and synced; HTTP 200 with 118 records, split 57/61.                                         | Main local instance contains both real sources.   | One-time data, not recurring ingestion.                     |
| Desktop and phone Chromium against port 8090      | POND and Insane Clown Posse cards visible; POND detail shows 19:00 MDT and the expected AXS link; no page errors. | Real records reach the calendar and detail panel. | Ticket checkout was not exercised.                          |

Main catalog generation: `g-3b86903145f0a324a5595c2da78f7848`.
Reconciliation clock: `2026-09-09T18:36:11Z`; coverage ends September 9, 2027.
Observed event ranges: Gothic September 12, 2026–June 8, 2027; Mission September 12,
2026–April 29, 2027. These bounds do not imply uninterrupted coverage.

Download evidence remains in temporary `/private/tmp/event-calendar-live-28XtXM`:

- Gothic URL: `https://aegwebprod.blob.core.windows.net/json/events/37/events.json`.
  SHA-256: `0e8659fc7bc104188d909f1d19142bde6c3795efe685e3284569ee12a968acc9`.
- Mission URL: `https://aegwebprod.blob.core.windows.net/json/events/89/events.json`.
  SHA-256: `aee7ab144d54261d5086816b936ec19fc084dc4defdea0b1cb4a437f6c34845d`.

The directory also contains staging artifacts and desktop/phone screenshots. It is
temporary diagnostic evidence, not a permanent raw-data archive. Application-ready
artifacts persist in `.artifacts`. Future refreshes must use established-source
configuration and the current generation; the new-source fixture configs cannot
overwrite these records. The main app is no longer a valid target for an empty-startup
test; use `CALENDAR_EMPTY_URL` with a separate empty instance.

The browser skill's Node REPL was unavailable; installed Playwright provided the
fallback. The desktop screenshot was inspected. No new executable behavior required
test-driven implementation or a full code-test rerun. The actual container import,
checksums, HTTP response, and browser behavior were checked instead. Documentation
formatting and local links were checked; no documentation renderer exists.

## Status and link UI revision — 2026-09-09

Approved scope: show Scheduled/Cancelled in the popup, rename the popup links to
Link/Tickets, and reduce cards to time/title/venue with a bold red Cancelled exception.
Off-site metadata remains in the popup. Raw provider status, source artifacts, link
destinations, and ingestion behavior are unchanged. The shared display module was
extended instead of normalizing or republishing stored records. The product and
artifact ADRs and README describe the new display contract.

Unit and browser tests were written first and failed on the absent status mapping
and old card contents. The first full browser command could not launch/access local
resources under the sandbox; the approved unrestricted rerun reached the app. An old
sparse-event assertion expected only Venue; it was updated to include the now-required
Status row. No production change was needed for that assertion correction.

| Evidence source                                                                         | Raw observation or result                                                                                 | Supported finding                                                                                         | Material limit                                                 |
| --------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `npm test`                                                                              | 66 pass, one Go handoff test skipped.                                                                     | Display mapping and existing frontend unit tests pass.                                                    | Go/schema code unchanged; handoff not rerun in this UI change. |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`            | Exit 0.                                                                                                   | Applicable frontend checks pass.                                                                          | No deployment beyond local Docker.                             |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait app fixture-app` | Both images built and services started.                                                                   | Updated UI runs in containers.                                                                            | Existing imported data retained.                               |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:e2e`                             | 26 pass; two replay-only checks skip.                                                                     | Both layouts, all three views, bold red cancellation text, short links, and prior calendar behavior pass. | Synthetic HTTP overrides exercise status variants.             |
| `npm run test:aeg`                                                                      | Two replay/browser checks pass.                                                                           | Generated artifacts still reach the revised UI.                                                           | Synthetic data in an isolated store.                           |
| Main-app desktop and phone browser checks                                               | POND shows Scheduled and short links; Yesenia Then shows Cancelled in the popup and bold red on its card. | Real imported records use the new presentation.                                                           | Chromium; ticket destinations checked, not checkout.           |
| Catalog SHA-256 and every referenced artifact checksum                                  | Unchanged from before the edit.                                                                           | Imported event data was preserved.                                                                        | No new source refresh performed.                               |

Browser skill setup was attempted through tool discovery; its Node REPL was unavailable,
so installed Playwright was used. The real-data desktop screenshot was inspected.
The cancellation color is `#f87171` at weight 700 on the existing `#22382c` card.
The temporary empty app on port 8093 was stopped after verification. No commit was
created. Markdown source and relative links were checked; no documentation renderer
exists in this repository.

## Discovery controls — 2026-09-09

Approved scope: venue checkboxes, public-metadata text search, upcoming/past
visibility, and browser-local persistence. Keep all events on the actual venue's
current date. Filter before display limits without changing the view or URL.
Age/cost filtering, event routes, ingestion, and outside-click dismissal are not
part of this milestone. Outside-click dismissal remains item 6 after the five
remaining MVP work groups.

The existing React state and FullCalendar projection were extended. A small pure
filter module and guarded storage functions were added after repository searches
found no existing filter or persistence implementation. Client-side filtering
uses the already-loaded full date range. Server-side search or a new state library
would add an interface or dependency without being needed for this contract.
Go handlers, stored artifacts, schemas, and ingestion were not changed.

The first unit run failed because the filter module did not exist. The first
browser run failed because the search control did not exist. After implementation,
a test incorrectly searched Mission for an upcoming fixture whose actual venue is
Warehouse. The fixture was inspected and the test changed to the known Gothic
record. No production correction was required. A live smoke check likewise selected
Mission for POND; inspecting the artifact confirmed Gothic, and the corrected check
passed. The full sandbox browser run failed at browser startup; its approved
unrestricted rerun passed.

| Evidence source                                                              | Raw observation or result                                                                                                              | Supported finding                                                                             | Material limit                                                                |
| ---------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| `npm test -- web/filters.test.ts`                                            | Six tests pass after the expected initial missing-module failure.                                                                      | Combined matching, date cutoffs, metadata search, and storage fallback work.                  | Pure functions, not browser rendering.                                        |
| `npm test`                                                                   | 72 pass; one Go handoff test skipped.                                                                                                  | Frontend regression suite passes.                                                             | Go/schema unchanged; cross-language handoff not rerun.                        |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build` | Exit 0.                                                                                                                                | Applicable frontend checks pass.                                                              | No public deployment.                                                         |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait`      | Both images build and both services start.                                                                                             | Updated UI runs in Docker.                                                                    | Local containers only.                                                        |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:e2e`                  | 32 pass, two dedicated AEG checks skip.                                                                                                | Desktop/phone filtering, persistence, navigation, empty startup, and existing UI checks pass. | Browser clock fixed to the fixture dates; server expiry still uses real time. |
| `npm run test:aeg`                                                           | Two replay-to-browser checks pass.                                                                                                     | The published AEG contract still reaches the updated UI.                                      | Isolated synthetic store, not a live refresh.                                 |
| Populated app at localhost:8090                                              | Gothic selection plus POND search displays one matching card and survives reload on desktop and phone.                                 | Real imported events work with the controls.                                                  | Isolated browser contexts; no user browser preferences changed.               |
| Local catalog and referenced source SHA-256 checks                           | Catalog remains `d42ecbc7a95b08d9dc294818631708a6247d55f558fea7ee2e319f1656f7ab6b`; both source checksums match the unchanged catalog. | Imported event data was preserved.                                                            | No ingestion run against the development store.                               |

The browser skill's Node REPL tool was unavailable, so installed Playwright was
used. The phone screenshot was inspected at
`/private/tmp/event-calendar-filters-phone.png`; the desktop screenshot is alongside
it. The temporary empty container was stopped and auto-removed; it had no persistent
mounts or event data. README and the product ADR now describe persistence and date
cutoffs. No documentation renderer exists, so Markdown source is inspected instead.
No commit was created.

## Federal import and description compatibility — September 10, 2026

Implemented the approved Federal HoldMyTicket profile and one-time live import.
The subsequent approved fix discards unused malformed DESCRIPTION blocks only for
Federal before standard parsing. It does not repair arbitrary iCalendar, switch
to HTML discovery, alter HQ/Oriental parsing, or change `.ics` exports.
The boundary contract and admission decisions are in
[ADR 0006](../adapters/0006-icalendar-feeds.md#federal-implementation--september-10-2026).

Capture, original feed, detail pages, diagnostic program, staging store, and prior
catalog remain under `/private/tmp/event-calendar-federal-R1gmBJ`. The backup is
`catalog-before.json`; original immutable source artifacts remain available.
The snapshot clock is `2026-09-10T17:15:00Z`. Public read-only feed/detail capture
used no login, checkout, or bypass. Observed robots exclusions concern admin,
staff, venues, and boxoffice paths, none of which were fetched. Recurring access
and redistribution permission are not inferred from public readability.

Six raw feed/detail pairs were inspected before counts: Extortionist, Federal
Nights, Mad Caddies, The Black Queen, Charlotte Sands, and Emo Night Brooklyn.
They contained provider IDs, matching titles/show timestamps, Federal venue,
doors, exact restrictions, and labeled USD offers with ticket URLs. The first
five said All Ages; Emo Night Brooklyn said 18+ / Bar with ID. Later inspection
of six staged records included a price-less future show and a 21+ show. All staged
titles, show timestamps, and age labels were compared with their raw JSON-LD.
No extra accompaniment requirement was found by the captured visible-text review;
unknown restricted-show permissions remain unknown, not explicit prohibitions.

| Evidence / command                                                                                                                                                                                                                                                                                                                   | Raw result                                                                                                                                                                 | Supported finding                                                                       | Material limit                                                                                                             |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Original replay and parser diagnostic                                                                                                                                                                                                                                                                                                | `hmt: invalid iCalendar`; underlying error at line 20 in Chapel text. Diagnostic removal of descriptions parsed all 35 VEVENTs.                                            | Malformed unused descriptions caused the failure.                                       | Diagnostic replacement is not production logic; raw capture preserved.                                                     |
| `npm run test:contracts`, test-first runs                                                                                                                                                                                                                                                                                            | New profile and description tests failed before their production changes; final image passed gofmt, vet, and all Go race tests.                                            | Regression reproduced and fixed; existing packages pass.                                | Final wrapper failed at the host `test-results/contracts` Docker bind mount, not Go tests.                                 |
| `docker run --rm --mount type=bind,src=/private/tmp/event-calendar-federal-R1gmBJ/handoff,dst=/handoff -e CONTRACT_HANDOFF_DIR=/handoff event-calendar-contract-tests go test ./internal/artifact -run TestConsumerHandoff -count=1`, followed by `CONTRACT_HANDOFF_DIR=/private/tmp/event-calendar-federal-R1gmBJ/handoff npm test` | Handoff passed; 86 TypeScript tests passed.                                                                                                                                | Real Go-file-to-TypeScript contract verified using a mountable temporary path.          | Established wrapper's host mount problem remains; no script changed.                                                       |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`                                                                                                                                                                                                                                                         | Passed.                                                                                                                                                                    | Repository frontend checks remain valid.                                                | No frontend behavior change.                                                                                               |
| `docker build --target ingestion -t event-calendar-ingestion .` and isolated `replay-hmt`                                                                                                                                                                                                                                            | Image built; staged durable publication with no rejected events.                                                                                                           | Original malformed feed imports through the corrected path.                             | Available captured events span September 10, 2026–May 13, 2027, not proof of a complete 12-month upstream horizon.         |
| `FEDERAL_BASE_URL=http://127.0.0.1:8094 npm run test:e2e -- tests/browser/federal.spec.ts`                                                                                                                                                                                                                                           | Two tests passed; phone modal screenshot inspected.                                                                                                                        | Staged details, prices, event/ticket links, and age-14 filtering work on desktop/phone. | Long offer labels require modal scrolling on phones.                                                                       |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:publication`                                                                                                                                                                                                                                                                  | 73 passed, 21 skipped.                                                                                                                                                     | Publication regressions and standard browser suite pass.                                | Opt-in live-source tests run separately. Earlier run had two failures because empty tests targeted the populated main app. |
| `npm run test:aeg`                                                                                                                                                                                                                                                                                                                   | Replay assertions and two browser tests passed.                                                                                                                            | Existing AEG workflow remains functional.                                               | Synthetic regression data only.                                                                                            |
| Local publication and digest/reference checks                                                                                                                                                                                                                                                                                        | Generation `g-297b6262c1e3c30696e6a687c37df106`; 35 Federal events, 32 with reviewed All Ages metadata; no rejections; all prior references unchanged; file digests valid. | Federal alone added to the local calendar.                                              | One-time snapshot; no scheduled refresh.                                                                                   |
| `FEDERAL_BASE_URL=http://127.0.0.1:8090 HMT_BASE_URL=http://127.0.0.1:8090 LIVE_ADMISSION_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/federal.spec.ts tests/browser/hmt.spec.ts tests/browser/live-admission.spec.ts`                                                                                           | All 16 passed against the main container.                                                                                                                                  | Federal, HQ/Oriental details, and prior reviewed venue policies work in the local app.  | Chromium desktop/phone; no third-party checkout or calendar-client action.                                                 |

Federal's local onboarding is complete for this reviewed snapshot. Exact All Ages
labels receive event-linked metadata for ages 0–17. Other categories and unknown
labels do not gain accompaniment exceptions. Boundary tests, policy overrides,
off-site rejection, and artifact-to-filter behavior are verified. This decision
does not complete pending HQ/Oriental policy work or authorize recurring ingestion.

## Venue dropdown — 2026-09-09

Approved scope: place existing venue checkboxes in a compact dropdown, show an
All/single-name/count summary, preserve selection persistence, and close on Escape,
outside pointer interaction, or focus leaving the selector. The event popup is
unchanged. A native button and checkbox fieldset extend the existing React state;
a single-select control would break multi-select, and no new UI dependency is needed.
Repository searches found no existing dropdown implementation to reuse.

The browser test was added first and failed because the dropdown button was absent.
After implementation, all focused checks passed without a production retry.
README and the product ADR now specify dropdown behavior. The phone screenshot at
`test-results/phone-venue-dropdown.png` was inspected; browser assertions check both
layouts for horizontal overflow, focus, summary labels, and dismissal.

| Evidence source                                                                                  | Raw observation or result                                                                                                              | Supported finding                                                                     | Material limit                                                                 |
| ------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| `npm run test:e2e -- tests/browser/filters.spec.ts`                                              | Eight checks pass.                                                                                                                     | Dropdown interaction and existing filters work on desktop and phone.                  | Fixture container; Chromium.                                                   |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:e2e`                                      | 34 pass; two dedicated replay checks skip.                                                                                             | Existing calendar and filter browser regressions pass.                                | Isolated AEG publication workflow not rerun for this presentation-only change. |
| `npm test`                                                                                       | 72 pass; one Go handoff test skips.                                                                                                    | Existing filtering and frontend contracts still pass.                                 | No Go/schema change; handoff not rerun.                                        |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `git diff --check` | Exit 0.                                                                                                                                | Applicable frontend checks pass.                                                      | Repository files are untracked; diff check alone does not inspect them.        |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait`                          | App and fixture images built; both services running.                                                                                   | Revised UI is available at localhost:8090 and verified against the fixture container. | No public deployment.                                                          |
| Catalog and source SHA-256 checks                                                                | Catalog stays `d42ecbc7a95b08d9dc294818631708a6247d55f558fea7ee2e319f1656f7ab6b`; Gothic/Mission bytes match its unchanged references. | Development event data was preserved.                                                 | No ingestion performed.                                                        |

The temporary empty test container had no persistent mounts and was stopped with
auto-removal after testing. No commit was created. Markdown source was inspected;
the repository has no documentation rendering workflow.

## Age/cost filtering and display corrections — 2026-09-09

Approved scope: exact-category Age and Cost multi-select dropdowns, effective
venue/event policy inheritance, lowest-advertised-USD-price classification rounded
up to whole dollars, configurable cost brackets, preference migration, show-only
time labels, and artist-based tie-breaking after time/venue. Existing source
artifacts, schemas, extraction, and event-popup dismissal remain unchanged.

The server adds optional `age_category` and `cost_category` to its display projection.
It reuses `EffectivePolicy` and validated integer-minor-unit price fields. The
`COST_BRACKET_LIMITS` environment setting follows the existing application settings
pattern and is passed through Compose. No browser extraction or schema revision is
needed. The venue disclosure was extracted into `FilterDropdown` and reused for all
three dimensions rather than copying its focus/dismissal logic or adding a library.
The old preference key remains valid; absent age/cost selections default to empty.

Before implementation, existing dropdown browser tests passed. New tests failed
on missing API fields, missing Go classification, ignored age/cost selections,
preference loss, and title-before-artist ordering. A browser ordering test initially
read cards before loading and before expanding the capped day; it was corrected to
wait for the day view and reproduced the actual incorrect order before rebuilding.
An edit patch failed its context check and made no changes; it was reapplied after
inspecting the target. The first phone filter test attempted to click Cost beneath
the open Age overlay; it now dismisses Age with Escape before opening Cost. No
production change was needed for that test correction.

An overlapping focused browser rerun removed trace files used by the first
publication run. That run also had already loaded the old phone test. It was not
accepted as verification. A clean publication rerun ran alone and passed. The AEG
workflow ran afterward, not concurrently. The first live API read was denied by
the sandbox (`EPERM`); the approved unrestricted read succeeded.

| Evidence source                                                                                  | Raw observation or result                                                                                                                     | Supported finding                                                                                                           | Material limit                                                                      |
| ------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| `npm test`                                                                                       | 75 passed; ordinary Go handoff test skipped.                                                                                                  | Matching, preference migration, and artist sorting pass.                                                                    | Handoff was separately exercised below.                                             |
| `npm run test:contracts`                                                                         | Go formatting, vet, all package race tests, and handoff pass; 76 frontend tests pass with the handoff.                                        | Price boundaries, policy overrides, configuration, HTTP projection, and cross-language artifacts pass.                      | Synthetic fixtures; no external source requests.                                    |
| `npm run test:e2e -- tests/browser/filters.spec.ts`                                              | Corrected run: 12 passed.                                                                                                                     | Dropdowns, combined filters, persistence, Show/Doors labels, artist sorting, and prior venue behavior pass on both layouts. | Fixture container and Chromium.                                                     |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8093 npm run test:publication`                              | Clean run publishes through the CLI and passes 38 browser tests; two AEG-specific checks skip.                                                | File publication, checksum validation, HTTP transport, and browser consumption work through the real interface.             | Isolated synthetic store; no live data writes.                                      |
| `npm run test:aeg`                                                                               | Both replay/browser checks pass.                                                                                                              | AEG artifacts still reach the app with structured age categories.                                                           | No live refresh or new numeric-price extraction.                                    |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `git diff --check` | Exit 0.                                                                                                                                       | Applicable host checks pass.                                                                                                | Repository files are untracked, so diff check is not a standalone formatting check. |
| `docker compose -f compose.yaml -f compose.test.yaml up --build --wait`                          | Both images built and services running.                                                                                                       | Updated app is available at localhost:8090 and verified in the container workflow.                                          | No public deployment.                                                               |
| Five initial imported records from each source, then five records from the running API           | Titles, venues, dates, and age categories are present; sampled prices and cost categories are absent.                                         | Existing age metadata is exposed without inventing price data.                                                              | Samples, not a claim of complete source price coverage.                             |
| Catalog/source SHA-256 checks                                                                    | Catalog stays `d42ecbc7a95b08d9dc294818631708a6247d55f558fea7ee2e319f1656f7ab6b`; Gothic/Mission source bytes match its unchanged references. | Development data was preserved.                                                                                             | No source refresh performed.                                                        |

The phone age/cost screenshot was inspected before the subsequent AEG test replaced
the browser output directory. The final publication artifacts remain at
`/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-publication-ruYTMM`;
AEG artifacts remain at
`/var/folders/3b/0wkqplln75jg377z421hdn280000gn/T/event-calendar-aeg-nlcfQx`.
Temporary workflow containers/networks and the empty test container were removed;
the empty container had no persistent mounts. README, product ADR, and artifact ADR
were updated and their Markdown source inspected. No documentation renderer exists.
No commit was created.
## Dazzle integration verification — September 14, 2026

Added the explicit Dazzle VenuePilot profile, locale configuration, distinct accent,
synthetic Go tests, browser checks and a tracked source snapshot. See
[the adapter record](../adapters/0020-venuepilot-widgets.md#dazzle-implementation--september-14-2026).
Nocturne and Seventh Circle remain research-only. No remote deployment was run.

| Evidence source | Raw observation / result | Supported finding | Material limit |
| --- | --- | --- | --- |
| New Go tests before implementation | Unsupported Dazzle configuration; later membership-specific test accepted a product | Tests reproduced missing behavior before each production change | Synthetic clocks, cancellations and invalid records |
| `docker build --target go-test -t event-calendar-contract-tests .` | gofmt, go vet and full race-enabled Go suite passed | Adapter and existing Go behavior verified | Go absent on host; checks ran in Docker |
| `npm run test:venuepilot`, `npm run test:capture-profiles`, `npm run test:refresh`, `npm run test:locale-tools` | All passed | Capture selection, pagination and refresh coordination remain compatible | Network failures are synthetic in these tests |
| `npm run test:contracts` | Go handoff and all 91 consumer tests passed | Artifact producer/consumer boundary verified | No schema change |
| `npm run test:locales` | Independent images and four browser checks passed | Locale isolation preserved | Local containers, not remote Railway |
| `npm run build`, `npm run lint`, `npm run typecheck`, `npm run format:check`, `git diff --check` | Passed | Build and configured static checks pass | Markdown ADRs inspected as source; no documentation renderer |
| Dazzle and Levitt browser specs with both base URLs set to port 8090 | Four tests passed on desktop and phone | Age-14 filter, policy text/link, event/ticket links and ICS work | Other source-specific checks require separate opt-in URLs |

Captured at `2026-09-14T21:36:29.255Z`; staged at
`/private/tmp/dazzle-stage.ayklZ5`. The staging directory retains the original
local `catalog-before.json` for recovery; previous immutable source files remain.
Local publication generation is `g-f705ab50b82d12276ab4fa923a09790c`.
Tracked snapshot generation is `g-1f407d8da5f407e52a7159939e7b1f50`.
Both publications used the generation-guarded publisher and reported durable success.
Five raw, five normalized and five API records were inspected. All earlier local
and tracked source references were unchanged. Only Dazzle was added.

The initial full browser run had 162 passing tests, 64 optional skips and two
empty-startup failures: `CALENDAR_EMPTY_URL` was unset and defaulted to the populated
port 8090. Reverification uses a separate empty instance on port 8096. A focused
browser assertion also needed the existing `.admission-clearance` scope because
both policy rows can contain a link with the same accessible name. Neither failure
required an application change.

The corrected full run passed 164 tests with 64 optional skips using
`CALENDAR_EMPTY_URL=http://127.0.0.1:8096 DAZZLE_BASE_URL=http://127.0.0.1:8090 LEVITT_BASE_URL=http://127.0.0.1:8090 npm run test:e2e`.
The temporary empty container was stopped after verification. The populated app
remains running on port 8090. No commit or push was performed.
## Seventh Circle integration — September 14, 2026

Approved scope: source-specific public HTML capture/replay, reviewed All Ages
policy with `$5 annual fee; show donations encouraged`, confirmed doors only,
locale configuration, tests, local publication and tracked snapshot. Nocturne
remains research-only. No external deployment, commit or push was performed.

| Evidence source | Raw observation / result | Supported finding | Material limit |
| --- | --- | --- | --- |
| Capture and Go tests before implementation | Missing capture module and decoder; browser test found zero published records | Tests first demonstrated the missing feature | Synthetic fixtures exercise unobserved failures |
| `npm run test:seventh-circle` | Three capture tests passed | Paired discovery, bounds and failure handling verified | Live capture separately checked |
| `docker build --target go-test -t event-calendar-contract-tests .` | gofmt, go vet and full race-enabled Go suite passed | Parser, reconciliation and existing Go tests pass | Go is not installed on host |
| Live paired capture and replay | All six raw and normalized records inspected; six published, zero rejected | Listing/detail mapping works for the current source | Unknown empty layout and future pagination fail safely |
| `npm run test:refresh`, `npm run test:refresh-dry-run`, `npm run test:deploy`, `npm run test:capture-profiles` | Passed | New runner fits refresh configuration; workflow regressions pass | Test servers needed sandbox escalation; no real deployment invoked |
| `docker build --target refresh-test -t event-calendar-refresh-tests .` and `docker run --rm event-calendar-refresh-tests` | Build passed; 13 tests passed | Container refresh workflow remains functional | Synthetic network fixtures |
| `npm run test:locales` | Independent builds and four browser checks passed | Locale packaging and runtime isolation preserved | Local Docker only |
| Contract handoff and `CONTRACT_HANDOFF_DIR=test-results/contracts npm test` | Go handoff and 91 consumer tests passed | Producer/consumer validation completed | Standard `npm run test:contracts` twice failed at Docker's nested bind mount; same handoff used repository-root mount instead |
| `npm run lint`, `npm run typecheck`, `npm run format:check`, `git diff --check`; Compose app build | Passed | Static checks and runtime image build verified | ADRs reviewed as Markdown source; no renderer exists |
| Seventh Circle desktop/phone browser test and API | Both tests passed; all six API records inspected | Approved fee text, age-14 filtering, event link and ICS work | Five unlabeled clocks deliberately omitted; one confirmed door time retained |

Capture: `event-calendar-seventh-circle-7LClxu`, captured at
`2026-09-14T22:01:54.688Z`. Staging and backup catalog files remain at
`/private/tmp/seventh-circle-stage.ELC4Bs`.
Local generation: `g-b8ed1ccded41be7b5c34d3c20887b256`.
Tracked generation: `g-561c6147162f887a6f7ef9b060cbae66`.
Both publications reported durable success through the generation-guarded
publisher. All prior local and tracked source references were unchanged. The
established source config requires this prior artifact on subsequent refreshes.

The contract workaround ran the existing Go `TestConsumerHandoff` in the same
image with `/Users/will/Documents/g/event-calendar` mounted at `/repo` and
`CONTRACT_HANDOFF_DIR=/repo/test-results/contracts`, then ran the existing npm
consumer tests against those generated files. No contract check was removed.

The full browser run passed 166 tests with 64 optional skips using
`CALENDAR_EMPTY_URL=http://127.0.0.1:8096 SEVENTH_CIRCLE_BASE_URL=http://127.0.0.1:8090 DAZZLE_BASE_URL=http://127.0.0.1:8090 LEVITT_BASE_URL=http://127.0.0.1:8090 npm run test:e2e`.
The temporary empty-calendar container was stopped; the populated app remains
running at port 8090. Browser skill fallback used repository Playwright because
the required Node REPL execution tool was unavailable.
