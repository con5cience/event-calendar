// Exercise the real container CLI and app using an isolated, retained temp store.
import { mkdtempSync, readFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import assert from "node:assert/strict";

const directory = mkdtempSync(join(tmpdir(), "event-calendar-publication-"));
// The app's non-root UID needs traversal; only the creating UID can write.
chmodSync(directory, 0o755);
const env = {
  ...process.env,
  PUBLICATION_TEST_STORE: directory,
  PUBLICATION_TEST_UID: String(process.getuid()),
  PUBLICATION_TEST_GID: String(process.getgid()),
};
const compose = [
  "compose",
  "-p",
  `calendar-publication-${process.pid}`,
  "-f",
  "compose.publish.test.yaml",
];
function command(bin, args, options = {}) {
  const result = spawnSync(bin, args, {
    env,
    encoding: "utf8",
    stdio: options.capture ? "pipe" : "inherit",
  });
  if (result.error) throw result.error;
  assert.equal(
    result.status,
    options.status ?? 0,
    `${bin} ${args.join(" ")}\n${result.stderr ?? ""}\n${result.stdout ?? ""}`,
  );
  return result.stdout;
}
function publish(expected, files, status = 0) {
  return JSON.parse(
    command(
      "docker",
      [
        ...compose,
        "run",
        "--rm",
        "--no-deps",
        "publisher",
        "publish",
        "--store",
        "/data",
        "--expect",
        expected,
        ...files,
      ],
      { capture: true, status },
    ),
  );
}

console.log(`Publication test store (retained): ${directory}`);
try {
  command("docker", [...compose, "build"]);
  command("docker", [...compose, "up", "--wait", "published-app"]);
  const empty = await fetch("http://127.0.0.1:8092/api/calendar");
  assert.equal(empty.status, 200);
  assert.deepEqual((await empty.json()).events, []);
  const first = publish("none", [
    "/input/gothic/preview.json",
    "/input/mission/preview.json",
    "/input/hq/preview.json",
  ]);
  assert.equal(first.published, true);
  assert.equal(first.durable, true);
  const before = readFileSync(join(directory, "catalog.json"));
  const rejected = publish("none", ["/input/gothic/preview.json"], 1);
  assert.equal(rejected.published, false);
  assert.deepEqual(readFileSync(join(directory, "catalog.json")), before);
  const second = publish(first.generation, ["/input/gothic/preview.json"]);
  assert.equal(second.published, true);
  const a = JSON.parse(before);
  const b = JSON.parse(readFileSync(join(directory, "catalog.json")));
  assert.notEqual(a.generation, b.generation);
  assert.deepEqual(
    b.sources.filter((s) => s.source_id !== "gothic"),
    a.sources.filter((s) => s.source_id !== "gothic"),
  );
  env.CALENDAR_BASE_URL = "http://127.0.0.1:8092";
  command("npm", ["run", "test:e2e"]);
} finally {
  command("docker", [...compose, "down"]);
  console.log(`Inspect retained test artifacts at ${directory}`);
}
