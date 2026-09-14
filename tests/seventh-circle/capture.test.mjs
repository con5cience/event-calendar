import { test } from "node:test";
import assert from "node:assert/strict";
import { capture } from "./capture.mjs";

const profile = {
  page: "https://venue.example/posts/",
  policy: "https://venue.example/home/about",
  timezone: "America/Denver",
};
const list =
  '<div id="posts"><a class="post-link" href="/posts/12"></a><a class="post-link" href="/posts/13"></a></div>';
const now = new Date("2026-09-14T18:00:00Z");
test("paired bounded HTTP capture discovers only same-site post details", async () => {
  const urls = [];
  const result = await capture(
    async (url, options) => {
      urls.push(url);
      assert.equal(options.redirect, "error");
      assert.ok(options.signal);
      return new Response(url === profile.page ? list : "<p>Fixture</p>", {
        headers: { "content-type": "text/html" },
      });
    },
    now,
    profile,
  );
  assert.equal(result.snapshot.from, "2026-09-14");
  assert.equal(result.snapshot.through, "2027-09-14");
  assert.deepEqual(result.snapshot.pages, result.snapshot.check);
  assert.deepEqual(Object.keys(result.snapshot.pages.details), ["12", "13"]);
  assert.equal(urls.length, 8);
});
test("missing details stay explicit; listing and policy failures abort", async () => {
  const fetcher = async (url) => {
    if (url.endsWith("/12")) throw Error("unavailable");
    return new Response(url === profile.page ? list : "<p>Fixture</p>", {
      headers: { "content-type": "text/html" },
    });
  };
  const result = await capture(fetcher, now, profile);
  assert.equal(result.snapshot.pages.details["12"], "");
  await assert.rejects(
    capture(async () => new Response("bad", { status: 500 }), now, profile),
  );
});
test("unknown empty listing and oversized documents fail safely", async () => {
  for (const html of ["<div>No events</div>", "x".repeat(1024 * 1024 + 1)]) {
    await assert.rejects(
      capture(
        async () =>
          new Response(html, { headers: { "content-type": "text/html" } }),
        now,
        profile,
      ),
    );
  }
});
