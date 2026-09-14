import { spawn } from "node:child_process";
import {
  proxyConfiguration,
  cleanEnvironment,
  quote,
} from "./proxy-config.mjs";

const bodyLimit = 1024 * 1024;
const headerLimit = 16384;

export async function withTransportRetries(
  attempt,
  signal,
  clock = () => performance.now(),
) {
  const deadline = clock() + 30000;
  for (let count = 0; count < 3; count++) {
    if (signal?.aborted) throw Error("curl request aborted");
    const remaining = deadline - clock();
    if (remaining <= 0) throw Error("curl request timeout");
    try {
      return await attempt(remaining);
    } catch (error) {
      if (
        signal?.aborted ||
        count === 2 ||
        clock() >= deadline ||
        ![7, 28, 35, 52, 56].includes(error.curlExitCode) ||
        ![0, 200].includes(error.proxyConnectStatus)
      )
        throw error;
      console.error(
        `roxy transport: retry ${count + 1}/2 after curl exit ${error.curlExitCode}`,
      );
    }
  }
}

export function decodeCurlResponse(bytes) {
  let rest = bytes,
    headerBytes = 0;
  for (;;) {
    const end = rest.indexOf("\r\n\r\n");
    if (end < 0 || (headerBytes += end + 4) > headerLimit)
      throw Error("curl response headers invalid or oversized");
    const lines = rest.subarray(0, end).toString("latin1").split("\r\n");
    const match = /^HTTP\/(?:1\.[01]|2|3) ([1-5][0-9]{2})(?: |$)/.exec(
      lines.shift(),
    );
    if (!match) throw Error("curl response headers invalid");
    rest = rest.subarray(end + 4);
    const status = Number(match[1]);
    if (status < 200) continue;
    if (status >= 300 && status < 400) throw Error("curl redirect rejected");
    if (rest.length > bodyLimit)
      throw Error("curl response size limit exceeded");
    const headers = new Headers();
    for (const line of lines) {
      const colon = line.indexOf(":");
      if (colon < 1) throw Error("curl response headers invalid");
      const key = line.slice(0, colon).toLowerCase(),
        value = line.slice(colon + 1).trim();
      if (key === "content-type") {
        const mime = value.split(";", 1)[0].trim().toLowerCase();
        if (headers.has(key)) throw Error("curl duplicate content type");
        headers.set(
          key,
          ["application/json", "text/html"].includes(mime)
            ? mime
            : "application/octet-stream",
        );
      }
      if (
        ["x-amzn-waf-action", "cf-mitigated"].includes(key) &&
        value === "challenge"
      )
        headers.set(key, "challenge");
    }
    return new Response(status === 204 || status === 205 ? null : rest, {
      status,
      headers,
    });
  }
}

export function createRoxyFetcher(endpoint, proxyURL) {
  let proxy;
  try {
    proxy = proxyConfiguration(proxyURL);
  } catch {
    throw Error("Invalid Roxy proxy configuration");
  }
  const listing = new URL(endpoint);
  if (
    listing.origin !== "https://aftontickets.com" ||
    listing.pathname !== "/api/get-events" ||
    listing.username ||
    listing.password ||
    !endpoint.endsWith("page=")
  )
    throw Error("Unreviewed Roxy endpoint");
  return async (url, options = {}) => {
    // Match exact configured listing queries and canonical native event URLs.
    if (
      typeof url !== "string" ||
      !(
        (url.startsWith(endpoint) &&
          /^[1-9][0-9]?$/.test(url.slice(endpoint.length))) ||
        /^https:\/\/aftontickets\.com\/event\/buyticket\/[a-z0-9]{10}$/.test(
          url,
        )
      )
    )
      throw Error("Unreviewed Roxy request");
    if (options.method && options.method !== "GET")
      throw Error("Unsupported Roxy request method");
    if (options.signal?.aborted) throw Error("curl request aborted");
    const config =
      [
        `proxy = ${quote(proxy.server)}`,
        `proxy-user = ${quote(proxy.username + ":" + proxy.password)}`,
        `url = ${quote(url)}`,
      ].join("\n") + "\n";
    return withTransportRetries(
      (remaining) =>
        new Promise((accept, reject) => {
          const child = spawn(
            "curl",
            [
              "--disable",
              "--config",
              "-",
              "--silent",
              "--noproxy",
              "",
              "--proxy-basic",
              "--proto",
              "=https",
              "--max-time",
              String(remaining / 1000),
              "--connect-timeout",
              String(Math.min(15, remaining / 1000)),
              "--include",
              "--suppress-connect-headers",
              "--write-out",
              "\n%{http_connect}",
            ],
            { env: cleanEnvironment(), stdio: ["pipe", "pipe", "ignore"] },
          );
          const chunks = [];
          let size = 0,
            failure;
          const stop = (message) => {
            failure = message;
            child.kill("SIGKILL");
          };
          const abort = () => stop("curl request aborted");
          const timer = setTimeout(
            () => stop("curl request timeout"),
            remaining,
          );
          options.signal?.addEventListener("abort", abort, { once: true });
          if (options.signal?.aborted) abort();
          child.stdin.on("error", () => {});
          child.on("error", () => {
            failure = "curl unavailable";
          });
          child.stdout.on("data", (chunk) => {
            size += chunk.length;
            if (size > bodyLimit + headerLimit + 4)
              stop("curl response size limit exceeded");
            else chunks.push(chunk);
          });
          child.on("close", (code) => {
            clearTimeout(timer);
            options.signal?.removeEventListener("abort", abort);
            const bytes = Buffer.concat(chunks);
            const footer = bytes.subarray(-4).toString("ascii");
            const hasFooter = /^\n[0-9]{3}$/.test(footer);
            const proxyConnectStatus = hasFooter
              ? Number(footer.slice(1))
              : null;
            if (failure || code !== 0) {
              const error = Error(
                failure ||
                  `curl transfer failed (exit ${Number.isInteger(code) ? code : "unknown"})`,
              );
              // Only curl's own numeric CONNECT status controls retry eligibility.
              if (!failure)
                Object.assign(error, {
                  curlExitCode: code,
                  proxyConnectStatus,
                });
              return reject(error);
            }
            try {
              if (!hasFooter || proxyConnectStatus !== 200)
                throw Error("curl proxy tunnel was not confirmed");
              accept(decodeCurlResponse(bytes.subarray(0, -4)));
            } catch (error) {
              reject(error);
            }
          });
          child.stdin.end(config);
        }),
      options.signal,
    );
  };
}

export function roxyFetcher(endpoint, env = process.env, direct = fetch) {
  if (env.ROXY_PROXY_URL)
    return createRoxyFetcher(endpoint, env.ROXY_PROXY_URL);
  if (env.ROXY_PROXY_REQUIRED === "1") throw Error("Roxy proxy is required");
  return direct;
}
