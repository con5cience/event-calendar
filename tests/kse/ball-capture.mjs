import { captureProfile } from "../capture-profile.mjs";
const captureSettings = captureProfile("ball-arena");
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { isDeepStrictEqual } from "node:util";

export async function captureBall(fetcher = fetch, now = new Date()) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: captureSettings.timezone,
  }).format(now);
  const [year, month, day] = from.split("-");
  const through = `${Number(year) + 1}-${month}-${day}`;
  const api = `${captureSettings.endpoint}&start=${from}&end=${through}`;
  const listingURL = captureSettings.page;
  async function request(url) {
    const r = await fetcher(url, {
      redirect: "error",
      signal: AbortSignal.timeout(30000),
    });
    if (!r.ok) throw new Error(`HTTP ${r.status}; capture aborted`);
    const text = await r.text();
    if (Buffer.byteLength(text) > 4 * 1024 * 1024)
      throw new Error("Response exceeds 4 MiB");
    return text;
  }
  const events = JSON.parse(await request(api));
  const listing = await request(listingURL);
  const check = JSON.parse(await request(api));
  const listing_check = await request(listingURL);
  if (
    !Array.isArray(events) ||
    events.length >= 1000 ||
    !isDeepStrictEqual(events, check)
  )
    throw new Error("Invalid, capped or changed calendar");
  const snapshot = { events, check, listing, listing_check };
  if (Buffer.byteLength(JSON.stringify(snapshot)) > 4 * 1024 * 1024)
    throw new Error("Snapshot exceeds 4 MiB");
  return { snapshot, captured_at: now.toISOString(), api, listingURL };
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length !== 2)
    throw new Error("Usage: node tests/kse/ball-capture.mjs");
  const r = await captureBall();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-ball-"));
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    flag: "wx",
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      source: "ball-arena",
      captured_at: r.captured_at,
      api: r.api,
      listing_url: r.listingURL,
    }),
    { flag: "wx" },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
