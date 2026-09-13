import test from "node:test";
import assert from "node:assert/strict";
import { capture } from "./capture.mjs";
test("full venue array is rechecked without invented pagination", async () => {
  const calls = [];
  const rows = [{ id: "one" }];
  const r = await capture(async (url, opts) => {
    calls.push({ url, opts });
    return new Response(JSON.stringify(rows));
  });
  assert.equal(calls.length, 2);
  assert.equal(calls[0].url, calls[1].url);
  assert.ok(calls.every((c) => c.opts.signal && c.opts.redirect === "error"));
  assert.deepEqual(r.snapshot.events, rows);
});
for (const [name, rows] of [
  ["object", {}],
  ["duplicate", [{ id: "one" }, { id: "one" }]],
  ["invalid identity", [{}]],
  ["cap", Array.from({ length: 1000 }, (_, i) => ({ id: String(i) }))],
])
  test(`rejects ${name}`, async () => {
    await assert.rejects(
      capture(async () => new Response(JSON.stringify(rows))),
    );
  });
test("rejects changing data", async () => {
  let n = 0;
  await assert.rejects(
    capture(async () => new Response(JSON.stringify([{ id: String(++n) }]))),
  );
});
test("HTTP errors are not empty", async () => {
  await assert.rejects(
    capture(async () => new Response("blocked", { status: 403 })),
  );
});
test("empty needs two successful arrays", async () => {
  let n = 0;
  const r = await capture(async () => {
    n++;
    return new Response("[]");
  });
  assert.equal(n, 2);
  assert.deepEqual(r.snapshot.events, []);
});
