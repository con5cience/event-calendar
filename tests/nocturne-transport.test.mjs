import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { mkdtempSync, writeFileSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  createNocturneFetcher,
  nocturneFetcher,
} from "../scripts/nocturne-transport.mjs";
import { decodeCurlResponse } from "../scripts/roxy-transport.mjs";

const endpoint = "https://nocturnejazz.com/music?format=ical";
const policy = "https://nocturnejazz.com/faq";
const calendar = (date) =>
  `https://nocturnejazz.com/music?format=ical&date=${date}`;

test("proxied fetch performs one curl attempt per request and decodes calendars", async () => {
  const root = mkdtempSync(join(tmpdir(), "nocturne-curl-fixture-"));
  const countFile = join(root, "count");
  writeFileSync(countFile, "0");
  writeFileSync(
    join(root, "curl"),
    `#!${process.execPath}\nconst fs = require('node:fs');\nconst stdin = fs.readFileSync(0, 'utf8');\nif (!/proxy = "http:\\/\\/proxy\\.example:823"/.test(stdin) || !/proxy-user = "fixture-user:fixture-password"/.test(stdin)) process.exit(7);\nconst url = /url = "(.*)"/.exec(stdin)[1];\nconst path = ${JSON.stringify(countFile)};\nconst count = Number(fs.readFileSync(path, 'utf8'));\nfs.writeFileSync(path, String(count + 1));\nif (url.endsWith('/faq'))\n  process.stdout.write('HTTP/1.1 200 OK\\r\\nContent-Type: text/html\\r\\n\\r\\n<html>policy</html>\\n200');\nelse if (count === 1)\n  process.stdout.write('HTTP/1.1 503 Service Unavailable\\r\\nContent-Type: text/html\\r\\n\\r\\nbusy\\n200');\nelse if (count === 2)\n  process.stdout.write('HTTP/1.1 200 OK\\r\\nContent-Type: text/html\\r\\n\\r\\nchallenge\\n200');\nelse\n  process.stdout.write('HTTP/1.1 200 OK\\r\\nContent-Type: text/calendar; charset=utf-8\\r\\n\\r\\nBEGIN:VCALENDAR\\r\\nEND:VCALENDAR\\n200');\n`,
    { mode: 0o755 },
  );
  const previous = process.env.PATH;
  process.env.PATH = `${root}:${previous}`;
  try {
    const get = createNocturneFetcher(
      endpoint,
      policy,
      "http://fixture-user:fixture-password@proxy.example:823",
    );
    // The transport itself never retries; the capture loop owns retries.
    const good = await get(calendar("2026-09-01"));
    assert.equal(good.status, 200);
    assert.equal(good.headers.get("content-type"), "text/calendar");
    assert.equal(await good.text(), "BEGIN:VCALENDAR\r\nEND:VCALENDAR");
    assert.equal(readFileSync(countFile, "utf8"), "1");
    const busy = await get(calendar("2026-10-01"));
    assert.equal(busy.status, 503);
    assert.equal(busy.headers.get("content-type"), "text/html");
    assert.equal(readFileSync(countFile, "utf8"), "2");
    const interstitial = await get(calendar("2026-11-01"));
    assert.equal(interstitial.status, 200);
    assert.equal(interstitial.headers.get("content-type"), "text/html");
    assert.equal(readFileSync(countFile, "utf8"), "3");
    const faq = await get(policy);
    assert.equal(faq.status, 200);
    assert.equal(faq.headers.get("content-type"), "text/html");
    assert.equal(await faq.text(), "<html>policy</html>");
    assert.equal(readFileSync(countFile, "utf8"), "4");
  } finally {
    process.env.PATH = previous;
  }
});

test("proxied fetch rejects unreviewed destinations and redacts proxy failures", async () => {
  const proxy = createServer();
  let calls = 0;
  proxy.on("connect", (req, socket) => {
    calls++;
    assert.equal(req.url, "nocturnejazz.com:443");
    socket.end(
      "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n",
    );
  });
  proxy.listen(0, "127.0.0.1");
  await once(proxy, "listening");
  const fetcher = createNocturneFetcher(
    endpoint,
    policy,
    `http://fixture-user:fixture-password@127.0.0.1:${proxy.address().port}`,
  );
  try {
    for (const url of [
      "https://example.com/",
      "https://nocturnejazz.com/music?format=ical",
      "https://nocturnejazz.com/music?format=ical&date=2026-9-1",
      "https://nocturnejazz.com/music?format=ical&date=2026-09-01&extra=1",
      "https://nocturnejazz.com/faq?edit=1",
      "https://nocturnejazz.com/other",
    ])
      await assert.rejects(fetcher(url), /unreviewed/i);
    assert.equal(calls, 0);
    await assert.rejects(
      fetcher(calendar("2026-09-01"), { signal: AbortSignal.abort() }),
      /aborted/,
    );
    assert.equal(calls, 0);
    await assert.rejects(
      fetcher(calendar("2026-09-01")),
      (error) =>
        !String(error).includes("fixture") && /curl/i.test(error.message),
    );
    assert.equal(calls, 1);
  } finally {
    await new Promise((resolve) => proxy.close(resolve));
  }
});

test("proxied fetch fails closed on unconfirmed tunnels and non-GET methods", async () => {
  const root = mkdtempSync(join(tmpdir(), "nocturne-curl-fixture-"));
  writeFileSync(
    join(root, "curl"),
    `#!${process.execPath}\nconst fs = require('node:fs');\nfs.readFileSync(0);\nprocess.stdout.write('HTTP/1.1 200 OK\\r\\nContent-Type: text/calendar\\r\\n\\r\\nBEGIN:VCALENDAR\\n407');\n`,
    { mode: 0o755 },
  );
  const previous = process.env.PATH;
  process.env.PATH = `${root}:${previous}`;
  try {
    const fetcher = createNocturneFetcher(
      endpoint,
      policy,
      "http://fixture-user:fixture-password@proxy.example:823",
    );
    await assert.rejects(
      fetcher(calendar("2026-09-01")),
      /tunnel was not confirmed/,
    );
    await assert.rejects(
      fetcher(calendar("2026-09-01"), { method: "POST" }),
      /Unsupported/,
    );
  } finally {
    process.env.PATH = previous;
  }
});

test("curl response keeps calendar and HTML types and rejects unsafe input", async () => {
  const types = ["text/calendar", "text/html"];
  for (const type of types) {
    const response = decodeCurlResponse(
      Buffer.from(`HTTP/1.1 200 OK\r\nContent-Type: ${type}\r\n\r\nbody`),
      types,
    );
    assert.equal(response.headers.get("content-type"), type);
    assert.equal(await response.text(), "body");
  }
  assert.equal(
    decodeCurlResponse(
      Buffer.from(
        "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n\r\n{}",
      ),
      types,
    ).headers.get("content-type"),
    "application/octet-stream",
  );
  assert.equal(
    decodeCurlResponse(
      Buffer.from(
        "HTTP/1.1 202 Accepted\r\nContent-Type: text/html\r\nx-amzn-waf-action: challenge\r\n\r\n",
      ),
      types,
    ).headers.get("x-amzn-waf-action"),
    "challenge",
  );
  for (const bytes of [
    Buffer.from(
      "HTTP/1.1 302 Found\r\nLocation: https://other.example\r\n\r\n",
    ),
    Buffer.from("bad"),
    Buffer.concat([
      Buffer.from("HTTP/1.1 200 OK\r\nContent-Type: text/calendar\r\n\r\n"),
      Buffer.alloc(1024 * 1024 + 1),
    ]),
  ])
    assert.throws(() => decodeCurlResponse(bytes, types));
});

test("proxy selection is explicit and required configuration fails closed", () => {
  const direct = () => {};
  assert.equal(nocturneFetcher(endpoint, policy, {}, direct), direct);
  assert.throws(
    () =>
      nocturneFetcher(endpoint, policy, { ROXY_PROXY_REQUIRED: "1" }, direct),
    /required/i,
  );
  assert.throws(
    () => nocturneFetcher(endpoint, policy, { ROXY_PROXY_URL: "bad" }, direct),
    /proxy/i,
  );
  for (const [badEndpoint, badPolicy] of [
    ["https://example.com/music?format=ical", policy],
    ["https://nocturnejazz.com/music", policy],
    ["https://nocturnejazz.com/music?format=ical&other=1", policy],
    [endpoint, "https://nocturnejazz.com/faq?x=1"],
    [endpoint, "https://nocturnejazz.com/other"],
  ])
    assert.throws(
      () => createNocturneFetcher(badEndpoint, badPolicy, "http://u:p@proxy:1"),
      /Unreviewed/,
    );
});
