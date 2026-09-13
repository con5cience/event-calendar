import test from "node:test";
import assert from "node:assert/strict";
import { capture } from "./capture.mjs";

const first = Array.from({ length: 5 }, (_, i) => ({ id: i + 1, maxPages: 2 }));
test("enumerates pages, verifies an empty terminal and repeats the first page", async () => {
  const calls = [];
  const responses = [first, [{ id: 6, maxPages: 2 }], [], first];
  const r = await capture(async (url, options) => {
    calls.push({ url, options });
    return new Response(JSON.stringify(responses.shift()));
  });
  assert.deepEqual(
    calls.map((c) => new URL(c.url).searchParams.get("currentpage")),
    ["1", "2", "3", "1"],
  );
  assert.ok(
    calls.every((c) => c.options.redirect === "error" && c.options.signal),
  );
  assert.equal(r.snapshot.pages.length, 2);
  assert.deepEqual(r.snapshot.first_check, first);
});
for (const [name, responses] of [
  ["missing page", [first, []]],
  ["changed metadata", [first, [{ id: 6, maxPages: 3 }]]],
  ["duplicate identity", [first, [{ id: 1, maxPages: 2 }]]],
  [
    "nonempty terminal",
    [first, [{ id: 6, maxPages: 2 }], [{ id: 7, maxPages: 2 }]],
  ],
  ["changed first page", [first, [{ id: 6, maxPages: 2 }], [], []]],
  ["wrong envelope", [{}]],
  ["page cap", [[{ id: 1, maxPages: 201 }]]],
])
  test(`rejects ${name}`, async () => {
    const queue = [...responses];
    await assert.rejects(
      capture(async () => new Response(JSON.stringify(queue.shift()))),
    );
  });
test("HTTP failure is not an empty calendar", async () => {
  await assert.rejects(
    capture(async () => new Response("blocked", { status: 403 })),
  );
});
test("empty capture must be confirmed", async () => {
  let requests = 0;
  const r = await capture(async () => {
    requests++;
    return new Response("[]");
  });
  assert.equal(requests, 3);
  assert.deepEqual(r.snapshot.pages, [[]]);
});
