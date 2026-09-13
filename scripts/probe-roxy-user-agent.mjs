// Opt-in diagnostics only. Proxy mode is explicit; no publication or challenge interaction.
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { captureProfile } from "../tests/capture-profile.mjs";

export async function probeRoxyProxy(endpoint, proxyURL, requestAPI) {
  let context;
  try {
    const target = new URL(endpoint),
      proxy = new URL(proxyURL);
    if (
      target.origin !== "https://aftontickets.com" ||
      target.pathname !== "/api/get-events" ||
      target.username ||
      target.password ||
      !["http:", "https:"].includes(proxy.protocol) ||
      proxy.pathname !== "/" ||
      proxy.search ||
      proxy.hash
    )
      throw Error("Invalid probe configuration");
    const api = requestAPI ?? (await import("@playwright/test")).request;
    context = await api.newContext({
      proxy: {
        server: proxy.origin,
        ...(proxy.username
          ? { username: decodeURIComponent(proxy.username) }
          : {}),
        ...(proxy.password
          ? { password: decodeURIComponent(proxy.password) }
          : {}),
      },
      ignoreHTTPSErrors: false,
    });
    const response = await context.get(endpoint, {
      timeout: 30000,
      maxRedirects: 0,
      maxRetries: 0,
    });
    const headers = response.headers();
    const type = headers["content-type"]?.split(";", 1)[0].trim().toLowerCase();
    return {
      mode: "proxy",
      status: response.status(),
      content_type: ["application/json", "text/html"].includes(type)
        ? type
        : "other",
      challenge:
        headers["x-amzn-waf-action"] === "challenge" ? "challenge" : null,
      cf_mitigated:
        headers["cf-mitigated"] === "challenge" ? "challenge" : null,
    };
  } catch {
    return { mode: "proxy", request_failed: true };
  } finally {
    try {
      await context?.dispose();
    } catch {
      /* Never expose transport errors. */
    }
  }
}

export async function probeRoxyBrowser(page, profile, observationMs = 15000) {
  if (profile.page !== "https://www.theroxydenver.com/calendar")
    throw Error("Unreviewed browser page");
  const endpoint = new URL(profile.endpoint);
  if (
    endpoint.origin !== "https://aftontickets.com" ||
    endpoint.pathname !== "/api/get-events" ||
    !endpoint.searchParams.get("key")
  )
    throw Error("Unreviewed browser feed");
  const result = {
    page_status: null,
    navigation_failed: false,
    feed_responses: [],
    document_responses: [],
    feed_request_failed: false,
    observations_truncated: false,
  };
  const matches = (raw) => {
    const url = new URL(raw);
    return (
      url.origin === endpoint.origin &&
      url.pathname === endpoint.pathname &&
      url.searchParams.get("key") === endpoint.searchParams.get("key")
    );
  };
  const response = (r) => {
    const feed = matches(r.url());
    if (!feed && r.request().resourceType() !== "document") return;
    const target = feed ? result.feed_responses : result.document_responses;
    if (target.length >= 32) {
      result.observations_truncated = true;
      return;
    }
    const headers = r.headers();
    target.push({
      origin: new URL(r.url()).origin,
      status: r.status(),
      content_type: headers["content-type"]?.slice(0, 200) ?? null,
      challenge: headers["x-amzn-waf-action"]?.slice(0, 200) ?? null,
      cf_mitigated: headers["cf-mitigated"]?.slice(0, 200) ?? null,
    });
  };
  const failed = (request) => {
    if (matches(request.url())) result.feed_request_failed = true;
  };
  page.on("response", response);
  page.on("requestfailed", failed);
  try {
    try {
      result.page_status =
        (
          await page.goto(profile.page, {
            waitUntil: "domcontentloaded",
            timeout: 30000,
          })
        )?.status() ?? null;
    } catch {
      result.navigation_failed = true;
    }
    await page.waitForTimeout(observationMs);
    result.frame_origins = [
      ...new Set(
        page.frames().map((frame) => {
          try {
            return new URL(frame.url()).origin;
          } catch {
            return "unknown";
          }
        }),
      ),
    ].slice(0, 32);
  } finally {
    page.off("response", response);
    page.off("requestfailed", failed);
  }
  result.json_feed_response = result.feed_responses.some(
    (r) =>
      r.status >= 200 &&
      r.status < 300 &&
      r.content_type?.includes("application/json"),
  );
  return result;
}

export async function probeRoxy(endpoint, userAgent, fetcher = fetch) {
  const url = new URL(endpoint);
  if (
    url.origin !== "https://aftontickets.com" ||
    url.pathname !== "/api/get-events" ||
    url.username ||
    url.password
  )
    throw Error("Unreviewed probe endpoint");
  const results = [];
  for (const mode of ["default", "browser-user-agent"]) {
    try {
      const response = await fetcher(endpoint, {
        redirect: "error",
        signal: AbortSignal.timeout(30000),
        ...(mode === "browser-user-agent"
          ? { headers: { "user-agent": userAgent } }
          : {}),
      });
      const result = {
        mode,
        status: response.status,
        content_type:
          response.headers.get("content-type")?.slice(0, 200) ?? null,
        challenge:
          response.headers.get("x-amzn-waf-action")?.slice(0, 200) ?? null,
        cf_mitigated:
          response.headers.get("cf-mitigated")?.slice(0, 200) ?? null,
      };
      // Do not read or log the body, cookies, URL, or exception text.
      try {
        await response.body?.cancel();
      } catch {
        result.body_cancel_failed = true;
      }
      results.push(result);
    } catch {
      results.push({ mode, request_failed: true });
    }
  }
  return { user_agent: userAgent, results };
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const browserMode =
      process.argv.length === 3 && process.argv[2] === "--browser";
    const proxyMode =
      process.argv.length === 3 && process.argv[2] === "--proxy";
    if (process.argv.length !== 2 && !browserMode && !proxyMode)
      throw Error("Unexpected arguments");
    const profile = captureProfile("roxy");
    if (proxyMode) {
      const proxyURL = process.env.HTTP_PROXY;
      delete process.env.HTTP_PROXY;
      console.log(
        JSON.stringify(await probeRoxyProxy(profile.endpoint + "1", proxyURL)),
      );
    } else {
      const { chromium } = await import("@playwright/test");
      const browser = await chromium.launch();
      let userAgent, result;
      try {
        const page = await browser.newPage();
        userAgent = await page.evaluate(() => navigator.userAgent);
        if (browserMode)
          result = {
            user_agent: userAgent,
            ...(await probeRoxyBrowser(page, profile)),
          };
      } finally {
        await browser.close();
      }
      console.log(
        JSON.stringify(
          result ?? (await probeRoxy(profile.endpoint + "1", userAgent)),
        ),
      );
    }
  } catch {
    console.error("Roxy probe setup or observation failed");
    process.exitCode = 1;
  }
}
