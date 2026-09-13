import { captureProfile } from "../capture-profile.mjs";
const captureSettings = captureProfile("black-box");
// Operator-only, public event query. No credentials are persisted.
import { chromium } from "@playwright/test";
import { isDeepStrictEqual } from "node:util";
import { mkdtempSync, writeFileSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const endpoint = captureSettings.endpoint;
export function externalTicketRedirect(location) {
  if (!location) return false;
  return (
    new URL(location, captureSettings.ticket_origin).origin !==
    captureSettings.ticket_origin
  );
}
export async function readTicket(url, fetcher = fetch) {
  const response = await fetcher(url, {
    redirect: "manual",
    signal: AbortSignal.timeout(30000),
  });
  if (
    response.status >= 300 &&
    response.status < 400 &&
    externalTicketRedirect(response.headers.get("location"))
  )
    return { ticket_error: "external ticket provider" };
  if (!response.ok) throw Error(`Ticket HTTP ${response.status}: ${url}`);
  const html = await response.text();
  if (Buffer.byteLength(html) > 4 * 1024 * 1024)
    throw Error("Oversized ticket page");
  return { html };
}
export async function enumerate(request) {
  const events = [];
  let total;
  const seen = new Set();
  for (let offset = 0; offset <= 500; offset += 25) {
    const { rows, range } = await request(offset);
    if (!Array.isArray(rows)) throw Error("Expected event array");
    if (offset === 0 && range === "*/0" && rows.length === 0) return [];
    const m = /^(\d+)-(\d+)\/(\d+)$/.exec(range ?? "");
    if (!m) throw Error("Missing exact count");
    const [start, end, count] = m.slice(1).map(Number);
    if (
      count > 500 ||
      count < 1 ||
      start !== offset ||
      end !== Math.min(offset + 24, count - 1) ||
      rows.length !== end - start + 1 ||
      (total !== undefined && count !== total)
    )
      throw Error("Incomplete or changed range");
    total = count;
    for (const e of rows) {
      if (typeof e?.id !== "string" || !e.id || seen.has(e.id))
        throw Error("Invalid identity");
      seen.add(e.id);
      events.push(e);
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
  for (const e of events) {
    if (typeof e?.id !== "string" || !e.id || ids.has(e.id))
      throw Error("Invalid identity");
    ids.add(e.id);
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
  const browser = await chromium.launch();
  try {
    const context = await browser.newContext();
    const page = await context.newPage();
    const observed = page.waitForRequest(
      (r) => r.url().startsWith(endpoint + "?"),
      { timeout: 30000 },
    );
    await page.goto(captureSettings.page, {
      waitUntil: "domcontentloaded",
      timeout: 30000,
    });
    const headers = await (await observed).allHeaders();
    const key = headers.apikey;
    if (
      !key ||
      JSON.parse(Buffer.from(key.split(".")[1], "base64url").toString())
        .role !== "anon"
    )
      throw Error("Expected public anonymous key");
    const query = new URL(endpoint);
    query.search = new URLSearchParams({
      select:
        "id,slug,title,date,time,end_date,end_time,venue,location,address,ticket_url,status,summary_html",
      status: "eq.published",
      venue: 'in.("The Black Box","The Lounge")',
      and: `(date.gte.${from},date.lte.${through})`,
      order: "date.asc,id.asc",
      limit: "25",
    });
    async function read() {
      const rows = await enumerate(async (offset) => {
        const url = new URL(query);
        url.searchParams.set("offset", String(offset));
        const response = await fetch(url, {
          headers: {
            apikey: key,
            ...(headers.authorization
              ? { Authorization: headers.authorization }
              : {}),
            Prefer: "count=exact",
          },
          redirect: "error",
          signal: AbortSignal.timeout(30000),
        });
        if (!response.ok) throw Error(`Event HTTP ${response.status}`);
        const text = await response.text();
        if (Buffer.byteLength(text) > 4 * 1024 * 1024)
          throw Error("Oversized response");
        return {
          rows: JSON.parse(text),
          range: response.headers.get("content-range"),
        };
      });
      for (const e of rows) {
        if (
          !/^[a-z0-9-]+$/.test(e.slug) ||
          e.ticket_url !==
            `${captureSettings.ticket_origin}/e/${e.slug}/tickets`
        )
          throw Error("Unexpected ticket URL");
        const ticket = await readTicket(e.ticket_url);
        if (ticket.ticket_error) {
          e.ticket_error = ticket.ticket_error;
          continue;
        }
        const extracted = await page.evaluate((html) => {
          const document = new DOMParser().parseFromString(html, "text/html");
          const sections = [...document.querySelectorAll(".general-text-area")];
          const values = sections
            .filter(
              (s) =>
                s.querySelector(".header")?.textContent.trim() ===
                "Age Restriction",
            )
            .map((s) => s.querySelector("p")?.textContent.trim() ?? "");
          return {
            values,
            title: document.querySelector("h1")?.textContent.trim(),
          };
        }, ticket.html);
        if (!extracted.title) throw Error("Missing ticket page identity");
        // H1 may include decoded entities. Store it for producer identity validation.
        e.ticket_title = extracted.title;
        if (extracted.values.length > 1)
          throw Error("Ambiguous age restriction");
        e.age_restriction = extracted.values[0] ?? null;
      }
      return rows;
    }
    const events = await read();
    const check = await read();
    return {
      snapshot: { from, through, ...validateSnapshot(events, check) },
      captured_at,
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
    throw Error("Usage: node tests/blackbox/capture.mjs");
  const result = await capture();
  const directory = mkdtempSync(join(tmpdir(), "event-calendar-blackbox-"));
  chmodSync(directory, 0o755);
  for (const [name, value] of Object.entries({
    "snapshot.json": result.snapshot,
    "report.json": {
      source: "black-box",
      captured_at: result.captured_at,
      endpoint,
    },
  }))
    writeFileSync(join(directory, name), JSON.stringify(value), { flag: "wx" });
  console.log(JSON.stringify({ directory, captured_at: result.captured_at }));
}
