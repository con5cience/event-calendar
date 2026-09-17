import { captureProfile } from "../capture-profile.mjs";
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
    throw Error("Calendar HTTP or content-type failure");
  const reader = response.body.getReader();
  const chunks = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > maxPageBytes) throw Error("Calendar exceeds 2 MiB");
      chunks.push(value);
    }
  } finally {
    await reader.cancel();
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(
    Buffer.concat(chunks),
  );
}

export function warmupJSON(html) {
  const matches = [
    ...html.matchAll(
      /<script type="application\/json" id="wix-warmup-data">([\s\S]*?)<\/script>/g,
    ),
  ];
  if (matches.length !== 1 || !matches[0][1].startsWith("{"))
    throw Error("Missing Wix events payload");
  if (Buffer.byteLength(matches[0][1]) > 512 * 1024)
    throw Error("Wix events payload exceeds 512 KiB");
  return matches[0][1];
}

export function policyEvidence(html) {
  const stripped = html
    .replace(/<script[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?<\/style>/gi, "");
  if (
    !/people of all ages/i.test(stripped) ||
    !/sober/i.test(stripped) ||
    !/2130/.test(stripped) ||
    !/platte river/i.test(stripped)
  )
    throw Error("Unreviewed venue policy");
  if (Buffer.byteLength(stripped) > 256 * 1024)
    throw Error("Policy evidence exceeds 256 KiB");
  return stripped;
}

export async function capture(
  fetcher = fetch,
  now = new Date(),
  settings = captureProfile(process.env.CAPTURE_SOURCE || "ante-up"),
) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: settings.timezone,
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const read = async () => {
    const events = await readHTML(settings.page, fetcher);
    const venue = await readHTML(settings.policy, fetcher);
    return { warmup: warmupJSON(events), policy: policyEvidence(venue) };
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
    throw Error("Usage: node tests/anteup/capture.mjs");
  const r = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-ante-up-"));
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    mode: 0o644,
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      captured_at: r.captured_at,
      validation:
        "Replay must compare both Wix payload and venue-policy passes",
    }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
