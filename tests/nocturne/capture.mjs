import { captureProfile } from "../capture-profile.mjs";
import { readHTML } from "../html/http.mjs";
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

// One workflow refresh lost its all-source deployment gate to a single
// transient response from this public feed while the endpoint was healthy
// immediately after. Calendar requests therefore retry response-level
// failures at most twice — network errors, unsuccessful statuses, or a
// non-calendar content type — with 500ms then 1s backoff. A fresh attempt
// discards all bytes, and one 30-second deadline spans every attempt.
// Body-level invalid data (size, encoding, framing) and the policy page
// remain single-attempt.
async function openCalendar(url, fetcher) {
  const signal = AbortSignal.timeout(30000);
  let last = "no response";
  for (let attempt = 1; attempt <= 3; attempt++) {
    if (attempt > 1)
      await new Promise((resolve) => setTimeout(resolve, 500 * (attempt - 1)));
    try {
      const response = await fetcher(url, { redirect: "error", signal });
      if (
        response.ok &&
        response.headers.get("content-type")?.startsWith("text/calendar")
      )
        return response;
      last = response.ok
        ? "non-calendar content type"
        : `HTTP ${response.status}`;
    } catch (error) {
      last = error.message;
    }
  }
  throw Error(
    `Calendar HTTP or content-type failure after 3 attempts (${last})`,
  );
}

async function calendar(url, fetcher) {
  const response = await openCalendar(url, fetcher);
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
