import { test } from "node:test";
import assert from "node:assert/strict";
import { capture } from "./capture.mjs";

const settings = {
  endpoint: "https://nocturnejazz.com/music?format=ical",
  policy: "https://nocturnejazz.com/faq",
  timezone: "America/Denver",
};
test("captures every horizon month twice and preserves raw calendars", async () => {
  const urls = [];
  const result = await capture(
    async (u) => {
      urls.push(String(u));
      return new Response(
        String(u).includes("format=ical")
          ? "BEGIN:VCALENDAR\r\nEND:VCALENDAR"
          : "policy",
        {
          headers: {
            "content-type": String(u).includes("format=ical")
              ? "text/calendar"
              : "text/html",
          },
        },
      );
    },
    new Date("2026-09-14T18:00:00Z"),
    settings,
  );
  assert.equal(urls.length, 28);
  assert.equal(result.snapshot.through, "2027-09-14");
  assert.equal(Object.keys(result.snapshot.pages.months).length, 13);
  assert.equal(
    result.snapshot.pages.months["2027-09-01"],
    "BEGIN:VCALENDAR\r\nEND:VCALENDAR",
  );
});
test("failed month aborts capture", async () => {
  await assert.rejects(
    capture(
      async () => new Response("error", { status: 500 }),
      new Date("2026-09-14T18:00:00Z"),
      settings,
    ),
  );
});
test("HTML challenge is not an empty feed", async () => {
  await assert.rejects(
    capture(
      async () =>
        new Response("challenge", { headers: { "content-type": "text/html" } }),
      new Date("2026-09-14T18:00:00Z"),
      settings,
    ),
  );
});
