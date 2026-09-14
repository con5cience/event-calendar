import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { createServer as createTCPServer } from "node:net";
import { once } from "node:events";
import { execute } from "../scripts/refresh-locale.mjs";
import {
  curlReport,
  curlProbe,
  proxyConfiguration,
  traceMilestones,
  curlCommand,
} from "../scripts/proxy-diagnostics.mjs";

test("direct curl disables every proxy route and only the user agent differs", () => {
  const plain = curlCommand("https://example.com/", null);
  const browser = curlCommand(
    "https://example.com/",
    null,
    "default",
    30,
    "fixture-browser",
  );
  assert.equal(plain.args[plain.args.indexOf("--proxy") + 1], "");
  assert.equal(plain.args[plain.args.indexOf("--noproxy") + 1], "*");
  assert(!plain.config.includes("proxy-user"));
  assert.deepEqual(browser.args, [
    ...plain.args,
    "--user-agent",
    "fixture-browser",
  ]);
  assert.equal(browser.config, plain.config);
});

test("direct reports accept HTTPS without CONNECT and normalize challenge headers", () => {
  const result = curlReport(
    0,
    JSON.stringify({
      http_code: 202,
      time_connect: 0.1,
      time_appconnect: 0.2,
      content_type: "text/html; secret=value",
    }),
    null,
    {},
    "< x-amzn-waf-action: challenge\r\n< Set-Cookie: secret\r\n",
  );
  assert.equal(result.failure_stage, null);
  assert.equal(result.content_type, "text/html");
  assert.equal(result.challenge, "challenge");
  assert.equal(result.proxy_tunnel_confirmed, false);
  assert.equal(result.destination_tls_completed, true);
  assert(!JSON.stringify(result).includes("secret"));
  assert.equal(curlReport(6, "{}", null).failure_stage, "destination_dns");
  assert.equal(curlReport(7, "{}", null).failure_stage, "destination_tcp");
});

test("direct curl reaches the destination despite inherited proxy variables", async () => {
  let proxyRequests = 0,
    destinationConnections = 0;
  const sockets = new Set();
  const destination = createTCPServer((socket) => {
    destinationConnections++;
    sockets.add(socket);
    socket.on("close", () => sockets.delete(socket));
  });
  const proxy = createServer();
  proxy.on("connect", (_request, socket) => {
    proxyRequests++;
    socket.end("HTTP/1.1 407 Proxy Authentication Required\r\n\r\n");
  });
  destination.listen(0, "127.0.0.1");
  proxy.listen(0, "127.0.0.1");
  await Promise.all([once(destination, "listening"), once(proxy, "listening")]);
  const keys = [
    "HTTP_PROXY",
    "HTTPS_PROXY",
    "ALL_PROXY",
    "http_proxy",
    "https_proxy",
    "all_proxy",
  ];
  const previous = keys.map((key) => process.env[key]);
  try {
    for (const key of keys)
      process.env[key] =
        `http://fixture-user:fixture-password@127.0.0.1:${proxy.address().port}`;
    const result = await curlProbe(
      `https://127.0.0.1:${destination.address().port}/`,
      null,
      "default",
      1,
    );
    assert.equal(result.failure_stage, "destination_tls");
    assert.equal(destinationConnections, 1);
    assert.equal(proxyRequests, 0);
    assert.equal(result.connect_sent, false);
  } finally {
    keys.forEach((key, i) => {
      if (previous[i] === undefined) delete process.env[key];
      else process.env[key] = previous[i];
    });
    for (const socket of sockets) socket.destroy();
    await Promise.all([
      new Promise((resolve) => destination.close(resolve)),
      new Promise((resolve) => proxy.close(resolve)),
    ]);
  }
});

test("proxy configuration rejects malformed values and separates credentials", () => {
  const config = proxyConfiguration("http://fixture-user:p%40ss@localhost:823");
  assert.equal(config.server, "http://localhost:823");
  assert.equal(config.password, "p@ss");
  for (const value of [
    undefined,
    "<url>",
    '"http://localhost"',
    "socks5://localhost",
    "http://localhost/path",
    "http://u:%ZZ@localhost",
    "http://u:p%0Asecret@localhost",
  ])
    assert.throws(() => proxyConfiguration(value));
});

test("curl report identifies confirmed milestones and preserves uncertain stages", () => {
  const cases = [
    [5, {}, "proxy_dns"],
    [7, {}, "proxy_tcp"],
    [28, {}, "proxy_dns_or_tcp"],
    [28, { time_connect: 0.1 }, "proxy_connect"],
    [56, { time_connect: 0.1, http_connect: 407 }, "proxy_authentication"],
    [56, { time_connect: 0.1, http_connect: 502 }, "proxy_connect_rejected"],
    [60, { time_connect: 0.1, http_connect: 200 }, "destination_tls"],
    [
      28,
      { time_connect: 0.1, http_connect: 200, time_appconnect: 0.2 },
      "http_response",
    ],
    [
      0,
      {
        time_connect: 0.1,
        http_connect: 200,
        time_appconnect: 0.2,
        http_code: 200,
      },
      null,
    ],
  ];
  for (const [exit, values, expected] of cases) {
    const result = curlReport(
      exit,
      JSON.stringify({
        ...values,
        url_effective: "secret",
        errormsg: "secret",
      }),
      "http:",
    );
    assert.equal(result.failure_stage, expected);
    assert(!JSON.stringify(result).includes("secret"));
  }
  assert.equal(
    curlReport(28, '{"time_connect":0.1}', "https:").failure_stage,
    "proxy_tls_or_connect",
  );
  assert.equal(
    curlReport(0, "unparseable secret", "http:").failure_stage,
    "diagnostic_output",
  );
});

test("trace parser retains only fixed milestones, not credentials or arbitrary response text", () => {
  const marks = traceMilestones(
    "* Connected to secret (secret) port 823\n> CONNECT example.com:443 HTTP/1.1\r\n> Proxy-Authorization: Basic secret\r\n< X-Untrusted: * Connected to fake\r\n",
  );
  assert.deepEqual(marks, {
    tcp_connected: true,
    connect_sent: true,
    request_sent: false,
  });
  assert(!JSON.stringify(marks).includes("secret"));
  assert.equal(
    curlReport(28, "{}", "http:", marks).failure_stage,
    "proxy_connect",
  );
});

test("real curl handles proxy authentication rejection, tunnel timeout, and refusal safely", async () => {
  let mode = "reject";
  const requests = [];
  const sockets = new Set();
  const server = createServer();
  server.on("connect", (req, socket) => {
    sockets.add(socket);
    socket.on("close", () => sockets.delete(socket));
    requests.push(req.headers["proxy-authorization"]);
    if (mode === "reject")
      socket.end(
        "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n",
      );
    if (mode === "tls-stall")
      socket.write("HTTP/1.1 200 Connection Established\r\n\r\n");
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const proxy = proxyConfiguration(
    `http://fixture-user:fixture-password@127.0.0.1:${server.address().port}`,
  );
  try {
    const rejected = await curlProbe("https://example.com/", proxy, "ipv4", 1);
    assert.equal(rejected.failure_stage, "proxy_authentication");
    assert.equal(
      requests[0],
      "Basic " +
        Buffer.from("fixture-user:fixture-password").toString("base64"),
    );
    mode = "stall";
    const stalled = await curlProbe("https://example.com/", proxy, "ipv4", 1);
    assert.equal(stalled.failure_stage, "proxy_connect");
    mode = "tls-stall";
    const tlsStalled = await curlProbe(
      "https://example.com/",
      proxy,
      "ipv4",
      1,
    );
    assert.equal(tlsStalled.failure_stage, "destination_tls");
    for (const result of [rejected, stalled, tlsStalled]) {
      assert(!JSON.stringify(result).includes("fixture"));
      assert(!JSON.stringify(result).includes("127.0.0.1"));
    }
  } finally {
    for (const socket of sockets) socket.destroy();
    await new Promise((resolve) => server.close(resolve));
  }
  const refused = await curlProbe("https://example.com/", proxy, "ipv4", 1);
  assert.equal(refused.failure_stage, "proxy_tcp");
});

test("standalone workflow does not refresh, deploy, or expose the proxy in arguments", () => {
  const workflow = readFileSync(
    new URL("../.github/workflows/proxy-diagnostics.yml", import.meta.url),
    "utf8",
  );
  assert.match(workflow, /workflow_dispatch/);
  assert.match(workflow, /contents: read/);
  assert.match(workflow, /HTTP_PROXY: \$\{\{ secrets.HTTP_PROXY \}\}/);
  assert.match(workflow, /-e HTTP_PROXY/);
  assert(!workflow.includes("$HTTP_PROXY"));
  assert(!workflow.includes("refresh-locale.mjs"));
  assert(!workflow.includes("railway"));
  assert.match(workflow, /if: always\(\)/);
  const directStep = workflow
    .split("      - name: Diagnose direct curl\n")[1]
    .split("      - name: Diagnose proxy\n")[0];
  assert.match(directStep, /--direct/);
  assert(!directStep.includes("HTTP_PROXY"));
  assert(!directStep.includes("secrets."));
});

test("CLI streams both clients and families and does not leak proxy credentials", async () => {
  const server = createServer();
  server.on("connect", (_req, socket) =>
    socket.end(
      "HTTP/1.1 407 Proxy Authentication Required\r\nContent-Length: 0\r\n\r\n",
    ),
  );
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  let stderr = "";
  const observed = [];
  try {
    const output = await execute(
      process.execPath,
      [new URL("../scripts/proxy-diagnostics.mjs", import.meta.url).pathname],
      {
        env: {
          HTTP_PROXY: `http://fixture-user:fixture-password@127.0.0.1:${server.address().port}`,
          SITE_DIR:
            process.env.SITE_DIR ||
            new URL("../locales/denver", import.meta.url).pathname,
        },
        onStdout: (chunk) => observed.push(chunk),
        onStderr: (chunk) => {
          stderr += chunk;
        },
      },
    );
    const rows = output
      .trim()
      .split("\n")
      .map((row) => JSON.parse(row));
    assert.equal(rows.at(-1).kind, "complete");
    for (const target of ["neutral", "roxy"]) {
      const results = rows.filter(
        (row) => row.kind === "result" && row.target === target,
      );
      assert.deepEqual(
        results.map((row) => row.client),
        ["playwright", "curl", "curl", "curl"],
      );
      assert.equal(results[0].status, 407);
      assert.equal(results[1].failure_stage, "proxy_authentication");
      assert.deepEqual(
        results.slice(1).map((row) => row.family),
        ["default", "ipv4", "ipv6"],
      );
    }
    assert(observed.length > 1);
    assert(!output.includes("fixture"));
    assert(!output.includes("127.0.0.1"));
    assert(
      !output.includes(
        Buffer.from("fixture-user:fixture-password").toString("base64"),
      ),
    );
    assert.equal(stderr, "");
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
});
