import { test } from "node:test";
import assert from "node:assert/strict";
import { capture } from "./capture.mjs";

const events = `<script type="application/json" id="wix-warmup-data">{"events":[]}</script>`;
const venue = `<main>A sober community space for people of all ages at 2130 S. Platte River Drive.</main>`;

test("reads the events page and venue policy twice", async () => {
  const calls = [];
  const r = await capture(async (url, options) => {
    calls.push(url);
    assert.equal(options.redirect, "error");
    const body = url.endsWith("/venue") ? venue : events;
    return new Response(body, { headers: { "content-type": "text/html" } });
  }, new Date("2026-09-17T18:00:00Z"));
  assert.deepEqual(calls, [
    "https://www.anteupdenver.com/events-1",
    "https://www.anteupdenver.com/venue",
    "https://www.anteupdenver.com/events-1",
    "https://www.anteupdenver.com/venue",
  ]);
  assert.equal(r.snapshot.from, "2026-09-17");
  assert.equal(r.snapshot.through, "2027-09-17");
  assert.equal(r.snapshot.pages.warmup, '{"events":[]}');
  assert.deepEqual(r.snapshot.pages, r.snapshot.check);
});

test("missing payload fails", async () => {
  await assert.rejects(
    capture(
      async (url) =>
        new Response(url.endsWith("/venue") ? venue : "<html></html>", {
          headers: { "content-type": "text/html" },
        }),
    ),
    /Wix events payload/,
  );
});

for (const [name, response] of [
  ["HTTP", () => new Response("denied", { status: 403 })],
  ["type", () => new Response(events)],
  [
    "size",
    () =>
      new Response("x".repeat(2 * 1024 * 1024 + 1), {
        headers: { "content-type": "text/html" },
      }),
  ],
])
  test(name + " fails", async () => {
    await assert.rejects(capture(async () => response()));
  });
