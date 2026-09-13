import { test } from "node:test";
import assert from "node:assert/strict";
import { capture, enumerate } from "./capture.mjs";
const event = (id) => ({
  event_type: "external_event",
  event_id: id,
  event_name: "Fixture",
  start_time: "2026-09-20 19:00:00",
  buy_ticket_url: "https://tickets.strongsurvivepresents.com/tickets/464607",
});
const page = (n, total = 13) => ({
  current_page: n,
  per_page: 12,
  total_records: total,
  next_page_url:
    n * 12 < total
      ? "https://aftontickets.com/api/get-events?page=" + (n + 1)
      : null,
  data: Array.from({ length: Math.min(12, total - (n - 1) * 12) }, (_, i) =>
    event((n - 1) * 12 + i + 1),
  ),
});
test("enumerates every page with exact counts", async () => {
  const r = await enumerate(async (n) => page(n));
  assert.equal(r.length, 2);
  assert.equal(r[1].events.length, 1);
});
for (const kind of [
  "short",
  "duplicate",
  "total",
  "page",
  "next",
  "empty",
  "cap",
]) {
  test("rejects " + kind, async () => {
    await assert.rejects(
      enumerate(async (n) => {
        const p = page(n);
        if (kind === "short") p.data.pop();
        if (kind === "duplicate") p.data[1] = p.data[0];
        if (kind === "total" && n === 2) p.total_records++;
        if (kind === "page") p.current_page++;
        if (kind === "next") p.next_page_url = null;
        if (kind === "empty") return page(1, 0);
        if (kind === "cap") p.total_records = 501;
        return p;
      }),
    );
  });
}
test("anonymous canonical GETs; external pages are not fetched; prices are omitted", async () => {
  const calls = [];
  const r = await capture(async (u, o) => {
    calls.push(u);
    assert.equal(o.redirect, "error");
    assert.equal(o.headers, undefined);
    const p = page(Number(new URL(u).searchParams.get("page")));
    p.data[0].min_price = 30;
    return Response.json(p);
  }, new Date("2026-09-12T18:00:00Z"));
  assert.equal(calls.length, 4);
  assert.ok(
    calls.every((u) =>
      u.startsWith("https://aftontickets.com/api/get-events?key="),
    ),
  );
  assert.equal(r.snapshot.from, "2026-09-12");
  assert.equal(r.snapshot.through, "2027-09-12");
  assert.ok(!JSON.stringify(r.snapshot).includes("min_price"));
  assert.deepEqual(r.snapshot.details, {});
});
test("HTTP failure is fatal", async () => {
  await assert.rejects(
    capture(async () => new Response("denied", { status: 403 })),
    /Afton HTTP or type failure.*pass=1.*page=1.*status=403.*content_type="text\/plain;charset=UTF-8"/,
  );
});
test("wrong content type reports its page and pass without logging the body or widget key", async () => {
  let calls = 0;
  await assert.rejects(
    capture(async () => {
      calls++;
      if (calls === 3)
        return new Response("private diagnostic body", {
          headers: { "content-type": "text/html" },
        });
      return Response.json(page(calls));
    }),
    (error) => {
      assert.match(
        error.message,
        /pass=2.*page=1.*status=200.*content_type="text\/html"/,
      );
      assert(!error.message.includes("private diagnostic body"));
      assert(!error.message.includes("key="));
      return true;
    },
  );
});
test("native details are bounded and only exact Afton event URLs are fetched", async () => {
  const e = {
    ...event(1),
    event_type: "real_world",
    event_id: "3px8g401j1",
    buy_ticket_url: "https://aftontickets.com/event/buyticket/3px8g401j1",
  };
  let details = 0;
  const fetcher = async (u) => {
    if (u === e.buy_ticket_url) {
      details++;
      return new Response(
        '<script>dynamic()</script><span class="modal-event-info-value">All Ages</span>',
        { headers: { "content-type": "text/html" } },
      );
    }
    return Response.json({ ...page(1, 1), data: [e] });
  };
  const r = await capture(fetcher);
  assert.equal(details, 2);
  assert.ok(r.snapshot.details[e.event_id].includes("All Ages"));
  assert.ok(!r.snapshot.details[e.event_id].includes("dynamic()"));
  e.buy_ticket_url = "https://evil.example/event";
  await assert.rejects(
    capture(async () => Response.json({ ...page(1, 1), data: [e] })),
    /URL/,
  );
});
