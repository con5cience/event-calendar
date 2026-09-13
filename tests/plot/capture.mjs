import { captureProfile } from "../capture-profile.mjs";
const captureSettings = captureProfile("hi-dive");
// Explicit operator capture; never runs during app startup or builds.
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { isDeepStrictEqual } from "node:util";

export async function capture(fetcher = fetch, now = new Date()) {
  const raw = [];
  const pageSize = 5;
  async function request(page) {
    const url = `${captureSettings.endpoint}?currentpage=${page}&listingsPerPage=${pageSize}`;
    const response = await fetcher(url, {
      redirect: "error",
      signal: AbortSignal.timeout(30000),
    });
    if (!response.ok)
      throw new Error(`HTTP ${response.status}; capture aborted`);
    const text = await response.text();
    if (Buffer.byteLength(text) > 4 * 1024 * 1024)
      throw new Error("Response exceeds 4 MiB");
    const rows = JSON.parse(text);
    if (!Array.isArray(rows) || rows.length > pageSize)
      throw new Error("Invalid page envelope");
    raw.push({ url, text });
    return rows;
  }
  const first = await request(1);
  const maxPages = first.length ? first[0].maxPages : 1;
  if (!Number.isInteger(maxPages) || maxPages < 1 || maxPages > 200)
    throw new Error("Invalid page count");
  const pages = [first];
  const seen = new Set();
  function validate(rows, page) {
    if (
      (page < maxPages && rows.length !== pageSize) ||
      (maxPages > 1 && !rows.length)
    )
      throw new Error("Missing page records");
    for (const row of rows) {
      if (
        !Number.isSafeInteger(row.id) ||
        row.id < 1 ||
        seen.has(row.id) ||
        row.maxPages !== maxPages
      )
        throw new Error("Invalid identity or changing pagination");
      seen.add(row.id);
    }
  }
  validate(first, 1);
  for (let page = 2; page <= maxPages; page++) {
    const rows = await request(page);
    validate(rows, page);
    pages.push(rows);
  }
  const terminal = await request(maxPages + 1);
  const firstCheck = await request(1);
  if (terminal.length || !isDeepStrictEqual(first, firstCheck))
    throw new Error("Calendar changed during enumeration");
  const snapshot = {
    page_size: pageSize,
    pages,
    terminal,
    first_check: firstCheck,
  };
  if (Buffer.byteLength(JSON.stringify(snapshot)) > 4 * 1024 * 1024)
    throw new Error("Snapshot exceeds 4 MiB");
  return { snapshot, raw, captured_at: now.toISOString() };
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length !== 2)
    throw new Error("Usage: node tests/plot/capture.mjs");
  const result = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-plot-hi-dive-"));
  chmodSync(directory, 0o755);
  for (const [name, value] of Object.entries({
    "snapshot.json": result.snapshot,
    "responses.json": result.raw,
    "report.json": { source: "hi-dive", captured_at: result.captured_at },
  }))
    writeFileSync(join(directory, name), JSON.stringify(value), { flag: "wx" });
  console.log(JSON.stringify({ directory, captured_at: result.captured_at }));
}
