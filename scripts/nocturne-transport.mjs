import { spawn } from "node:child_process";
import {
  proxyConfiguration,
  cleanEnvironment,
  quote,
} from "./proxy-config.mjs";
import { decodeCurlResponse } from "./roxy-transport.mjs";

const bodyLimit = 1024 * 1024;
const headerLimit = 16384;
const mimeTypes = ["text/calendar", "text/html"];

// The GitHub-hosted refresh runner is served non-calendar interstitials by
// this venue's edge while identical requests succeed from other networks.
// Nocturne therefore shares the Roxy capture proxy when one is configured.
// Each request is one confirmed-tunnel curl attempt; the capture's own
// bounded retry loop stays the only retry loop.
export function createNocturneFetcher(endpointValue, policyValue, proxyURL) {
  let proxy;
  try {
    proxy = proxyConfiguration(proxyURL);
  } catch {
    throw Error("Invalid Nocturne proxy configuration");
  }
  const endpoint = new URL(endpointValue);
  const policy = new URL(policyValue);
  if (
    endpoint.origin !== "https://nocturnejazz.com" ||
    endpoint.pathname !== "/music" ||
    endpoint.username ||
    endpoint.password ||
    endpoint.hash ||
    endpoint.searchParams.get("format") !== "ical" ||
    endpoint.searchParams.size !== 1
  )
    throw Error("Unreviewed Nocturne endpoint");
  if (
    policy.origin !== "https://nocturnejazz.com" ||
    policy.pathname !== "/faq" ||
    policy.username ||
    policy.password ||
    policy.hash ||
    policy.search
  )
    throw Error("Unreviewed Nocturne policy URL");
  const isCalendarRequest = (url) =>
    url.origin === endpoint.origin &&
    url.pathname === endpoint.pathname &&
    !url.username &&
    !url.password &&
    !url.hash &&
    url.searchParams.get("format") === "ical" &&
    /^\d{4}-\d{2}-\d{2}$/.test(url.searchParams.get("date") ?? "") &&
    url.searchParams.size === 2;
  return async (url, options = {}) => {
    if (typeof url !== "string") throw Error("Unreviewed Nocturne request");
    const target = new URL(url);
    const calendar = isCalendarRequest(target);
    if (!calendar && target.href !== policy.href)
      throw Error("Unreviewed Nocturne request");
    if (options.method && options.method !== "GET")
      throw Error("Unsupported Nocturne request method");
    if (options.signal?.aborted) throw Error("curl request aborted");
    const config =
      [
        `proxy = ${quote(proxy.server)}`,
        `proxy-user = ${quote(proxy.username + ":" + proxy.password)}`,
        `url = ${quote(target.href)}`,
      ].join("\n") + "\n";
    return new Promise((accept, reject) => {
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
          "30",
          "--connect-timeout",
          "15",
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
      const timer = setTimeout(() => stop("curl request timeout"), 30000);
      options.signal?.addEventListener("abort", abort, { once: true });
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
        const proxyConnectStatus = hasFooter ? Number(footer.slice(1)) : null;
        const path = target.pathname + target.search;
        if (failure || code !== 0) {
          const error = Error(
            failure ||
              `curl transfer failed (exit ${Number.isInteger(code) ? code : "unknown"})`,
          );
          console.error(
            `nocturne transport: ${error.message} connect=${proxyConnectStatus ?? "none"} url=${path}`,
          );
          return reject(error);
        }
        try {
          if (!hasFooter || proxyConnectStatus !== 200)
            throw Error("curl proxy tunnel was not confirmed");
          const response = decodeCurlResponse(bytes.subarray(0, -4), mimeTypes);
          const type = response.headers.get("content-type");
          const challenged =
            response.headers.get("x-amzn-waf-action") === "challenge" ||
            response.headers.get("cf-mitigated") === "challenge";
          if (
            response.status !== 200 ||
            challenged ||
            type !== (calendar ? "text/calendar" : "text/html")
          )
            console.error(
              `nocturne transport: status=${response.status} type=${type ?? "none"} bytes=${bytes.length - 4} challenge=${challenged} url=${path}`,
            );
          accept(response);
        } catch (error) {
          console.error(`nocturne transport: ${error.message} url=${path}`);
          reject(error);
        }
      });
      child.stdin.end(config);
    });
  };
}

export function nocturneFetcher(
  endpoint,
  policy,
  env = process.env,
  direct = fetch,
) {
  if (env.ROXY_PROXY_URL)
    return createNocturneFetcher(endpoint, policy, env.ROXY_PROXY_URL);
  if (env.ROXY_PROXY_REQUIRED === "1")
    throw Error("Nocturne proxy is required");
  return direct;
}
