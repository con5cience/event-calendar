import { test } from "node:test";
import assert from "node:assert/strict";
import {
  mkdtempSync,
  mkdirSync,
  writeFileSync,
  readFileSync,
  cpSync,
  existsSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  refreshLocale,
  snapshotLocale,
  runnerFor,
  execute,
} from "../scripts/refresh-locale.mjs";

function fixture() {
  const root = mkdtempSync(join(tmpdir(), "refresh-test-"));
  const site = join(root, "locales", "test-city");
  mkdirSync(join(site, "catalog"), { recursive: true });
  writeFileSync(
    join(site, "site.json"),
    JSON.stringify({
      id: "test-city",
      sources: { one: { adapter: "aeg-json" }, two: { adapter: "aeg-json" } },
    }),
  );
  writeFileSync(
    join(site, "catalog", "catalog.json"),
    JSON.stringify({ one: "original", two: "original" }),
  );
  const pack = async (_site, input, output) =>
    cpSync(input, output, {
      recursive: true,
      errorOnExist: true,
      force: false,
    });
  return { root, site, pack, capture: async () => {} };
}
const read = (dir) =>
  JSON.parse(readFileSync(join(dir, "catalog.json"), "utf8"));
test("capture pool overlaps independent providers, serializes shared providers and publication", async () => {
  const f = fixture();
  const sources = {
    one: { adapter: "aeg-json" },
    two: { adapter: "aeg-json" },
    three: { adapter: "holdmyticket-ical" },
    four: { adapter: "kse-calendar" },
    five: { adapter: "kse-venue-events" },
  };
  writeFileSync(
    join(f.site, "site.json"),
    JSON.stringify({ id: "test-city", sources }),
  );
  let active = 0,
    maximum = 0,
    writers = 0;
  const groups = new Set(),
    completed = [];
  const result = await refreshLocale("test-city", {
    ...f,
    capture: async ({ sourceID, runner }) => {
      const group =
        sourceID === "four" || sourceID === "five"
          ? "kse"
          : sources[sourceID].adapter;
      assert(!groups.has(group));
      assert(runner.command);
      groups.add(group);
      maximum = Math.max(maximum, ++active);
      await new Promise((resolve) =>
        setTimeout(resolve, sourceID === "one" ? 35 : 10),
      );
      groups.delete(group);
      active--;
      completed.push(sourceID);
      if (sourceID === "three") throw Error("capture failed");
    },
    refresh: async ({ sourceID, store }) => {
      assert(completed.includes(sourceID));
      assert.equal(++writers, 1);
      await new Promise((resolve) => setTimeout(resolve, 2));
      writeFileSync(
        join(store, "catalog.json"),
        JSON.stringify({ ...read(store), [sourceID]: "updated" }),
      );
      writers--;
      return { published: true, durable: true };
    },
  });
  assert.equal(maximum, 2);
  assert(completed.indexOf("three") < completed.indexOf("one"));
  assert.deepEqual(
    result.sources.map((s) => s.source),
    Object.keys(sources),
  );
  assert.equal(result.sources[2].status, "failed");
  assert.deepEqual(read(join(f.site, "catalog")), {
    one: "updated",
    two: "updated",
    four: "updated",
    five: "updated",
  });
  assert.equal(active, 0);
});
test("capture concurrency can be set to one and invalid limits fail before writes", async () => {
  for (const captureConcurrency of [0, -1, 1.5, "2", 5]) {
    const f = fixture();
    await assert.rejects(
      refreshLocale("test-city", { ...f, captureConcurrency }),
      /concurrency/i,
    );
    assert(!existsSync(join(f.site, ".refresh-lock")));
  }
  const f = fixture();
  let active = 0;
  writeFileSync(
    join(f.site, "site.json"),
    JSON.stringify({
      id: "test-city",
      sources: {
        one: { adapter: "aeg-json" },
        two: { adapter: "holdmyticket-ical" },
      },
    }),
  );
  await refreshLocale("test-city", {
    ...f,
    captureConcurrency: 1,
    capture: async () => {
      assert.equal(++active, 1);
      await new Promise((resolve) => setTimeout(resolve, 5));
      active--;
    },
    refresh: async () => ({ published: true, durable: true }),
  });
  assert.equal(active, 0);
});
test("refresh retains failed source even if its staging store was modified", async () => {
  const f = fixture();
  const result = await refreshLocale("test-city", {
    ...f,
    refresh: async ({ sourceID, store }) => {
      writeFileSync(
        join(store, "catalog.json"),
        JSON.stringify({ ...read(store), [sourceID]: "updated" }),
      );
      if (sourceID === "two") throw Error("failed after write");
      return { published: true, durable: true, rejected: [] };
    },
  });
  assert.deepEqual(read(join(f.site, "catalog")), {
    one: "updated",
    two: "original",
  });
  assert.equal(result.status, "partial");
  assert.equal(result.sources[1].status, "failed");
  assert.ok(existsSync(result.report));
});
test("all failures leave tracked snapshot unchanged and report failure", async () => {
  const f = fixture();
  await assert.rejects(
    refreshLocale("test-city", {
      ...f,
      refresh: async () => {
        throw Error("offline");
      },
    }),
    /No source refreshed/,
  );
  assert.deepEqual(read(join(f.site, "catalog")), {
    one: "original",
    two: "original",
  });
});
test("all capture failures settle without ingestion or replacing the catalog", async () => {
  const f = fixture();
  const completed = [];
  await assert.rejects(
    refreshLocale("test-city", {
      ...f,
      capture: async ({ sourceID }) => {
        completed.push(sourceID);
        throw Error("capture offline");
      },
      refresh: async () => {
        assert.fail("failed capture must never ingest");
      },
    }),
    /No source refreshed/,
  );
  assert.deepEqual(completed, ["one", "two"]);
  assert.deepEqual(read(join(f.site, "catalog")), {
    one: "original",
    two: "original",
  });
  assert(!existsSync(join(f.site, ".refresh-lock")));
});
test("final validation failure does not replace tracked snapshot", async () => {
  const f = fixture();
  await assert.rejects(
    refreshLocale("test-city", {
      ...f,
      pack: async (site, input, output) => {
        if (output.endsWith("/export")) throw Error("invalid catalog");
        return f.pack(site, input, output);
      },
      refresh: async () => ({ published: true, durable: true, rejected: [] }),
    }),
    /invalid catalog/,
  );
  assert.deepEqual(read(join(f.site, "catalog")), {
    one: "original",
    two: "original",
  });
});
test("locale lock prevents overlapping refresh and snapshot export", async () => {
  const f = fixture();
  mkdirSync(join(f.site, ".refresh-lock"));
  await assert.rejects(refreshLocale("test-city", f), /lock/i);
  await assert.rejects(
    snapshotLocale("test-city", join(f.site, "catalog"), f),
    /lock/i,
  );
});
test("rejects unsafe locale IDs and missing adapter before capture", async () => {
  const f = fixture();
  await assert.rejects(refreshLocale("../test-city", f));
  assert.throws(() => runnerFor("unknown"), /Unsupported adapter/);
});
test("all configured Denver adapters have a runner", () => {
  const site = JSON.parse(
    readFileSync(new URL("../locales/denver/site.json", import.meta.url)),
  );
  for (const source of Object.values(site.sources))
    assert.ok(runnerFor(source.adapter).command);
});

test("every configured capture runner imports its existing entry point", async () => {
  const siteURL = new URL("../locales/denver/", import.meta.url);
  const site = JSON.parse(readFileSync(new URL("site.json", siteURL)));
  const profiles = JSON.parse(readFileSync(new URL("capture.json", siteURL)));
  for (const [sourceID, source] of Object.entries(site.sources)) {
    assert.ok(profiles[sourceID], `Missing capture profile: ${sourceID}`);
    const runner = runnerFor(source.adapter);
    if (!runner.script) continue;
    const moduleURL = new URL(`./${runner.script}`, import.meta.url).href;
    await execute(
      process.execPath,
      [
        "--input-type=module",
        "-e",
        `const m = await import(${JSON.stringify(moduleURL)}); if (typeof m[${JSON.stringify(runner.fn || "capture")}] !== "function") throw Error("Missing capture export");`,
      ],
      {
        env: { SITE_DIR: siteURL.pathname, CAPTURE_SOURCE: sourceID },
      },
    );
  }
});

test("hung subprocess is terminated and reported as a timeout", async () => {
  await assert.rejects(
    execute(process.execPath, ["-e", "setInterval(() => {}, 1000)"], {
      timeout: 100,
    }),
    /Process timeout/,
  );
});

test("subprocess output streams before exit while returned stdout stays parseable", async () => {
  const gate = join(mkdtempSync(join(tmpdir(), "stream-gate-")), "continue");
  let completed = false,
    output = "",
    diagnostics = "";
  const progress = [];
  const pending = execute(
    process.execPath,
    [
      "-e",
      'process.stdout.write(JSON.stringify({title:"Beyoncé"})); process.stderr.write("diagnostic\\n"); const gate=process.argv[1]; setTimeout(()=>{const timer=setInterval(()=>{if(require("fs").existsSync(gate)) clearInterval(timer);},10);},200);',
      gate,
    ],
    {
      label: "gothic capture",
      timeout: 2000,
      heartbeatMs: 40,
      progress: (message) => progress.push(message),
      onStdout: (chunk) => {
        assert.equal(completed, false);
        output += chunk;
        if (diagnostics) writeFileSync(gate, "continue");
      },
      onStderr: (chunk) => {
        assert.equal(completed, false);
        diagnostics += chunk;
        // The child cannot exit until the parent receives both streams.
        if (output) writeFileSync(gate, "continue");
      },
    },
  );
  const raw = await pending;
  completed = true;
  assert.equal(output, raw);
  assert.equal(JSON.parse(raw).title, "Beyoncé");
  assert.equal(diagnostics, "diagnostic\n");
  assert(progress.some((line) => line.includes("gothic capture: started")));
  assert(progress.some((line) => line.includes("still running")));
  assert(progress.some((line) => line.includes("completed")));
  const count = progress.length;
  await new Promise((done) => setTimeout(done, 80));
  assert.equal(progress.length, count, "heartbeat stops after exit");
});

test("failed and timed-out subprocesses stream diagnostics and stop progress", async () => {
  for (const code of [
    'process.stderr.write("failed now\\n"); process.exitCode=7',
    'process.stderr.write("waiting\\n"); setInterval(()=>{},100)',
  ]) {
    let diagnostics = "";
    const progress = [];
    await assert.rejects(
      execute(process.execPath, ["-e", code], {
        timeout: 150,
        heartbeatMs: 30,
        label: "test capture",
        progress: (message) => progress.push(message),
        onStderr: (chunk) => {
          diagnostics += chunk;
        },
      }),
      /exited 7|Process timeout/,
    );
    assert(diagnostics.length > 0);
    assert(progress.some((line) => line.includes("failed")));
    const count = progress.length;
    await new Promise((done) => setTimeout(done, 70));
    assert.equal(progress.length, count);
  }
});

test("refresh reports source stages and retention without changing the result", async () => {
  const f = fixture(),
    messages = [];
  const result = await refreshLocale("test-city", {
    ...f,
    progress: (message) => messages.push(message),
    refresh: async ({ sourceID }) => {
      if (sourceID === "two") throw Error("offline");
      return { published: true, durable: true, rejected: [] };
    },
  });
  assert.equal(result.status, "partial");
  for (const expected of [
    "test-city: refresh started",
    "one: source started",
    "one: published",
    "two: source started",
    "two: failed",
    "test-city: refresh partial",
  ])
    assert(
      messages.some((message) => message.includes(expected)),
      expected,
    );
});

test("streaming retains output limits and stops timers after spawn failure", async () => {
  await assert.rejects(
    execute(
      process.execPath,
      ["-e", 'process.stdout.write("x".repeat(2*1024*1024))'],
      { onStdout: () => {} },
    ),
    /Process output limit exceeded/,
  );
  const messages = [];
  await assert.rejects(
    execute("/nonexistent-withadult-test-command", [], {
      progress: (message) => messages.push(message),
      heartbeatMs: 20,
    }),
    /ENOENT/,
  );
  await new Promise((done) => setTimeout(done, 40));
  const count = messages.length;
  await new Promise((done) => setTimeout(done, 60));
  assert.equal(messages.length, count);
  assert(!messages.some((message) => message.includes("still running")));
});
