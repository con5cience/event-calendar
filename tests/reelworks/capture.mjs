import { captureProfile } from "../capture-profile.mjs";
// Explicit read-only operator capture of ReelWorks' public WordPress listing
// and event pages. Never called by app builds or startup.
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const maxPageBytes = 2 * 1024 * 1024;
const origin = "https://reelworksdenver.com";

async function readHTML(url, fetcher) {
  const response = await fetcher(url, {
    redirect: "error",
    signal: AbortSignal.timeout(30000),
  });
  if (
    !response.ok ||
    !response.headers.get("content-type")?.startsWith("text/html")
  )
    throw Error(`Listing HTTP or content-type failure: ${url}`);
  const reader = response.body.getReader();
  const chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > maxPageBytes) throw Error("Page exceeds 2 MiB");
      chunks.push(value);
    }
  } finally {
    await reader.cancel();
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(
    Buffer.concat(chunks),
  );
}

// URL discovery only, not an event parser. Go validates the listing structure,
// the discovered set, and every event page. An absent age sidebar stays absent
// here so Go can reject that record instead of failing the whole source.
export function listingSlugs(listing) {
  if (!listing.includes('<div class="event-item">'))
    throw Error("Missing ReelWorks event listing");
  const slugs = [];
  for (const match of listing.matchAll(
    /href="https:\/\/reelworksdenver\.com\/event\/([a-z0-9-]+)\/"/g,
  ))
    if (!slugs.includes(match[1])) slugs.push(match[1]);
  if (!slugs.length || slugs.length > 200)
    throw Error("Missing or capped ReelWorks event listing");
  return slugs;
}

export function schemaJSONLD(html) {
  const matches = [
    ...html.matchAll(
      /<script type="application\/ld\+json"[^>]*>([\s\S]*?)<\/script>/g,
    ),
  ];
  if (matches.length !== 1 || !matches[0][1].trimStart().startsWith("{"))
    throw Error("Expected one schema graph");
  if (Buffer.byteLength(matches[0][1]) > 64 * 1024)
    throw Error("Schema graph exceeds 64 KiB");
  return matches[0][1];
}

export function sidebarBlocks(html) {
  const sidebar = {};
  for (const [key, label, required] of [
    ["date", "Date", true],
    ["time", "Time", true],
    ["age", "Age", false],
  ]) {
    const matches = [
      ...html.matchAll(
        new RegExp(`<h3>${label}</h3>([\\s\\S]*?)(?=<h3>|</div>)`, "g"),
      ),
    ];
    if (matches.length > 1) throw Error(`Duplicate ${label} sidebar`);
    if (!matches.length) {
      if (required) throw Error(`Missing ${label} sidebar`);
      sidebar[key] = "";
      continue;
    }
    if (Buffer.byteLength(matches[0][1]) > 4096)
      throw Error(`${label} sidebar exceeds 4 KiB`);
    sidebar[key] = matches[0][1].trim();
  }
  return sidebar;
}

export function eventPage(html) {
  return {
    jsonld: schemaJSONLD(html),
    sidebar: sidebarBlocks(html),
  };
}

export async function capture(
  fetcher = fetch,
  now = new Date(),
  settings = captureProfile(process.env.CAPTURE_SOURCE || "reelworks"),
) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: settings.timezone,
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const read = async () => {
    const listing = await readHTML(settings.page, fetcher);
    const faq = await readHTML(settings.faq, fetcher);
    const events = {};
    for (const slug of listingSlugs(listing))
      events[slug] = eventPage(
        await readHTML(`${origin}/event/${slug}/`, fetcher),
      );
    return { listing, faq, events };
  };
  const snapshot = {
    from,
    through: end.toISOString().slice(0, 10),
    pages: await read(),
    check: await read(),
  };
  if (Buffer.byteLength(JSON.stringify(snapshot)) > 4 * 1024 * 1024)
    throw Error("Snapshot exceeds 4 MiB");
  return { captured_at: now.toISOString(), snapshot };
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length !== 2)
    throw Error("Usage: node tests/reelworks/capture.mjs");
  const r = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-reelworks-"));
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    mode: 0o644,
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      source: "reelworks",
      captured_at: r.captured_at,
      validation:
        "Replay must compare both listing and event-page passes and validate the venue FAQ",
    }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
