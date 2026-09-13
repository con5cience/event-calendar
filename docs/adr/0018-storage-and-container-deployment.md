# ADR 0018: Filesystem development and Railway snapshot deployment

- Status: Image-baked snapshots accepted for initial Railway deployment; independent bucket publication deferred.
- Date: 2026-09-08
- Scope: Deployment design and implementation references.
- Authority: [Product contract](0014-product-and-delivery-contract.md).
- Related: [Artifact publication](0017-artifact-contract-and-publication.md), [verification](0019-implementation-and-verification-plan.md)

## Current decision — September 10, 2026

The user accepts rebuilding and redeploying whenever event data changes. The
initial Railway service uses `Dockerfile.railway`: React assets, the existing Go
server, and one validated snapshot baked into `/data`. No persistent volume,
bucket, database, or scheduled service is needed. The application continues to
read files directly and enforce expiry at request time.

`cmd/package-snapshot` uses `store.Validate` and bounded `store.ReadDocument`
reads. It preserves the exact validated bytes, copies only catalog references,
and refuses an existing output directory. Validation finishes before output is
created. An I/O failure can leave a partial new directory; a failed Docker stage
does not produce a deployable image. This tool is not a live publisher.

The Dockerfile-specific ignore file admits local artifacts without changing Git
or Compose ignores. The `upload-context` Docker target exports explicit source
inputs plus the validated snapshot to a fresh temporary directory. Upload that
directory with Railway CLI `--path-as-root --no-gitignore`, selecting the project,
service, and environment explicitly. Do not use this flag to upload the whole
workspace. The [README](../../README.md#railway-snapshot-deployment) defines the
commands and service settings. Git-only autodeploys cannot supply ignored data.

Refresh sequence: local ingestion → validation/package → build → upload/deploy.
Keep previous images/deployments for rollback. No external deployment is performed
by adding this workflow. Container API, event routes, downloads, missing/corrupt
input rejection, and the exported build context require verification.

This supersedes the bucket requirement and objection to image-baked data below
for the initial deployment. Those sections remain the deferred design for
independently published data. A volume-backed first deployment was considered but
adds storage that this immutable snapshot workflow does not need. Pages would
require adapting or separately hosting the Go server, so Railway retains the
existing application contract with fewer changes.

## Earlier design: independent ingestion and object storage (deferred)

### Context

The application and ingestion jobs must build and run independently. Compose should start only the app by default. Railway deployment should avoid operating a database or Redis.

## Decision

Provide two application-owned container images:

- App: compiled React assets plus the generic Go host.
- Ingestion: currently the Go publication command; the coordinator and source jobs remain proposed.

Use a mounted artifact directory for local Compose development. Use a private Railway S3-compatible bucket for production artifacts, subject to a real integration test. Share the artifact contract, not assumptions that object storage behaves like a mounted directory.

## Platform evidence

| Evidence source                                                                  | Documented observation                                                                             | Supported finding                                           | Material limit                                                                       |
| -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| [Railway volumes](https://docs.railway.com/volumes)                              | Volumes attach to services at runtime, not build or pre-deploy time.                               | Useful for service-owned persistent files.                  | A shared filesystem arrangement between the two services was not established.        |
| [Railway buckets](https://docs.railway.com/storage-buckets)                      | Private S3-compatible buckets expose authenticated object operations.                              | Both containers can use durable artifact storage.           | Public bucket URLs are not supported; browser access needs signed URLs or a backend. |
| [Railway bucket guide](https://docs.railway.com/guides/storage-buckets-guide)    | Documents worker-produced files and backend serving patterns.                                      | A server-side artifact reader matches the deployment model. | Actual SDK and credential behavior still require testing.                            |
| [Railway bucket reference](https://docs.railway.com/storage-buckets)             | Object versioning and bucket lifecycle configuration are not supported in the inspected reference. | Explicit artifact naming and cleanup are needed.            | Do not assume all AWS S3 features or backup mechanisms exist.                        |
| [Railway cron](https://docs.railway.com/cron-jobs)                               | Jobs must exit; schedules use UTC; an overlapping scheduled run is skipped.                        | A terminating run-all command fits native scheduling.       | Manual invocations and exact execution timing still need operational rules.          |
| [Docker multi-stage builds](https://docs.docker.com/build/building/multi-stage/) | Build stages can produce artifacts for a separate runtime stage.                                   | Keep frontend/build tools out of runtime images.            | Actual images have not been built.                                                   |

The preceding research inspected these references. No Railway service, bucket, schedule, or credentials were provisioned.

## Local Compose

- Default Compose configuration starts the app only.
- Include ingestion service definitions as comments, as the user requested; do not silently replace that choice with active services or profiles.
- With ingestion disabled and no data, the app starts and shows No events available.
- App mount is read-only; an explicitly enabled ingestion service can write the artifact directory.
- Use test fixtures only in test workflows, not as the normal demonstration dataset.
- The initial empty directory and established missing/corrupt data must be distinguishable.
- Document the exact enable/run procedure when the Compose file exists.
- Do not fetch live sources during app builds or startup.

For verification, a temporary test configuration may activate the documented commented service. The checked-in default must remain app-only.

The `ingestion` Docker target builds independently of frontend assets and uses a non-root
scratch runtime. The complete service definition remains commented in `compose.yaml`.
`compose.publish.test.yaml` activates only test services with an isolated temporary store;
`npm run test:publication` runs the real command and checks its output through the app.
See the root README for manual enabling, permissions, generation preconditions, exit
status, and recovery limits. There is no active cron service or live fetching in this image.
The fixture-only `replay-aeg` command uses the same publication boundary. Its separate
`npm run test:aeg` workflow mounts authored snapshots read-only and disables networking
on the publisher container. Both test workflows use port 8092 and must run sequentially.

## Railway deployment

Use separate app and cron services built from their respective Dockerfiles. The app receives a public endpoint; ingestion requires no public listener.

The app reads artifacts through server-held bucket credentials. It serves browser requests on its own origin and does not expose credentials in JavaScript, JSON, logs, or presigned write URLs.

Use the [AWS SDK for Go v2 custom endpoint support](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html) as the initial client candidate. Verify Railway's virtual-hosted endpoint style with the real bucket rather than forcing path-style addressing without evidence.

Keep configuration generic:

- Local directory or object-storage mode.
- Catalog key/path.
- Bucket connection details and credentials when needed.
- Listen address/port.
- Display-limit configuration.

Source URLs, adapter settings, venue policies, and overrides belong to ingestion configuration, not app deployment settings.

Use Railway's platform-provided port and a generic health endpoint. Exact environment names, Dockerfile paths, and runtime commands must be established and tested during implementation.

## Scheduling and failures

Use native Railway cron to invoke the coordinator, rather than a scheduler daemon inside the app. The command must exit after publication and reporting.

Schedule cadence remains undecided. Interpret schedule expressions in UTC. Do not promise minute-exact invocation.

Keep one publishing coordinator active. Native scheduled-run overlap handling does not prove that an operator cannot start another writer. Validate manual-run procedures or a simple supported concurrency mechanism before production.

A failed source retains its prior publication reference; a failed coordinator before publication retains the prior catalog. An established source without readable prior state fails explicitly.

Report outcomes through logs and exit status. No admin panel, public source-health display, or alerting service is introduced.

## Serving, caching, and cleanup

Cache only validated snapshots in app memory. Storage failure must not be represented as successful ingestion, an empty calendar, or authoritative event deletion.

Serve only allowed catalog-referenced data, not an arbitrary bucket/file proxy. Validate paths and external link schemes. Return appropriate cache headers; expiration must still be enforced for direct event routes and downloads.

Object cleanup is an ingestion/maintenance responsibility. Do not enable indefinite generations accidentally. Backups, disaster recovery, retention-cleanup cadence, and production object-store refresh cadence remain open operational choices. The local reader refreshes on each calendar API request, retains its last valid snapshot in memory on failure, and rechecks expiry on every response. It does not persist fallback state or poll in the background. Runtime bucket access may carry broader permissions than read-only application code; verify available credential scoping rather than claiming least privilege is configured.

No hosting cost or capacity estimate has been measured. No required resource limits or source request cadence are selected here.

## Alternatives and consequences

- A service-owned volume is simpler for one process, but cross-service sharing was not verified and must not be assumed.
- Combining ingestion with the app would conflict with independent execution and failure isolation.
- A public external object store could serve files directly, but adds another provider and is not necessary for the proposed Railway setup.
- Baking event data into the app image couples publication to rebuilding the app and does not fit independently published artifacts.
- A database or Redis adds an operating dependency not justified by this MVP.
- A Node host remains feasible, but the Go host can reuse schema, time, and ICS code.

The storage abstraction requires only the operations used by the artifact protocol. Do not build a general-purpose storage framework. Railway suitability remains documentation-backed until the approved deployment smoke test succeeds.
