// Capture only. The Go adapters remain authoritative for calendar/event validation.
const limit = 4 * 1024 * 1024;
async function request(url, fetcher) {
  const response = await fetcher(url, {
    redirect: "error",
    signal: AbortSignal.timeout(30000),
  });
  if (!response.ok) throw Error(`HTTP ${response.status}: ${url}`);
  const chunks = [];
  let size = 0;
  for await (const chunk of response.body) {
    size += chunk.length;
    if (size > limit) throw Error("Response exceeds 4 MiB");
    chunks.push(chunk);
  }
  return new TextDecoder("utf-8", { fatal: true }).decode(
    Buffer.concat(chunks),
  );
}
export async function captureAEG(profile, fetcher = fetch) {
  // Keep the raw JSON so duplicate keys are not lost before Go validation.
  return { snapshot: await request(profile.endpoint, fetcher) };
}
export function hmtURLs(calendar) {
  const text = calendar.trim();
  if (!text.startsWith("BEGIN:VCALENDAR") || !text.endsWith("END:VCALENDAR"))
    throw Error("Incomplete calendar");
  // URL discovery only, not an iCalendar parser. Go validates every event and
  // rejects malformed/omitted identities before any candidate can be published.
  const lines = calendar.replace(/\r?\n[ \t]/g, "").split(/\r?\n/);
  const urls = [];
  for (const line of lines) {
    if (!/^URL[;:]/.test(line)) continue;
    const match =
      /^URL(?:;VALUE=URI)?:https?:\/\/holdmyticket\.com\/event\/([1-9][0-9]*)\s*$/.exec(
        line,
      );
    if (!match) throw Error("Unexpected HMT event URL");
    const url = `https://holdmyticket.com/event/${match[1]}`;
    if (urls.includes(url)) throw Error("Duplicate HMT event URL");
    urls.push(url);
    if (urls.length > 1000) throw Error("HMT capture cap exceeded");
  }
  return urls;
}
export async function eventJSONLD(page, html) {
  return page.evaluate((html) => {
    const document = new DOMParser().parseFromString(html, "text/html");
    const events = [];
    function visit(value) {
      if (Array.isArray(value)) return value.forEach(visit);
      if (!value || typeof value !== "object") return;
      if (value["@type"] === "Event") events.push(value);
      if (value["@graph"]) visit(value["@graph"]);
    }
    for (const script of document.querySelectorAll(
      'script[type="application/ld+json"]',
    ))
      visit(JSON.parse(script.textContent));
    if (events.length !== 1)
      throw Error("Expected exactly one Event JSON-LD object");
    return events[0];
  }, html);
}
// HMT emits multi-line TEXT values as raw lines without RFC 5545 escaping.
// Fold those lines into their property so Go validation stays authoritative;
// lines the fold cannot attribute fail closed.
export function foldHMTCalendar(calendar) {
  const newline = /\r\n/.test(calendar) ? "\r\n" : "\n";
  const property = (line) =>
    /^[A-Za-z0-9-]+[;:]/.test(line) && !/^(BEGIN|END):/.test(line);
  const stray = (line) =>
    typeof line === "string" &&
    line !== "" &&
    !property(line) &&
    !/^[ \t]/.test(line) &&
    !/^(BEGIN|END):/.test(line);
  const lines = calendar.split(/\r?\n/);
  const folded = [];
  let component = false;
  let open = false;
  const attribute = () => {
    const previous = folded[folded.length - 1];
    if (!property(previous)) throw Error("Unattributed HMT calendar line");
    return previous;
  };
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (/^(BEGIN|END):[A-Za-z0-9-]+$/.test(line)) {
      component = line.startsWith("BEGIN");
      open = false;
      folded.push(line);
      continue;
    }
    if (component && open && /^[ \t]/.test(line)) {
      folded[folded.length - 1] += line.slice(1);
      continue;
    }
    if (component && open && line === "") {
      folded[folded.length - 1] += "\\n";
      continue;
    }
    if (component && stray(line)) {
      const previous = attribute();
      folded[folded.length - 1] = `${previous}\\n${line}`;
      open = true;
      continue;
    }
    // A blank line before a stray line opens a multi-line value; otherwise
    // the blank lines HMT writes between properties stay as observed.
    if (component && line === "" && stray(lines[i + 1])) {
      folded[folded.length - 1] = `${attribute()}\\n`;
      open = true;
      continue;
    }
    open = false;
    folded.push(line);
  }
  return folded.join(newline);
}
export async function captureHMT(profile, fetcher = fetch, extract) {
  const calendar = foldHMTCalendar(await request(profile.endpoint, fetcher));
  const urls = hmtURLs(calendar);
  const details = {},
    failures = [];
  const rawPages = {};
  for (const url of urls) {
    try {
      const html = await request(url, fetcher);
      rawPages[url] = html;
      details[url.split("/").at(-1)] = await extract(html);
    } catch (error) {
      failures.push({ url, error: String(error) });
    }
  }
  const snapshot = { calendar, details };
  if (Buffer.byteLength(JSON.stringify(snapshot)) > limit)
    throw Error("Snapshot exceeds 4 MiB");
  return { snapshot, rawPages, failures };
}
