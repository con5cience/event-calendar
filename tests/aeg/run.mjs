// Exercise local snapshot -> reconciliation -> publication -> container HTTP/UI.
// Fixtures and the normal store stay read-only; retain the isolated output store.
import { mkdtempSync, readFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import assert from "node:assert/strict";

const directory = mkdtempSync(join(tmpdir(), "event-calendar-aeg-"));
chmodSync(directory, 0o755);
const env = {
  ...process.env,
  PUBLICATION_TEST_STORE: directory,
  PUBLICATION_TEST_UID: String(process.getuid()),
  PUBLICATION_TEST_GID: String(process.getgid()),
  AEG_BASE_URL: "http://127.0.0.1:8092",
};
const compose = [
  "compose",
  "-p",
  `calendar-aeg-${process.pid}`,
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
function replay(venue, status = 0) {
  return JSON.parse(
    command(
      "docker",
      [
        ...compose,
        "run",
        "--rm",
        "--no-deps",
        "publisher",
        "replay-aeg",
        "--store",
        "/data",
        "--config",
        `/aeg/${venue}.yaml`,
        "--snapshot",
        `/aeg/${venue}.json`,
        "--now",
        "2026-09-09T12:00:00Z",
      ],
      { capture: true, status },
    ),
  );
}
console.log(`AEG test store (retained): ${directory}`);
try {
  command("docker", [...compose, "build"]);
  command("docker", [...compose, "up", "--wait", "published-app"]);
  for (const venue of [
    "gothic",
    "mission",
    "bluebird",
    "ogden",
    "fiddlers-green",
  ]) {
    const report = replay(venue);
    assert.equal(report.published, true);
    assert.equal(report.durable, true);
    assert.deepEqual(report.rejected, []);
  }
  const before = readFileSync(join(directory, "catalog.json"));
  assert.equal(replay("gothic", 1).published, false);
  assert.deepEqual(readFileSync(join(directory, "catalog.json")), before);
  const response = await fetch(`${env.AEG_BASE_URL}/api/calendar`);
  assert.equal(response.status, 200);
  const { events } = await response.json();
  // All five available synthetic rows are inspected through the public interface.
  for (const event of events) {
    assert.equal(event.title, "Fixture Ensemble");
    assert.equal(event.status, "Cancelled");
    assert.equal(event.doors_at, "2026-09-12T19:00:00-06:00");
    assert.equal(event.price, undefined);
    assert.equal(event.ticket_url, "https://example.com/tickets/1001");
    console.log(JSON.stringify(event));
  }
  assert.deepEqual(events.map((e) => e.venue).sort(), [
    "Bluebird Theater",
    "Fiddler's Green Amphitheatre",
    "Gothic Theatre",
    "Mission Ballroom",
    "Ogden Theatre",
  ]);
  const bluebird = events.find((e) => e.venue === "Bluebird Theater");
  assert.equal(
    bluebird.with_adult.ranges[1].condition,
    "Ticketed adult required",
  );
  assert.equal(bluebird.with_adult.reviewed_on, "2026-09-09");
  for (const event of events) {
    if (event.venue === "Fiddler's Green Amphitheatre") {
      // The synthetic fixture is deliberately 16+: no published exception.
      assert.equal(event.with_adult, undefined);
    } else {
      assert.equal(
        event.with_adult.ranges[1].condition,
        "Ticketed adult required",
      );
      assert.equal(event.with_adult.reviewed_on, "2026-09-09");
    }
  }
  command("npm", [
    "run",
    "test:e2e",
    "--",
    "tests/browser/aeg.spec.ts",
    "--grep",
    "AEG replay artifact",
  ]);
} finally {
  command("docker", [...compose, "down"]);
  console.log(`Inspect retained test artifacts at ${directory}`);
}
