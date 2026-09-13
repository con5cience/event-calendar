import { test } from "node:test";
import assert from "node:assert/strict";
import {
  enumerate,
  validateSnapshot,
  externalTicketRedirect,
  readTicket,
} from "./capture.mjs";

test("ticket fetch never follows external redirects", async () => {
  const r = await readTicket(
    "https://events.blackboxdenver.co/e/test/tickets",
    async (_, opts) => {
      assert.equal(opts.redirect, "manual");
      return new Response(null, {
        status: 302,
        headers: { location: "https://dice.fm/example" },
      });
    },
  );
  assert.deepEqual(r, { ticket_error: "external ticket provider" });
});
test("ticket HTTP failures abort instead of becoming empty policy", async () => {
  await assert.rejects(
    readTicket(
      "https://events.blackboxdenver.co/e/test/tickets",
      async () => new Response("blocked", { status: 403 }),
    ),
  );
});

test("external ticket redirects are identified without fetching the destination", () => {
  assert.equal(
    externalTicketRedirect("https://dice.fm/partner/tickets/event/test"),
    true,
  );
  assert.equal(externalTicketRedirect("/e/test/tickets"), false);
  assert.equal(externalTicketRedirect(null), false);
});

const row = (i) => ({ id: String(i), date: "2026-09-12" });
test("counted pagination stops at the declared total", async () => {
  const calls = [];
  const result = await enumerate(async (offset) => {
    calls.push(offset);
    const rows = Array.from({ length: offset === 0 ? 25 : 2 }, (_, i) =>
      row(offset + i),
    );
    return { rows, range: offset === 0 ? "0-24/27" : "25-26/27" };
  });
  assert.equal(result.length, 27);
  assert.deepEqual(calls, [0, 25]);
});
for (const kind of [
  "unknown total",
  "gap",
  "changed total",
  "duplicate",
  "short",
  "oversized",
]) {
  test("rejects " + kind, async () => {
    await assert.rejects(
      enumerate(async (offset) => {
        const rows = Array.from({ length: offset === 0 ? 25 : 2 }, (_, i) =>
          row(offset + i),
        );
        let range = offset === 0 ? "0-24/27" : "25-26/27";
        if (kind === "unknown total") range = "0-24/*";
        if (kind === "gap") range = "1-25/27";
        if (kind === "changed total" && offset) range = "25-26/28";
        if (kind === "duplicate" && offset) rows[0] = row(0);
        if (kind === "short") rows.pop();
        if (kind === "oversized") range = "0-24/10001";
        return { rows, range };
      }),
    );
  });
}
test("empty exact count is valid", async () =>
  assert.deepEqual(
    await enumerate(async () => ({ rows: [], range: "*/0" })),
    [],
  ));
test("snapshot requires identical event data and valid identity", () => {
  const events = [row(1)];
  assert.equal(validateSnapshot(events, structuredClone(events)).total, 1);
  assert.throws(() => validateSnapshot(events, [row(2)]));
  assert.throws(() => validateSnapshot([row(1), row(1)], [row(1), row(1)]));
});
