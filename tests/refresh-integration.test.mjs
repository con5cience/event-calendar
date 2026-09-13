// Run in the refresh-test image. All HTTP responses are synthetic and local.
import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { spawn, spawnSync } from "node:child_process";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
  cpSync,
  chmodSync,
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

test("Roxy browser probe observes iframe feed success, challenge and absence", async () => {
  const { probeRoxyBrowser } =
    await import("../scripts/probe-roxy-user-agent.mjs");
  const browser = await chromium.launch();
  const profile = {
    page: "https://www.theroxydenver.com/calendar",
    endpoint: "https://aftontickets.com/api/get-events?key=fixture&page=",
  };
  try {
    for (const status of [200, 202, null]) {
      const context = await browser.newContext();
      try {
        await context.route("**/*", async (route) => {
          const url = route.request().url();
          if (url === profile.page)
            return route.fulfill({
              contentType: "text/html",
              body: '<iframe src="https://embed.example/frame"></iframe>',
            });
          if (url === "https://embed.example/frame")
            return route.fulfill({
              contentType: "text/html",
              body:
                status === null
                  ? "No feed"
                  : `<script>fetch(${JSON.stringify(profile.endpoint + "1")})</script>`,
            });
          if (url === profile.endpoint + "1")
            return route.fulfill({
              status,
              headers: {
                "content-type":
                  status === 200 ? "application/json" : "text/html",
                "access-control-allow-origin": "*",
                ...(status === 202 ? { "x-amzn-waf-action": "challenge" } : {}),
              },
              body: status === 200 ? "{}" : "",
            });
          return route.abort();
        });
        const result = await probeRoxyBrowser(
          await context.newPage(),
          profile,
          500,
        );
        assert.equal(result.page_status, 200);
        assert(result.frame_origins.includes("https://embed.example"));
        assert.equal(result.json_feed_response, status === 200);
        assert.equal(result.feed_responses.length, status === null ? 0 : 1);
        if (status === 202)
          assert.equal(result.feed_responses[0].challenge, "challenge");
        assert(!JSON.stringify(result).includes("key="));
      } finally {
        await context.close();
      }
    }
  } finally {
    await browser.close();
  }
});

test("container report export is readable by a runner without exposing raw captures", () => {
  assert.equal(
    process.getuid(),
    0,
    "Run this permission check inside refresh-test",
  );
  const root = mkdtempSync(join(tmpdir(), "report-permissions-"));
  chmodSync(root, 0o755);
  const site = join(root, "locales/test-city");
  mkdirSync(site, { recursive: true });
  save(join(site, "site.json"), { sources: { gothic: {} } });
  const parent = join(root, ".artifacts/refresh/test-city");
  mkdirSync(parent, { recursive: true });
  const privateRun = mkdtempSync(join(parent, "run-"));
  const report = {
    locale: "test-city",
    status: "success",
    sources: [{ source: "gothic", status: "published", rejected: [] }],
  };
  save(join(privateRun, "report.json"), report);
  const output = join(root, "diagnostics");
  mkdirSync(output);
  writeFileSync(join(output, "refresh-exit-code.txt"), "0\n");
  const helper = new URL("../scripts/refresh-dry-run.mjs", import.meta.url)
    .pathname;
  const hostRead = spawnSync(
    process.execPath,
    [helper, "report", "test-city", root, output],
    { uid: 1001, gid: 1001, encoding: "utf8" },
  );
  assert.equal(hostRead.status, 1);
  assert.match(hostRead.stderr, /EACCES/);
  const exported = spawnSync(
    process.execPath,
    [helper, "report", "test-city", root, output],
    { encoding: "utf8" },
  );
  assert.equal(exported.status, 0, exported.stderr);
  const runnerRead = spawnSync(
    process.execPath,
    [
      "--input-type=module",
      "-e",
      'import {readFileSync} from "node:fs"; console.log(readFileSync(process.argv[1],"utf8")); console.log(readFileSync(process.argv[2],"utf8"));',
      join(output, "report.json"),
      join(output, "summary.md"),
    ],
    { uid: 1001, gid: 1001, encoding: "utf8" },
  );
  assert.equal(runnerRead.status, 0, runnerRead.stderr);
  assert.match(runnerRead.stdout, /test-city/);
  const privateRead = spawnSync(
    process.execPath,
    [
      "-e",
      'require("fs").readFileSync(process.argv[1])',
      join(privateRun, "report.json"),
    ],
    { uid: 1001, gid: 1001, encoding: "utf8" },
  );
  assert.notEqual(privateRead.status, 0);
  assert.match(privateRead.stderr, /EACCES/);
});
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
  await assert.rejects(
    execute(process.execPath, [script, "test-city"], {
      env: { ...env, CAPTURE_CONCURRENCY: "0" },
    }),
    /Capture concurrency must be an integer/,
  );
  assert.equal(
    spawnSync("test", ["-e", join(site, ".refresh-lock")]).status,
    1,
  );
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
  let progress = "",
    resultJSON = "";
  try {
    // Partial success is deliberately nonzero but exports the validated merged catalog.
    await assert.rejects(
      execute(process.execPath, [script, "test-city"], {
        env,
        onStderr: (chunk) => {
          progress += chunk;
        },
        onStdout: (chunk) => {
          resultJSON += chunk;
        },
      }),
      /exited 2/,
    );
    assert.equal(JSON.parse(resultJSON).status, "partial");
    assert.match(progress, /gothic capture: started/);
    assert.match(progress, /capturing with concurrency 2/);
    assert.match(progress, /gothic ingest: completed/);
    assert.match(progress, /mission: failed; retaining last valid data/);
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
      execute(process.execPath, [script, "test-city"], {
        env: { ...env, CAPTURE_CONCURRENCY: "1" },
      }),
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

test("Meow Wolf mismatch reaches Go rejection and retains prior event while another updates", async () => {
  const root = mkdtempSync(join(tmpdir(), "meowwolf-mismatch-"));
  const site = join(root, "site");
  mkdirSync(site);
  save(join(site, "site.json"), {
    id: "test-city",
    sources: { "meow-wolf-denver": { adapter: "embedded-nextjs" } },
  });
  save(join(site, "capture.json"), {
    "meow-wolf-denver": {
      origin: "https://tickets.meowwolf.com",
      city: "denver",
      seller_id: "017a7f54-e443-a261-3c55-46ef4d921efb",
    },
  });
  const config = join(root, "source.yaml");
  const yaml = readFileSync(
    join(fixtureRoot, "internal/meowwolf/testdata/meow-wolf-denver.yaml"),
    "utf8",
  );
  writeFileSync(config, yaml);
  const snapshot = json(
    join(fixtureRoot, "internal/meowwolf/testdata/meow-wolf-denver.json"),
  );
  const second = structuredClone(snapshot.events[0]);
  second.id =
    "119f1aa0-d978-0ee7-4153-a9518ceb4d9a__ce822f61-3e04-0559-7a43-ca5b19313b79";
  second.detail.id = second.id.split("__")[0];
  second.detail.timeslots[0].id = second.id.split("__")[1];
  second.url = "second-fixture";
  second.title = second.detail.title = "Second fixture";
  snapshot.events.push(second);
  snapshot.total = 2;
  snapshot.check = structuredClone(snapshot.events);
  const input = join(root, "snapshot.json"),
    store = join(root, "store");
  mkdirSync(store);
  const replay = async () => {
    save(input, snapshot);
    return JSON.parse(
      await execute("ingest", [
        "replay-meowwolf",
        "--store",
        store,
        "--config",
        config,
        "--snapshot",
        input,
        "--now",
        "2026-09-11T18:00:00Z",
      ]),
    );
  };
  assert.equal((await replay()).published, true);
  const artifact = () => {
    const catalog = json(join(store, "catalog.json"));
    return json(join(store, catalog.sources[0].artifact));
  };
  const prior = artifact().events.find(
    (event) => event.upstream_id === snapshot.events[0].id,
  );
  writeFileSync(config, yaml.replace("state: new", "state: established"));
  const mismatched = structuredClone(snapshot.events[0].detail);
  mismatched.id = "219f1aa0-d978-0ee7-4153-a9518ceb4d9a";
  mismatched.meta[0].value = "21+";
  mismatched.timeslots[0].startTime = "2027-01-17T23:00:00Z";
  const moduleURL = new URL("./meowwolf/capture.mjs", import.meta.url).href;
  snapshot.events[0].detail = JSON.parse(
    await execute(
      process.execPath,
      [
        "--input-type=module",
        "-e",
        `import {detail} from ${JSON.stringify(moduleURL)}; const e=JSON.parse(process.argv[1]); console.log(JSON.stringify(detail({isEvents:true,isDetails:true,seller:{id:e.sellerId},events:{events:[e]}})));`,
        JSON.stringify(mismatched),
      ],
      { env: { SITE_DIR: site, CAPTURE_SOURCE: "meow-wolf-denver" } },
    ),
  );
  snapshot.events[1].title = snapshot.events[1].detail.title =
    "Updated second fixture";
  snapshot.check = structuredClone(snapshot.events);
  const result = await replay();
  assert.equal(result.published, true);
  assert.equal(result.durable, true);
  assert.equal(result.rejected.length, 1);
  assert.match(result.rejected[0].reason, /detail identity mismatch/);
  const after = artifact();
  const retained = after.events.find(
    (event) => event.upstream_id === prior.upstream_id,
  );
  assert.deepEqual(retained, prior);
  assert(
    after.events.some((event) => event.title === "Updated second fixture"),
  );
});
