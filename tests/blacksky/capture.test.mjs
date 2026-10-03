import { test } from "node:test";
import assert from "node:assert/strict";
import { capture, hasEventSections } from "./capture.mjs";

const page = (ids) =>
  ids
    .map((id) => `<section id="${id}"><h2>Event ${id}</h2></section>`)
    .join("");

test("reads the events page twice", async () => {
  const calls = [];
  const r = await capture(async (url, options) => {
    calls.push(url);
    assert.equal(options.redirect, "error");
    return new Response(page(["3475887", "3404899"]), {
      headers: { "content-type": "text/html" },
    });
  }, new Date("2026-10-03T18:00:00Z"));
  assert.deepEqual(calls, [
    "https://blackskydenver.com/denver-santa-fe-arts-district-black-sky-brewery-events",
    "https://blackskydenver.com/denver-santa-fe-arts-district-black-sky-brewery-events",
  ]);
  assert.equal(r.snapshot.from, "2026-10-03");
  assert.equal(r.snapshot.through, "2027-10-03");
  assert.ok(hasEventSections(r.snapshot.page));
  assert.equal(r.snapshot.page, r.snapshot.check);
});

test("a page without event sections fails", () => {
  assert.ok(!hasEventSections("<html><body></body></html>"));
});

test("missing structure fails the capture", async () => {
  await assert.rejects(
    capture(
      async () =>
        new Response("<html></html>", {
          headers: { "content-type": "text/html" },
        }),
    ),
    /Missing SpotHopper events page/,
  );
});

for (const [name, response] of [
  ["HTTP", () => new Response("denied", { status: 503 })],
  ["type", () => new Response(page(["1"]))],
  [
    "size",
    () =>
      new Response("x".repeat(2 * 1024 * 1024 + 1), {
        headers: { "content-type": "text/html" },
      }),
  ],
])
  test(`${name} failure closes the capture`, async () => {
    await assert.rejects(
      capture(async () => response(), new Date("2026-10-03T18:00:00Z")),
    );
  });
