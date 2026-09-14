// Read-only assessment and HTTP verification for the manual Actions dry run.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import {
  readFileSync,
  readdirSync,
  writeFileSync,
  appendFileSync,
} from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { validateLocaleID } from "./package-locale.mjs";

export function assessRefresh(report, exitCode, locale, expectedSources) {
  validateLocaleID(locale);
  assert.equal(report.locale, locale, "Report locale differs");
  assert(
    [0, 2].includes(exitCode),
    "Refresh failed; do not build the seed snapshot",
  );
  assert(Array.isArray(report.sources), "Source results are missing");
  assert.deepEqual(
    report.sources.map((s) => s.source).sort(),
    [...expectedSources].sort(),
    "Source results differ from the registry",
  );
  assert.equal(
    new Set(report.sources.map((s) => s.source)).size,
    report.sources.length,
    "Duplicate source results",
  );
  assert(
    report.sources.some((s) => s.status === "published"),
    "No source published",
  );
  for (const source of report.sources) {
    validateLocaleID(source.source);
    assert(
      ["published", "failed"].includes(source.status),
      "Unknown source status",
    );
    if (source.status === "published")
      assert(Array.isArray(source.rejected), "Rejections missing");
  }
  const partial = report.sources.some(
    (s) => s.status === "failed" || s.rejected.length > 0,
  );
  assert.equal(
    report.status,
    partial ? "partial" : "success",
    "Inconsistent report status",
  );
  assert.equal(exitCode, partial ? 2 : 0, "Inconsistent exit code");
  return report;
}

export async function checkCalendar(base, locale) {
  const get = async (path) => {
    const response = await fetch(new URL(path, base), {
      redirect: "error",
      signal: AbortSignal.timeout(10000),
    });
    assert.equal(response.status, 200, `${path} did not return 200`);
    return response;
  };
  await get("/healthz");
  const site = await (await get("/api/site")).json();
  assert.equal(site.id, locale, "Wrong locale served");
  const calendar = await (await get("/api/calendar")).json();
  assert(
    Array.isArray(calendar.events) && calendar.events.length > 0,
    "Calendar is empty or malformed",
  );
  for (const event of calendar.events) {
    for (const key of ["id", "title", "venue", "date", "public_path"])
      assert(
        typeof event[key] === "string" && event[key].length > 0,
        `Missing event ${key}`,
      );
    assert(event.public_path.startsWith("/events/"), "Invalid event path");
  }
  return {
    locale,
    health: "ok",
    calendar: "nonempty",
    calendar_sha256: createHash("sha256")
      .update(JSON.stringify(calendar.events))
      .digest("hex"),
    sample: calendar.events
      .slice(0, 5)
      .map(({ id, title, venue, date }) => ({ id, title, venue, date })),
  };
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const [command, locale, input, output] = process.argv.slice(2);
    validateLocaleID(locale);
    if (command === "report") {
      const root = resolve(input);
      const directories = readdirSync(join(root, ".artifacts/refresh", locale));
      assert.equal(directories.length, 1, "Expected one isolated refresh run");
      const report = JSON.parse(
        readFileSync(
          join(
            root,
            ".artifacts/refresh",
            locale,
            directories[0],
            "report.json",
          ),
          "utf8",
        ),
      );
      // Save diagnostics even when assessment fails. Never include raw captures.
      writeFileSync(
        join(output, "report.json"),
        JSON.stringify(report, null, 2) + "\n",
      );
      const site = JSON.parse(
        readFileSync(join(root, "locales", locale, "site.json"), "utf8"),
      );
      const rawExitCode = readFileSync(
        join(output, "refresh-exit-code.txt"),
        "utf8",
      ).trim();
      assert(
        /^[0-9]+$/.test(rawExitCode),
        "Missing or malformed refresh exit code",
      );
      const exitCode = Number(rawExitCode);
      assessRefresh(report, exitCode, locale, Object.keys(site.sources));
      const summary =
        `## ${locale} refresh dry run\n\nResult: **${report.status}**. No Git publishing or deployment.\n\n| Source | Result | Rejected records |\n|---|---|---|\n` +
        report.sources
          .map(
            (s) =>
              `| ${s.source} | ${s.status} | ${s.rejected?.length ?? "—"} |`,
          )
          .join("\n") +
        "\n";
      writeFileSync(join(output, "summary.md"), summary);
      if (process.env.GITHUB_STEP_SUMMARY)
        appendFileSync(process.env.GITHUB_STEP_SUMMARY, summary);
      if (report.status === "partial")
        console.log(
          "::warning::Partial refresh: inspect report.json. Build verification will continue; deployment requires every source to publish.",
        );
    } else if (command === "smoke") {
      let result;
      for (let attempt = 0; attempt < 30; attempt++) {
        try {
          result = await checkCalendar(input, locale);
          break;
        } catch (error) {
          if (attempt === 29) throw error;
          await new Promise((accept) => setTimeout(accept, 1000));
        }
      }
      writeFileSync(output, JSON.stringify(result, null, 2) + "\n");
    } else
      throw Error(
        "Usage: refresh-dry-run.mjs report LOCALE ROOT OUTPUT_DIR | smoke LOCALE BASE_URL OUTPUT_FILE",
      );
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
