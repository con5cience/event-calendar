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
export async function captureHMT(profile, fetcher = fetch, extract) {
  const calendar = await request(profile.endpoint, fetcher);
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
