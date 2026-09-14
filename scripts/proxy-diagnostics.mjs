// Read-only operator diagnostics. Never print raw errors, URLs, headers, or bodies.
import { spawn, spawnSync } from "node:child_process";
import { lookup } from "node:dns/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { request } from "@playwright/test";
import { captureProfile } from "../tests/capture-profile.mjs";
import { probeRoxyProxy } from "./probe-roxy-user-agent.mjs";

export function proxyConfiguration(value) {
  if (
    typeof value !== "string" ||
    value !== value.trim() ||
    /[\r\n\0]/.test(value)
  )
    throw Error("Invalid proxy configuration");
  const url = new URL(value);
  if (
    !["http:", "https:"].includes(url.protocol) ||
    url.pathname !== "/" ||
    url.search ||
    url.hash
  )
    throw Error("Invalid proxy configuration");
  const username = decodeURIComponent(url.username);
  const password = decodeURIComponent(url.password);
  if (
    [...(username + password)].some(
      (char) => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127,
    )
  )
    throw Error("Invalid proxy configuration");
  return {
    server: url.origin,
    hostname: url.hostname.replace(/^\[|\]$/g, ""),
    protocol: url.protocol,
    username,
    password,
  };
}

const numeric = (value) =>
  typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : 0;

export function traceMilestones(trace) {
  return {
    tcp_connected: /^\* Connected to /m.test(trace),
    connect_sent: /^> CONNECT [^\r\n]+ HTTP\/1\.[01]\r?$/m.test(trace),
    request_sent: /^> GET [^\r\n]+ HTTP\/(?:1\.[01]|2|3)\r?$/m.test(trace),
  };
}

export function curlReport(
  exitCode,
  output,
  protocol,
  milestones = {},
  trace = "",
) {
  const direct = protocol === null;
  let raw;
  try {
    raw = JSON.parse(output);
  } catch {
    return { failure_stage: "diagnostic_output" };
  }
  if (!raw || typeof raw !== "object" || Array.isArray(raw))
    return { failure_stage: "diagnostic_output" };
  const code = numeric(raw.http_code);
  const connect = numeric(raw.http_connect);
  const tcp = numeric(raw.time_connect);
  const tcpConnected =
    tcp > 0 ||
    milestones.tcp_connected === true ||
    milestones.connect_sent === true ||
    connect > 0;
  const tls = numeric(raw.time_appconnect);
  const tlsCompleted =
    (direct || connect === 200) &&
    (tls > 0 || milestones.request_sent === true);
  let failure = null;
  if (direct && (connect || milestones.connect_sent))
    failure = "unexpected_proxy";
  else if (direct && exitCode === 6) failure = "destination_dns";
  else if (direct && exitCode === 7) failure = "destination_tcp";
  else if (connect === 407) failure = "proxy_authentication";
  else if (connect && connect !== 200) failure = "proxy_connect_rejected";
  else if (exitCode === 5) failure = "proxy_dns";
  else if (exitCode === 7) failure = "proxy_tcp";
  else if (exitCode !== 0 || !code || (!direct && connect !== 200)) {
    if (!tcpConnected)
      failure = direct ? "destination_dns_or_tcp" : "proxy_dns_or_tcp";
    else if (!direct && connect !== 200)
      failure =
        protocol === "https:" && !milestones.connect_sent
          ? "proxy_tls_or_connect"
          : "proxy_connect";
    else if (!tlsCompleted) failure = "destination_tls";
    else failure = code ? "response_body" : "http_response";
  }
  return {
    exit_code: Number.isInteger(exitCode) ? exitCode : null,
    http_status: code || null,
    content_type: ["application/json", "text/html"].includes(
      raw.content_type?.split(";", 1)[0].trim().toLowerCase(),
    )
      ? raw.content_type.split(";", 1)[0].trim().toLowerCase()
      : "other",
    challenge: /^< x-amzn-waf-action:\s*challenge\r?$/im.test(trace)
      ? "challenge"
      : null,
    cf_mitigated: /^< cf-mitigated:\s*challenge\r?$/im.test(trace)
      ? "challenge"
      : null,
    connect_status: connect || null,
    proxy_tunnel_confirmed: connect === 200,
    tcp_connected: tcpConnected,
    connect_sent: milestones.connect_sent === true || connect > 0,
    destination_tls_completed: tlsCompleted,
    request_sent: milestones.request_sent === true,
    failure_stage: failure,
    times_ms: Object.fromEntries(
      ["namelookup", "connect", "appconnect", "starttransfer", "total"].map(
        (name) => [name, Math.round(numeric(raw[`time_${name}`]) * 1000)],
      ),
    ),
  };
}

function cleanEnvironment() {
  const env = { ...process.env };
  for (const key of Object.keys(env)) {
    if (
      /proxy/i.test(key) ||
      [
        "NODE_DEBUG",
        "DEBUG",
        "SSLKEYLOGFILE",
        "CURL_HOME",
        "CURL_CA_BUNDLE",
        "SSL_CERT_FILE",
        "SSL_CERT_DIR",
        "NODE_EXTRA_CA_CERTS",
        "NODE_TLS_REJECT_UNAUTHORIZED",
      ].includes(key)
    )
      delete env[key];
  }
  return env;
}

// curl config uses its documented quoting rules, not shell escaping. The secret
// goes through stdin, never argv, a temporary file, or a verbose log.
const quote = (value) =>
  '"' + value.replaceAll("\\", "\\\\").replaceAll('"', '\\"') + '"';
export function curlCommand(
  endpoint,
  proxy,
  family = "default",
  timeoutSeconds = 30,
  userAgent,
) {
  const config =
    [
      ...(proxy
        ? [
            `proxy = ${quote(proxy.server)}`,
            `proxy-user = ${quote(proxy.username + ":" + proxy.password)}`,
          ]
        : []),
      `url = ${quote(endpoint)}`,
    ].join("\n") + "\n";
  const args = [
    "--disable",
    "--config",
    "-",
    "--silent",
    "--verbose",
    "--noproxy",
    proxy ? "" : "*",
    "--proxy-basic",
    "--proto",
    "=https",
    "--max-time",
    String(timeoutSeconds),
    "--connect-timeout",
    String(Math.min(timeoutSeconds, 15)),
    "--max-filesize",
    "1048576",
    "--output",
    "/dev/null",
    "--write-out",
    "%{json}",
  ];
  if (family === "ipv4") args.push("--ipv4");
  if (family === "ipv6") args.push("--ipv6");
  if (!proxy) args.push("--proxy", "");
  if (userAgent) args.push("--user-agent", userAgent);
  return { args, config };
}

export async function curlProbe(
  endpoint,
  proxy,
  family = "default",
  timeoutSeconds = 30,
  userAgent,
) {
  const { args, config } = curlCommand(
    endpoint,
    proxy,
    family,
    timeoutSeconds,
    userAgent,
  );
  return new Promise((resolveResult) => {
    let output = "";
    let trace = "";
    let unavailable = false;
    const child = spawn("curl", args, {
      env: cleanEnvironment(),
      stdio: ["pipe", "pipe", "pipe"],
    });
    const timer = setTimeout(
      () => child.kill("SIGKILL"),
      (timeoutSeconds + 5) * 1000,
    );
    child.on("error", () => {
      unavailable = true;
    });
    child.stdin.on("error", () => {});
    // Raw trace may contain credentials. Keep it bounded in memory, never log
    // or persist it; only traceMilestones' fixed booleans can leave this function.
    child.stderr.on("data", (chunk) => {
      if (trace.length + chunk.length <= 65536) trace += chunk.toString();
      else child.kill("SIGKILL");
    });
    child.stdout.on("data", (chunk) => {
      if (output.length + chunk.length <= 65536) output += chunk.toString();
      else child.kill("SIGKILL");
    });
    child.on("close", (code) => {
      clearTimeout(timer);
      resolveResult(
        unavailable
          ? { failure_stage: "curl_unavailable" }
          : curlReport(
              code,
              output,
              proxy?.protocol ?? null,
              traceMilestones(trace),
              trace,
            ),
      );
    });
    child.stdin.end(config);
  });
}

// Browser-style UA only: no browser TLS fingerprint, cookies, or client hints.
const browserUserAgent =
  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36";
export async function runDirectDiagnostics(
  endpoint,
  emit = (row) => console.log(JSON.stringify(row)),
) {
  const target = new URL(endpoint);
  if (
    target.origin !== "https://aftontickets.com" ||
    target.pathname !== "/api/get-events" ||
    target.username ||
    target.password
  )
    throw Error("Invalid target");
  for (const [userAgentMode, userAgent] of [
    ["default", undefined],
    ["browser", browserUserAgent],
  ]) {
    const labels = {
      client: "curl",
      target: "roxy",
      mode: "direct",
      user_agent_mode: userAgentMode,
    };
    emit({ kind: "start", ...labels });
    emit({
      kind: "result",
      ...labels,
      ...(userAgent ? { user_agent: userAgent } : {}),
      ...(await curlProbe(endpoint, null, "default", 30, userAgent)),
    });
  }
  emit({ kind: "complete", mode: "direct" });
}

async function dnsProbe(hostname, family) {
  const started = performance.now();
  let timer;
  try {
    const rows = await Promise.race([
      lookup(hostname, { family, all: true }),
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(Error("timeout")), 5000);
      }),
    ]);
    return {
      family: `ipv${family}`,
      resolved: rows.some((row) => row.family === family),
      elapsed_ms: Math.round(performance.now() - started),
    };
  } catch {
    return {
      family: `ipv${family}`,
      resolved: false,
      elapsed_ms: Math.round(performance.now() - started),
    };
  } finally {
    clearTimeout(timer);
  }
}

export async function runDiagnostics(
  proxyURL,
  endpoint,
  emit = (row) => console.log(JSON.stringify(row)),
) {
  const proxy = proxyConfiguration(proxyURL);
  const target = new URL(endpoint);
  if (
    target.origin !== "https://aftontickets.com" ||
    target.pathname !== "/api/get-events" ||
    target.username ||
    target.password
  )
    throw Error("Invalid target");
  emit({
    kind: "configuration",
    proxy_scheme: proxy.protocol.slice(0, -1),
    credentials_present: Boolean(proxy.username && proxy.password),
    node: process.version,
    platform: process.platform,
    arch: process.arch,
    curl_version:
      spawnSync("curl", ["--version"], {
        env: cleanEnvironment(),
        encoding: "utf8",
        timeout: 5000,
      }).stdout?.match(/^curl (\d+\.\d+\.\d+)/)?.[1] ?? null,
  });
  for (const family of [4, 6])
    emit({ kind: "proxy_dns", ...(await dnsProbe(proxy.hostname, family)) });
  // Sequential, bounded requests avoid conflating provider concurrency limits
  // with reachability. Each completed result is available before the next starts.
  for (const [name, url] of [
    ["neutral", "https://example.com/"],
    ["roxy", endpoint],
  ]) {
    emit({ kind: "start", client: "playwright", target: name });
    const started = performance.now();
    const api = {
      newContext: async (options) => {
        const context = await request.newContext(options);
        return {
          get: (_url, settings) => context.get(url, settings),
          dispose: () => context.dispose(),
        };
      },
    };
    const result = await probeRoxyProxy(endpoint, proxyURL, api);
    emit({
      kind: "result",
      client: "playwright",
      target: name,
      ...result,
      elapsed_ms: Math.round(performance.now() - started),
    });
    for (const family of ["default", "ipv4", "ipv6"]) {
      emit({ kind: "start", client: "curl", target: name, family });
      emit({
        kind: "result",
        client: "curl",
        target: name,
        family,
        ...(await curlProbe(url, proxy, family)),
      });
    }
  }
  emit({ kind: "complete" });
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const proxyURL = process.env.HTTP_PROXY;
  // Prevent implicit proxy bypass/debugging or inherited TLS overrides.
  const cleaned = cleanEnvironment();
  for (const key of Object.keys(process.env))
    if (!(key in cleaned)) delete process.env[key];
  try {
    if (
      process.argv.length > 3 ||
      (process.argv[2] && process.argv[2] !== "--direct")
    )
      throw Error("Invalid arguments");
    const endpoint = captureProfile("roxy").endpoint + "1";
    if (process.argv[2] === "--direct") await runDirectDiagnostics(endpoint);
    else await runDiagnostics(proxyURL, endpoint);
  } catch {
    console.log(
      JSON.stringify({
        kind: "diagnostic_error",
        category: "configuration_or_setup",
      }),
    );
    process.exitCode = 1;
  }
}
