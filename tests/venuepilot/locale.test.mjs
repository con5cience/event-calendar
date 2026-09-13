import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { spawnSync } from "node:child_process";

test("VenuePilot capture selects another locale's source and account", () => {
  const directory = mkdtempSync(join(tmpdir(), "venuepilot-locale-"));
  writeFileSync(
    join(directory, "site.json"),
    JSON.stringify({
      id: "coastal",
      sources: { harbor: { adapter: "venuepilot" } },
    }),
  );
  writeFileSync(
    join(directory, "capture.json"),
    JSON.stringify({
      harbor: {
        endpoint: "https://harbor.example/graphql",
        account_id: 987,
        timezone: "Pacific/Auckland",
      },
    }),
  );
  const result = spawnSync(
    process.execPath,
    [
      "--input-type=module",
      "-e",
      `
    import assert from "node:assert/strict";
    import { queryPage } from "./tests/venuepilot/capture.mjs";
    await queryPage("2026-09-12", "2027-09-12", 1, async (url, options) => {
      assert.equal(url, "https://harbor.example/graphql");
      assert.deepEqual(JSON.parse(options.body).variables.accountIds, [987]);
      return new Response("{}");
    });
  `,
    ],
    {
      encoding: "utf8",
      env: { ...process.env, SITE_DIR: directory, CAPTURE_SOURCE: "harbor" },
    },
  );
  assert.equal(result.status, 0, result.stderr);
});
