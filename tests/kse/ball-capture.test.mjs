import { test } from "node:test";
import assert from "node:assert/strict";
import { captureBall } from "./ball-capture.mjs";

test("captures and rechecks both official interfaces", async () => {
  const calls = [];
  const result = await captureBall(async (url, opts) => {
    calls.push(url);
    assert.equal(opts.redirect, "error");
    assert.ok(opts.signal);
    return new Response(
      url.includes("alttix")
        ? '[{"title":"Test","url":"https://www.ticketmaster.com/event/123","start":"2026-09-12"}]'
        : "<html>listing</html>",
    );
  }, new Date("2026-09-11T20:00:00Z"));
  assert.equal(calls.length, 4);
  assert.equal(calls[0], calls[2]);
  assert.equal(calls[1], calls[3]);
  assert.deepEqual(result.snapshot.events, result.snapshot.check);
  assert.equal(result.snapshot.listing, result.snapshot.listing_check);
});
for (const kind of ["http", "shape", "changed", "oversized"]) {
  test(`rejects ${kind}`, async () => {
    let i = 0;
    await assert.rejects(
      captureBall(async (url) => {
        i++;
        if (kind === "http") return new Response("blocked", { status: 403 });
        if (kind === "oversized")
          return new Response("x".repeat(4 * 1024 * 1024 + 1));
        return new Response(
          url.includes("alttix")
            ? kind === "shape"
              ? "{}"
              : JSON.stringify([
                  { title: kind === "changed" ? String(i) : "Test" },
                ])
            : "listing",
        );
      }),
    );
  });
}
