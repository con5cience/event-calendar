// Operator-only capture through the venue's normal public scroll flow.
import { chromium } from "@playwright/test";
import { isDeepStrictEqual } from "node:util";
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export function venueProfile(source) {
  const id =
    source === "marquis"
      ? "KovZpZAJeFkA"
      : source === "summit"
        ? "KovZpZAFFt1A"
        : source === "fillmore"
          ? "KovZpZAE6eJA"
          : null;
  if (!id) throw new Error("Unsupported venue");
  return {
    endpoint: `https://content.livenationapi.com/v1/venues/${id}/events`,
    page: `https://www.${source}denver.com/shows`,
  };
}
export function validatePages(pages, check) {
  if (!isDeepStrictEqual(pages, check)) throw new Error("Capture changed");
  if (!Array.isArray(pages) || !pages.length || pages.length > 32)
    throw new Error("Missing or capped pages");
  const seen = new Set();
  for (const [i, rows] of pages.entries()) {
    if (
      !Array.isArray(rows) ||
      rows.length > 36 ||
      (i === pages.length - 1 ? rows.length !== 0 : rows.length === 0) ||
      (i < pages.length - 2 && rows.length !== 36)
    )
      throw new Error("Incomplete pages");
    for (const e of rows) {
      if (!e || typeof e.tm_id !== "string" || !e.tm_id || seen.has(e.tm_id))
        throw new Error("Invalid or duplicate identity");
      seen.add(e.tm_id);
    }
  }
  const result = { pages, check };
  if (Buffer.byteLength(JSON.stringify(result)) > 4 * 1024 * 1024)
    throw new Error("Snapshot exceeds 4 MiB");
  return result;
}

export async function capture(source = "marquis") {
  const { endpoint, page: pageURL } = venueProfile(source);
  const browser = await chromium.launch();
  async function enumerate() {
    const context = await browser.newContext();
    const page = await context.newPage();
    const pages = new Map();
    const pending = [];
    let failure;
    page.on("response", (response) => {
      const url = new URL(response.url());
      if (url.origin + url.pathname !== endpoint) return;
      pending.push(
        (async () => {
          if (!response.ok())
            throw new Error(`HTTP ${response.status()}; capture aborted`);
          const offset = Number(url.searchParams.get("offset"));
          if (
            !url.searchParams.has("offset") ||
            !Number.isInteger(offset) ||
            offset < 0 ||
            offset % 36 ||
            offset >= 36 * 32 ||
            url.searchParams.get("limit") !== "36"
          )
            throw new Error("Unexpected pagination");
          const text = await response.text();
          if (Buffer.byteLength(text) > 4 * 1024 * 1024)
            throw new Error("Oversized response");
          const rows = JSON.parse(text);
          if (!Array.isArray(rows)) throw new Error("Expected array");
          if (pages.has(offset) && !isDeepStrictEqual(pages.get(offset), rows))
            throw new Error("Page changed during enumeration");
          pages.set(offset, rows);
        })().catch((error) => {
          failure = error;
        }),
      );
    });
    try {
      await page.goto(pageURL, {
        waitUntil: "domcontentloaded",
        timeout: 30000,
      });
      for (let i = 0; i < 40; i++) {
        await page.waitForTimeout(1000);
        if (failure) throw failure;
        if ([...pages.values()].some((rows) => rows.length === 0)) break;
        await page.evaluate(() =>
          window.scrollTo(0, document.body.scrollHeight),
        );
      }
      await Promise.all(pending);
      if (failure) throw failure;
      const ordered = [...pages].sort((a, b) => a[0] - b[0]);
      if (ordered.some(([offset], i) => offset !== i * 36))
        throw new Error("Page gap");
      const result = ordered.map((entry) => entry[1]);
      validatePages(result, result);
      return result;
    } finally {
      await context.close();
    }
  }
  try {
    const pages = await enumerate();
    const check = await enumerate();
    return {
      snapshot: validatePages(pages, check),
      captured_at: new Date().toISOString(),
    };
  } finally {
    await browser.close();
  }
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length > 3)
    throw new Error(
      "Usage: node tests/marquis/capture.mjs [marquis|summit|fillmore]",
    );
  const source = process.argv[2] ?? "marquis";
  const { endpoint } = venueProfile(source);
  const result = await capture(source);
  const directory = mkdtempSync(join(tmpdir(), `event-calendar-${source}-`));
  chmodSync(directory, 0o755);
  writeFileSync(
    join(directory, "snapshot.json"),
    JSON.stringify(result.snapshot),
    { flag: "wx" },
  );
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      source,
      captured_at: result.captured_at,
      endpoint,
    }),
    { flag: "wx" },
  );
  console.log(JSON.stringify({ directory, captured_at: result.captured_at }));
}
