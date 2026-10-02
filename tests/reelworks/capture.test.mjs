import { test } from "node:test";
import assert from "node:assert/strict";
import { capture, listingSlugs, eventPage } from "./capture.mjs";

const listing = (slugs) =>
  slugs
    .map(
      (slug) =>
        `<div class="event-item"><a href="https://reelworksdenver.com/event/${slug}/" class="event-image"></a><a href="https://reelworksdenver.com/event/${slug}/"><h3>Friday, Oct 2, 2026</h3><h1>Test</h1></a></div>`,
    )
    .join("");
const page = (age) =>
  `<html><script type="application/ld+json" class="yoast-schema-graph">{"@context":"https://schema.org","@graph":[{"@type":"WebPage"}]}</script><div class="sidebar"><div class="sidebar-content"><h3>Venue</h3> <a>ReelWorks</a> </div><div class="divider"></div><div class="sidebar-content"><h3>Date</h3> Friday, Oct 2, 2026 - Saturday, Oct 3, 2026 <h3>Time</h3> 9:00 pm - 2:00 am </div><div class="divider"></div><div class="sidebar-content"><h3>Age</h3> ${age} </div></div></html>`;
const faq = `<main>Is the Show 18+? That depends on the promoter putting on the show.</main>`;

test("reads the listing, FAQ, and every event page twice", async () => {
  const calls = [];
  const r = await capture(async (url) => {
    calls.push(url);
    const body = url.endsWith("/events/")
      ? listing(["deorro-3", "bassvictim"])
      : url.endsWith("/faq/")
        ? faq
        : page(
            url.includes("deorro") ? "18+" : "16+ (Under 16 w/ adult guardian)",
          );
    return new Response(body, { headers: { "content-type": "text/html" } });
  }, new Date("2026-10-02T18:00:00Z"));
  assert.deepEqual(calls, [
    "https://reelworksdenver.com/events/",
    "https://reelworksdenver.com/faq/",
    "https://reelworksdenver.com/event/deorro-3/",
    "https://reelworksdenver.com/event/bassvictim/",
    "https://reelworksdenver.com/events/",
    "https://reelworksdenver.com/faq/",
    "https://reelworksdenver.com/event/deorro-3/",
    "https://reelworksdenver.com/event/bassvictim/",
  ]);
  assert.equal(r.snapshot.from, "2026-10-02");
  assert.equal(r.snapshot.through, "2027-10-02");
  assert.deepEqual(r.snapshot.pages, r.snapshot.check);
  assert.equal(
    r.snapshot.pages.events["bassvictim"].sidebar.age,
    "16+ (Under 16 w/ adult guardian)",
  );
});

test("a missing age sidebar stays absent for Go to reject per record", () => {
  const extracted = eventPage(page(""));
  assert.equal(extracted.sidebar.age, "");
  assert.equal(
    extracted.sidebar.date,
    "Friday, Oct 2, 2026 - Saturday, Oct 3, 2026",
  );
  assert.equal(extracted.sidebar.time, "9:00 pm - 2:00 am");
  assert.ok(extracted.jsonld.startsWith("{"));
});

for (const [name, listingHTML] of [
  ["structure", "<html></html>"],
  ["empty", listing([])],
  ["capped", listing(Array.from({ length: 201 }, (_, i) => `event-${i}`))],
])
  test(`listing ${name} fails`, () => {
    assert.throws(() => listingSlugs(listingHTML));
  });

for (const [name, html] of [
  [
    "schema",
    `<html><script type="application/ld+json"></script><script type="application/ld+json"></script></html>`,
  ],
  ["sidebar", `<html><script type="application/ld+json">{}</script></html>`],
])
  test(`event page ${name} fails`, () => {
    assert.throws(() => eventPage(html));
  });

for (const [name, response] of [
  ["HTTP", () => new Response("denied", { status: 403 })],
  ["type", () => new Response(listing(["deorro-3"]))],
])
  test(`${name} failure closes the capture`, async () => {
    await assert.rejects(
      capture(async () => response(), new Date("2026-10-02T18:00:00Z")),
    );
  });
