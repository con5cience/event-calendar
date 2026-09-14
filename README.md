# Event calendar

## Locale selection

Each deployment contains one locale, with no shared runtime state. Start Denver:

```sh
docker compose --env-file locales/denver/compose.env up --build -d
```

`locales/<id>/site.json` owns branding, domain, timezone, formatting, view defaults,
venue colors and source ownership. Its `sources/` directory owns operational
source configs; `capture.json` owns capture endpoints. Artifacts live under
`.artifacts/<id>/`. Other cities need their own configuration, asset, source
profiles, data directory, Compose project/port, and Railway service. No real
second locale is preconfigured. See [ADR 0023](docs/adr/0023-locale-isolation.md).

Before running the capture commands below, select their locale explicitly:

```sh
export SITE_DIR="$PWD/locales/denver"
```

Use `locales/denver/sources/<source>.yaml` for operational replay and admission
updates. Files under `internal/*/testdata` remain test fixtures. Capture never
runs during an application build or startup. Use a separate temporary output
directory for each job; `TMPDIR` can select a per-locale capture parent.

Denver's operational profiles use `state: established`. Seed each replay staging
directory with a validated copy of the current `.artifacts/denver` generation;
do not start an established source in an empty store. This preserves event IDs
and URLs. The standalone snapshot packager can make this copy:

```sh
docker build --target backend -f Dockerfile.railway -t event-calendar-snapshot-tools .
stage_parent=$(mktemp -d)
docker run --rm --mount type=bind,src="$PWD",dst=/repo,readonly --mount type=bind,src="$stage_parent",dst=/stage --entrypoint /package-snapshot event-calendar-snapshot-tools --site /repo/locales/denver /repo/.artifacts/denver /stage/data
```

Mount `$stage_parent/data` at `/data` for replay. For a genuinely new source,
start its reviewed operational profile with `state: new`, then switch it to
`established` after the first successful publication. Never change an existing
source back to `new` to bypass missing prior data.

Standalone deployment contexts are created with
`node scripts/package-locale.mjs <locale> <new-directory>`; see the Railway
section below. `npm run test:locales` verifies two isolated builds and running
instances using a synthetic second-locale fixture. It does not deploy remotely.

Denver's favicon is `locales/denver/assets/favicon.png`, copied from the approved
`docs/denver-purple.png`: transparent background and `#6C5CE7` artwork.
The Go host serves the selected locale's image at `/assets/favicon.png`.

The Go host serves page-specific titles, descriptions, canonical URLs, Open Graph
and summary-card metadata, plus basic HTML content before React starts. The
approved Denver logo is also the shared preview image. Canonical URLs use the selected locale's fixed origin (`https://denver.withadult.com`
for Denver); request hosts and query strings cannot change them.
`/sitemap.xml` includes the homepage and listed, unexpired events; `/robots.txt`
points crawlers to it. Removed-but-retained events remain accessible with
`noindex`; expired events still return 404. Copy Event Link still copies the
venue's event URL, not this site's URL. External preview rendering must be checked
after deployment. These features require the Go host, not the Vite dev server.

The server-rendered fallback list uses calendar order: date, venue-local doors
(otherwise show time), venue, then artist/title. Untimed events come first within
each date. With JavaScript enabled, a small same-origin startup script shows a
dark loading shell before the app bundle starts, so the fallback list does not
flash. The sorted fallback remains visible without JavaScript and returns if
startup fails or stalls for 15 seconds. SEO metadata and event links stay in the
HTML. Only same-origin module entry scripts' load/runtime errors trigger
immediate fallback; blocked optional scripts such as injected analytics do not.
The Content Security Policy remains unchanged. Configuration failures still
signal fallback explicitly, and unknown startup failures retain the bounded wait.

The app is a React/FullCalendar interface served by Go, with local JSON input.
Its charcoal-and-purple palette matches music-finder: near-black background,
charcoal surfaces, purple controls, and gray text. Small links and focus outlines
use lighter purple for contrast; cancellation remains red. Layout and fonts are unchanged.
Startup with an empty data directory has no events. No ingestion jobs, database, or external API calls run.
The app reads version-one catalogs and source artifacts. A separate ingestion image
can publish prepared artifacts locally or replay source snapshots. Supported capture
scripts run only when an operator invokes them. Scheduled jobs are not enabled.

This workspace's main app was populated with one-time real imports on
September 9, 2026: 248 events across Gothic, Mission, Bluebird, Ogden, and Fiddler's
Green at [localhost:8090](http://localhost:8090). The data lives
in ignored `.artifacts/denver/`, not in committed fixtures. A validated copy is now
available under `locales/denver/catalog/` for the tracked-snapshot workflow below.
The original `.artifacts/`
generation remains available as a migration backup. Reload the page to read it.
It will not refresh automatically. See the [initial import record](docs/adr/0019-implementation-and-verification-plan.md#aeg-source-expansion--2026-09-09) and the source additions below.

## Run locally

### Refresh a locale's tracked snapshot

`locales/<id>/catalog/` contains the deployment snapshot: the catalog manifest and
only its referenced source artifacts. Keep these exact bytes; the manifest checksums
cover them, and `.prettierignore` excludes them. Raw captures, reports, savepoints,
and recovery copies stay under ignored `.artifacts/refresh/<id>/run-*/`.

Build the separate operator image (this does not fetch events):

```sh
docker build --target refresh -t event-calendar-refresh .
```

To initialize or explicitly replace Denver's tracked snapshot from a validated
local store:

```sh
docker run --rm --mount type=bind,src="$PWD",dst=/repo event-calendar-refresh --snapshot denver /repo/.artifacts/denver
```

To refresh all configured sources, starting from that tracked snapshot:

```sh
docker run --rm --mount type=bind,src="$PWD",dst=/repo event-calendar-refresh denver
```

The equivalent host command is `node scripts/refresh-locale.mjs denver`, with
`ingest` and `package-snapshot` on PATH (or `INGEST_BIN` and `SNAPSHOT_BIN` set to
their executable paths), npm dependencies, and Playwright Chromium installed.
The container supplies these dependencies. It runs only the explicitly requested
operator action; Compose and application startup never refresh sources.

Captures run with concurrency 2 by default. Set `CAPTURE_CONCURRENCY` to an integer
from 1 to 4 to change the limit; use 1 for serial capture or a low-memory runner.
For Docker, add `-e CAPTURE_CONCURRENCY=1` before the image name. Each adapter
family is serialized, with shared groups for KSE's two adapters and the two Wix
adapters. Capture passes and requests within a source stay sequential.
All captures finish before serial ingestion and validated catalog publication.
Failures retain the source's previous data. No concurrent catalog writers run.

The CLI streams child stdout/stderr to stderr while retaining child stdout for
JSON parsing. Its own stdout remains one final JSON result. Timestamped progress
identifies each source and capture/ingestion/validation stage. Quiet subprocesses
emit periodic “still running” messages (30-second checks); these messages do not
mean a source succeeded or extend the existing timeout.

Each source runs in an isolated process with a fifteen-minute process limit.
Capture output goes to an explicit directory. Existing Go adapters apply the
normal twelve-month window, retention, identity, rejection, and admission rules.
Jobs run sequentially. Each gets a separate validated store copy; failed jobs
cannot affect later jobs. The final validated snapshot replaces only the selected
locale's tracked catalog. The prior snapshot remains in the run's `previous/`
directory for recovery. No Git command or deployment runs.

Exit codes: `0` means all jobs published without rejected records; `2` means a
validated snapshot was exported with source failures or rejected records; `1`
means the run failed. If every source fails, the tracked snapshot stays unchanged.
The JSON report identifies individual failures and rejections. Successful refreshes
currently produce new generation IDs/timestamps even when event data is unchanged;
semantic no-op detection is not implemented.

A per-locale `.refresh-lock/` prevents overlapping export/refresh commands. An
interrupted process can leave this lock behind. Before removing an abandoned
empty lock with `rmdir`, confirm no refresh for that locale is running. A process
interrupted during snapshot replacement can leave the previous catalog under the
reported run directory; restore that directory before another refresh. Do not use
this replacement operation on a live application's mounted store.

Verification:

```sh
npm run test:refresh
docker build --target refresh-test -t event-calendar-refresh-test .
docker run --rm event-calendar-refresh-test
```

The integration image uses local synthetic HTTP responses and the real Go
publisher, snapshot validator, and calendar HTTP consumer. It also checks HMT
HTML extraction against the Go parser. These checks do not prove live access
from GitHub-hosted runners. The manual Actions dry run is described below;
scheduling remains separate work. Manual Git publishing and Railway deployment
are described below.
`Dockerfile.railway` consumes the tracked catalog; the local
Compose app still consumes its independent `.artifacts/<id>` working store.

### Manual GitHub Actions refresh dry run

`.github/workflows/refresh-dry-run.yml` provides **Locale refresh and deploy** with a
manual `workflow_dispatch` trigger. After this workflow is pushed to the default
branch, select **Actions → Locale refresh and deploy → Run workflow → denver**.
Defaults are locale `denver`, `deploy` enabled, and capture concurrency `3`.
Select branch `main` to deploy. Uncheck `deploy` for a read-only dry run.
Pushes do not trigger this workflow.
No remote run is implied by local verification.

The manual run accepts `capture_concurrency` (1–4, default 3) and retains the
selected value in `capture-concurrency.txt`. Temporary Roxy probe inputs,
the standalone diagnostics workflow, and memory sampling have been retired.
Refresh logs, reports, snapshot artifacts, and deployment checks remain enabled.

Roxy ingestion supports proxied curl through `ROXY_PROXY_URL`. The refresh
workflow maps the existing Actions `HTTP_PROXY` secret to that variable and sets
`ROXY_PROXY_REQUIRED=1`: missing or invalid configuration fails Roxy and retains
its last-valid data. It does not silently fall back to direct access. The refresh
coordinator passes these variables only to Roxy's capture process; all other
capture, ingestion, and packaging children have them removed. Curl also removes
proxy-related environment variables and receives credentials through stdin.

Local runs without `ROXY_PROXY_URL` keep the existing direct Node fetch behavior,
unless `ROXY_PROXY_REQUIRED=1` is set. This selection applies to the scheduled
capture entry point (`scripts/capture-source.mjs`), not the legacy standalone
`tests/afton/capture.mjs` command. Both listing passes and native Afton detail pages
use the selected transport. External ticket-provider pages are not fetched.

The curl transport accepts only the configured Afton listing and canonical
native event URLs. It keeps TLS validation enabled, rejects
redirects, limits each request to 30 seconds across all attempts (15 seconds for each connection, capped by the remaining budget), and
rejects bodies over one MiB or headers over 16 KiB. The combined receive buffer
is bounded to these limits before parsing. It preserves body bytes for the
existing JSON/HTML validators and omits cookies and arbitrary response headers.
Raw curl errors and credentials are never printed. Validation, reconciliation,
and publication still use the existing Go adapter and artifact contract.
At most two retries are allowed for curl transport exits 7, 28, 35, 52, or 56,
and only when curl reports no CONNECT response or an accepted tunnel. A fresh
attempt discards all bytes from the failed attempt. Proxy rejection (including
407), certificate verification failure, cancellation, invalid data, redirects,
and size-limit failures do not retry. The 30-second deadline never resets.

For native Roxy detail pages, the observed HTTP 202 HTML response carrying
`x-amzn-waf-action: challenge` is also eligible for those same two retries.
Transport errors and detail challenges share one attempt count and deadline;
there is no nested retry loop. Other HTTP failures and empty HTTP 200 responses
do not retry. Capture requires HTTP 200, no challenge header, and nonempty
compacted HTML before saving a detail. Listing behavior is unchanged.
Safe detail diagnostics report event ID, capture pass, status, and byte counts,
not response bodies or credentials. Exhausted retries still fail the source and
retain its last-valid data; the all-source deployment gate is unchanged.

Run `npm run test:roxy-transport` for transport and secret-isolation tests; curl
and loopback permission are required. `docker run --rm event-calendar-refresh-test`
also runs these tests in the refresh image.

The refresh job uses read-only repository permissions and no Railway secret. It
builds the existing refresh image, copies only the selected locale into a new
runner directory, and captures fresh source data there. It does not start from
an empty catalog: the tracked snapshot supplies identity and last-valid records.
Only the runner's disposable checkout changes for the subsequent application build.

Exit code `1` (or any unexpected exit code), a missing report, or inconsistent
source results prevents the application build and snapshot upload. Exit code `2`
continues with the validated partial snapshot, but the final workflow status is
failed so operators see the source failures or rejected records. A successful
build must also serve `/healthz`, the selected locale at `/api/site`, and a
nonempty event list at `/api/calendar`. This dry run treats an empty calendar as
a failed smoke check; the application itself still supports empty calendars.

Download `snapshot-<locale>-<run>-<attempt>` and
`report-<locale>-<run>-<attempt>` from the run's artifacts. Retention is seven days.
The snapshot contains the manifest and referenced source files, not raw captures
or backup generations. Diagnostics include the refresh log, JSON report, source
summary, and HTTP-check sample when those stages complete. Treat reports as public
repository diagnostics. Only the dedicated Roxy proxy configuration reaches
capture; the Railway credential never reaches ingestion.

The refresh step streams its combined output through `tee` and keeps the same
output in `refresh.log`. It preserves Docker's exit code, checks log-write
failure separately, and disables GitHub workflow-command interpretation while
streaming source output. The report helper runs inside the refresh container
with read-only access to the capture tree and write access only to the diagnostics
mount. It exports the JSON report and summary for the runner. Root-owned private
run directories remain private; no recursive permission change is needed.

Runs for one locale cannot overlap. The 180-minute job timeout is an operational
cap, not a duration estimate. A timeout can leave only partial diagnostics and
does not produce a verified snapshot. With `deploy` unchecked there is no Git
commit/push or Railway deployment. Neither mode changes the local Compose data
store. There is no schedule yet.

Local checks: `npm run test:refresh-dry-run`, `npm run test:refresh`, and the
existing `refresh-test` container. Workflow syntax is checked with `actionlint`.
GitHub runner access to each venue and artifact upload permissions require a
separately authorized remote run.

### Manual Railway deployment through Actions

Select branch `main`, locale `denver`, and enable `deploy` in **Locale refresh
and deploy**. Keep the tested capture concurrency and Roxy proxy configuration.
Optional probes are not required for deployment.

Required repository secrets are `HTTP_PROXY` (the existing full Roxy proxy URL)
and `WITHADULT_RAILWAY_API_TOKEN` (the Railway project token for `withadult.com` /
`production`). Actions maps the latter to `RAILWAY_TOKEN`, not `RAILWAY_API_TOKEN`,
only for credential checking and deployment. Never commit either secret value.

Only the deployment job requests `contents: write`. The repository must permit
its `GITHUB_TOKEN` to push to `main`; branch-protection failures stop the workflow
before upload. Deployment requires every registered source to publish. Individual
record rejections remain in the report but do not block deployment. Provider
validation and source failure retention are unchanged.

The job downloads this run attempt's verified snapshot and report, validates and
packages the locale, and commits only its catalog. It rejects unrelated changes
and stops if `main` advanced during refresh. Pushes are fast-forward only. A
second branch check runs immediately before upload. Do not run concurrent manual
deployments to the same Railway service.

Railway CLI `5.26.2` uploads the isolated context to the exact target in
`.github/railway-denver.json`. The job polls the upload's deployment ID and checks
`/healthz`, `/api/site`, and `/api/calendar`. The event-array SHA-256 must match the
local container check. An event-retention expiry during deployment can cause a
safe mismatch failure; inspect and rerun instead of weakening the check.

`deployment-<locale>-<run>-<attempt>` retains the snapshot commit, upload receipt,
and live-check result when available, for seven days. Upload alone is not success.
The first remote run remains unverified until it is executed.

Denver uses `Dockerfile.railway`, `LOCALE=denver`, healthcheck `/healthz`, and port
8080 at `https://denver-production.up.railway.app`. No volume, Git autodeploy, or
custom DNS was added. The site's canonical origin remains `https://denver.withadult.com`;
connect that domain separately before launch.

Recovery: capture, validation, or push failures do not deploy. Deployment failure
can leave a newer snapshot commit in Git; start a fresh run after fixing the
failure. Do not force-push or roll back Git automatically. Retain previous healthy
Railway deployments for later operator rollback. The first deployment has no
previous image. A failed post-deployment check reports failure but does not
automatically remove or replace the deployment.

Checks: `npm run test:deploy`, `npm run test:refresh-dry-run`, `npm run test:refresh`,
repository checks, `actionlint`, and the refresh-test and Railway app containers.
Local checks do not prove Actions secret access, branch-write permission, or
remote service health.

### Roxy / Afton capture and replay

Run `node tests/afton/capture.mjs` to read all pages of Roxy's public Afton widget
twice and enrich its Afton-hosted events with linked event pages. The
[Afton profile](docs/adapters/0022-afton-venue-events.md) validates identities,
pagination, both detail layouts, Denver clocks and explicit event admission.
External ticket pages are not fetched; their unverified ages and times stay unset.
No prices are published. No browser or credentials are needed for capture.

Build the ingestion image, mount the reported capture read-only at `/capture`,
a seeded staging directory at `/data`, and `locales/denver/sources` read-only at
`/config`, then run:

```sh
replay-afton --store /data --config /config/roxy.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Never publish the synthetic HTML fixture. Subsequent refreshes need an operator
config with `state: established`. Validate staging before generation-guarded
publication. Run `npm run test:afton`, `npm run test:rhp` (shared compaction),
`npm run test:contracts`, and `tests/browser/afton.spec.ts` with `ROXY_BASE_URL`.
Unknown empty or incomplete responses fail safely. Recurring jobs remain disabled.

The September 12 local import added 26 Roxy events through March 4, 2027: 20 All
Ages, one 21+, and five externally ticketed events with unknown ages. The first 20
qualify for the child-age filter; no blanket venue exception is inferred. The
catalog now contains all 26 dedicated venue candidates, with the previous 25
sources unchanged. Aggregators remain deferred.

### Herb's capture and replay

Run `node tests/herbs/capture.mjs` to read the ordinary public calendar twice.
The [Herb's HTML profile](docs/adapters/0012-html-event-listings.md#herbs-implementation-and-admission-review--september-12-2026)
does not fetch JSON, month queries, incoming ICS or ticket pages. Printed dates
and clocks are treated as Denver-local by explicit approval, despite the site's
New York setting. This is an assumption, not confirmation from the venue.

Build the ingestion image, mount the reported capture read-only at `/capture`,
a seeded staging directory at `/data`, and `locales/denver/sources` read-only at
`/config`, then run:

```sh
replay-herbs --store /data --config /config/herbs.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Never publish the synthetic HTML fixture. Later refreshes need an operator config
with `state: established`. Validate staging before generation-guarded publication.
Run `npm run test:herbs`, `npm run test:contracts`, and `tests/browser/herbs.spec.ts`
with `HERBS_BASE_URL` pointing to the staged app. Empty or changed layout fails
safely. Recurring jobs remain disabled.

The September 12 import added 22 events through September 30. All start before
22:30 and support the age-14 parent filter with the condition
`Parent required; minors must leave by 10:30 PM`. No prices, ticket links or doors
are inferred. The local catalog now contains 25 dedicated venue sources; all 24
earlier imports are unchanged. Roxy remains to be integrated.

### Black Buzzard capture and replay

Run `node tests/buzzard/capture.mjs` for two reads each of the public calendar and
homepage. The [Black Buzzard HTML profile](docs/adapters/0012-html-event-listings.md)
cross-checks calendar cards, homepage Event JSON-LD and age cards before accepting
the snapshot. It never follows `/events/` pages or ticket links. No prices or
ambiguous times are published. The reviewed 18+ venue default supplies no guardian
exception; placeholder age text is ignored.

Build the ingestion image, mount the reported capture read-only at `/capture`,
a seeded staging directory at `/data`, and `locales/denver/sources`
read-only at `/config`, then run:

```sh
replay-buzzard --store /data --config /config/black-buzzard.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Never publish the synthetic fixtures. For later refreshes, use an operator config
with `state: established`. Validate staging before generation-guarded publication.
Run `npm run test:buzzard`, `npm run test:ophelias` (shared HTTP transport),
`npm run test:contracts`, and `tests/browser/buzzard.spec.ts` with `BUZZARD_BASE_URL`.
Empty or changed markup fails safely. Recurring jobs remain disabled.

The September 12 local import added 13 events through December 5, 2026, all using
the reviewed 18+ default without guardian clearance. The catalog contains 24
dedicated venue sources; earlier imports are unchanged.

### Ophelia's capture and replay

Run `node tests/ophelias/capture.mjs` for two ordinary HTTP reads of the public
calendar. Replay uses the [scoped HTML profile](docs/adapters/0012-html-event-listings.md)
to compare parsed event fields; dynamic page scripts are never executed. Raw pages
are temporary operator evidence, not application input. No prices are published.

Build the ingestion image, mount the reported capture read-only at `/capture`,
a seeded staging directory at `/data`, and `locales/denver/sources`
read-only at `/config`, then run:

```sh
replay-ophelias --store /data --config /config/ophelias.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Use an operator config with `state: established` for later refreshes. Never
publish the synthetic HTML fixture. Validate staging before guarded publication.
Run `npm run test:ophelias`, `npm run test:contracts`, and
`tests/browser/ophelias.spec.ts` with `OPHELIAS_BASE_URL` set.
An unrecognized or empty page fails without removing prior events; the site's
empty-calendar markup has not been verified. No recurring jobs are enabled.

The September 12 import added 38 events through December 12, 2026: seven 16+,
four 18+, and 27 21+. Eleven have reviewed age-14 guardian eligibility.
The local catalog contains 23 dedicated venue sources; prior imports are unchanged.

### Meow Wolf capture and replay

The [Meow Wolf embedded-data profile](docs/adapters/0011-embedded-application-json.md)
uses `node tests/meowwolf/capture.mjs`. Chromium reads the venue's normal public
calendar and detail pages. A snapshot is written only after two complete passes
agree. The reader supports the inspected legacy and replacement serializations.
No prices or automatic jobs are added.

Build the ingestion image, mount a verified capture read-only at `/capture`,
a seeded staging directory at `/data`, and `locales/denver/sources`
read-only at `/config`, then run:

```sh
replay-meowwolf --store /data --config /config/meow-wolf-denver.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Never publish the synthetic JSON fixture. Use an operator configuration with
`state: established` for later refreshes. Validate staging before guarded
publication. Checks are `npm run test:meowwolf`, `npm run test:contracts`, and
`tests/browser/meowwolf.spec.ts` with `MEOWWOLF_BASE_URL` set. With Adult eligibility
requires explicit All Ages terms; restricted events receive no underage waiver.

The September 12 local import added 50 events through January 23, 2027: 24 All
Ages, 14 18+, 11 21+, and one with unknown admission. One event is cancelled.
The catalog now contains 22 dedicated venue sources. Earlier imports are unchanged.

### Seventh Circle capture and replay

Seventh Circle uses a paired public HTML listing/detail capture, not its
login-gated Google Calendar export:

```sh
SITE_DIR=locales/denver CAPTURE_SOURCE=seventh-circle node tests/seventh-circle/capture.mjs
```

Replay the reported snapshot with `replay-seventh-circle --store /data --config /config/seventh-circle.yaml --snapshot /capture/snapshot.json --now <captured_at>`.
Mount the capture and `locales/denver/sources` read-only and use the established
source config with its prior artifact. Only explicitly new staging stores use a
config copy with `state: new`. Validate staging before guarded publication.

The source's All Ages rule displays `$5 annual fee; show donations encouraged`.
Only explicit, matching door labels produce door times. Unlabeled clocks, ticket
URLs and price fields are not inferred. Unknown empty layouts and pagination fail
safely. Missing details are rejected observations, preserving prior valid records.
Tests: `npm run test:seventh-circle`, the Docker Go suite, and
`SEVENTH_CIRCLE_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/seventh-circle.spec.ts`.

### Levitt capture and replay

Dazzle uses the same capture with its own profile:

```sh
SITE_DIR=locales/denver CAPTURE_SOURCE=dazzle node tests/venuepilot/capture.mjs
```

Replay with `locales/denver/sources/dazzle.yaml` and the reported capture clock.
The operational configuration is established and requires its prior snapshot.
For an explicitly empty staging store only, use a copy with `state: new`.
The `layout: dazzle` profile supports NO COVER listings without inventing ticket
links, rejects the membership product, and applies the reviewed 11 PM cutoff only
to scheduled, explicitly All Ages shows with a known start before that time.
The With Adult condition is `Under 21 must leave by 11 PM`. Restricted and unknown
events get no exception. `DAZZLE_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/dazzle.spec.ts`
verifies the local publication. See the [Dazzle implementation record](docs/adapters/0020-venuepilot-widgets.md#dazzle-implementation--september-14-2026).

Levitt uses the [VenuePilot GraphQL adapter](docs/adapters/0020-venuepilot-widgets.md).
Run `node tests/venuepilot/capture.mjs` to capture and recheck the public calendar.
No credentials or browser session are required. The request covers today through
12 months ahead; only available events are returned.

Build the ingestion image, mount the reported capture read-only at `/capture`,
a seeded staging directory at `/data`, and `locales/denver/sources`
read-only at `/config`, then run:

```sh
replay-venuepilot --store /data --config /config/levitt.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Use an operator config with `state: established` for later refreshes. Validate
staging before generation-guarded publication. Never publish synthetic fixtures.
Run `npm run test:venuepilot`, `npm run test:contracts`, and the browser spec
`tests/browser/venuepilot.spec.ts` with `LEVITT_BASE_URL` set.
Explicit All Ages events support With Adult filtering; guests 16 and younger need
an adult. Restricted or unknown events receive no inferred adult exception.
No prices, automatic jobs, or application changes are added.
The September 12 local import added ten events through October 11, 2026: eight
All Ages and two 21+. The catalog contains 21 dedicated venue sources.

### Black Box capture and replay

Black Box uses the [scoped Supabase adapter](docs/adapters/0004-supabase-rest-api.md)
with admission enrichment from official ticket pages.
Run `node tests/blackbox/capture.mjs`; it enumerates and rechecks the public
event query and event-level admission terms. Anonymous credentials stay in memory.
The main room and Lounge share The Black Box venue filter. External ticket
provider redirects are rejected without fetching the external destination.

Build the ingestion image, mount the reported capture read-only at `/capture`,
a seeded staging directory at `/data`, and `locales/denver/sources`
read-only at `/config`, then run:

```sh
replay-blackbox --store /data --config /config/black-box.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Use an operator config with `state: established` when refreshing a store that
already contains Black Box. Validate staging before generation-guarded publication.
Do not publish synthetic fixtures. Run `npm run test:blackbox`,
`npm run test:contracts`, and the browser spec `tests/browser/blackbox.spec.ts`
with `BLACKBOX_BASE_URL` set. No adult exception to the reviewed 18+ policy
was found; unknown terms do not receive clearance. No prices or schedules are added.
The September 11 local import added 43 events through November 21, 2026, all 18+.
One Dice-linked event was rejected. There are now 20 dedicated venue sources.

### Fillmore capture and replay

Fillmore uses the [Live Nation browser-flow adapter](docs/adapters/0021-livenation-venue-events.md#fillmore-profile-and-admission-review--september-11-2026)
against the official venue site, not the supplied independent resale guide.
Run `node tests/marquis/capture.mjs fillmore`, then use the Marquis container
workflow below with `/config/fillmore.yaml`. Subsequent imports into a store
containing Fillmore require an operator config with `state: established`.
Do not publish synthetic JSON fixtures.

The September 11 local import added 46 valid events through April 3, 2027.
Three multi-day passes reject on conflicting time data; daily events remain.
All Ages requires a ticket at every age. The 16+ policy permits ages 16–17 with
valid ID, but never younger guests even with an adult. Missing restrictions remain
unknown. Set `FILLMORE_BASE_URL` for `tests/browser/marquis.spec.ts`.
No prices, schedules, or app changes are introduced.
The local catalog now contains 19 dedicated venue sources.

### Summit capture and replay

Summit uses the [Live Nation browser-flow adapter](docs/adapters/0021-livenation-venue-events.md#summit-profile-and-admission-review--september-11-2026).
It includes Summit and Moonroom under one venue filter, with separately ticketed
events kept separate. Run `node tests/marquis/capture.mjs summit` explicitly.
Use the Marquis container workflow below, replacing the config with
`/config/summit.yaml`. Use `state: established` for subsequent imports into a
store that already contains Summit. Never publish synthetic JSON fixtures.

The September 11 local import added 53 events through May 2, 2027: 48 All Ages,
four 18+, and one 21+. The Itchy-O two-day pass rejects because its midnight
listing timestamp conflicts with its 7 p.m. doors note; daily listings remain.
Summit's reviewed policy permits All Ages clearance but provides no adult waiver
for restricted shows. Set `SUMMIT_BASE_URL` for `tests/browser/marquis.spec.ts`.
No prices, schedules, or application changes are required.
The local catalog now contains 18 dedicated venue sources.

### Ball Arena capture and replay

Ball uses the [KSE calendar variant](docs/adapters/0009-kse-event-apis.md), enriched
with its official HTML listing. The September 11 import added 119 dated events
through May 16, 2027; one undated listing was rejected. There are now 17 dedicated
venue sources locally. Run `node tests/kse/ball-capture.mjs` explicitly;
it captures and rechecks both interfaces without publishing. Build the ingestion
image, mount the reported capture directory read-only at `/capture`, a staging
directory at `/data`, and `locales/denver/sources` read-only at `/config`, then run:

```sh
replay-kse-calendar --store /data --config /config/ball-arena.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Use `state: established` in an operator config copy for subsequent imports into a
store containing Ball. Validate staging before generation-guarded `publish`; never
publish the synthetic fixture. `npm run test:kse` and `npm run test:contracts`
verify capture and normalization. Set `BALL_BASE_URL` for `tests/browser/ball.spec.ts`.
Game-specific admission does not imply concert clearance. No prices or scheduling
are added; the app continues to consume local artifacts without venue requests.

### Marquis capture and replay

Marquis uses the [Live Nation browser-flow adapter](docs/adapters/0021-livenation-venue-events.md).
The September 11 import added 74 events through February 10, 2027: 73 All Ages
and one 18+ event. The local catalog now contains 16 dedicated venue sources.
With the existing Playwright Chromium installation available, run:

```sh
node tests/marquis/capture.mjs marquis
docker build --target ingestion -t event-calendar-ingestion .
```

Capture scrolls the normal public page twice and requires matching complete event
pages. It does not publish. Mount the reported capture directory read-only at
`/capture`, a seeded staging directory at `/data`, and `locales/denver/sources`
read-only at `/config`, then run the ingestion image with:

```sh
replay-livenation --store /data --config /config/marquis.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

For subsequent imports into a store containing Marquis, use an operator config
copy with `state: established`. Validate staging before generation-guarded
`publish`; do not publish synthetic fixtures. `npm run test:marquis` tests capture
validation; `npm run test:contracts` tests Go and artifact handoff. Set
`MARQUIS_BASE_URL` for `tests/browser/marquis.spec.ts`. Browser capture runs outside
the Go container; the app has no new browser dependency, prices, or automatic jobs.

### Paramount capture and replay

Paramount uses the [KSE venue-events adapter](docs/adapters/0009-kse-event-apis.md).
The September 11 local import added 72 events through April 10, 2027; two records
with inconsistent doors dates were rejected. The catalog now has 15 venue sources.
Capture is explicit: `node tests/kse/capture.mjs`. Build the ingestion image, mount
the reported capture directory read-only at `/capture`, a staging directory at
`/data`, and `locales/denver/sources` read-only at `/config`, then run:

```sh
replay-kse --store /data --config /config/paramount.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Use an operator config copy with `state: established` for subsequent imports into
a store containing Paramount. Validate staging before generation-guarded `publish`.
Never publish the synthetic fixture. `npm run test:kse` tests capture;
`npm run test:contracts` tests normalization and publication. Set `KSE_BASE_URL`
for `tests/browser/kse.spec.ts`. Unknown admission restrictions do not grant
With Adult clearance. No prices or automatic refresh are enabled.

### Hi-Dive capture and replay

Hi-Dive uses the [Plot adapter](docs/adapters/0005-plot-listings-api.md), with
reviewed parent/legal-guardian admission conditions. A one-time September 11
import added 33 events to the main local catalog. Capture is operator-triggered:

```sh
node tests/plot/capture.mjs
docker build --target ingestion -t event-calendar-ingestion .
```

Mount the reported capture directory read-only at `/capture`, a staging directory
at `/data`, and `locales/denver/sources` read-only at `/config`. Run the image with:

```sh
replay-plot --store /data --config /config/hi-dive.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

Use `state: established` in an operator config copy for a store that already
contains Hi-Dive. Validate staging before generation-guarded `publish`; never
publish the synthetic fixture. `npm run test:plot` checks capture and
`npm run test:contracts` checks normalization, reconciliation, publication, and
consumer contracts. Use `PLOT_BASE_URL` with `tests/browser/plot.spec.ts` for UI
checks against the chosen staging or main app. No schedule or automatic refresh
is enabled.

### Red Rocks capture and replay

Red Rocks uses the [Clique adapter](docs/adapters/0003-wordpress-clique-api.md),
including its reviewed admission policy. Capture does not publish or run on startup:

```sh
node tests/clique/capture.mjs
docker build --target ingestion -t event-calendar-ingestion .
```

The capture prints its new temporary directory and timestamp. Mount that directory
read-only at `/capture`, a staging directory at `/data`, and
`locales/denver/sources` read-only at `/config`, then run the ingestion image with:

```sh
replay-clique --store /data --config /config/red-rocks.yaml --snapshot /capture/snapshot.json --now <captured_at>
```

The supplied YAML declares a new source. For subsequent replay into a store that
already contains Red Rocks, use an operator copy with `state: established`.
Validate the staged source before promoting it with the existing `publish --expect`
workflow. Do not publish the synthetic JSON fixture. Capture aborts on HTTP errors
or invalid responses; replay rejects inconsistent coverage. Run `npm run test:clique`
for capture tests and `npm run test:contracts` for adapter and publication tests.

### Start the application

Install Docker with Compose, then run:

```sh
docker compose --env-file locales/denver/compose.env up --build -d
```

Open [localhost:8090](http://localhost:8090). Stop the application with `docker compose --env-file locales/denver/compose.env stop app`.
The read-only `.artifacts/<locale>` mount is empty on first startup. The server returns an empty
event list when that directory is empty. A missing directory or a nonempty directory
without a valid catalog is an error, not an empty calendar.

Optional environment settings:

| Variable            | Default                           | Meaning                                       |
| ------------------- | --------------------------------- | --------------------------------------------- |
| `CALENDAR_PORT`     | `8090`                            | Local HTTP port; bound to loopback            |
| `CALENDAR_DATA_DIR` | `./.artifacts/${CALENDAR_LOCALE}` | Directory containing `catalog.json`           |
| `WEEK_EVENT_LIMIT`  | `10`                              | Visible event cards per date in week view     |
| `MONTH_EVENT_LIMIT` | `5`                               | Visible event cards per date in month view    |
| `DAY_EVENT_LIMIT`   | `0`                               | Per-date day-view limit; zero means unlimited |

Week and month limits must be positive integers. These limits apply only to
the phone's scrolling lists; desktop Week and Month use available cell height instead.
A finite day limit also shows a
Show All control; full day expansion is covered by the UI tests when configured.

## Railway snapshot deployment

`Dockerfile.railway` replaces the former Pages build reference. It builds the
React frontend and Go server, validates `locales/<locale>/catalog/catalog.json` and every
referenced source file, then includes only that generation in `/data`. Missing
or invalid input fails the build. Unreferenced generations and capture reports
are not copied into the final image. IDs, public paths, and checksums stay unchanged.
No volume, bucket, database, ingestion job, or runtime write access is required.
Application image builds require explicit locale selection when building from the repository.
The shared ingestion and Go-test targets do not select a locale at build time.

Build and run locally without a data mount:

```sh
docker build -f Dockerfile.railway --build-arg LOCALE=denver -t event-calendar-denver .
docker run --rm --read-only -p 127.0.0.1:8095:8080 event-calendar-denver
```

### Git builds and optional upload contexts

Git-based builds use the committed snapshot in `locales/<locale>/catalog/` and
need no ignored local artifacts. Refresh and validate the snapshot, review its
report and diff, then commit it before triggering a build. Merely rebuilding the
same commit does not fetch new events.

For a CLI deployment, optionally prepare a dedicated single-locale upload context:

```sh
railway_parent=$(mktemp -d)
railway_context="$railway_parent/denver"
node scripts/package-locale.mjs denver "$railway_context"
docker build -f "$railway_context/Dockerfile.railway" -t event-calendar-railway "$railway_context"
```

`Dockerfile.railway.dockerignore` excludes `.artifacts/` and permits tracked locale inputs.
The `upload-context` target exports the required build inputs and only the validated,
catalog-referenced artifacts. Inspect the directory before upload. It contains no
Git metadata, `.env` files, `node_modules`, or unrelated workspace directories.
Source-code fixtures are build inputs, not published calendar data.

Create or select a Railway service and configure:

- Dockerfile variable: `RAILWAY_DOCKERFILE_PATH=Dockerfile.railway`.
- Select the locale with build variable `LOCALE=denver` (or the selected city).
- Healthcheck path: `/healthz` (process health; also check `/api/calendar` after deployment).
- Public networking: the app listens on `PORT`, default `8080`.
- Leave the start command unset; use the image entry point. Do not attach a volume
  at `/data`, which would hide the packaged snapshot.
- When deploying explicitly from a CLI or future Actions workflow, disable Git
  autodeploys to avoid duplicate deployments. A Git-only deployment can instead
  use Railway autodeploys, but must not also enable the Actions deployment mode.

After authenticating with the Railway CLI, replace the three placeholders below
with the exact intended deployment target. This command uploads and deploys:

```sh
railway up "$railway_context" --path-as-root --project YOUR_PROJECT_ID --service YOUR_SERVICE --environment YOUR_ENVIRONMENT
```

The snapshot is no longer ignored, so `--no-gitignore` is unnecessary. The exported
context omits unrelated locales and working data. See Railway's [CLI file handling](https://docs.railway.com/cli/up#file-handling)
and [Dockerfile selection](https://docs.railway.com/builds/dockerfiles#custom-dockerfile-path).

Refresh the tracked snapshot, then either commit it for a Git build or export a
**new** context, rebuild, and upload again. Redeploying an old upload does not
collect new local data. Retain the previous
image/deployment for rollback. The server still enforces event expiry at read time,
so a snapshot loses expired events even without a refresh. No live source fetching
runs during image build or application startup. Remote Railway deployment requires
its own smoke test; local image verification is not evidence of a successful upload.

## Explicit test-data preview

```sh
docker compose --env-file locales/denver/compose.env -f compose.yaml -f compose.test.yaml up --build -d
```

The normal app keeps its own data at port 8090. A separate app at
[localhost:8091](http://localhost:8091) reads only synthetic fixtures from
`tests/fixtures/catalog.json` and its referenced source files, and opens the week of
September 6, 2026. These dated preview events start expiring on December 7, 2026;
update their dates, paths, expiry values, and checksums together when refreshing
fixtures. `CALENDAR_INITIAL_DATE` selects a view date, not the server clock.
Stop the preview with `docker compose --env-file locales/denver/compose.env -f compose.yaml -f compose.test.yaml stop fixture-app`.
Do not use the fixture override for ordinary startup or deployment.

## Checks

Node 24 or newer is required for host-side frontend development. Go runs in Docker;
there is no host Go requirement.

```sh
npm ci
npm run format:check
npm run lint
npm run typecheck
npm test
npm run build
docker build --target go-test .
npm run test:contracts
docker compose --env-file locales/denver/compose.env -f compose.yaml -f compose.test.yaml up --build --wait
npm exec playwright -- install chromium --no-remove
npm run test:e2e
npm run test:aeg
npm run test:publication
```

Browser tests exercise both containers through their real HTTP interfaces. They
cover desktop and touch-sized Chromium layouts. Traces for failures and screenshots
are written to ignored `test-results/`. `CALENDAR_BASE_URL` changes the fixture app
address; `CALENDAR_EMPTY_URL` changes the empty app address.

`npm run test:publication` builds both images, starts a separate app on port 8092,
publishes synthetic candidates through the real ingestion command, checks stale-generation
rejection and untouched references, then runs the browser suite against that app.
Keep the empty normal app on port 8090 running for the empty-startup check. The script
uses the host UID/GID on macOS/Linux, stops its own Compose project afterward, and
prints a retained temporary-store path for inspection. It never writes `.artifacts`.

If port 8090 contains imported data, point `CALENDAR_EMPTY_URL` at a separate empty
test instance when running the browser or publication suites. Do not clear the
development data to satisfy the empty-startup test.

`npm run test:contracts` builds and tests Go in Docker, writes synthetic source and
catalog files to `test-results/contracts/`, and checks those exact bytes and their
checksum in TypeScript. Ordinary `npm test` skips that handoff test because it needs
the Go output. Browser tests can replace `test-results/`; rerun the contract command
to regenerate the files. These files are not application input or live venue data.

Use `npm run format` for frontend formatting. For Go formatting without host Go:

```sh
docker run --rm --mount type=bind,src="$PWD",dst=/workspace --workdir /workspace golang:1.26-bookworm gofmt -w cmd internal
```

`npm run dev` starts Vite on port 5173 and proxies `/api` to the Go app on port 8090.
Build changes into the container with `docker compose --env-file locales/denver/compose.env up --build -d`.

## Application input

Place `catalog.json` and its referenced source artifacts under `CALENDAR_DATA_DIR`.
See the [preview catalog](tests/fixtures/catalog.json), [source example](tests/fixtures/sources/mission/preview.json),
and [artifact contract](docs/adr/0017-artifact-contract-and-publication.md).
The old prototype `events.json` format is no longer supported.

Each API request reads one catalog and validates every referenced source file,
including its SHA-256 and source identity. Paths are confined to the data directory.
Documents must be regular files, at most 4 MiB each; one generation is limited to
64 MiB of input bytes. This is a safety limit, not a measured capacity target.
The publication command writes complete source files under new names, then replaces
the catalog last. See the local publication workflow below.

Only a fully valid generation replaces the in-memory snapshot. On a refresh error,
the server logs the error and serves the last valid snapshot. With no valid snapshot,
it returns HTTP 503. Empty startup does not count as a valid published snapshot.
A valid catalog with an empty `sources` array explicitly clears the calendar.
Fallback state exists only in memory and does not survive a process restart.

The server resolves actual venue attribution and admission-policy inheritance.
The UI shows policy links when supplied and marks off-site events in the popup. It does not show
source health or provider configuration. Unlisted and expired events are omitted.
Month and Week cards show only the event title on one line, with an ellipsis when
it overflows. Day cards show `Event @ Venue` and allow wrapping. No card shows
times. Full titles remain accessible and available in event details.
Cancelled cards use a red strikethrough across their text,
without a dot or `Cancelled` suffix. Event ordering is unchanged.
The popup heading combines a purple date and white event title at the same 1.6rem
font size. It shows the
event title as `YYYY-MM-DD : Event title`, with natural wrapping on narrow screens.
Popup status is `Scheduled` or `Cancelled`, and its links are labeled `View Event` and
`Buy Tickets`. The popup's `Cancelled` status is bold red; `Scheduled` is unchanged.
This display mapping does not modify the stored provider status.
Popup actions use icon-only, locally bundled Font Awesome SVGs with accessible names:
`Copy Event Link`, `Download Calendar Entry`, `View Event`, and `Buy Tickets`. All four have matching button
styles, targets at least 44px wide and tall, and visible keyboard focus.
`aria-label` provides each accessible name. Equal-width slots distribute available
actions evenly across the row. Custom tooltips appear after 300ms of mouse hover
or immediately on keyboard focus; Escape dismisses the tooltip before the modal.
Native `title` tooltips are omitted to avoid duplicates. Decorative
SVGs are hidden from screen readers. Links and downloads retain native behavior.
The icons are Font Awesome Free 7.3.1 by Fonticons, Inc., licensed under
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/); their geometry is unchanged.

The event title is the popup heading; there is no separate Artist row. Artist
metadata remains available for search and sorting.
Expiry uses midnight at the actual venue on `expires_on`; untimed off-site records
without a timezone use the source venue timezone for expiry only.
The calendar shows all retained, listed events, including past events, by default.
There is no past-events toggle. Old saved past-visibility settings are ignored;
saved venue, With Adult, child age, and search filters remain intact. The 90-day retention rule is
unchanged. Event details retain times and timezone labels; there is no timezone footer.

The toolbar starts with date navigation and the date range at the left content edge.
Venue selection and With Adult stay between the date range and search. There is no
filter drawer, hamburger button, Reset filters button, or age-category dropdown.
Retired age-category selections are ignored so they cannot silently hide events.

Venue accents use the fixed palette in `web/venueColors.ts`. Each integrated venue
has a unique color, shown as a 2px calendar-card edge and a
decorative marker beside its name in the venue picker and event details. Titles
remain white; card backgrounds share the same dark surface without venue tints.
Cancellations keep their red strikethrough. Colors do not change
when filtering or reloading. Add a fixed assignment when integrating a venue;
unknown venue names temporarily use the app's purple accent. These are UI colors,
not official venue branding, and venue names remain available without using color.

The `Venues` dropdown contains multi-select checkboxes. Its button shows `All venues`,
the selected venue's name, or `N venues selected`. It stays open while checking venues;
Escape closes it and returns focus to the button. Clicking outside or tabbing away
also closes it. Opening the dropdown is not a saved preference.
Venue checkboxes match any selected venue; `All` removes the venue restriction.
Removing the last selected venue also returns to `All`. Search matches every query
word, case-insensitively, across available public event metadata throughout the
loaded date range. It does not change the calendar date or view. All active filters
combine before the day/week/month display limits are applied.

Advertised age policies remain available in event details and search. Event policies
replace venue defaults. There is no separate age-category filter.

`With Adult` reveals a native `Child’s age` dropdown (0–17, default 14).
The adjacent information button, named `About With Adult`, explains reviewed
policies and advises checking event details and venue policy before buying tickets.
Its help opens on hover, keyboard focus or tap; Escape or an outside tap dismisses
it. Opening help never changes the filter.
Display `With Adult · Age:` beside numeric options `0` through `17`. The dot is
decorative and hidden from assistive technology. Labels and the select inherit
the same font family, size and weight and remain vertically centered.
It offers only those 18 integer values, with a fixed 64px select width and a 44px minimum
height. The saved age and accessible policy help remain unchanged.
In the open venue picker, typing a name prefix focuses and scrolls to its first
match without selecting it. Repeating a letter cycles matches; the prefix resets
after 750ms or reopening. Space toggles the focused checkbox; Escape closes.
It matches only reviewed admission ranges; unknown policies do not match. Turning
it off removes the admission restriction. Venue and search filters still
apply. The checkbox and age are saved with the other browser preferences.
Bluebird, Gothic, Mission, Ogden, and Fiddler's Green have reviewed rules. Advertised restrictions stay unchanged;
event details show a `With Adult` metadata row below `Age Policy`, with the child age
and applicable condition followed by a policy-link icon. The icon has an accessible
`Venue admission policy` name and the shared tooltip behavior. The row aligns text
baselines for single-line and wrapped values. The icon is 14px with a 44px touch
target positioned outside text flow so it does not increase metadata row spacing.
The icon is left-aligned within that target to keep it close to the policy text.
Review dates remain in the data
but are not displayed in the modal. Follow those conditions; this filter is not a
guarantee of admission.

`Age Policy` always uses plain text. When its own policy URL is available, a
`Venue admission policy` icon follows it, using the same styling as With Adult.
All external links open a new tab/window with `noopener noreferrer`; internal
navigation and calendar downloads are unchanged. The browser chooses tab versus
window. Display-only corrections in `displayAgePolicy` normalize the reviewed
Globe Hall guardian phrase and Paramount's `RECOMMENDED AGE 18+`. Unfamiliar text,
abbreviations, source records, age numbers, and admission filtering are unchanged.

Price data is retired across all venues. Adapters do not extract it, and reconciliation
and publication strip prices from new artifacts, including legacy candidates and retained
records. Price overrides cannot restore it. Existing immutable artifacts and raw source
captures are not rewritten; the v1 price schema remains readable for compatibility.
The API omits `price` and `cost_category`, and the UI, search, and `.ics` exports do not
use them. Rebuilding the app removes price information from existing local events without
re-ingestion. Ticket and event links remain available for visitors to check current prices.
The Cost filter and classification are removed. `COST_BRACKET_LIMITS` is ignored and no
longer passed by Compose. Previously saved cost selections are ignored; other preferences
remain intact. The API still supplies optional `age_category` metadata.

Venue selections, search text, With Adult, and child age are saved in browser
`localStorage` under `event-calendar.filters.v1`, never in URLs. Clear each control
directly: select All venues, clear search, or uncheck With Adult. Retired age, cost,
and past-visibility selections are ignored. Invalid saved settings use defaults;
blocked or full storage does not prevent in-memory filtering. A saved venue absent
from the latest data stays in the selector so it can be deselected.

The browser still consumes the compact `/api/calendar` display projection, not raw
source artifacts. Validation runs in Go; no browser schema compiler or relaxed CSP is
needed. Reload the page to request refreshed data. There is no background polling.

The brand is `withAdult(denver)` for the intended `denver.withadult.com` site.
A 36px header reads `withAdult(denver): Bring your people.` in the existing system
font, 20px on desktop and weight 700. It uses the original purple for `withAdult()`
and light gray for the city, colon, and tagline. Below 768px the entire brand line
remains visible with a shared responsive font size. There is no footer or header border. The gap below the header is
halved: `clamp(6px, 1dvh, 12px)` on desktop and 6px on phones. Keep the existing
proportional gutters. Calendar sizing reserves only the header's 36px,
with scrolling on short screens. The browser title uses the same branding string.
This does not configure
DNS, TLS, or deployment for the domain.
The header's right edge has a Font Awesome envelope link to
`mailto:contact@withadult.com` on desktop and mobile. Its accessible name and
hover/focus tooltip are both `Contact Us`. The link opens the configured email
handler; the app does not send email.
First-time visits open Week on desktop and phone, with today's existing highlight.
Explicit view choices (including date drill-in) persist in `event-calendar.view.v1`
in local storage. Reload restores that view; invalid or unavailable storage falls
back to Week. Direct event URLs open the event's Week with the detail panel visible
without replacing the saved view.
Search sits to the left of Day/Week/Month on desktop and last on phones.
Phones use a view dropdown beside date navigation and Today. Below 360px,
Today and the view dropdown move to a second row to retain usable targets.
Current-year dates omit the year visually on phones; the accessible label retains
the full date. Venue and With Adult share a stable row; enabling With Adult reveals
Age below that checkbox without moving the venue picker. Custom dropdown triggers
toggle open/closed on repeat activation, with outside-click and Escape dismissal.
The native age select retains the platform's standard dismissal behavior.
The empty field has no visible placeholder text; its accessible name remains
Search events. On a single toolbar row, the venue and With Adult controls form
one group centered between the date range and search, including the age picker
when enabled. This desktop alignment does not apply to the mobile grid.
A right-aligned Font Awesome magnifying glass appears only when search is empty
and unfocused. The decorative icon does not intercept clicks; the input retains
its accessible Search events label. The venue dropdown and With Adult checkbox
sit between the date range and search. With Adult reveals the adjacent child's
age picker on desktop and an age row below on mobile. The event modal Close button
uses a centered 44px target.
FullCalendar owns date layout, navigation, and event placement. Desktop uses DayGrid;
phones use List views. A presentation-only Show All row caps mobile dates without
changing the domain data. All-day placement is used only to arrange date cards; it
does not assign an event duration. Visible time labels always use the venue timezone.
Month view shows only the selected month's dates and events. The desktop grid uses
only the required week rows, with blank cells for weekday alignment.
Desktop Month and Week fit the available viewport height.
Desktop Week marks today's full weekday-and-date label with a purple
pill and gives today's column a faint plum tint; phone and Day remain unchanged.
Neither desktop grid has a
count cap: `Show All` appears only when events overflow the available day-cell
height. FullCalendar shows as many cards as fit. Desktop grids disable event
slicing to reserve only the measured overflow-link height, not a whole event row. `Show All`
opens the full day. Month cards use tighter vertical padding without smaller text.
Desktop event cards size to their text and padding; Day cards can grow when text
wraps. Phone event cards retain a 44px minimum touch height; toolbar
and Show All controls keep their existing sizing.
`Show All` is horizontally centered within its date cell or mobile date section.
It uses a muted plum background with bold lavender text, a brighter hover state,
and an inset keyboard-focus outline without adding height.
Phone lists and desktop Day retain their scrolling layout. Side gutters are each
5% of the window width, with a 12px minimum; there is no fixed calendar-width cap.
Month has a 36rem minimum grid height and Week has a 16rem minimum. Short windows
scroll vertically instead of compressing those grids. Spacing scales within
12–24px. The date heading scales within 1.15–1.35rem;
event text and 44px control targets are not shrunk. Resize preserves the current
date, view, selections, and open event. No page zoom or scaling transform is used.

Event cards use their stored `public_path`. Direct links open the event's week and
detail panel with temporary default filters; saved preferences remain unchanged
unless the visitor changes a filter. Back/Forward and closing the panel retain
calendar context. Retained unlisted records show `No longer listed`; expired or
unknown records return HTTP 404. Unavailable storage uses the last valid snapshot,
or HTTP 503 when none exists.

Clicking or tapping the background closes event details, as do Escape and the close
button. Clicks inside the popup and drags that start inside it do not dismiss it.
Closing restores focus and keeps the current calendar date/view.

`Copy Event Link` copies the venue event URL used by `View Event`, not the app URL.
If clipboard access fails, show that URL for manual copying. Omit the copy action
when no venue event URL exists; keep `.ics` export independently available.
`Download Calendar Entry` exports one event as an `.ics` file using doors time, then show time, or a
date-only entry marked `Time not provided`. No end time is invented. The UID stays
stable across title/time changes. The export uses CRLF line endings and includes
available metadata and source/ticket links. Routes are `/events/…`,
`/api/events/…`, and either route with `.ics` appended. Downloads use the API route
so they also work through the Vite development proxy.

Live ingestion, object-store publication/cleanup, and deployment remain later work. See
[the delivery plan](docs/adr/0019-implementation-and-verification-plan.md) and
[the product contract](docs/adr/0014-product-and-delivery-contract.md).

## Version-one contract library

The bundled [schemas](internal/artifact/schema/v1/), Go package
[`internal/artifact`](internal/artifact/), and TypeScript validation module
[`web/artifacts`](web/artifacts/) share valid and rejected fixtures in
[`tests/contracts`](tests/contracts/). Operator configuration uses YAML; published
source artifacts and catalogs use JSON. See [ADR 0017](docs/adr/0017-artifact-contract-and-publication.md)
for the implemented fields and remaining integration work.

The Go reconciler takes configuration, prior state, normalized observations, coverage,
and a caller-supplied civil date. It returns a validated candidate and record rejections,
or an error with no replacement. It assigns stable paths, applies overrides, preserves
history, and removes expired records. Policy inheritance is available as a shared
helper. The library does not fetch providers, publish files, or run jobs. The web
reader consumes and validates its artifact format.

## Local publication

For local AEG fixture replay, see the workflow below this publication section.

`ingest publish` accepts **already reconciled version-one source artifacts**, not raw
provider responses or operator YAML. It does not fetch events or invoke the reconciler.
All supplied candidates must pass validation; untouched sources keep their existing
catalog references. There is no source-removal command or automatic artifact cleanup.

To enable manual publication, uncomment only the `ingestion` service block in
`compose.yaml`. Set `CALENDAR_CANDIDATE_DIR` to your prepared input directory and
`CALENDAR_DATA_DIR` to the existing destination store. Ensure the store is writable
by `INGEST_UID`/`INGEST_GID` (default 65532:65532) and readable by the app. Keep inputs
mounted read-only. Do not make a shared production directory world-writable.

For a deliberately new store with no catalog, and an input named `source.json`:

```sh
docker compose --env-file locales/denver/compose.env build ingestion
docker compose --env-file locales/denver/compose.env run --rm ingestion publish --store /data --expect none /input/source.json
```

For later updates, replace `none` with the catalog's `generation` captured when preparing
the candidates. Put all options before file arguments. Multiple file arguments publish
as one generation. A mismatch fails without replacing the catalog; do not simply retry
with a newer generation unless the candidates have been reconciled against that state.
`none` is reserved for an absent catalog, not a way to recover a lost established catalog.
The command supports `publish --help`. Default `docker compose --env-file locales/denver/compose.env up` remains app-only
unless you explicitly uncomment the service.

The command emits a JSON report with `generation`, `published`, `durable`, and optional
`error`. Exit 0 means publication and filesystem sync calls succeeded. Exit 1 means
the command did not publish; exit 2 means invalid usage. Exit 3 means publication
occurred but durability confirmation or report delivery failed. After interruption or
exit 3, inspect the current catalog before retrying; a missing report does not establish
that no publication occurred. `durable` records successful sync calls, not a tested
hardware power-loss guarantee.

Writers use an advisory lock on the destination directory inode. Cooperating writers
fail fast on contention; process death releases the lock without stale lock files.
Do not rename/replace the store directory or bypass the lock with other writers.
Linux Docker is verified; native macOS and network filesystems need separate checks.

Before catalog replacement, a failure leaves the prior catalog unchanged. It may leave
unreferenced immutable files or a `.catalog-*.json` staging file. A fresh invocation
uses new names and does not overwrite these files. If the first publication was
interrupted, a known-new store can be retried with `--expect none`; until success the
reader can report 503 because that store is no longer empty. No command removes orphan
files, old generations, or backups. Inspect and retain the prior catalog when planning
an operator rollback; automatic rollback and cleanup remain out of scope.

## Local RHP snapshot replay

RHP snapshot ingestion is available for Lost Lake, Larimer Lounge, Globe Hall,
and on-site Cervantes events. See the [RHP operator and fixture instructions](internal/rhp/testdata/README.md)
for explicit captures, `replay-rhp`, and validation. The shared adapter enriches
reviewed event-level guardian conditions, preserves actual off-site venues, and
ignores prices. These jobs do not run on app startup.

## Local HoldMyTicket snapshot replay

HQ, The Oriental Theater, and The Federal Theatre use one local-only HoldMyTicket adapter. It reads a
complete saved iCalendar feed plus saved Event JSON-LD details. It does not fetch
URLs or run a schedule. The snapshot format and synthetic inputs are documented in
[the fixture README](internal/hmt/testdata/README.md).

Inside the ingestion container, with a writable isolated store at `/data` and
`internal/hmt/testdata` mounted read-only at `/hmt`:

```sh
/ingest replay-hmt --store /data --config /hmt/hq.yaml --snapshot /hmt/hq.json --now 2026-09-10T14:30:00Z
```

Use the matching `oriental.yaml` and `oriental.json` for Oriental. Never publish
these synthetic fixtures into `.artifacts`. `replay-hmt --help` lists the flags.
The existing replay publication workflow validates and publishes the artifact;
the application needs no provider-specific configuration or restart.

For an approved live capture, save the entire feed from
`https://holdmyticket.com/ics/6457` (HQ) or `https://holdmyticket.com/ics/801`
(Oriental), then save the single Event JSON-LD object from each feed event URL
under its numeric provider ID. Parse HTML as data; do not execute page scripts.
Stop on failed fetches, incomplete feeds, ambiguous detail selection, or unexpected
redirects. Review raw samples and stage the bundle before publication. Snapshots
are limited to 4 MiB. Capture and recurring access remain operator-managed.

Federal uses `https://holdmyticket.com/ics_user/8693` and `federal.yaml`, not an
inferred `/ics/8693` route. Its unused DESCRIPTION blocks contain malformed raw
paragraph breaks. Only the Federal profile removes these blocks in memory before
standard parsing. It requires the observed valid CREATED timestamp boundary and
rejects intervening property-like lines, duplicate blocks, or missing boundaries.
Raw captures remain unchanged. All event fields still undergo normal validation.

Federal's explicitly All Ages details receive reviewed child-admission metadata
linked to that event's page (reviewed September 10, 2026). Restricted or ambiguous
labels receive no inferred permission. Event overrides still win. HQ/Oriental
policies and the application's independent `.ics` export are unchanged.

After an approved Federal publication, verify desktop and phone behavior with:

```sh
FEDERAL_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/federal.spec.ts
```

The replay keeps only available events within the next 12 months. `complete` means
the supplied snapshot was enumerated, not that the provider publishes every show.
Do not automate established-source refresh/removal until feed coverage is reviewed.
Offer URLs supply ticket links; offer price data is ignored. Age labels support exact
category filtering. With Adult remains unavailable
for these venues until accompaniment policies are reviewed.

After explicit publication into a local test or development instance:

```sh
HMT_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/hmt.spec.ts
```

This opt-in test checks both venues on desktop and phone. Run browser workflows
sequentially because they share test output directories.

## Local AEG fixture replay

The AEG snapshot adapter supports Gothic, Mission, Bluebird, Ogden, and Fiddler's
Green. It has no HTTP client or schedule. Approved one-time live imports are
recorded in ADR 0019; recurring access and completeness remain separate work. Its authored
[fixtures and configurations](internal/aeg/testdata/) must not become public listings.
See [the AEG adapter contract](docs/adapters/0002-aeg-json-feeds.md).

Run the isolated container workflow with Docker, Node, and installed Chromium:

```sh
npm run test:aeg
```

This builds the ingestion and application images, replays each venue in a separate
process, validates the resulting HTTP records, and checks the calendar on desktop
and phone. The publisher container has networking disabled. The script uses a fresh
temporary store, never writes `.artifacts`, and removes only its own test containers
and network afterward. It retains the store and prints its path. Do not run it at the
same time as `test:publication`: both use port 8092 and browser output directories.
Run browser suites sequentially to preserve their failure artifacts.

Inside an ingestion container with a writable test store at `/data` and the synthetic
fixture directory mounted read-only at `/aeg`, the command is:

```sh
/ingest replay-aeg --store /data --config /aeg/gothic.yaml --snapshot /aeg/gothic.json --now 2026-09-09T12:00:00Z
```

Use the matching `.yaml` and `.json` pair for `mission`, `bluebird`, `ogden`, or
`fiddlers-green`. `replay-aeg --help` lists the flags.

Reviewed rules live in optional source-config `admission_rules`, keyed by the exact
age category. See the Bluebird YAML and the artifact ADR for the range contract.
Deploy the updated reader before publishing these optional version-one fields;
older strict readers reject them. Ordinary replay applies the current configured
rules to newly normalized records, after configured policy overrides.

To annotate existing records without fetching or reconciling upstream listings,
mount the intended store at `/data` and its matching established-source config at
`/input/bluebird.yaml`, then run inside the ingestion container:

```sh
/ingest enrich-admission --store /data --config /input/bluebird.yaml
```

Back up the catalog first. This publishes only that source, preserves event IDs,
URLs and records, and does not refresh or remove events. It preserves existing
`with_adult` metadata and already-applied policy overrides; it is not a command for
applying new override values or replacing previously enriched rules. Use normal
replay for those updates. Do not publish the synthetic JSON fixture into live data.

The same enrichment command supports Gothic, Mission, Ogden, and Fiddler's Green
with their matching established-source configurations. Fiddler's Green has only an
All ages rule; restricted or missing categories remain unknown. Its Stick Figure
listing currently has no restriction and therefore does not match With Adult.
Gothic, Mission, and Ogden use the same reviewed ranges as Bluebird, each citing
its own policy page. Policies are curated, not automatically refreshed.

After publishing reviewed data for all five venues, verify the local reader and UI with:

```sh
LIVE_ADMISSION_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/live-admission.spec.ts
```

After an explicit live import, run the opt-in local smoke check:

```sh
LIVE_AEG_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/live-aeg.spec.ts
```

This checks new venue details, link destinations, and filter options without
changing stored events. It skips in the default fixture test run.
The explicit clock controls reconciliation, not the app's runtime expiry clock.
These synthetic September 12 events expire on December 11, 2026; update fixtures
and browser expectations together before running the workflow after that date.

The first replay of each source uses `state: new`. For subsequent replays, use a
separate configuration copy with `state: established`; the prior source artifact
must exist and validate. Overrides use the existing config structure. The command
captures the catalog generation before normalization and fails if another writer
publishes first. Retry from the new prior state, not from an old prepared candidate.

Reports include the normal publication fields plus `rejected` records. Exit statuses
match `publish`: 0 committed and synced, 1 not published, 2 usage error, 3 committed
but durability confirmation or report delivery failed. A valid empty snapshot removes
future listings within its coverage; a failed replay does not change the catalog.
Invalid known updates retain the last valid event. Metadata remains optional: `$0`
placeholders do not become Free, and literal status text does not alter listing state.

Rebuild readers before publishing artifacts with `status`. This is an additive change
to the local version-one schema; older strict readers reject that property.
