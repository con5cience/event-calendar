import { test } from "node:test";
import assert from "node:assert/strict";
import { enumerate, validateSnapshot, queryPage } from "./capture.mjs";

const response = (page, total, rows) => ({
  data: {
    paginatedEvents: {
      collection: rows,
      metadata: {
        currentPage: page,
        limitValue: 5,
        totalCount: total,
        totalPages: Math.ceil(total / 5),
      },
    },
  },
});
const rows = Array.from({ length: 6 }, (_, i) => ({
  id: i + 1,
  name: "Event",
}));
test("enumerates exact pages and accepts confirmed empty", async () => {
  assert.deepEqual(
    await enumerate(async (p) =>
      response(p, 6, p === 1 ? rows.slice(0, 5) : rows.slice(5)),
    ),
    rows,
  );
  assert.deepEqual(await enumerate(async (p) => response(p, 0, [])), []);
});
for (const kind of [
  "errors",
  "count",
  "page",
  "short",
  "duplicate",
  "missing",
  "cap",
]) {
  test(`rejects ${kind}`, async () => {
    await assert.rejects(
      enumerate(async (p) => {
        const r = response(p, 6, p === 1 ? rows.slice(0, 5) : rows.slice(5));
        const d = r.data.paginatedEvents;
        if (kind === "errors") r.errors = [{ message: "partial result" }];
        if (kind === "count" && p === 2) d.metadata.totalCount = 7;
        if (kind === "page") d.metadata.currentPage = 2;
        if (kind === "short") d.collection.pop();
        if (kind === "duplicate") d.collection[1] = d.collection[0];
        if (kind === "missing") delete d.metadata;
        if (kind === "cap") d.metadata.totalCount = 501;
        return r;
      }),
    );
  });
}
test("paired capture must match and fit size limit", () => {
  assert.equal(validateSnapshot(rows, structuredClone(rows)).total, 6);
  assert.throws(() => validateSnapshot(rows, []));
  assert.throws(() =>
    validateSnapshot(
      [{ id: 1, name: "x".repeat(4 * 1024 * 1024) }],
      [{ id: 1, name: "x".repeat(4 * 1024 * 1024) }],
    ),
  );
});
test("request is scoped, bounded, unauthenticated, and rejects HTTP failures", async () => {
  await queryPage("2026-09-11", "2027-09-11", 1, async (url, opts) => {
    assert.equal(url, "https://www.venuepilot.co/graphql");
    assert.deepEqual(JSON.parse(opts.body).variables, {
      accountIds: [1105],
      startDate: "2026-09-11",
      endDate: "2027-09-11",
      limit: 5,
      page: 1,
    });
    assert.deepEqual(opts.headers, { "content-type": "application/json" });
    assert.equal(opts.redirect, "error");
    return new Response(JSON.stringify(response(1, 0, [])));
  });
  await assert.rejects(
    queryPage(
      "2026-09-11",
      "2027-09-11",
      1,
      async () => new Response("Denied", { status: 403 }),
    ),
  );
});
