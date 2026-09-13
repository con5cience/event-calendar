// Operator-only normal-browser capture. No purchase flow or direct private API.
import { chromium } from "@playwright/test";
import { isDeepStrictEqual } from "node:util";
import { mkdtempSync, chmodSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const base = "https://tickets.meowwolf.com";
const index = base + "/events/denver/";
const sellerID = "017a7f54-e443-a261-3c55-46ef4d921efb";
const identity =
  /^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}__[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/;
const selected = new Set([
  "venue",
  "eventAge",
  "doorsOpen",
  "showStart",
  "externalTicketUrl",
  "eventNotificationText",
  "externalBtnTxt",
]);
export function listing(props, hrefs) {
  if (
    props?.isEvents !== true ||
    props.events?.seller?.id !== sellerID ||
    props.events.seller.timezone !== "America/Denver" ||
    !Array.isArray(props.events.events) ||
    props.events.events.length > 500
  )
    throw Error("Unrecognized listing");
  const ids = new Set();
  const rows = props.events.events.map((e) => {
    if (
      !identity.test(e.id) ||
      ids.has(e.id) ||
      !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(e.url)
    )
      throw Error("Invalid identity");
    ids.add(e.id);
    return {
      id: e.id,
      title: e.title,
      startDateTime: e.startDateTime,
      url: e.url,
      text: {
        date: e.text?.date,
        banner: e.text?.banner,
        supportingActs: e.text?.supportingActs,
      },
      tags: e.tags,
    };
  });
  if (
    !isDeepStrictEqual(
      hrefs
        .map((h) => h.replace(/^\/denver\/events\//, "/events/denver/"))
        .sort(),
      rows.map((e) => `/events/denver/${e.url}/`).sort(),
    )
  )
    throw Error("Rendered listing mismatch");
  return rows;
}
export function detail(props, row) {
  if (
    props?.__N_REDIRECT === "/events/denver/" ||
    props?.isExhibitions === true
  )
    return { error: "unrelated redirect" };
  if (
    props?.isEvents !== true ||
    props.isDetails !== true ||
    props.seller?.id !== sellerID ||
    props.events?.events?.length !== 1
  )
    throw Error("Unrecognized detail");
  const e = props.events.events[0];
  if (
    e.id !== row.id.split("__")[0] ||
    !Array.isArray(e.meta) ||
    !Array.isArray(e.timeslots)
  )
    throw Error("Detail identity mismatch");
  return {
    id: e.id,
    title: e.title,
    sellerId: e.sellerId,
    venueId: e.venueId,
    meta: e.meta
      .filter((m) => selected.has(m.metakey))
      .map((m) => ({ metakey: m.metakey, value: m.value })),
    timeslots: e.timeslots.map((t) => ({ id: t.id, startTime: t.startTime })),
  };
}
export function validateSnapshot(events, check) {
  if (!isDeepStrictEqual(events, check)) throw Error("Capture changed");
  const result = { total: events.length, events, check };
  if (Buffer.byteLength(JSON.stringify(result)) > 4 * 1024 * 1024)
    throw Error("Snapshot exceeds 4 MiB");
  return result;
}
export function documentProps(html) {
  if (Buffer.byteLength(html) > 4 * 1024 * 1024)
    throw Error("Oversized document");
  const legacy = html.match(
    /<script\b[^>]*id="__NEXT_DATA__"[^>]*>([\s\S]*?)<\/script>/,
  );
  if (legacy) return JSON.parse(legacy[1]).props.pageProps;
  const chunks = [
    ...html.matchAll(/self\.__next_f\.push\((\[1,\s*"(?:\\.|[^"\\])*"\])\)/g),
  ].map((m) => JSON.parse(m[1])[1]);
  const nodes = [];
  function walk(v) {
    if (!v || typeof v !== "object") return;
    if (!Array.isArray(v)) nodes.push(v);
    for (const x of Object.values(v)) walk(x);
  }
  // Flight text rows use hexadecimal UTF-8 byte lengths. Never scan their
  // contents as model records or execute module/reference records.
  const stream = Buffer.from(chunks.join(""));
  let offset = 0;
  while (offset < stream.length) {
    const colon = stream.indexOf(58, offset);
    if (
      colon < 0 ||
      (!(
        colon === offset &&
        stream.subarray(colon + 1, colon + 3).toString() === "HL"
      ) &&
        !/^[a-f0-9]+$/.test(stream.subarray(offset, colon).toString()))
    )
      throw Error("Invalid Flight record");
    offset = colon + 1;
    if (stream[offset] === 84) {
      const comma = stream.indexOf(44, offset);
      const hex = stream.subarray(offset + 1, comma).toString();
      if (comma < 0 || !/^[a-f0-9]+$/.test(hex))
        throw Error("Invalid text frame");
      const length = Number.parseInt(hex, 16);
      if (!Number.isSafeInteger(length) || comma + 1 + length > stream.length)
        throw Error("Truncated text frame");
      offset = comma + 1 + length;
      continue;
    }
    const end = stream.indexOf(10, offset);
    if (end < 0) throw Error("Truncated model frame");
    const body = stream.subarray(offset, end).toString();
    if (body.startsWith("[") || body.startsWith("{")) walk(JSON.parse(body));
    else if (
      !/^(?:I|H|D|E|N|W|P|J|R|C|X|r|x|#|"|null|true|false|[0-9-])/.test(body)
    )
      throw Error("Unsupported Flight record");
    offset = end + 1;
  }
  const listings = nodes.filter(
    (v) =>
      Array.isArray(v.eventsData) &&
      typeof v.containsMultipleCategories === "boolean",
  );
  const sellers = nodes.filter(
    (v) =>
      v.seller?.id === sellerID &&
      ["denver/events", "events/denver"].includes(v.siteSlug),
  );
  if (listings.length === 1 && sellers.length === 1)
    return {
      isEvents: true,
      events: { seller: sellers[0].seller, events: listings[0].eventsData },
    };
  const details = nodes.filter(
    (v) =>
      v.event &&
      v.eventVenue &&
      v.associatedEvents &&
      typeof v.associatedEvents === "object" &&
      !Array.isArray(v.associatedEvents),
  );
  if (details.length === 1)
    return {
      isEvents: true,
      isDetails: true,
      seller: { id: details[0].event.sellerId },
      events: { events: [details[0].event] },
    };
  throw Error("Unrecognized embedded document");
}
async function read(browser) {
  const context = await browser.newContext();
  const page = await context.newPage();
  page.setDefaultTimeout(20000);
  try {
    const response = await page.goto(index, {
      waitUntil: "domcontentloaded",
      timeout: 30000,
    });
    if (!response?.ok()) throw Error("Listing HTTP failure");
    await page.waitForFunction(
      () =>
        document.querySelector('[data-testid="event-card-wrapper"]') ||
        document.querySelector('[data-testid="events-no-results"]'),
    );
    const props = documentProps(await response.text());
    const hrefs = await page
      .getByTestId("event-card-wrapper")
      .evaluateAll((xs) => xs.map((x) => x.getAttribute("href")));
    const rows = listing(props, hrefs);
    for (let i = 0; i < rows.length; i++) {
      const row = rows[i];
      const response = await page.goto(base + hrefs[i], {
        waitUntil: "domcontentloaded",
        timeout: 30000,
      });
      if (!response?.ok())
        throw Error(`Detail HTTP ${response?.status()}: ${row.url}`);
      const path = new URL(page.url()).pathname;
      if (["/events/denver/", "/denver/events/", "/denver/"].includes(path))
        row.detail = { error: "unrelated redirect" };
      else row.detail = detail(documentProps(await response.text()), row);
      console.log(
        JSON.stringify({
          event: row.url,
          detail: row.detail.error ?? "captured",
        }),
      );
    }
    return rows;
  } finally {
    await context.close();
  }
}
export async function capture() {
  const captured_at = new Date().toISOString();
  const from = new Intl.DateTimeFormat("en-CA", {
    timeZone: "America/Denver",
  }).format(new Date(captured_at));
  const end = new Date(from + "T12:00:00Z");
  end.setUTCFullYear(end.getUTCFullYear() + 1);
  const through = end.toISOString().slice(0, 10);
  const browser = await chromium.launch();
  try {
    const events = await read(browser);
    const check = await read(browser);
    return {
      captured_at,
      snapshot: { from, through, ...validateSnapshot(events, check) },
    };
  } finally {
    await browser.close();
  }
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length !== 2)
    throw Error("Usage: node tests/meowwolf/capture.mjs");
  const result = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-meowwolf-"));
  chmodSync(directory, 0o755);
  for (const [name, value] of Object.entries({
    "snapshot.json": result.snapshot,
    "report.json": {
      source: "meow-wolf-denver",
      captured_at: result.captured_at,
      url: index,
    },
  }))
    writeFileSync(join(directory, name), JSON.stringify(value), { flag: "wx" });
  console.log(JSON.stringify({ directory, captured_at: result.captured_at }));
}
