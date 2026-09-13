// Opt-in comparison only. Does not publish, use proxies, or execute challenges.
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { captureProfile } from "../tests/capture-profile.mjs";

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
    if (process.argv.length !== 2) throw Error("Unexpected arguments");
    const profile = captureProfile("roxy");
    const { chromium } = await import("@playwright/test");
    const browser = await chromium.launch();
    let userAgent;
    try {
      const page = await browser.newPage();
      userAgent = await page.evaluate(() => navigator.userAgent);
    } finally {
      await browser.close();
    }
    console.log(
      JSON.stringify(await probeRoxy(profile.endpoint + "1", userAgent)),
    );
  } catch {
    console.error("Roxy user-agent probe setup failed");
    process.exitCode = 1;
  }
}
