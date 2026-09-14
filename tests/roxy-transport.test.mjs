import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import {
  createRoxyFetcher,
  decodeCurlResponse,
  roxyFetcher,
  withTransportRetries,
} from "../scripts/roxy-transport.mjs";
import { execute } from "../scripts/refresh-locale.mjs";

test("transport retries are capped and share one 30-second deadline", async () => {
  let time = 0,
    calls = 0;
  const budgets = [];
  const value = await withTransportRetries(
    async (remaining) => {
      budgets.push(remaining);
      calls++;
      time += 4000;
      if (calls < 3)
        throw Object.assign(Error("transport"), {
          curlExitCode: 35,
          proxyConnectStatus: 200,
        });
      return "ok";
    },
    undefined,
    () => time,
  );
  assert.equal(value, "ok");
  assert.deepEqual(budgets, [30000, 26000, 22000]);
  calls = 0;
  await assert.rejects(
    withTransportRetries(async () => {
      calls++;
      throw Object.assign(Error("transport"), {
        curlExitCode: 28,
        proxyConnectStatus: 0,
      });
    }),
  );
  assert.equal(calls, 3);
  calls = 0;
  time = 0;
  await assert.rejects(
    withTransportRetries(
      async () => {
        calls++;
        time = 30000;
        throw Object.assign(Error("transport"), {
          curlExitCode: 28,
          proxyConnectStatus: 0,
        });
      },
      undefined,
      () => time,
    ),
  );
  assert.equal(calls, 1);
});

test("authentication, validation, certificate failures and cancellation do not retry", async () => {
  for (const error of [
    Error("invalid JSON"),
    Object.assign(Error("auth"), { curlExitCode: 56, proxyConnectStatus: 407 }),
    Object.assign(Error("proxy rejection"), {
      curlExitCode: 56,
      proxyConnectStatus: 502,
    }),
    Object.assign(Error("certificate"), {
      curlExitCode: 60,
      proxyConnectStatus: 200,
    }),
  ]) {
    let calls = 0;
    await assert.rejects(
      withTransportRetries(async () => {
        calls++;
        throw error;
      }),
    );
    assert.equal(calls, 1);
  }
  let calls = 0;
  await assert.rejects(
    withTransportRetries(async () => {
      calls++;
    }, AbortSignal.abort()),
    /aborted/,
  );
  assert.equal(calls, 0);
});

test("curl response preserves bounded bytes and only safe headers", async () => {
  const response = decodeCurlResponse(
    Buffer.from(
      'HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nSet-Cookie: secret\r\n\r\n{"data":[]}',
    ),
  );
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { data: [] });
  assert.equal(response.headers.get("set-cookie"), null);
  await assert.rejects(
    decodeCurlResponse(
      Buffer.from(
        "HTTP/2 200\r\nContent-Type: application/json\r\n\r\nnot JSON",
      ),
    ).json(),
  );
  assert.throws(
    () =>
      decodeCurlResponse(
        Buffer.from(
          "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Type: text/html\r\n\r\n{}",
        ),
      ),
    /duplicate/i,
  );
  assert.throws(
    () =>
      decodeCurlResponse(
        Buffer.from(
          "HTTP/1.1 200 OK\r\nX-Large: " + "x".repeat(16384) + "\r\n\r\n",
        ),
      ),
    /headers/i,
  );
  assert.throws(
    () =>
      decodeCurlResponse(
        Buffer.from(
          "HTTP/1.1 302 Found\r\nLocation: https://other.example\r\n\r\n",
        ),
      ),
    /redirect/i,
  );
  assert.throws(() => decodeCurlResponse(Buffer.from("bad")), /headers/i);
  assert.throws(
    () =>
      decodeCurlResponse(
        Buffer.concat([
          Buffer.from("HTTP/1.1 200 OK\r\n\r\n"),
          Buffer.alloc(1024 * 1024 + 1),
        ]),
      ),
    /size/i,
  );
});

test("Roxy proxy selection is explicit and required configuration fails closed", () => {
  const direct = () => {};
  const endpoint = "https://aftontickets.com/api/get-events?key=fixture&page=";
  assert.equal(roxyFetcher(endpoint, {}, direct), direct);
  assert.throws(
    () => roxyFetcher(endpoint, { ROXY_PROXY_REQUIRED: "1" }, direct),
    /required/i,
  );
  assert.throws(
    () => roxyFetcher(endpoint, { ROXY_PROXY_URL: "bad" }, direct),
    /proxy/i,
  );
});

test("proxied fetch rejects unreviewed destinations and redacts proxy failures", async () => {
  const proxy = createServer();
  let calls = 0;
  proxy.on("connect", (req, socket) => {
    calls++;
    assert.equal(req.url, "aftontickets.com:443");
    socket.end(
      "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n",
    );
  });
  proxy.listen(0, "127.0.0.1");
  await once(proxy, "listening");
  const endpoint = "https://aftontickets.com/api/get-events?key=fixture&page=";
  const fetcher = createRoxyFetcher(
    endpoint,
    `http://fixture-user:fixture-password@127.0.0.1:${proxy.address().port}`,
  );
  try {
    for (const url of [
      "https://example.com/",
      endpoint + "1&other=1",
      "https://aftontickets.com/event/buyticket/bad",
    ])
      await assert.rejects(fetcher(url), /unreviewed/i);
    assert.equal(calls, 0);
    await assert.rejects(
      fetcher(endpoint + "1", { signal: AbortSignal.abort() }),
      /aborted/,
    );
    assert.equal(calls, 0);
    for (const url of [
      endpoint + "1",
      "https://aftontickets.com/event/buyticket/3px8g401j1",
    ]) {
      await assert.rejects(
        fetcher(url),
        (error) =>
          !String(error).includes("fixture") && /curl/i.test(error.message),
      );
    }
    assert.equal(calls, 2);
  } finally {
    await new Promise((resolve) => proxy.close(resolve));
  }
});

test("unrelated subprocesses do not inherit the Roxy proxy secret", async () => {
  const previous = process.env.ROXY_PROXY_URL;
  process.env.ROXY_PROXY_URL = "fixture-secret";
  try {
    const script =
      'console.log(process.env.ROXY_PROXY_URL ? "present" : "absent")';
    assert.equal(
      (await execute(process.execPath, ["-e", script])).trim(),
      "absent",
    );
    assert.equal(
      (
        await execute(process.execPath, ["-e", script], {
          env: { ROXY_PROXY_URL: "fixture-secret" },
        })
      ).trim(),
      "present",
    );
  } finally {
    if (previous === undefined) delete process.env.ROXY_PROXY_URL;
    else process.env.ROXY_PROXY_URL = previous;
  }
});
