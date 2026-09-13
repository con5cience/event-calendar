# ADR 0023: Independent locale configuration and deployment

- Status: Accepted; implementation verification recorded in ADR 0019.
- Date: 2026-09-12
- Supersedes the single-Denver deployment assumptions in ADRs 0014, 0015 and 0018.

## Decision

Keep one shared codebase and one locale per application process and image. Do
not route tenants by Host, URL, cookie, or request parameter. Each image contains
one selected site's public configuration, assets, and validated catalog. There
is no shared runtime database, writable volume, scheduler, or dependency on
another locale's deployment. Shared source-code changes do not redeploy other
instances automatically.

```
locales/<id>/site.json          # public settings and source ownership registry
locales/<id>/assets/favicon.png # favicon and square social preview
locales/<id>/sources/*.yaml     # operational source configs and reviewed policies
locales/<id>/capture.json       # operator-only capture endpoints/provider scopes
locales/<id>/compose.env        # explicit local project, locale and port selection
.artifacts/<id>/                # ignored locale catalog and source artifacts
.captures/<id>/                 # optional ignored capture output parent
.builds/<id>/                   # optional ignored exported build directories
```

`site.json` is versioned and rejects unknown fields. It specifies `id`, `city`,
`name`, `tagline`, `description`, fixed HTTPS `origin`, IANA `timezone`, BCP 47
formatting `language`, `week_start`, `default_view`, `storage_namespace`,
`ics_namespace`, image alt text/dimensions, display `limits`, `venue_colors`, and
`sources`. Source entries bind an ID to its adapter and canonical venue key.
Language affects formatting, not translation of the English interface.

The Go host requires `SITE_DIR`; it has no production Denver fallback. It
validates configuration and PNG dimensions before listening. Default local data
is `.artifacts/<id>` unless `DATA_DIR` is explicit. `/api/site` exposes only
presentation settings, not the source registry or capture/source configs. React
loads these settings before rendering. Metadata, sitemap, branding, venue
accents, calendar defaults, local today, formatting and storage use the selected
configuration. Event times and expiry still use each venue's timezone.

The catalog reader rejects any source whose ID, adapter or venue key is outside
the selected registry. A rejected refresh cannot replace a last-valid snapshot.
The packager also validates each operational source config and catalog ownership
before creating output. This protects accidental cross-locale input, not a
malicious operator who deliberately rewrites both configuration and data.

## Sources and captures

Move operational configs out of testdata; tests retain their own fixtures.
Provider options are explicit: AEG feed/venue IDs, Live Nation venue IDs,
HoldMyTicket feed/layout, VenuePilot event-link base, and Meow Wolf seller/rooms.
These adapters validate returned identities against the supplied profile instead
of a built-in Denver venue registry. Federal's malformed-description treatment
is an explicit `layout: federal` selection; it is not applied to other calendars.

Capture commands require `SITE_DIR` and read only that locale's capture profiles.
Unknown sources fail before requests. Offline capture tests explicitly select
the Denver test profile through their setup module. Tests do not fetch sources.
RHP and Live Nation select a configured source through their CLI argument.
VenuePilot and Meow Wolf use `CAPTURE_SOURCE` for a different configured source;
their existing Denver source remains the command default after explicit locale
selection. Specialist venue capture commands require their matching source profile.

Established operational profiles require a staging copy of the current locale
generation. Only a genuinely new source starts with `state: new`; an empty staging
store must not cause existing event identities to be regenerated.

Specialist website parsers retain their reviewed layout and admission checks.
Configuration isolation does not mean that every new venue can use an existing
parser without source evaluation. Unsupported layouts still need adapter work
and the venue admission-policy review. No new live sources are added here.

## Build and deployment

`node scripts/package-locale.mjs <id> <new-output-directory>` exports a fresh,
self-contained Docker build context using the existing validated snapshot
packager. The selected locale is required. Existing output directories are
rejected. Only its locale directory and active catalog generation are exported.
Shared code and test fixtures remain shared build inputs, not published data.

The exported Dockerfile pins the selected locale as its build-argument default.
It builds outside the repository, with no other locale directory available. Each
Railway service receives its own exported context and has independent domain,
deployment history and rollback. No remote deployment is implied by local tests.
For explicit CLI deployment, keep Git auto-deploy disabled to avoid duplicate
deployments. Git-only builds can use autodeploy with the tracked snapshot.

Direct repository builds must pass `--build-arg LOCALE=<id>`. Compose selects the
same argument through an explicit locale env file, with independent project names
and ports. `CALENDAR_API_ORIGIN` can select the Go server for a separate Vite dev
process. The built frontend is shared and does not import operational locale
configuration at compile time.

This is preferred over duplicated repositories, which would require copying
fixes, and over a multi-tenant runtime, which would share failures and data access.
An environment-variable-only approach is insufficient for reviewed source rules,
assets and source ownership. The configuration directory is the unit of selection.

## Denver migration and recovery

Copy the validated current generation to `.artifacts/denver`; do not remove the
original `.artifacts/catalog.json` or its source generations. Preserve catalog
bytes, checksums, event IDs and URLs. Denver retains `event-calendar` for storage
and ICS namespaces to preserve existing preferences and calendar export identity.
New locales should use their own namespaces. Keep the approved colors and image.

Rollback uses the previous image and original data directory. No retention or
admission rule is changed by the locale migration. Fixture PNGs/configs used to
test another locale are synthetic and are not a new public deployment.

## Verification

Use `npm run test:locales` for real standalone builds and two running containers.
It creates a synthetic coastal fixture in a temporary directory, verifies
different domains, branding, data, assets, timezone, defaults and ICS namespace,
and checks that rebuilding/restarting it does not change Denver's image or API
data. Test containers are stopped; temporary contexts are retained for inspection.
Run shared Go race/contract checks, capture tests and the existing browser suite.

## Local refresh and tracked snapshots — September 13, 2026

The operator refresh command is `node scripts/refresh-locale.mjs <locale>`, also
available through the Dockerfile's `refresh` target. The `--snapshot <locale>
<input-store>` mode seeds or replaces `locales/<locale>/catalog/` from an existing
validated store. Both modes use the existing Go snapshot exporter and retain its
manifest, source paths, checksums, and exact artifact bytes. Do not reformat these
files. Only the referenced generation is kept in the tracked tree.

The refresh coordinator uses the locale registry and capture profiles, with a
code-owned adapter runner table. Capture subprocesses have explicit output paths
and do not publish. Go remains responsible for normalization, admission rules,
identity, retention, and publication. No aggregator or new venue is introduced.

Captures use a bounded pool (default 2; `CAPTURE_CONCURRENCY=1` restores serial
capture; accepted range 1–4). Each adapter family shares a provider slot. The
two KSE adapters share a slot, as do the two Wix adapters. This deliberately
serializes RHP venues even though their public hostnames differ. New adapters
must review shared provider infrastructure when assigning their group.
Requests and paired passes inside each capture remain unchanged. All capture
processes settle before ingestion begins, including failed captures. Capture
failures become per-source report entries, not unhandled promise rejections.

Ingestion and publication execute sequentially against isolated savepoints in
registry order, regardless of capture completion order. A source must confirm
durable publication and pass snapshot validation before its output becomes the
next job's input. Failed sources retain their prior data. Final export replaces
only the selected locale's tracked snapshot; raw data, reports, and recovery
copies remain ignored under `.artifacts/refresh/<locale>/`. A per-locale lock
prevents competing refresh/export commands. This is not a live-store replacement
protocol. Preserve the previous snapshot for recovery after an interrupted rename.

This avoids concurrent catalog writers and keeps the existing failure recovery
contract. A GitHub job matrix would require a separate artifact merge protocol;
parallel detail requests would increase load on individual providers. Neither is
part of this change. There is no database, queue service, or schema change.

Run `34785826381` provides the sequential baseline: its refresh step lasted
464 seconds, with 442 seconds inside successful capture subprocesses. Eight raw
start/end timestamp pairs were checked before summing. These times describe one
CI run, not a predicted speedup. Measure the next CI run and runner memory before
raising the default. Provider serialization and network variability limit gains.

Verification for the capture-pool change:

| Evidence source                                                                                                      | Raw observation or test result                                                                                                   | Supported finding                                                                                              | Material limit                               |
| -------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| `npm run test:refresh`                                                                                               | 19 tests passed; independent captures overlapped, same-provider captures did not; serial writes preserved all successful updates | Pool bounds, ordering, and failure retention hold for controlled inputs                                        | No live-provider timing claim                |
| `docker build --target refresh-test -t event-calendar-refresh-test .`; `docker run --rm event-calendar-refresh-test` | Build and four integration tests passed                                                                                          | CLI capture, Go ingestion, export, and HTTP consumption still work; invalid concurrency rejects before locking | Local synthetic upstreams                    |
| `npm run test:contracts`                                                                                             | Go checks reused the unchanged cached layer; handoff and 91 frontend tests passed                                                | Existing producer/consumer contract remains valid                                                              | UI behavior unchanged; no new browser checks |
| `npm run test:refresh-dry-run`                                                                                       | Five tests passed after rerunning with loopback permission                                                                       | Report and workflow-shell contracts remain valid                                                               | First sandbox run failed with `listen EPERM` |

All capture-adapter unit suites, capture-profile and locale-tool tests, lint,
formatting, and the frontend build (including type checking) passed. No workflow
or source artifact was changed. A new CI run is required to measure speed and
memory use; this change does not resolve Roxy's upstream HTML response.

### Dry-run concurrency comparison

The manual workflow exposes `capture_concurrency` choices 1–4, retaining 2 as
the default, and forwards the selection through Docker's environment. The next
comparison should use 3 rather than changing the coordinator default globally.
Provider groups and serial publication remain unchanged.

During capture, a background loop samples the named refresh container with
`docker stats --no-stream`, waiting five seconds between calls. Timestamped JSON
records go to `refresh-memory.jsonl`; poll errors go to a separate diagnostic
file. The chosen limit is saved in `capture-concurrency.txt`. The exit trap stops
the monitor, and monitoring errors never replace the captured refresh exit code.
Samples are not peak memory or total runner memory; short jobs can have none.
The existing diagnostic artifact includes these files. No deployment is added.

The exact workflow-shell regression checks input forwarding and preserves exit
codes 0, 1, 2, and 137 with monitoring enabled. Local Docker sampling and workflow
lint supplement the existing container integration tests. GitHub runner memory
and the effect of concurrency 3 remain unverified until a new manual run.

Exit code 2 means partial publication with a usable validated snapshot; code 1
means failure. All-source failure does not replace the tracked snapshot. Successful
jobs can change generation metadata even when event data is identical; semantic
no-op detection remains future work. No Git writes, deployment, or schedule is
part of this command. Runtime input format is unchanged.

AEG capture preserves raw JSON for the existing strict decoder. HMT capture uses
URL discovery only, not an alternate iCalendar parser; the existing Go parser
still validates the full feed, Federal description handling, and event/detail
agreement. HMT Event JSON-LD is extracted in a detached browser document without
executing provider scripts. These operator capture paths do not establish access
permission or automated-runner network compatibility. Those remain live-rollout
checks before scheduling. See README for invocation, recovery, and tests.

### Local verification

| Evidence source                                                                                                         | Raw observation or test result                                                                                            | Supported finding                                                                                                                  | Material limit                                                                                                   |
| ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `npm run test:refresh`                                                                                                  | 12 tests passed                                                                                                           | Coordinator isolation, failure retention, runner imports, and subprocess timeout work with test inputs                             | Does not contact live venues                                                                                     |
| `docker build --target refresh-test -t event-calendar-refresh-test .` and `docker run --rm event-calendar-refresh-test` | Build and both integration tests passed                                                                                   | Local HTTP capture reaches Go publication, snapshot export, and the running calendar API; HMT extraction agrees with its Go parser | Synthetic AEG/HMT inputs, not all live capture paths                                                             |
| Snapshot export plus manifest/artifact byte comparison                                                                  | Exported Denver manifest and every referenced artifact match the existing local store; checksums match                    | Initial Git-ready snapshot preserves the existing data and identity bytes                                                          | No new venue data fetched                                                                                        |
| Existing capture npm test commands and `npm run test:contracts`                                                         | Capture suites passed; contract handoff and 91 frontend tests passed; Go test image checks reused unchanged cached inputs | Existing parsing and consumer contracts remain valid                                                                               | Fixture verification only                                                                                        |
| `npm run test:locales`                                                                                                  | Independent builds and four browser checks passed                                                                         | Existing two-locale deployment isolation remains intact                                                                            | Local containers, no Railway deployment                                                                          |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8096 npm run test:e2e`                                                             | 148 passed, 66 skipped                                                                                                    | Default fixture browser suite passed after correcting the empty test instance                                                      | Opt-in source/locale tests and layout exclusions remain skipped in this invocation; locale checks ran separately |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, `git diff --check`                        | Passed                                                                                                                    | Formatting, static checks, and frontend build passed                                                                               | Not evidence of remote deployment                                                                                |

The initial browser run used the populated port-8090 default for its empty-store
checks. A replacement test container initially used `/tmp`, which contained Node
cache files; the reader correctly rejected that nonempty directory without a
catalog. A dedicated empty mount corrected the test setup, and the final suite
passed. The temporary empty container was removed. No application data was
deleted or replaced. No live refresh, Git commit/push, or deployment was performed.

## Tracked-snapshot build integration — September 13, 2026

`Dockerfile.railway` and `scripts/package-locale.mjs` now select
`locales/<locale>/catalog/`, not `.artifacts/<locale>`. The Docker allowlist no
longer includes ignored working stores. The exporter copies explicit site config,
assets, source profiles, and capture profiles, then the validated referenced
snapshot. It does not copy the entire locale directory or any refresh lock.

Git checkouts can therefore build without local ignored data. Exported contexts
retain the same tracked layout and pin the selected locale's Docker build
argument. The existing two-locale test now constructs both contexts without an
`.artifacts` directory and checks their running HTTP interfaces. Compose still
reads its separate working store; neither builds nor app startup run ingestion.
No remote workflow or deployment configuration is changed here.

## Supervised live refresh — September 13, 2026

The first full run is retained under `.artifacts/refresh/denver/run-WMqyM4/`.
It published 23 sources and retained the three HMT sources after redirect errors.
Their prior catalog references and checksums were confirmed unchanged. The exact
canonical feed paths were verified and fixed separately; see adapter ADR 0006.
Only those three captures were retried under
`.artifacts/refresh/denver/retry-ifjRMz/`. Their successful Go publications were
merged into a validated copy of the first run's output, then exported with the
existing `--snapshot` command. The resulting catalog generation is
`g-6f389a49a733f3d66d94e124839e835d`.

| Evidence source                                                 | Raw observation or test result                                                                                                          | Supported finding                                                                                        | Material limit                                                   |
| --------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Full run report and three retry ingestion reports               | 23 initial durable publications plus three successful HMT retries; 21 rejected observations in the original run and none in the retries | All 26 configured sources published; invalid observations were reported rather than weakening validation | One local run, not scheduled GitHub execution                    |
| Final manifest and every referenced artifact                    | Checksums match; source IDs match the seed; surviving event IDs retain their dates and public paths                                     | Refresh preserved source ownership and existing URL identity                                             | Does not prove every upstream listing is correct                 |
| Five final event samples and six HMT detail samples             | Venue identity, event title, date/time, provider ID, and public event path inspected where applicable                                   | Samples represent event records for the intended venues                                                  | Sampling is supplementary to full Go validation                  |
| Original working-store manifest versus pre-refresh snapshot     | Exact bytes match                                                                                                                       | The existing Compose application's store was not changed                                                 | Tracked deployment snapshot was intentionally replaced           |
| Git-only export of commit `b776154`                             | Docker build succeeded without ignored files; no-volume container returned HTTP 200 for health, site config, and calendar               | Committed inputs suffice for the Denver build                                                            | Local Docker, not Railway                                        |
| Final refreshed Railway image and HMT/Federal/AEG browser tests | Build validation passed; all eight desktop/phone checks passed on port 8097                                                             | Refreshed artifacts reach the running app and event detail/export flows                                  | Other source browser suites were not rerun against this snapshot |

Rejected observations cover TBA dates, unsupported ticket URLs/providers,
Cervantes off-site listings outside its reviewed scope, and invalid/conflicting
times. Existing rejection and last-valid retention rules remain unchanged.
Previous snapshots remain in the ignored run directories and Git history.
No remote push, deployment, or schedule was enabled.

## Manual Actions dry run — September 13, 2026

The first automation step is `.github/workflows/refresh-dry-run.yml`. It uses
manual dispatch, a locale choice (currently Denver), read-only Git permissions,
immutable action revisions, and per-locale concurrency. It reuses the existing
refresh container and Railway Dockerfile rather than duplicating adapter logic
in the workflow. A host-native ingestion setup was not selected because the
container already supplies the required Go tools and browser dependencies.

The input is the selected locale's committed catalog and configuration. Fresh
captures update a disposable runner directory, not the operator's store or Git.
Usable partial results reach the application build and HTTP checks, then leave
the workflow failed with diagnostics. Total failures cannot silently build the
unchanged seed as if refresh succeeded. A nonempty calendar and correct locale
are required by the dry-run smoke check; normal empty-app behavior is unchanged.

Only the validated snapshot and operator diagnostics are uploaded, for seven
days. Raw captures and recovery copies are excluded. Artifact transport and
venue access from a GitHub runner remain unverified until a remote run occurs.
The workflow has no schedule, publishing credentials, Git writes, or Railway
deployment. Git publishing and daily deployment remain separate steps.

Local verification for this step:

| Evidence / command                                                                                                                                                                                                                                                             | Observed result                                                                          | Supported finding                                                                    | Limit                                                                                                                                               |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `npm run test:refresh-dry-run`                                                                                                                                                                                                                                                 | Four tests passed, including CLI diagnostics, partial/failure assessment and HTTP errors | Helper rejects inconsistent or missing results and checks the served locale/calendar | Synthetic report and HTTP cases                                                                                                                     |
| `npm run test:refresh`; `docker build --target refresh-test -t event-calendar-refresh-test .` then `docker run --rm event-calendar-refresh-test`                                                                                                                               | Twelve coordinator tests and both container integration tests passed                     | Existing capture → Go publication → snapshot → HTTP path works with the new checker  | Local synthetic upstreams                                                                                                                           |
| `npm run test:contracts`; `docker run --rm event-calendar-contract-tests go test -race ./...`                                                                                                                                                                                  | Contract handoff, 91 frontend tests, and full Go suite passed                            | Existing contracts and Go behavior remain intact                                     | No GitHub runner                                                                                                                                    |
| Capture scripts `test:rhp`, `test:clique`, `test:plot`, `test:kse`, `test:marquis`, `test:blackbox`, `test:venuepilot`, `test:meowwolf`, `test:ophelias`, `test:buzzard`, `test:herbs`, `test:afton`, plus `test:capture-profiles` and `test:locale-tools` (all via `npm run`) | All commands passed                                                                      | Existing offline capture and packaging tests remain intact                           | Does not establish live venue access                                                                                                                |
| `docker build -f Dockerfile.railway --build-arg LOCALE=denver -t withadult-dry-run-check .`; helper `smoke denver http://127.0.0.1:8097 ...`                                                                                                                                   | Build and HTTP check passed; five sampled events contained IDs, titles, venues and dates | Tracked Denver snapshot reaches the real container's public API                      | Existing tracked snapshot, not a fresh GitHub capture                                                                                               |
| `CALENDAR_EMPTY_URL=http://127.0.0.1:8096 npm run test:e2e`                                                                                                                                                                                                                    | 148 passed, 66 opt-in tests skipped                                                      | Default browser regression suite passes with its required empty-data container       | Two earlier runs failed because the empty test used populated data, then a nonexistent directory; corrected by mounting an existing empty directory |
| `docker run --rm --mount type=bind,src=$PWD,dst=/repo,readonly -w /repo rhysd/actionlint:1.7.7 .github/workflows/refresh-dry-run.yml`                                                                                                                                          | Passed                                                                                   | Workflow syntax and embedded shell checks pass                                       | Not workflow execution                                                                                                                              |
| `npm run format:check`, `npm run lint`, `npm run typecheck`, `npm run build`, explicit Prettier check for workflow/ADR, `git diff --check`                                                                                                                                     | Passed                                                                                   | Static checks and frontend build pass                                                | Markdown inspected as source; no documentation render pipeline                                                                                      |

The empty-exit-file CLI regression test first failed, then passed after explicit
exit-code syntax validation. No tracked event data, local app data, Railway
settings, secrets, GitHub settings, or schedules changed during this step.

## Dry-run report access and streaming fix — September 13, 2026

Run `34783653518` returned a partial refresh, then failed when the host-side
report helper encountered `EACCES` in a root-owned `0700` run directory.
The same Linux permission boundary was reproduced locally. The workflow now
runs the existing report helper inside the refresh container and exports only
the report and summary into the runner-readable diagnostics mount. The capture
mount is read-only for this operation. Widening permissions on the raw capture
tree was rejected because the runner does not need access to those files.
Changing the ingestion UID was not selected because it would also change browser
cache access and publication ownership beyond the required report boundary.

The refresh subprocess wrapper now supports live stdout/stderr callbacks and
stage progress. The CLI forwards child output to stderr, preserving its final
JSON stdout and the existing captured JSON used by the Go ingestion consumer.
Each subprocess reports start/completion and periodic quiet-state progress.
Heartbeat timers stop on success, failure, spawn error, or timeout. Source
failures still retain last-valid data and do not weaken publication validation.

The workflow uses `tee` with explicit `PIPESTATUS` handling so log persistence
does not conceal the refresh exit code. A log-write failure fails the step.
While untrusted source output is streamed, workflow command processing is stopped
with a random per-step token, following [GitHub's documented mechanism](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands#stopping-and-starting-workflow-commands).
The token is not passed into the capture container.

Meow Wolf's identity mismatch and Roxy's HTTP/type failure from the first remote
run remain separate investigations. This change does not retry the run, push to
GitHub, deploy, enable a schedule, change source policy, or replace event data.

Verification for the report-access and streaming fix:

| Evidence / command                                                                                                                                                              | Result                                                   | Supported finding                                                                                                                                                                  | Limit                                                        |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| `npm run test:refresh`                                                                                                                                                          | 16 tests passed                                          | Live output, stage messages, quiet heartbeats, output limits, timer cleanup, and retention rules work; a gated child cannot exit until both output streams are received            | Synthetic subprocesses and fixtures                          |
| `npm run test:refresh-dry-run`                                                                                                                                                  | Five tests passed                                        | Exact workflow shell streams both channels to the console/log, preserves exit codes 0/1/2/137, and invokes the report helper inside Docker                                         | Shell test substitutes Docker; permissions tested separately |
| `docker build --target refresh-test -t event-calendar-refresh-test .` and `docker run --rm event-calendar-refresh-test`                                                         | Three tests passed                                       | Linux UID 1001 reproduces the old EACCES, reads the exported report/summary, and still cannot read the private run; real capture/publication/HTTP flow preserves JSON and progress | Local Linux container, not GitHub                            |
| `npm run test:contracts` and `docker run --rm event-calendar-contract-tests go test -race ./...`                                                                                | Contract handoff, 91 frontend tests, and Go suite passed | Existing artifact and consumer behavior remains intact                                                                                                                             | No live source calls                                         |
| Capture and packaging npm commands listed in the preceding verification table                                                                                                   | All passed again                                         | Existing capture and locale-tool regressions pass                                                                                                                                  | Live source failures not addressed                           |
| `actionlint:1.7.7` command listed above; `npm run format:check`; `npm run lint`; `npm run typecheck`; `npm run build`; explicit Prettier workflow/ADR check; `git diff --check` | Passed                                                   | Workflow syntax, static checks, and build pass                                                                                                                                     | Remote workflow execution remains pending                    |

The three initial streaming/progress tests and the workflow regression test
failed before implementation, then passed. The application UI and HTTP consumer
were not changed; the earlier browser verification remains applicable and was
not repeated. Documentation was inspected as Markdown source; no documentation
render pipeline exists in this repository.
