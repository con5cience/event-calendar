import { test } from "node:test";
import assert from "node:assert/strict";
import { validatePages, venueProfile } from "./capture.mjs";

test("only reviewed venue capture profiles are supported", () => {
  assert.equal(
    venueProfile("fillmore").endpoint,
    "https://content.livenationapi.com/v1/venues/KovZpZAE6eJA/events",
  );
  assert.equal(
    venueProfile("fillmore").page,
    "https://www.fillmoredenver.com/shows",
  );
  assert.equal(
    venueProfile("summit").endpoint,
    "https://content.livenationapi.com/v1/venues/KovZpZAFFt1A/events",
  );
  assert.equal(
    venueProfile("summit").page,
    "https://www.summitdenver.com/shows",
  );
  assert.equal(
    venueProfile("marquis").page,
    "https://www.marquisdenver.com/shows",
  );
  assert.throws(() => venueProfile("elsewhere"));
});

const row = (id) => ({ tm_id: id, name: `Event ${id}` });
const full = () => Array.from({ length: 36 }, (_, i) => row(String(i)));
test("complete browser pages retain primary records and terminal empty page", () => {
  const pages = [full(), [row("last")], []];
  assert.equal(validatePages(pages, structuredClone(pages)).pages.length, 3);
});
for (const kind of [
  "missing",
  "changed",
  "duplicate",
  "gap",
  "oversized",
  "identity",
  "null",
]) {
  test(`rejects ${kind}`, () => {
    let pages = [full(), [row("last")], []];
    if (kind === "missing") pages.pop();
    if (kind === "duplicate") pages[1] = [row("0")];
    if (kind === "gap") pages = [[row("first")], [row("second")], []];
    if (kind === "oversized") pages[0].push(row("extra"));
    if (kind === "identity") pages[0][0] = {};
    if (kind === "null") pages[0][0] = null;
    const check = structuredClone(pages);
    if (kind === "changed") check[0][0].name = "Changed";
    assert.throws(() => validatePages(pages, check));
  });
}
test("empty requires matching successful enumerations", () => {
  assert.deepEqual(validatePages([[]], [[]]), { pages: [[]], check: [[]] });
  assert.throws(() => validatePages([], []));
});
test("incomplete pagination reports the observed page sizes without relaxing validation", () => {
  const pages = [full(), [row("last")]];
  assert.throws(
    () => validatePages(pages, structuredClone(pages)),
    /Incomplete pages.*page_index=1.*page_sizes=\[36,1\]/,
  );
});
