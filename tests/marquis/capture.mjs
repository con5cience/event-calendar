import { captureProfile } from "../capture-profile.mjs";
// Operator-only capture through the venue's normal public scroll flow.
import { chromium } from "@playwright/test";
import { isDeepStrictEqual } from "node:util";
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export function venueProfile(source) {
  const p = captureProfile(source);
  if (p.adapter !== "livenation-venue-events") throw Error("Unsupported venue");
  return { endpoint: p.endpoint, page: p.page };
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
      throw new Error(
        `Incomplete pages: page_index=${i} page_sizes=${JSON.stringify(pages.map((page) => (Array.isArray(page) ? page.length : null)))}`,
      );
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

// The venue scroll flow stops once a partial page arrives, without
// requesting the empty page that used to mark the end of enumeration.
// Return the offset the flow previously requested next, or null when the
// pages already show a completed enumeration.
export function completionOffset(pages) {
  const offsets = [...pages.keys()].sort((a, b) => a - b);
  if (!offsets.length) return null;
  if ([...pages.values()].some((rows) => rows.length === 0)) return null;
  return offsets.at(-1) + 36;
}
export async function capture(source = "marquis") {
  const { endpoint, page: pageURL } = venueProfile(source);
  const browser = await chromium.launch();
  async function enumerate(pass) {
    const context = await browser.newContext();
    const page = await context.newPage();
    const pages = new Map();
    const pending = [];
    let failure;
    let flowHeaders;
    const absorb = async (offsetParam, limitParam, status, text, stage) => {
      console.error(
        JSON.stringify({
          source,
          pass,
          stage: `${stage} response`,
          offset: String(offsetParam).slice(0, 64),
          limit: String(limitParam).slice(0, 64),
          status,
        }),
      );
      const offset = Number(offsetParam);
      if (status < 200 || status > 299)
        throw new Error(`HTTP ${status}; capture aborted`);
      if (
        !Number.isInteger(offset) ||
        offset < 0 ||
        offset % 36 ||
        offset >= 36 * 32 ||
        String(limitParam) !== "36"
      )
        throw new Error("Unexpected pagination");
      if (Buffer.byteLength(text) > 4 * 1024 * 1024)
        throw new Error("Oversized response");
      const rows = JSON.parse(text);
      if (!Array.isArray(rows)) throw new Error("Expected array");
      console.error(
        JSON.stringify({
          source,
          pass,
          stage: `${stage} decoded`,
          offset,
          size: rows.length,
        }),
      );
      if (pages.has(offset) && !isDeepStrictEqual(pages.get(offset), rows))
        throw new Error("Page changed during enumeration");
      pages.set(offset, rows);
      return rows;
    };
    page.on("request", (request) => {
      const url = new URL(request.url());
      if (url.origin + url.pathname !== endpoint || request.method() !== "GET")
        return;
      flowHeaders = request.headers();
    });
    page.on("response", (response) => {
      const url = new URL(response.url());
      if (url.origin + url.pathname !== endpoint) return;
      pending.push(
        (async () => {
          const text = await response.text();
          await absorb(
            url.searchParams.get("offset"),
            url.searchParams.get("limit"),
            response.status(),
            text,
            "page",
          );
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
      let quiet = 0;
      for (let i = 0; i < 40; i++) {
        const before = pages.size;
        await page.waitForTimeout(1000);
        if (failure) throw failure;
        if ([...pages.values()].some((rows) => rows.length === 0)) break;
        await page.evaluate(() =>
          window.scrollTo(0, document.body.scrollHeight),
        );
        quiet = pages.size > before ? 0 : quiet + 1;
        const offsets = [...pages.keys()].sort((a, b) => a - b);
        const rows = offsets.length ? pages.get(offsets.at(-1)).length : 36;
        if (quiet >= 8 && rows > 0 && rows < 36) break;
      }
      await Promise.all(pending);
      if (failure) throw failure;
      const completion = completionOffset(pages);
      if (completion !== null) {
        // The scroll flow no longer requests the page past its final partial
        // page; complete enumeration with that same request, made from the
        // page itself with the flow's own headers, and require it to be empty.
        if (!flowHeaders) throw new Error("Missing venue request headers");
        const probe = await page.evaluate(
          async ({ endpoint, offset, headers }) => {
            const response = await fetch(
              `${endpoint}?offset=${offset}&limit=36`,
              { headers },
            );
            return { status: response.status, text: await response.text() };
          },
          { endpoint, offset: completion, headers: flowHeaders },
        );
        const rows = await absorb(
          completion,
          "36",
          probe.status,
          probe.text,
          "probe",
        );
        if (rows.length) throw new Error("Rows past the final page");
      }
      const ordered = [...pages].sort((a, b) => a[0] - b[0]);
      if (ordered.some(([offset], i) => offset !== i * 36))
        throw new Error("Page gap");
      const result = ordered.map((entry) => entry[1]);
      validatePages(result, result);
      return result;
    } catch (error) {
      console.error(
        JSON.stringify({
          source,
          pass,
          stage: "enumeration failed",
          pages: [...pages]
            .sort((a, b) => a[0] - b[0])
            .map(([offset, rows]) => ({ offset, size: rows.length })),
        }),
      );
      throw error;
    } finally {
      await context.close();
    }
  }
  try {
    const pages = await enumerate(1);
    const check = await enumerate(2);
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
  const source = process.argv[2];
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
