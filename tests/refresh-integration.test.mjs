// Run in the refresh-test image. All HTTP responses are synthetic and local.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { spawn } from "node:child_process";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
  cpSync,
} from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { chromium } from "@playwright/test";
import { execute } from "../scripts/refresh-locale.mjs";
import { captureHMT, eventJSONLD } from "../scripts/capture-feeds.mjs";
import { checkCalendar } from "../scripts/refresh-dry-run.mjs";

const fixtureRoot = process.env.TEST_REPO || "/fixtures";
const json = (path) => JSON.parse(readFileSync(path, "utf8"));
const save = (path, value) => writeFileSync(path, JSON.stringify(value));
test("CLI refresh uses real capture, Go reconciliation, export, and HTTP consumer", async () => {
  const root = mkdtempSync(join(tmpdir(), "refresh-integration-"));
  const site = join(root, "locales/test-city");
  mkdirSync(join(site, "sources"), { recursive: true });
  const config = json(join(fixtureRoot, "locales/denver/site.json"));
  config.id = "test-city";
  config.sources = {
    gothic: config.sources.gothic,
    mission: config.sources.mission,
  };
  save(join(site, "site.json"), config);
  cpSync(join(fixtureRoot, "locales/denver/assets"), join(site, "assets"), {
    recursive: true,
  });
  const seed = join(root, "seed");
  mkdirSync(seed);
  for (const source of ["gothic", "mission"]) {
    const yaml = readFileSync(
      join(fixtureRoot, `internal/aeg/testdata/${source}.yaml`),
      "utf8",
    );
    const path = join(site, `sources/${source}.yaml`);
    writeFileSync(path, yaml);
    await execute("ingest", [
      "replay-aeg",
      "--store",
      seed,
      "--config",
      path,
      "--snapshot",
      join(fixtureRoot, `internal/aeg/testdata/${source}.json`),
      "--now",
      "2026-09-11T12:00:00Z",
    ]);
    writeFileSync(path, yaml.replace("state: new", "state: established"));
  }
  const script = new URL("../scripts/refresh-locale.mjs", import.meta.url)
    .pathname;
  const env = { CALENDAR_REPO: root };
  await execute(process.execPath, [script, "--snapshot", "test-city", seed], {
    env,
  });
  const target = join(site, "catalog");
  const before = json(join(target, "catalog.json"));
  const previous = (id) =>
    json(
      join(
        target,
        before.sources.find((source) => source.source_id === id).artifact,
      ),
    );
  const gothicBefore = previous("gothic");
  const missionBefore = previous("mission");
  const data = json(join(fixtureRoot, "internal/aeg/testdata/gothic.json"));
  // Use the current date so this is an in-range observation, not a historical replay.
  const date = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Denver",
  }).format(new Date());
  const offset = new Intl.DateTimeFormat("en-US", {
    timeZone: "America/Denver",
    timeZoneName: "shortOffset",
  })
    .formatToParts(new Date())
    .find((p) => p.type === "timeZoneName")
    .value.includes("-6")
    ? "-06:00"
    : "-07:00";
  const event = data.events[0];
  event.eventDateTimeISO = `${date}T20:00:00${offset}`;
  event.eventDateTime = `${date}T20:00:00`;
  event.eventDateTimeUTC = new Date(event.eventDateTimeISO)
    .toISOString()
    .slice(0, 19);
  event.doorDateTime = `${date}T19:00:00`;
  event.doorDateTimeUTC = new Date(`${date}T19:00:00${offset}`)
    .toISOString()
    .slice(0, 19);
  event.title.eventTitleText = "Refreshed fixture";
  let failAll = false;
  const upstream = createServer((req, res) => {
    if (failAll || req.url === "/mission") {
      res.writeHead(503);
      res.end("unavailable");
    } else {
      res.setHeader("content-type", "application/json");
      res.end(JSON.stringify(data));
    }
  });
  upstream.listen(0, "127.0.0.1");
  await once(upstream, "listening");
  const base = `http://127.0.0.1:${upstream.address().port}`;
  save(join(site, "capture.json"), {
    gothic: { endpoint: `${base}/gothic` },
    mission: { endpoint: `${base}/mission` },
  });
  let server;
  try {
    // Partial success is deliberately nonzero but exports the validated merged catalog.
    await assert.rejects(
      execute(process.execPath, [script, "test-city"], { env }),
      /exited 2/,
    );
    const after = json(join(target, "catalog.json"));
    assert.equal(
      after.sources.find((source) => source.source_id === "mission").sha256,
      before.sources.find((source) => source.source_id === "mission").sha256,
    );
    const gothic = json(
      join(
        target,
        after.sources.find((source) => source.source_id === "gothic").artifact,
      ),
    );
    assert.notDeepEqual(gothic, gothicBefore);
    assert.deepEqual(previous("mission"), missionBefore);
    failAll = true;
    const bytes = readFileSync(join(target, "catalog.json"), "utf8");
    await assert.rejects(
      execute(process.execPath, [script, "test-city"], { env }),
      /No source refreshed/,
    );
    assert.equal(readFileSync(join(target, "catalog.json"), "utf8"), bytes);
    server = spawn("/calendar", [], {
      env: {
        ...process.env,
        SITE_DIR: site,
        DATA_DIR: target,
        ASSETS_DIR: "/app/dist",
        PORT: "8088",
      },
      stdio: "pipe",
    });
    let ready = false;
    for (let i = 0; i < 50; i++) {
      try {
        if ((await fetch("http://127.0.0.1:8088/healthz")).ok) {
          ready = true;
          break;
        }
      } catch {
        /* startup */
      }
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    assert.ok(ready, "calendar container process becomes healthy");
    assert.match(
      await (await fetch("http://127.0.0.1:8088/api/calendar")).text(),
      /Refreshed fixture/,
    );
    const checked = await checkCalendar("http://127.0.0.1:8088", "test-city");
    assert.equal(checked.calendar, "nonempty");
    assert(checked.sample.some((event) => event.title === "Refreshed fixture"));
  } finally {
    upstream.close();
    if (server) {
      server.kill();
      await once(server, "close");
    }
  }
});

test("HMT detached HTML extraction and existing Go parser agree on fixture snapshot", async () => {
  const fixture = json(join(fixtureRoot, "internal/hmt/testdata/hq.json"));
  const browser = await chromium.launch({ headless: true });
  try {
    const context = await browser.newContext({ offline: true });
    const page = await context.newPage();
    const html = `<script>throw Error('must not execute')</script><script type="application/ld+json">${JSON.stringify(fixture.details["1001"])}</script>`;
    const result = await captureHMT(
      { endpoint: "https://holdmyticket.com/ics/6457" },
      async (url) =>
        new Response(url.includes("/ics/") ? fixture.calendar : html),
      (html) => eventJSONLD(page, html),
    );
    assert.deepEqual(result.snapshot, fixture);
    const root = mkdtempSync(join(tmpdir(), "hmt-capture-integration-"));
    save(join(root, "snapshot.json"), result.snapshot);
    const report = JSON.parse(
      await execute("ingest", [
        "replay-hmt",
        "--store",
        root,
        "--config",
        join(fixtureRoot, "internal/hmt/testdata/hq.yaml"),
        "--snapshot",
        join(root, "snapshot.json"),
        "--now",
        "2026-09-11T12:00:00Z",
      ]),
    );
    assert.equal(report.published, true);
    assert.deepEqual(report.rejected, []);
  } finally {
    await browser.close();
  }
});
