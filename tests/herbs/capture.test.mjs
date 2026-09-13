import { test } from "node:test";
import assert from "node:assert/strict";
import { capture } from "./capture.mjs";
test("reads only the ordinary calendar twice", async () => {
  const calls = [];
  const r = await capture(async (url, options) => {
    calls.push(url);
    assert.equal(options.redirect, "error");
    return new Response("<html>calendar</html>", {
      headers: { "content-type": "text/html" },
    });
  }, new Date("2026-09-12T18:00:00Z"));
  assert.deepEqual(calls, [
    "https://www.herbsbar.com/live-music-calendar-1",
    "https://www.herbsbar.com/live-music-calendar-1",
  ]);
  assert.equal(r.snapshot.from, "2026-09-12");
  assert.equal(r.snapshot.through, "2027-09-12");
  assert.equal(r.snapshot.html, r.snapshot.check_html);
});
for (const [name, response] of [
  ["HTTP", () => new Response("denied", { status: 403 })],
  ["type", () => new Response("{}")],
  [
    "size",
    () =>
      new Response("x".repeat(1024 * 1024 + 1), {
        headers: { "content-type": "text/html" },
      }),
  ],
])
  test(name + " fails", async () => {
    await assert.rejects(capture(async () => response()));
  });
