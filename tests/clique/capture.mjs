// Explicit read-only operator capture. Never called by app builds or startup.
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export async function capture(fetcher = fetch, now = new Date()) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Denver",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(now);
  const nextYear = new Date(from + "T00:00:00Z");
  nextYear.setUTCFullYear(nextYear.getUTCFullYear() + 1);
  const through = nextYear.toISOString().slice(0, 10);
  nextYear.setUTCDate(nextYear.getUTCDate() + 1);
  const end = nextYear.toISOString().slice(0, 10);
  const raw = {};
  async function request(path) {
    const url = "https://www.redrocksonline.com/wp-json/clique/v1/" + path;
    const response = await fetcher(url, {
      redirect: "error",
      signal: AbortSignal.timeout(30000),
    });
    if (!response.ok)
      throw new Error(
        `HTTP ${response.status}: ${url}; capture aborted, retry explicitly later`,
      );
    const text = await response.text();
    if (Buffer.byteLength(text) > 4 * 1024 * 1024)
      throw new Error("Response exceeds 4 MiB");
    const rows = JSON.parse(text);
    if (!Array.isArray(rows) || rows.length >= 10000)
      throw new Error("Invalid or capped event response");
    raw[url] = text;
    return rows;
  }
  async function requestRange(a, z) {
    const padded = new Date(a + "T00:00:00Z");
    padded.setUTCDate(padded.getUTCDate() - 1);
    const rows = await request(
      `get_calendar_events_range?start=${padded.toISOString().slice(0, 10)}&end=${z}`,
    );
    // Date-only lower bounds omit some morning events. Retain original responses
    // for audit, then trim the overlap to the logical half-open window.
    return rows.filter(
      (e) =>
        typeof e.start !== "string" ||
        !/^\d{4}-\d{2}-\d{2}T/.test(e.start) ||
        (e.start.slice(0, 10) >= a && e.start.slice(0, 10) < z),
    );
  }
  const upcoming = await request("get_upcoming_events");
  const range = await requestRange(from, end);
  const windows = [];
  // Empty calendar months returned HTTP 500 in the implementation review.
  // Split at a known event date so both cross-check requests contain events.
  // Never convert HTTP errors into empty arrays or silently omit a window.
  const dates = [...new Set(range.map((e) => e.start?.slice(0, 10)))]
    .filter((d) => d >= from && d < end)
    .sort();
  if (dates.length > 1) {
    const split = dates[Math.floor(dates.length / 2)];
    windows.push({
      start: from,
      end: split,
      events: await requestRange(from, split),
    });
    windows.push({
      start: split,
      end,
      events: await requestRange(split, end),
    });
  } else windows.push({ start: from, end, events: range });
  const snapshot = { from, through, upcoming, range, windows };
  if (Buffer.byteLength(JSON.stringify(snapshot)) > 4 * 1024 * 1024)
    throw new Error("Snapshot exceeds 4 MiB");
  return { snapshot, raw, captured_at: now.toISOString() };
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length !== 2)
    throw new Error("Usage: node tests/clique/capture.mjs");
  const result = await capture();
  const directory = mkdtempSync(
    join(tmpdir(), "event-calendar-clique-red-rocks-"),
  );
  chmodSync(directory, 0o755);
  for (const [name, value] of Object.entries({
    "snapshot.json": result.snapshot,
    "responses.json": result.raw,
    "report.json": { source: "red-rocks", captured_at: result.captured_at },
  }))
    writeFileSync(join(directory, name), JSON.stringify(value), { flag: "wx" });
  console.log(JSON.stringify({ directory, captured_at: result.captured_at }));
}
