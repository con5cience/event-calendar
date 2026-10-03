import { test } from "node:test";
import assert from "node:assert/strict";
import { capture, calendarEnvelope } from "./capture.mjs";

const feed = `BEGIN:VCALENDAR\r
PRODID:-//Google Inc//Google Calendar 70.9054//EN\r
VERSION:2.0\r
X-WR-CALNAME:D3 Arts Events Calendar\r
BEGIN:VEVENT\r
UID:abc@google.com\r
DTSTART:20261010T010000Z\r
SUMMARY:Test Show\r
END:VEVENT\r
END:VCALENDAR`;

test("reads the iCal feed once and trims the envelope", async () => {
  const calls = [];
  const r = await capture(async (url, options) => {
    calls.push(url);
    assert.equal(options.redirect, "error");
    return new Response("\r\n" + feed + "\r\n", {
      headers: { "content-type": "text/calendar; charset=UTF-8" },
    });
  }, new Date("2026-10-03T18:00:00Z"));
  assert.deepEqual(calls, [
    "https://calendar.google.com/calendar/ical/mkjl322tg56ecg0mf7qjvtllcc%40group.calendar.google.com/public/basic.ics",
  ]);
  assert.equal(r.snapshot.from, "2026-10-03");
  assert.equal(r.snapshot.through, "2027-10-03");
  assert.ok(
    calendarEnvelope(r.snapshot.calendar).startsWith("BEGIN:VCALENDAR"),
  );
  assert.ok(!r.snapshot.calendar.startsWith("\r\n"));
});

test("an incomplete calendar envelope fails", () => {
  assert.throws(() => calendarEnvelope("BEGIN:VEVENT\nEND:VEVENT"), /calendar/);
});

for (const [name, response] of [
  ["HTTP", () => new Response("denied", { status: 503 })],
  [
    "size",
    () =>
      new Response("x".repeat(4 * 1024 * 1024 + 1), {
        headers: { "content-type": "text/calendar" },
      }),
  ],
])
  test(`${name} failure closes the capture`, async () => {
    await assert.rejects(
      capture(async () => response(), new Date("2026-10-03T18:00:00Z")),
    );
  });
