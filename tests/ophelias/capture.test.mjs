import { test } from "node:test";
import assert from "node:assert/strict";
import { capture } from "./capture.mjs";

test("two read-only bounded calendar requests", async () => {
  const calls = [];
  const result = await capture(async (url, options) => {
    calls.push({ url, options });
    return new Response("<html>calendar</html>", {
      headers: { "content-type": "text/html" },
    });
  }, new Date("2026-09-12T18:00:00Z"));
  assert.equal(calls.length, 2);
  assert.equal(calls[0].url, "https://opheliasdenver.com/calendar/");
  assert.equal(calls[0].options.redirect, "error");
  assert.equal(result.snapshot.from, "2026-09-12");
  assert.equal(result.snapshot.through, "2027-09-12");
  assert.equal(result.snapshot.html, result.snapshot.check_html);
});
for (const [name, response] of [
  ["HTTP", () => new Response("denied", { status: 403 })],
  [
    "JSON",
    () =>
      new Response("{}", { headers: { "content-type": "application/json" } }),
  ],
  [
    "oversize",
    () =>
      new Response("x".repeat(4 * 1024 * 1024), {
        headers: { "content-type": "text/html" },
      }),
  ],
])
  test(name + " fails", async () => {
    await assert.rejects(capture(async () => response()));
  });
