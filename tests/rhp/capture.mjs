// Explicit operator capture, never run by Compose startup. No browser scripts
// execute. Original responses are retained beside the bounded replay snapshot.
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const origins = {
  "lost-lake": "https://lost-lake.com",
  larimer: "https://larimerlounge.com",
  "globe-hall": "https://globehall.com",
  cervantes: "https://cervantesmasterpiece.com",
};
export function calendarForm() {
  return new URLSearchParams({
    action: "loadEtixMonthViewEventPageFn",
    "data[limit]": "10000",
    "data[display]": "false",
    "data[target]": "calendar-1",
    "data[archive]": "true",
  });
}
// Strip only non-evidence payloads to fit the existing 4 MiB artifact input
// bound. The Go HTML parser still reads the retained markup; this is not an
// event or admission parser. Keep original HTML for inspection and replay QA.
export function compactHTML(html) {
  return html
    .split(
      /(<script\b[^>]*\btype\s*=\s*["']application\/ld\+json["'][^>]*>[\s\S]*?<\/script\s*>)/gi,
    )
    .map((part, index) => (index % 2 === 1 ? part : compactMarkup(part)))
    .join("");
}
function compactMarkup(html) {
  return html
    .replace(/<script\b([^>]*)>[\s\S]*?<\/script\s*>/gi, (all, attrs) =>
      /\btype\s*=\s*["']application\/ld\+json["']/i.test(attrs) ? all : "",
    )
    .replace(/<style\b[^>]*>[\s\S]*?<\/style\s*>/gi, "")
    .replace(/<svg\b[^>]*>[\s\S]*?<\/svg\s*>/gi, "")
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/<(?:meta|link|img)\b[^>]*>/gi, "")
    .replace(/[\r\n\t]+/g, " ")
    .replace(/ {2,}/g, " ");
}
export async function capture(key, fetcher = fetch) {
  const origin = origins[key];
  if (!origin) throw new Error("Unsupported source");
  const request = async (url, extra = {}) => {
    const response = await fetcher(url, {
      redirect: "error",
      signal: AbortSignal.timeout(30000),
      ...extra,
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}: ${url}`);
    const bytes = await response.text();
    if (Buffer.byteLength(bytes) > 4 * 1024 * 1024)
      throw new Error("Response exceeds 4 MiB");
    return bytes;
  };
  const calendarRaw = await request(origin + "/wp-admin/admin-ajax.php", {
    method: "POST",
    body: calendarForm(),
  });
  const calendar = JSON.parse(calendarRaw);
  if (
    calendar.success !== true ||
    !Array.isArray(calendar.data?.events) ||
    calendar.data.events.length >= 10000
  )
    throw new Error("Invalid or capped calendar response");
  const urls = new Set();
  for (const event of calendar.data.events) {
    const url = new URL(event.url);
    if (
      url.origin !== origin ||
      url.username ||
      url.password ||
      url.search ||
      url.hash ||
      !url.pathname.startsWith("/event/") ||
      urls.has(event.url)
    )
      throw new Error("Invalid or duplicate event URL");
    urls.add(event.url);
  }
  const details = {},
    rawPages = {},
    failures = [];
  // Serial requests avoid a burst against the venue. Missing details remain
  // explicit rejected observations; they do not become calendar deletions.
  for (const url of urls) {
    try {
      const html = await request(url);
      rawPages[url] = html;
      details[url] = compactHTML(html);
    } catch (error) {
      failures.push({ url, error: String(error) });
    }
  }
  const snapshot = { calendar, details };
  if (Buffer.byteLength(JSON.stringify(snapshot)) > 4 * 1024 * 1024)
    throw new Error("Snapshot exceeds 4 MiB; do not publish a partial capture");
  return { snapshot, rawPages, failures, calendarRaw };
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const key = process.argv[2];
  if (!origins[key] || process.argv.length !== 3)
    throw new Error(
      "Usage: node tests/rhp/capture.mjs lost-lake|larimer|globe-hall|cervantes",
    );
  const result = await capture(key);
  const directory = mkdtempSync(join(tmpdir(), `event-calendar-rhp-${key}-`));
  chmodSync(directory, 0o755);
  const save = (name, data) =>
    writeFileSync(
      join(directory, name),
      typeof data === "string" ? data : JSON.stringify(data),
      { flag: "wx" },
    );
  save("snapshot.json", result.snapshot);
  save("calendar.json", result.calendarRaw);
  save("raw-pages.json", result.rawPages);
  save("report.json", {
    source: key,
    captured_at: new Date().toISOString(),
    failures: result.failures,
  });
  console.log(
    JSON.stringify({ source: key, directory, failures: result.failures }),
  );
}
