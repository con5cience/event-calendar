// Public, read-only widget query scoped to Levitt. No credentials required.
import { isDeepStrictEqual } from "node:util";
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const endpoint = "https://www.venuepilot.co/graphql";
const query = `query ($accountIds: [Int!]!, $startDate: String!, $endDate: String, $limit: Int, $page: Int) {
  paginatedEvents(arguments: {accountIds: $accountIds, startDate: $startDate, endDate: $endDate, limit: $limit, page: $page}) {
    collection { id name date doorTime startTime minimumAge status description ticketsUrl venue { name } }
    metadata { currentPage limitValue totalCount totalPages }
  }
}`;
export async function queryPage(from, through, page, fetcher = fetch) {
  const response = await fetcher(endpoint, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      query,
      variables: {
        accountIds: [1105],
        startDate: from,
        endDate: through,
        limit: 5,
        page,
      },
    }),
    redirect: "error",
    signal: AbortSignal.timeout(30000),
  });
  if (!response.ok) throw Error(`VenuePilot HTTP ${response.status}`);
  const text = await response.text();
  if (Buffer.byteLength(text) > 4 * 1024 * 1024)
    throw Error("Oversized response");
  return JSON.parse(text);
}
export async function enumerate(request) {
  const events = [];
  const ids = new Set();
  let total;
  for (let page = 1; page <= 100; page++) {
    const response = await request(page);
    if (response.errors !== undefined) throw Error("GraphQL errors");
    const data = response.data?.paginatedEvents;
    const m = data?.metadata;
    if (
      !m ||
      !Array.isArray(data.collection) ||
      !Number.isSafeInteger(m.totalCount) ||
      m.totalCount < 0 ||
      m.totalCount > 500 ||
      m.currentPage !== page ||
      m.limitValue !== 5 ||
      m.totalPages !== Math.ceil(m.totalCount / 5) ||
      (total !== undefined && total !== m.totalCount)
    )
      throw Error("Invalid pagination metadata");
    total = m.totalCount;
    if (data.collection.length !== Math.min(5, total - events.length))
      throw Error("Incomplete page");
    for (const row of data.collection) {
      if (!Number.isSafeInteger(row?.id) || row.id <= 0 || ids.has(row.id))
        throw Error("Invalid identity");
      ids.add(row.id);
      events.push(row);
    }
    if (events.length === total) return events;
  }
  throw Error("Capture cap exceeded");
}
export function validateSnapshot(events, check) {
  if (!isDeepStrictEqual(events, check)) throw Error("Capture changed");
  if (!Array.isArray(events) || events.length > 500)
    throw Error("Invalid events");
  const ids = new Set();
  for (const row of events) {
    if (!Number.isSafeInteger(row?.id) || row.id <= 0 || ids.has(row.id))
      throw Error("Invalid identity");
    ids.add(row.id);
  }
  const result = { total: events.length, events, check };
  if (Buffer.byteLength(JSON.stringify(result)) > 4 * 1024 * 1024)
    throw Error("Snapshot exceeds 4 MiB");
  return result;
}
export async function capture() {
  const captured_at = new Date().toISOString();
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Denver",
  }).format(new Date(captured_at));
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const through = end.toISOString().slice(0, 10);
  const read = () => enumerate((page) => queryPage(from, through, page));
  const events = await read();
  const check = await read();
  return {
    captured_at,
    snapshot: { from, through, ...validateSnapshot(events, check) },
  };
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length !== 2)
    throw Error("Usage: node tests/venuepilot/capture.mjs");
  const result = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-levitt-"));
  chmodSync(directory, 0o755);
  for (const [name, value] of Object.entries({
    "snapshot.json": result.snapshot,
    "report.json": {
      source: "levitt",
      account_id: 1105,
      endpoint,
      captured_at: result.captured_at,
    },
  })) {
    writeFileSync(join(directory, name), JSON.stringify(value), { flag: "wx" });
  }
  console.log(JSON.stringify({ directory, captured_at: result.captured_at }));
}
