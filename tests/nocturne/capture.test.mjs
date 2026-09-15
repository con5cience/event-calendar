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
test("failed policy page aborts capture", async () => {
  await assert.rejects(
    capture(
      async () => new Response("error", { status: 500 }),
      new Date("2026-09-14T18:00:00Z"),
      settings,
    ),
  );
});
test("transient response failures retry per request and capture succeeds", async () => {
  const calls = new Map();
  const respond = (url, count) => {
    if (!url.includes("format=ical"))
      return new Response("policy", {
        headers: { "content-type": "text/html" },
      });
    if (url.endsWith("date=2026-09-01") && count === 1)
      return new Response("busy", { status: 503 });
    if (url.endsWith("date=2026-10-01") && count === 1)
      throw TypeError("fetch failed");
    if (url.endsWith("date=2026-11-01") && count === 1)
      return new Response("challenge", {
        headers: { "content-type": "text/html" },
      });
    return new Response("BEGIN:VCALENDAR\r\nEND:VCALENDAR", {
      headers: { "content-type": "text/calendar" },
    });
  };
  const result = await capture(
    async (u) => {
      const url = String(u);
      const count = (calls.get(url) || 0) + 1;
      calls.set(url, count);
      return respond(url, count);
    },
    new Date("2026-09-14T18:00:00Z"),
    settings,
  );
  assert.equal(
    result.snapshot.pages.months["2026-09-01"],
    "BEGIN:VCALENDAR\r\nEND:VCALENDAR",
  );
  // Fourteen URLs are fetched once per pass; only the three transient
  // requests needed one extra attempt.
  assert.equal(calls.size, 14);
  for (const [url, count] of calls)
    assert.equal(count, /date=2026-(09|10|11)-01$/.test(url) ? 3 : 2, url);
  assert.equal(
    [...calls.values()].reduce((total, count) => total + count, 0),
    31,
  );
});
test("exhausted retries fail the month and abort capture", async () => {
  const calls = [];
  await assert.rejects(
    capture(
      async (u) => {
        calls.push(String(u));
        if (String(u).includes("format=ical"))
          return new Response("busy", { status: 503 });
        return new Response("policy", {
          headers: { "content-type": "text/html" },
        });
      },
      new Date("2026-09-14T18:00:00Z"),
      settings,
    ),
    /Calendar HTTP or content-type failure after 3 attempts \(HTTP 503\)/,
  );
  // One policy read, then the first month attempted three times.
  assert.equal(calls.length, 4);
  assert.equal(calls.filter((url) => url.includes("format=ical")).length, 3);
});
test("body-level invalid data does not retry", async () => {
  const calls = [];
  await assert.rejects(
    capture(
      async (u) => {
        calls.push(String(u));
        if (String(u).includes("format=ical"))
          return new Response("BEGIN:VCALENDAR", {
            headers: { "content-type": "text/calendar" },
          });
        return new Response("policy", {
          headers: { "content-type": "text/html" },
        });
      },
      new Date("2026-09-14T18:00:00Z"),
      settings,
    ),
    /Incomplete calendar/,
  );
  assert.equal(calls.filter((url) => url.includes("format=ical")).length, 1);
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
