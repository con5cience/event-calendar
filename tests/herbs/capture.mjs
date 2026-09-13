import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { readHTML } from "../html/http.mjs";
export async function capture(fetcher = fetch, now = new Date()) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Denver",
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const url = "https://www.herbsbar.com/live-music-calendar-1";
  const snapshot = {
    from,
    through: end.toISOString().slice(0, 10),
    html: await readHTML(url, fetcher),
    check_html: await readHTML(url, fetcher),
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
    throw Error("Usage: node tests/herbs/capture.mjs");
  const r = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-herbs-"));
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    mode: 0o644,
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      captured_at: r.captured_at,
      validation: "Replay must compare both HTML passes",
    }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
