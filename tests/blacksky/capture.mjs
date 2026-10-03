import { captureProfile } from "../capture-profile.mjs";
// Explicit read-only operator capture of Black Sky's public SpotHopper
// events page. Never called by app builds or startup.
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const maxPageBytes = 2 * 1024 * 1024;

async function readHTML(url, fetcher) {
  const response = await fetcher(url, {
    redirect: "error",
    signal: AbortSignal.timeout(30000),
  });
  if (
    !response.ok ||
    !response.headers.get("content-type")?.startsWith("text/html")
  )
    throw Error(`Events page HTTP or content-type failure: ${url}`);
  const reader = response.body.getReader();
  const chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > maxPageBytes) throw Error("Events page exceeds 2 MiB");
      chunks.push(value);
    }
  } finally {
    await reader.cancel();
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(
    Buffer.concat(chunks),
  );
}

// Structure discovery only, not an event parser. Go validates every section,
// the platform's recurring markers, and the paired reads.
export function hasEventSections(html) {
  return /<section id="[0-9]+"[^>]*>/.test(html);
}

export async function capture(
  fetcher = fetch,
  now = new Date(),
  settings = captureProfile(process.env.CAPTURE_SOURCE || "black-sky"),
) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: settings.timezone,
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const read = async () => {
    const page = await readHTML(settings.page, fetcher);
    if (!hasEventSections(page)) throw Error("Missing SpotHopper events page");
    return page;
  };
  const snapshot = {
    from,
    through: end.toISOString().slice(0, 10),
    page: await read(),
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
    throw Error("Usage: node tests/blacksky/capture.mjs");
  const r = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-black-sky-"));
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    mode: 0o644,
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      source: "black-sky",
      captured_at: r.captured_at,
      validation:
        "Replay must compare both paired reads and exclude recurring series",
    }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
