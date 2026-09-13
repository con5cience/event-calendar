import assert from "node:assert/strict";
import { test } from "node:test";
import { calendarForm, compactHTML, capture } from "./capture.mjs";

test("Cervantes capture uses its official RHP endpoint", async () => {
  const result = await capture("cervantes", async (url) => {
    assert.equal(
      url,
      "https://cervantesmasterpiece.com/wp-admin/admin-ajax.php",
    );
    return Response.json({ success: true, data: { events: [] } });
  });
  assert.deepEqual(result.snapshot.calendar.data.events, []);
});

test("public calendar form and compacted evidence", () => {
  assert.equal(calendarForm().get("action"), "loadEtixMonthViewEventPageFn");
  assert.equal(calendarForm().get("data[limit]"), "10000");
  assert.equal(calendarForm().get("data[archive]"), "true");
  const html =
    '<style>body{color:red}</style><script>unsafe()</script><script type="application/ld+json">{"@type":"Event"}</script><div class="eventAgeRestriction">Ages 16 and up</div><li>Ticketed guardian 21+</li>';
  const compact = compactHTML(html);
  assert.ok(!compact.includes("unsafe()"));
  assert.ok(compact.includes('"@type":"Event"'));
  assert.ok(compact.includes("Ages 16 and up"));
  assert.ok(compact.includes("Ticketed guardian 21+"));
});

test("compaction does not alter JSON-LD string values", () => {
  const json =
    '<script type="application/ld+json">{"name":"A  Band", "description":"line\\nnext"}</script>';
  assert.equal(compactHTML(json), json);
});

test("compaction strips an executable script at the beginning of a fragment", () => {
  assert.equal(
    compactHTML("<script>dynamic()</script><p>Evidence</p>"),
    "<p>Evidence</p>",
  );
});

test("capture rejects failed envelopes before detail requests", async () => {
  let calls = 0;
  await assert.rejects(
    capture("lost-lake", async () => {
      calls++;
      return new Response('{"success":false}');
    }),
    /calendar/,
  );
  assert.equal(calls, 1);
});

test("capture cannot fetch an off-origin event URL", async () => {
  let calls = 0;
  await assert.rejects(
    capture("lost-lake", async () => {
      calls++;
      return Response.json({
        success: true,
        data: { events: [{ url: "https://evil.example/event/test/" }] },
      });
    }),
    /URL/,
  );
  assert.equal(calls, 1);
});

test("capture preserves envelope, raw pages and missing detail failures", async () => {
  const origin = "https://lost-lake.com";
  const u = origin + "/event/test/";
  const result = await capture("lost-lake", async (url, options) => {
    assert.equal(options.redirect, "error");
    if (url.endsWith("admin-ajax.php"))
      return Response.json({ success: true, data: { events: [{ url: u }] } });
    assert.equal(url, u);
    return new Response("unavailable", { status: 503 });
  });
  assert.equal(result.snapshot.calendar.data.events.length, 1);
  assert.deepEqual(result.snapshot.details, {});
  assert.equal(result.failures.length, 1);
});
