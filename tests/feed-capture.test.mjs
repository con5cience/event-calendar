import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { captureAEG, captureHMT, hmtURLs } from "../scripts/capture-feeds.mjs";
const hq = JSON.parse(
  readFileSync(new URL("../internal/hmt/testdata/hq.json", import.meta.url)),
);
test("AEG preserves exact response bytes for the Go decoder", async () => {
  const raw = '{"meta":{},"events":[]}';
  assert.equal(
    (
      await captureAEG(
        {
          endpoint:
            "https://aegwebprod.blob.core.windows.net/json/events/2/events.json",
        },
        async () => new Response(raw),
      )
    ).snapshot,
    raw,
  );
});
test("feed HTTP errors and oversized responses fail closed", async () => {
  await assert.rejects(
    captureAEG(
      { endpoint: "https://example.com" },
      async () => new Response("no", { status: 503 }),
    ),
    /HTTP 503/,
  );
  await assert.rejects(
    captureAEG(
      { endpoint: "https://example.com" },
      async () => new Response("a".repeat(4 * 1024 * 1024 + 1)),
    ),
    /4 MiB/,
  );
});
test("HMT discovers folded public event URLs without modifying the calendar", () => {
  assert.deepEqual(hmtURLs(hq.calendar), [
    "https://holdmyticket.com/event/1001",
  ]);
  assert.deepEqual(
    hmtURLs(hq.calendar.replace("event/1001", "event/\r\n 1001")),
    ["https://holdmyticket.com/event/1001"],
  );
  assert.throws(
    () => hmtURLs(hq.calendar.replace("holdmyticket.com", "example.com")),
    /URL/,
  );
  assert.throws(() => hmtURLs("not a calendar"), /calendar/);
});
test("HMT snapshot contains original calendar and parsed detail; missing detail stays explicit", async () => {
  const calls = [];
  const fetcher = async (url) => {
    calls.push(url);
    return new Response(url.includes("/ics/") ? hq.calendar : "html");
  };
  const result = await captureHMT(
    { endpoint: "https://holdmyticket.com/ics/6457" },
    fetcher,
    async () => hq.details["1001"],
  );
  assert.deepEqual(result.snapshot, hq);
  assert.deepEqual(calls, [
    "https://holdmyticket.com/ics/6457",
    "https://holdmyticket.com/event/1001",
  ]);
  const missing = await captureHMT(
    { endpoint: "https://holdmyticket.com/ics/6457" },
    fetcher,
    async () => {
      throw Error("bad JSON-LD");
    },
  );
  assert.deepEqual(missing.snapshot.details, {});
  assert.equal(missing.failures.length, 1);
});
