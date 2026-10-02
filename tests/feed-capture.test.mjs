import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  captureAEG,
  captureHMT,
  foldHMTCalendar,
  hmtURLs,
} from "../scripts/capture-feeds.mjs";
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
test("HMT multi-line TEXT values fold into their property, escaped per RFC 5545", () => {
  const calendar = [
    "BEGIN:VCALENDAR",
    "BEGIN:VEVENT",
    "SUMMARY:Julien-K",
    "DESCRIPTION:Set Times:",
    "",
    "Julien-K 10:15-11:15pm",
    "",
    "Cruel Mourning 9:15-10pm",
    "",
    "Doors 6pm",
    "CREATED:20260929T150430Z",
    "URL;VALUE=URI:http://holdmyticket.com/event/467654 ",
    "",
    "DTEND:20261107T233000",
    "END:VEVENT",
    "END:VCALENDAR",
    "",
  ].join("\n");
  assert.equal(
    foldHMTCalendar(calendar),
    [
      "BEGIN:VCALENDAR",
      "BEGIN:VEVENT",
      "SUMMARY:Julien-K",
      "DESCRIPTION:Set Times:\\n\\nJulien-K 10:15-11:15pm\\n\\nCruel Mourning 9:15-10pm\\n\\nDoors 6pm",
      "CREATED:20260929T150430Z",
      "URL;VALUE=URI:http://holdmyticket.com/event/467654 ",
      "",
      "DTEND:20261107T233000",
      "END:VEVENT",
      "END:VCALENDAR",
      "",
    ].join("\n"),
  );
  assert.equal(foldHMTCalendar(hq.calendar), hq.calendar);
  assert.throws(
    () =>
      foldHMTCalendar(
        [
          "BEGIN:VCALENDAR",
          "BEGIN:VEVENT",
          "Not a property continuation",
          "END:VEVENT",
          "END:VCALENDAR",
          "",
        ].join("\n"),
      ),
    /Unattributed/,
  );
  assert.throws(
    () =>
      foldHMTCalendar(
        [
          "BEGIN:VCALENDAR",
          "BEGIN:VEVENT",
          "UID:fixture",
          "",
          "",
          "Not a property continuation",
          "END:VEVENT",
          "END:VCALENDAR",
          "",
        ].join("\n"),
      ),
    /Unattributed/,
  );
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
