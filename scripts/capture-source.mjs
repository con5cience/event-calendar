// One isolated process per source; explicit output directory, no console scraping.
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { captureProfile } from "../tests/capture-profile.mjs";
import { runnerFor } from "./refresh-locale.mjs";
import { captureAEG, captureHMT, eventJSONLD } from "./capture-feeds.mjs";
import { validateLocaleID } from "./package-locale.mjs";

const [sourceID, output, extra] = process.argv.slice(2);
if (!output || extra)
  throw Error("Usage: capture-source.mjs SOURCE NEW_OUTPUT_DIRECTORY");
validateLocaleID(sourceID);
const profile = captureProfile(sourceID);
const runner = runnerFor(profile.adapter);
mkdirSync(output);
const captured_at = new Date().toISOString();
let result;
if (runner.feed === "aeg") result = await captureAEG(profile);
else if (runner.feed === "hmt") {
  const { chromium } = await import("@playwright/test");
  const browser = await chromium.launch({ headless: true });
  try {
    const context = await browser.newContext({ offline: true });
    const page = await context.newPage();
    // Parsing occurs in a detached document. No provider scripts are executed.
    result = await captureHMT(profile, fetch, (html) =>
      eventJSONLD(page, html),
    );
  } finally {
    await browser.close();
  }
} else {
  const module = await import(
    new URL(`../tests/${runner.script}`, import.meta.url)
  );
  result = await module[runner.fn || "capture"](
    ...(sourceID === "roxy" && profile.adapter === "afton"
      ? [(await import("./roxy-transport.mjs")).roxyFetcher(profile.endpoint)]
      : sourceID === "nocturne"
        ? [
            (await import("./nocturne-transport.mjs")).nocturneFetcher(
              profile.endpoint,
              profile.policy,
            ),
          ]
        : runner.keyed
          ? [sourceID]
          : []),
  );
}
const snapshot =
  typeof result.snapshot === "string"
    ? result.snapshot
    : JSON.stringify(result.snapshot);
if (!snapshot || Buffer.byteLength(snapshot) > 4 * 1024 * 1024)
  throw Error("Invalid or oversized snapshot");
writeFileSync(join(output, "snapshot.json"), snapshot, { flag: "wx" });
const { snapshot: _snapshot, ...evidence } = result;
void _snapshot;
writeFileSync(join(output, "evidence.json"), JSON.stringify(evidence), {
  flag: "wx",
});
writeFileSync(
  join(output, "report.json"),
  JSON.stringify({
    source: sourceID,
    captured_at: result.captured_at || captured_at,
    failures: result.failures ?? [],
  }),
  { flag: "wx" },
);
