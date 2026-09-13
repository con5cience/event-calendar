import test from "node:test";
import assert from "node:assert/strict";
import { capture } from "./capture.mjs";

const rangeRows = [
  { id: "1", start: "2026-09-12T19:00:00-06:00" },
  { id: "2", start: "2027-09-02T19:00:00-06:00" },
];
test("captures a full year and contiguous populated windows with bounded GETs", async () => {
  const calls = [];
  const result = await capture(async (url, opts) => {
    calls.push({ url, opts });
    return new Response(JSON.stringify(rangeRows));
  }, new Date("2026-09-11T02:00:00Z"));
  assert.equal(result.snapshot.from, "2026-09-10");
  assert.equal(result.snapshot.through, "2027-09-10");
  assert.equal(result.snapshot.windows[0].start, "2026-09-10");
  assert.equal(result.snapshot.windows.at(-1).end, "2027-09-11");
  for (let i = 1; i < result.snapshot.windows.length; i++)
    assert.equal(
      result.snapshot.windows[i].start,
      result.snapshot.windows[i - 1].end,
    );
  assert.equal(calls.length, 4);
  assert.ok(calls[1].url.includes("start=2026-09-09"));
  assert.equal(result.snapshot.windows[0].end, "2027-09-02");
  assert.ok(
    calls.every(
      (c) =>
        c.url.startsWith("https://www.redrocksonline.com/wp-json/clique/v1/") &&
        c.opts.redirect === "error" &&
        c.opts.signal,
    ),
  );
});
for (const [name, response] of [
  ["HTTP failure", () => new Response("busy", { status: 429 })],
  ["non-array", () => new Response("{}")],
  ["cap", () => new Response(JSON.stringify(Array(10000).fill({})))],
  ["oversize", () => new Response(" ".repeat(4 * 1024 * 1024 + 1))],
]) {
  test(`rejects ${name} without a partial successful capture`, async () => {
    await assert.rejects(
      capture(async () => response(), new Date("2026-09-11T02:00:00Z")),
    );
  });
}
test("failed later range does not become an empty window", async () => {
  let i = 0;
  await assert.rejects(
    capture(
      async () =>
        ++i < 4
          ? new Response(JSON.stringify(rangeRows))
          : new Response("error", { status: 503 }),
      new Date("2026-09-11T02:00:00Z"),
    ),
  );
});
test("empty full-year arrays remain explicit and need no empty-month probes", async () => {
  let count = 0;
  const r = await capture(async () => {
    count++;
    return new Response("[]");
  }, new Date("2026-09-11T02:00:00Z"));
  assert.equal(count, 2);
  assert.deepEqual(r.snapshot.windows, [
    { start: "2026-09-10", end: "2027-09-11", events: [] },
  ]);
});
