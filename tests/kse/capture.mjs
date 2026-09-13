import { captureProfile } from "../capture-profile.mjs";
const captureSettings = captureProfile("paramount");
// Explicit operator capture of the complete array used by Paramount's own widget.
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { isDeepStrictEqual } from "node:util";

export async function capture(fetcher = fetch, now = new Date()) {
  const raw = [];
  async function request() {
    const url = captureSettings.endpoint;
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
    if (!Array.isArray(rows) || rows.length >= 1000)
      throw new Error("Invalid or capped venue array");
    const seen = new Set();
    for (const e of rows) {
      if (typeof e.id !== "string" || !e.id || seen.has(e.id))
        throw new Error("Invalid identity");
      seen.add(e.id);
    }
    raw.push({ url, text });
    return rows;
  }
  const events = await request(),
    check = await request();
  if (!isDeepStrictEqual(events, check))
    throw new Error("Venue array changed during capture");
  const snapshot = { events, check };
  if (Buffer.byteLength(JSON.stringify(snapshot)) > 4 * 1024 * 1024)
    throw new Error("Snapshot exceeds 4 MiB");
  return { snapshot, raw, captured_at: now.toISOString() };
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length !== 2)
    throw new Error("Usage: node tests/kse/capture.mjs");
  const r = await capture();
  const directory = mkdtempSync(
    join(tmpdir(), "event-calendar-kse-paramount-"),
  );
  chmodSync(directory, 0o755);
  for (const [name, value] of Object.entries({
    "snapshot.json": r.snapshot,
    "responses.json": r.raw,
    "report.json": { source: "paramount", captured_at: r.captured_at },
  }))
    writeFileSync(join(directory, name), JSON.stringify(value), { flag: "wx" });
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
