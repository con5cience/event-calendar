import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { readHTML } from "../html/http.mjs";
import { compactHTML } from "../rhp/capture.mjs";

const endpoint =
  "https://aftontickets.com/api/get-events?key=732e49e3d992ab7ff998f13c3a2d3f08&per_page=12&page=";
const fields = [
  "event_type",
  "event_id",
  "event_name",
  "start_time",
  "door_time",
  "venue_name",
  "city",
  "state_abbreviation",
  "buy_ticket_url",
  "hide_start_time",
  "display_door_start_time",
  "entry_type",
  "sold_out",
];
function identity(e) {
  if (e.event_type === "real_world" && /^[a-z0-9]{10}$/.test(e.event_id))
    return e.event_id;
  if (
    e.event_type === "external_event" &&
    Number.isSafeInteger(e.event_id) &&
    e.event_id > 0
  )
    return "external-" + e.event_id;
  throw Error("Invalid identity");
}
export async function enumerate(request) {
  const pages = [],
    seen = new Set();
  let total;
  for (let page = 1; page <= 42; page++) {
    const p = await request(page);
    if (
      !Number.isSafeInteger(p.total_records) ||
      p.total_records < 1 ||
      p.total_records > 500 ||
      p.current_page !== page ||
      p.per_page !== 12 ||
      !Array.isArray(p.data) ||
      p.data.length !== Math.min(12, p.total_records - (page - 1) * 12) ||
      (total !== undefined && total !== p.total_records) ||
      (p.next_page_url !== null) !== page * 12 < p.total_records
    )
      throw Error("Incomplete pagination");
    total = p.total_records;
    const events = p.data.map((e) => {
      const id = identity(e);
      if (seen.has(id)) throw Error("Duplicate identity");
      seen.add(id);
      return Object.fromEntries(
        fields.filter((k) => Object.hasOwn(e, k)).map((k) => [k, e[k]]),
      );
    });
    pages.push({ page, total, per_page: 12, next: page * 12 < total, events });
    if (!pages.at(-1).next) return pages;
  }
  throw Error("Capture cap exceeded");
}
export async function capture(fetcher = fetch, now = new Date()) {
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Denver",
  }).format(now);
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const read = async () => {
    const pages = await enumerate(async (n) => {
      const r = await fetcher(endpoint + n, {
        redirect: "error",
        signal: AbortSignal.timeout(30000),
      });
      if (!r.ok || !r.headers.get("content-type")?.includes("application/json"))
        throw Error("Afton HTTP or type failure");
      const reader = r.body.getReader(),
        chunks = [];
      let size = 0;
      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          size += value.byteLength;
          if (size > 1024 * 1024) throw Error("Oversized page");
          chunks.push(value);
        }
      } finally {
        await reader.cancel();
      }
      return JSON.parse(
        new TextDecoder("utf-8", { fatal: true }).decode(Buffer.concat(chunks)),
      );
    });
    const details = {};
    for (const e of pages.flatMap((p) => p.events)) {
      if (e.event_type !== "real_world") continue;
      const url = "https://aftontickets.com/event/buyticket/" + identity(e);
      if (e.buy_ticket_url !== url) throw Error("Unreviewed detail URL");
      details[e.event_id] = compactHTML(await readHTML(url, fetcher));
    }
    return { pages, details };
  };
  const first = await read(),
    second = await read();
  const snapshot = {
    from,
    through: end.toISOString().slice(0, 10),
    ...first,
    check_pages: second.pages,
    check_details: second.details,
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
    throw Error("Usage: node tests/afton/capture.mjs");
  const r = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-roxy-"));
  chmodSync(directory, 0o755);
  writeFileSync(join(directory, "snapshot.json"), JSON.stringify(r.snapshot), {
    mode: 0o644,
  });
  writeFileSync(
    join(directory, "report.json"),
    JSON.stringify({
      captured_at: r.captured_at,
      validation:
        "Replay validates complete pages and paired normalized details",
    }),
    { mode: 0o644 },
  );
  console.log(JSON.stringify({ directory, captured_at: r.captured_at }));
}
