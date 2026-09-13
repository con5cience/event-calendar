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
Keep Git auto-deploy disabled for this ignored-artifact snapshot workflow.

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
