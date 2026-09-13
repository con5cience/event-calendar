# RHP fixtures and explicit captures

The JSON files here are synthetic. Never publish them to `.artifacts`.
The matching YAML files define the four supported venue profiles with `state: new`.
For an existing source, use an operator copy with `state: established`.

Run parser/publication tests with `npm run test:contracts` and capture-helper
tests with `npm run test:rhp`. Browser checks use an explicit `RHP_BASE_URL`:

For retained live captures, `TestCapturedHTMLMatchesRaw` compares every original
page with the compacted snapshot through the production normalizer. Set
`RHP_CAPTURE_DIR` to the mounted capture directory and `RHP_SOURCE` to its source
key when running `go test ./internal/rhp -run TestCapturedHTMLMatchesRaw -count=1`
in the Go test container. `RHP_SNAPSHOT` optionally selects a regenerated snapshot
filename; the default is `snapshot.json`. This check is skipped without an explicit
live capture and is separate from synthetic fixture tests.

Cervantes uses the same replay command with `cervantes.yaml`. Its reviewed
on-site room routes map to venue `Cervantes`; external promotions are reported as
deferred in the rejection list. Its admission rules are configured in YAML and
must not inherit the other RHP venues' guardian exception. Test its live publication
with `CERVANTES_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/cervantes.spec.ts`.

```sh
RHP_BASE_URL=http://127.0.0.1:8090 npm run test:e2e -- tests/browser/rhp.spec.ts
```

With explicit network/import approval, capture a venue:

```sh
node tests/rhp/capture.mjs lost-lake
node tests/rhp/capture.mjs larimer
node tests/rhp/capture.mjs globe-hall
node tests/rhp/capture.mjs cervantes
```

Each command creates a new temporary directory and prints its location. It saves
`calendar.json`, `raw-pages.json`, `snapshot.json`, and `report.json`. The originals
remain operator evidence and can contain upstream price text; published events
cannot. Inspect the report and representative raw records before replay. Missing
details are reported and rejected by normalization, not treated as absent events.
Calendar retrieval failure stops capture. Requests are serial, bounded by a
30-second timeout and 4 MiB per response; redirects are not followed.

Mount an existing staging store at `/data`, this directory at `/config:ro`, and
the capture directory at `/capture:ro` in the ingestion image. Then run:

```sh
/ingest replay-rhp --store /data --config /config/lost-lake.yaml --snapshot /capture/snapshot.json --now 2026-09-10T23:00:00Z
```

Use the capture's actual RFC3339 clock, not the example clock for a later import.
The command publishes to its explicit store. Inspect rejected records and test
the staged app before promoting its source artifacts through `ingest publish`
with the current destination generation. Existing sources and immutable files
remain intact. Compose startup does not fetch or schedule these sources.
