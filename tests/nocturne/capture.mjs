import { captureProfile } from "../capture-profile.mjs";
import { readHTML } from "../html/http.mjs";
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

async function calendar(url, fetcher) {
  const response = await fetcher(url, {
    redirect: "error",
    signal: AbortSignal.timeout(30000),
  });
  if (
    !response.ok ||
    !response.headers.get("content-type")?.startsWith("text/calendar")
  )
    throw Error("Calendar HTTP or content-type failure");
  const reader = response.body.getReader(),
    chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > 1024 * 1024) throw Error("Calendar exceeds 1 MiB");
      chunks.push(value);
    }
  } finally {
    await reader.cancel();
  }
  const text = new TextDecoder("utf-8", { fatal: true }).decode(
    Buffer.concat(chunks),
  );
  if (
    !text.startsWith("BEGIN:VCALENDAR") ||
    !text.trim().endsWith("END:VCALENDAR")
  )
    throw Error("Incomplete calendar");
  return text;
}
export async function capture(
  fetcher = fetch,
  now = new Date(),
  settings = captureProfile(process.env.CAPTURE_SOURCE || "nocturne"),
) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: settings.timezone,
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const through = end.toISOString().slice(0, 10);
  const read = async () => {
    const pages = {
      months: {},
      policy: await readHTML(settings.policy, fetcher),
    };
    for (
      let month = new Date(from.slice(0, 7) + "-01T12:00:00Z");
      month.toISOString().slice(0, 7) <= through.slice(0, 7);
      month.setUTCMonth(month.getUTCMonth() + 1)
    ) {
      const date = month.toISOString().slice(0, 10),
        url = new URL(settings.endpoint);
      url.searchParams.set("date", date);
      pages.months[date] = await calendar(url.href, fetcher);
      if (Buffer.byteLength(JSON.stringify(pages)) > 2 * 1024 * 1024)
        throw Error("Capture pass exceeds 2 MiB");
    }
    return pages;
  };
  const snapshot = { from, through, pages: await read(), check: await read() };
  if (Buffer.byteLength(JSON.stringify(snapshot)) > 4 * 1024 * 1024)
    throw Error("Snapshot exceeds 4 MiB");
  return { captured_at: now.toISOString(), snapshot };
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const r = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-nocturne-"));
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    mode: 0o644,
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({ captured_at: r.captured_at }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
