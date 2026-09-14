# ADR 0022: Afton venue event listings

- Status: Implemented; local verification recorded in ADR 0019
- Date: 2026-09-12
- Related: [Evaluation contract](../adr/0001-data-source-evaluation.md), [registry](../adr/0013-source-adapter-registry.md), [verification](../adr/0019-implementation-and-verification-plan.md)

## Context and decision

The [Roxy calendar](https://www.theroxydenver.com/calendar) embeds an Afton widget
inside a Wix iframe. The public widget is scoped to Roxy, not an aggregator.
Use its anonymous paginated JSON listing, enriched with the linked Afton event
pages. AEG, VenuePilot and HoldMyTicket adapters do not fit this pagination or
record contract. Reuse the existing replay, artifact, retention and admission
interfaces; no schema or application change is needed. Do not scrape calendar
cells or depend on browser execution during capture.

The browser's public listing request established this endpoint:

```text
https://aftontickets.com/api/get-events?key=732e49e3d992ab7ff998f13c3a2d3f08&per_page=12&page=1
```

The key is the public venue-widget identifier, not a secret. Direct requests work
without cookies, authorization or the generated `aftclid` tracking parameter.
The Roxy and Afton robots files allow the inspected public paths. The embed host's
robots URL returns application HTML, not usable robots directives. This review
does not establish redistribution rights or approval for recurring ingestion.

## Retrieval and coverage

`tests/afton/capture.mjs` enumerates all pages twice and reads each Afton-hosted
event page in each pass. Each page must agree on total, page number, page size,
remaining length and next-page state. IDs must be valid and unique. Requests use
canonical URLs, never arbitrary returned next-page URLs. A capture has a
conservative 500-record guard, 1 MiB per response, 30-second request timeout,
redirect rejection and a 4 MiB snapshot bound. Unverified zero-event responses
fail safely rather than removing all prior events.

### CI HTTP diagnostics — September 13, 2026

The first two GitHub dry runs reported `Afton HTTP or type failure`, but did not
record status or content type. A later local capture completed both listing and
detail passes. This does not establish why the GitHub requests failed.

Listing responses now log pass, page number, HTTP status, and content type. A
rejected HTTP/content-type response includes those fields in its error as well.
Content type is bounded and JSON-escaped. Raw response bodies, cookies, and the
widget key are not logged. Tests cover HTTP 403 and a
200 HTML response during the second pass. Existing failure and pagination rules
remain unchanged; no retry, spoofing, or browser fallback is introduced.

Run `34785826381` narrowed the failure to page 1, pass 1: HTTP 202 with
`text/html; charset=UTF-8`. The same anonymous request later returned HTTP 200
JSON locally. The CI artifact contained neither the HTML body nor challenge
headers, so a bot challenge remains unconfirmed.

Failed responses now include a diagnostic object in the existing error report.
Only `server`, `retry-after`, `x-amzn-waf-action`, and `cf-mitigated` headers are
added, each capped at 200 characters and JSON-escaped. At most 16 KiB of the body
is inspected under the existing request timeout; the stream is then cancelled.
The report contains byte count, truncation/read-failure flags, and fixed markers
(`awswaf`, `challenge-platform`, `captcha`, `access denied`), never body excerpts.
Markers are clues, not proof of a specific provider or cause. A body read failure
does not replace the original HTTP failure. No challenge code is executed.

Tests first failed for the missing diagnostics, then passed with coverage for
header bounds, exclusion of cookies/body text, body caps, stream cancellation,
and body read errors. The existing JSON acceptance and pagination rules are
unchanged. A new CI run is still needed to observe these diagnostics on the
actual failing response.

### Optional user-agent comparison

GitHub run `34787560798` returned `x-amzn-waf-action: challenge` with HTTP 202
and an empty body. The challenge response is confirmed; its triggering rule is
not known. A local comparison returned HTTP 200 JSON both with Node's default
user agent and with the installed Chromium user agent. That does not establish
the outcome from a GitHub runner.

The manual dry run now offers an opt-in `roxy_user_agent_probe` (default false).
`scripts/probe-roxy-user-agent.mjs` reuses the locale capture profile and reads
Chromium's actual user agent from a blank page. It makes two sequential Node
requests with redirect rejection and 30-second timeouts, changing only the user
agent on the second request. No browser navigation, proxy, cookie replay, or
challenge execution occurs. Ingestion is untouched.

Only the user agent, modes, statuses, bounded content-type/challenge headers,
and fixed failure flags enter `roxy-user-agent.json`. Bodies and exception text
are not logged. Request failure does not prevent the second comparison. Setup
failure is a failed diagnostic step but does not prevent the normal refresh.
The two-minute workflow limit also bounds setup. The existing diagnostic upload
retains the file; a completed probe is not evidence of successful ingestion.

Local verification: the new tests first failed for the missing probe and opt-in
input, then all eight dry-run tests passed. A loopback test required a rerun
outside the filesystem/network sandbox (`listen EPERM`). Workflow lint, lint,
formatting, and build/type checks passed. The rebuilt refresh-test image passed
all four integration tests; the contract suite passed with 91 frontend tests.
The actual probe CLI in that Linux container returned HTTP 200 JSON for both
requests and emitted the installed Linux Chromium user agent. The GitHub response
still requires a manual run with the input enabled. No catalog data was replaced.

### Optional browser observation

The independent `roxy_browser_probe` input (default false) invokes the same
diagnostic script with `--browser`. It opens the reviewed Roxy calendar from the
locale capture profile in a fresh Chromium session, waits up to 30 seconds for
DOMContentLoaded, then observes responses for 15 seconds. No clicks, scrolling,
proxy, custom user agent, session reuse, or challenge interaction are added.
Normal website scripts execute as part of navigation.

Only feed requests matching the configured Afton origin, path, and venue-widget
key are classified as Roxy feed responses. The report records status, bounded
content type/challenge headers, navigation/request failures, and document/frame
origins. Response arrays are capped at 32 with a truncation flag. Query strings,
paths, bodies, cookies, and raw exception text are not emitted. Event JSON is not
parsed; `json_feed_response` does not establish event validity or visible cards.
No observed request is an inconclusive result, not proof that the feed is blocked.

The workflow saves `roxy-browser.json` in the existing diagnostic artifact and
allows the normal refresh to proceed even if the probe fails. The two-minute
workflow limit bounds setup and observation. This is diagnosis, not an ingestion
fallback. Tests cover JSON, challenge, absent feed, and unrelated widget keys;
the Linux container suite also exercises a real Chromium iframe with intercepted
synthetic responses.

Local verification passed all ten dry-run tests, five container integration
tests, coordinator/profile/Afton tests, workflow lint, code lint, formatting,
and build/type checks. The contract suite passed with 91 frontend tests. The
initial HTTP test hit sandbox `listen EPERM` and passed with loopback permission.
The actual Linux-container browser visit returned HTTP 200 for the Roxy page,
Wix frame, Afton embed document, and scoped JSON feed. This verifies local
observation only; GitHub access and rendered event-card correctness remain
unverified. No source artifacts were changed.

### Optional operator-supplied proxy probe

The `roxy_proxy_probe` manual input (default false) invokes `--proxy` using the
repository Actions secret `HTTP_PROXY`. It accepts a full HTTP(S) proxy URL and
decodes any username/password for the installed Playwright request client's
explicit proxy settings. No new dependency or normal-ingestion fallback is added.
The secret is passed by environment variable name, never expanded in shell
arguments, and exists only on the probe step. The script removes the environment
variable before creating its request client and emits no raw exceptions.

The client makes one request to the reviewed Afton feed with certificate
verification enabled, no redirects, no retries, and a 30-second timeout. The
context is disposed afterward. The two-minute workflow step limit remains.
`roxy-proxy.json` contains only a numeric status, normalized content-type and
challenge indicators, or a fixed request-failure category and elapsed milliseconds. Even arbitrary response
header values are not copied into the report. No cookies, response body, proxy
address, credentials, or event artifacts are logged or retained by this probe.
Success establishes access through that proxy at that time, not recurring-access
permission or a complete validated source capture. Failure categories are
`configuration`, `client_setup`, `timeout`, `dns`, `connection`, `tls`, and
`unknown`. Configuration and setup are separated by execution stage. Request
errors use recognized codes/names or anchored Playwright error prefixes; raw
messages are never returned. Unknown errors remain unknown. Categories do not
identify the failed network hop or establish a provider-side cause.
Run `34790353335` returned only `request_failed: true`; its cause is not known.
A new remote run is required to collect these additional diagnostics.
The real GitHub secret and
provider connection require remote verification.

The local CLI test uses a synthetic authenticated CONNECT proxy. Playwright
reports a proxy-generated HTTP 502 as a response, not an exception; the test was
corrected after observing that behavior. Therefore a reported HTTP status can
originate at the proxy and must not automatically be attributed to Afton.
The test checks the CONNECT destination and credentials, redacted JSON output,
and empty stderr. It does not contact the real proxy or Afton.

Verification passed 13 dry-run tests, all six rebuilt-container integration
tests, 19 coordinator/feed tests, and the contract suite (91 frontend tests;
unchanged Go checks reused their cached layer). Workflow lint, code lint,
formatting and build/type checks passed. The HTTP test needed loopback permission
after an initial sandbox `listen EPERM`. No real proxy connection, deployment,
or source-artifact replacement occurred during local verification.

The error-category update passed 14 dry-run tests, seven rebuilt-container
integration tests, 19 coordinator/feed tests, and all 91 frontend contract tests.
Lint, formatting, build/type checks and `git diff --check` passed. The container
test observed a real refused local proxy connection categorized as `connection`;
other request categories were tested with synthetic exceptions. No real provider
connection was made. Normal ingestion and HTTP-response reporting are unchanged.

### Standalone connection-stage diagnostics

The **Roxy proxy diagnostics** workflow is independent of ingestion. It uses the
existing Actions `HTTP_PROXY` secret, only on its diagnostic step. The
`proxy-diagnostics` Docker target provides Playwright's API request client and
curl without installing a browser or building Go/application binaries. Local and
Actions comparisons explicitly select Linux AMD64. See the [operating commands](../../README.md)
for the same local container invocation and report interpretation.

Each target (neutral HTTPS and the configured Afton listing) receives one
Playwright request and three curl requests (default, IPv4, IPv6), sequentially.
DNS results, proxy CONNECT codes, TCP/TLS/request milestones, safe failure stages,
and cumulative timings distinguish client and destination behavior. Proxy
authentication rejection is reported for CONNECT 407. Other rejected CONNECT
codes are not attributed to Afton. IPv4/IPv6 selection controls the connection
to the proxy, not its egress. No automatic fallback or production proxy routing
is introduced.

The workflow also runs a separate `--direct` step before the proxy comparison.
It sends two curl requests to Roxy, with default and fixed Chrome-style Linux
user agents. No secret enters this step. Inherited proxy variables are removed;
`--proxy "" --noproxy "*"` prevents proxy use even if an ambient configuration
exists. The direct results go to `direct-curl.jsonl` and include normalized MIME
type and fixed challenge indicators. Direct TLS connections do not need a
CONNECT response to count as successful. Both requests retain the same timeouts,
TLS checks, redirect/retry policy, and other curl settings. This compares user
agents, not browser session or TLS fingerprint behavior. Normal Roxy ingestion
still uses Node fetch; no ingestion transport change is included.

A synthetic stalled CONNECT test showed curl can report zero TCP time despite
having connected and sent CONNECT. The implementation therefore also parses
fixed trace milestones in memory. Neither raw trace nor raw write-out JSON is
persisted: both can contain sensitive data. Curl receives its escaped configuration
over stdin with the default curl config disabled. Proxy environment overrides,
bypass settings, debug settings, and custom TLS overrides are removed from child
environments. TLS checks stay enabled. Response bodies are discarded.

Each curl request is bounded to 30 seconds total, 15 seconds for connection,
and bounded diagnostic buffers. Advertised response lengths above one MiB are
rejected; this curl version does not guarantee a byte cap for unknown-length
responses, which are discarded and remain time-bounded. Playwright retains its
existing 30-second limit. Results stream before subsequent requests start. The
workflow always attempts to upload available reports, including partial output.
The workflow does not publish, deploy, or replace event artifacts.

Reports identify the first unconfirmed stage, not necessarily an internal
provider root cause. Separate DNS results and requests may use different proxy
addresses or egress sessions. A successful HTTP response is not validation of
the event payload. A local success does not establish GitHub reachability, and
the stored GitHub secret cannot be read back to confirm equality with local input.

Local verification on 2026-09-14:

| Evidence                                                         | Observation                                                                                                                                          | Supported finding                                                      | Limit                                                                                |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| `npm run test:proxy-diagnostics` and Linux AMD64 container tests | Six tests passed in each environment, including real CONNECT rejection, tunnel/TLS stalls, refusal, CLI streaming and redaction                      | Diagnostic paths work against controlled fixtures                      | Not every possible provider response is covered                                      |
| Live Linux AMD64 container, Node v24.21.0, curl 7.88.1           | Both targets returned HTTP 200 through Playwright and curl default/IPv4; curl CONNECT was 200 and TLS/request milestones were confirmed              | The proxy works locally through both clients in the diagnostic runtime | Does not establish GitHub connectivity or event validity                             |
| Same live run                                                    | No IPv6 resolution; both forced IPv6 requests exited 7 without a tunnel                                                                              | Forced IPv6 did not work locally                                       | Default and IPv4 succeeded; not evidence that IPv6 caused GitHub's timeout           |
| Repository checks                                                | 14 existing dry-run tests, 19 refresh tests, 91 frontend contract tests and Go handoff passed; lint, formatting, build/type and workflow lint passed | No regression observed in checked paths                                | Unchanged Go checks reused the cached image layer; new GitHub workflow remains unrun |

Direct-mode verification on 2026-09-14:

| Evidence                              | Observation                                                                                                                                  | Supported finding                              | Limit                                                                          |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- | ------------------------------------------------------------------------------ |
| Host and Linux AMD64 diagnostic tests | Nine tests passed in each; a synthetic destination received the direct connection while the configured proxy received none                   | Direct mode bypasses inherited proxy variables | Controlled local fixtures                                                      |
| Live local Linux AMD64 direct CLI     | Default UA: HTTP 200/JSON in 781 ms; browser-style UA: HTTP 200/JSON in 510 ms; neither response had a challenge indicator or CONNECT tunnel | Both direct variants work locally              | Single request per variant; GitHub result and event validity remain unverified |
| Regression checks                     | 14 existing dry-run tests, 91 frontend contract tests and Go handoff passed; lint, formatting, build/type and workflow lint passed           | No regression observed in checked paths        | Unchanged Go checks reused the cached image layer                              |

Listing output retains only reviewed fields and omits prices. Detail HTML is
compacted with the established RHP helper; JSON-LD and visible admission markup
remain intact. The helper now correctly strips executable scripts at the start of
a fragment, with a regression test. Go parses inert HTML and compares normalized
detail results across passes. Different templates, offer prices and unrelated
markup cannot establish a changed event. Material field disagreement fails the
whole refresh. No checkout, login, cart or purchase action runs.

The reviewed capture contains 26 upcoming listings on three pages through March
4, 2027. Six raw records were inspected before reporting counts. Twenty-one have
Afton details and five link to external ticket providers. The adapter filters the
available listing to the next twelve months; it does not claim twelve populated
months or backfill past pages. Normal retention preserves previously published
past records for 90 days after the event date.

## Mapping and identity

Native events use Afton's opaque event ID. External numeric IDs get an `external-`
prefix to prevent collisions. Preserve title, date and the exact reviewed ticket
link as both the event and ticket link. Only known Afton, Skeletix and Strong
Survive URL forms are accepted. External links are not fetched in this profile.

For native events, cross-check JSON-LD title, venue/address, link identity, start
instant and status. Offer URLs establish identity; the organizer's matching URL
is the verified fallback for free events with no offers. Prices are never mapped.
Sold-out inventory does not mean cancelled. Cancellation comes from event status.
One upstream listing remains one event, including multi-day passes; do not split
ticket tiers or infer extra performances. Separate upstream performances remain
separate records.

Convert the structured start instant to America/Denver and require agreement with
the listing's local clock. The source labels summer dates MST, but its structured
instant confirms Denver's seasonal offset. Respect time visibility flags. Doors
use the explicitly labeled local field; reject invalid, ambiguous, later-than-show
or different-date doors. External listings retain their dates but omit ambiguous
times, since inspected external details disagreed with the widget's clock meaning.

Malformed identified records retain their last valid version and are reported.
Missing identity, incomplete capture or changed paired results stop publication.
Successful absence unlists prior events through the existing reconciler, while
their public URLs retain the last version until expiry. Generation guards protect
unrelated sources during publication.

## Roxy admission review

The [venue rules](https://www.theroxydenver.com/rules) and
[box-office page](https://www.theroxydenver.com/box-office-tickets) do not establish
a blanket admission age or parent exception. Do not infer either from the rules
on underage drinking. The Roxy configuration therefore has no default age category
and no venue-wide adult-admission rule.

Read the explicit event restriction from either inspected Afton template:
`modal-event-info-label` with its associated value, or `.age-restriction` with the
Age Restriction label. Repeated values must agree. Exact All Ages labels qualify
ages 0–17 for the existing child filter with
`All ages; event ticket requirements apply`. This does not invent an accompaniment
exception. Recognized restricted categories retain their restriction; unknown
wording remains unclassified and receives no clearance. Cancelled events receive
no clearance. Configured event-policy overrides win.

Five externally ticketed listings remain age-unknown and do not match the child
filter. Their distinct Skeletix and HoldMyTicket enrichment paths are follow-up
work, not an inferred Roxy policy. Afton restrictions must not be copied onto them.

## Operation and verification

Build the ingestion image and run `replay-afton` with a real captured snapshot,
`internal/afton/testdata/roxy.yaml` and an empty staging store. Never publish the
synthetic HTML fixture. Subsequent refreshes use an operator configuration with
`state: established`. Validate staging before generation-guarded publication.

Run `npm run test:afton`, `npm run test:rhp`, the full `npm run test:contracts`,
host checks, and `tests/browser/afton.spec.ts` with `ROXY_BASE_URL`. These cover
pagination, identities, external unknowns, templates, clocks, admission, overrides,
failed refreshes, retention, app details, filtering and ICS. The app image and
Compose startup remain unchanged; ingestion is operator-triggered only.
