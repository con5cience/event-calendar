// Read-only HTTP capture. Replay parses and compares event fields in both pages.
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { readHTML } from "../html/http.mjs";

export async function capture(fetcher = fetch, now = new Date()) {
  const read = () => readHTML("https://opheliasdenver.com/calendar/", fetcher);
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Denver",
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const html = await read();
  const check_html = await read();
  const snapshot = {
    from,
    through: end.toISOString().slice(0, 10),
    html,
    check_html,
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
    throw Error("Usage: node tests/ophelias/capture.mjs");
  const result = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-ophelias-"));
  chmodSync(directory, 0o755);
  writeFileSync(
    join(directory, "snapshot.json"),
    JSON.stringify(result.snapshot),
    { mode: 0o644 },
  );
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      captured_at: result.captured_at,
      url: "https://opheliasdenver.com/calendar/",
      validation: "Replay must validate paired event fields before publication",
    }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: result.captured_at }));
}
