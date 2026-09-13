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
  return { root, site, pack };
}
const read = (dir) =>
  JSON.parse(readFileSync(join(dir, "catalog.json"), "utf8"));
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
