import { captureProfile } from "../capture-profile.mjs";
// Explicit read-only operator capture of D3's public Google Calendar iCal
// feed. Never called by app builds or startup.
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const maxFeedBytes = 4 * 1024 * 1024;

async function request(url, fetcher) {
  const response = await fetcher(url, {
    redirect: "error",
    signal: AbortSignal.timeout(30000),
  });
  if (!response.ok) throw Error(`Feed HTTP failure: ${url}`);
  const chunks = [];
  let size = 0;
  for await (const chunk of response.body) {
    size += chunk.length;
    if (size > maxFeedBytes) throw Error("Feed exceeds 4 MiB");
    chunks.push(chunk);
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(
    Buffer.concat(chunks),
  );
}

// Envelope discovery only, not an event parser. Google re-stamps every event
// on each fetch, so the feed is read once and Go validates it whole.
export function calendarEnvelope(text) {
  const trimmed = text.trim();
  if (
    !trimmed.startsWith("BEGIN:VCALENDAR") ||
    !trimmed.endsWith("END:VCALENDAR")
  )
    throw Error("Incomplete calendar");
  return trimmed;
}

export async function capture(
  fetcher = fetch,
  now = new Date(),
  settings = captureProfile(process.env.CAPTURE_SOURCE || "d3"),
) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: settings.timezone,
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const snapshot = {
    from,
    through: end.toISOString().slice(0, 10),
    calendar: calendarEnvelope(await request(settings.endpoint, fetcher)),
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
    throw Error("Usage: node tests/d3/capture.mjs");
  const r = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-d3-"));
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    mode: 0o644,
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      source: "d3",
      captured_at: r.captured_at,
      validation:
        "Replay must validate the calendar envelope and exclude recurring series",
    }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
