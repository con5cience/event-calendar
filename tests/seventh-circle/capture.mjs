import { captureProfile } from "../capture-profile.mjs";
import { readHTML } from "../html/http.mjs";
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export async function capture(
  fetcher = fetch,
  now = new Date(),
  settings = captureProfile(process.env.CAPTURE_SOURCE || "seventh-circle"),
) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: settings.timezone,
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const read = async () => {
    const calendar = await readHTML(settings.page, fetcher);
    // Discovery only: Go verifies scoped cards, IDs, enumeration and detail agreement.
    const ids = [
      ...new Set(
        [...calendar.matchAll(/href=["']\/posts\/(\d+)["']/g)].map((m) => m[1]),
      ),
    ];
    if (!ids.length || ids.length > 500)
      throw Error("Unknown empty listing or capture cap exceeded");
    const pages = {
      calendar,
      policy: await readHTML(settings.policy, fetcher),
      details: {},
    };
    for (const id of ids) {
      const url = new URL(`/posts/${id}`, settings.page).href;
      try {
        pages.details[id] = await readHTML(url, fetcher);
      } catch {
        pages.details[id] = "";
      }
      if (Buffer.byteLength(JSON.stringify(pages)) > 2 * 1024 * 1024)
        throw Error("Capture pass exceeds 2 MiB");
    }
    return pages;
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
    throw Error("Usage: node tests/seventh-circle/capture.mjs");
  const r = await capture();
  const directory = mkdtempSync(
    join(tmpdir(), "event-calendar-seventh-circle-"),
  );
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    mode: 0o644,
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      captured_at: r.captured_at,
      validation: "Replay must compare both listing, detail and policy passes",
    }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
