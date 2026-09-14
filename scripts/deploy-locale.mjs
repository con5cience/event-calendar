// Deployment gates reuse the refresh contract. No provider validation is relaxed.
import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { assessRefresh, checkCalendar } from "./refresh-dry-run.mjs";
import { validateLocaleID } from "./package-locale.mjs";

export function assessDeployment(report, exitCode, locale, expectedSources) {
  assessRefresh(report, exitCode, locale, expectedSources);
  assert(
    report.sources.every((source) => source.status === "published"),
    "Source failure blocks deployment; inspect report.json",
  );
  return report;
}

export function deploymentState(deployments, id) {
  assert(Array.isArray(deployments), "Malformed deployment list");
  const matches = deployments.filter((deployment) => deployment.id === id);
  assert(matches.length <= 1, "Duplicate deployment ID");
  if (!matches.length) return "pending";
  const { status } = matches[0];
  if (status === "SUCCESS") return "success";
  assert(
    ["BUILDING", "QUEUED", "DEPLOYING", "INITIALIZING", "WAITING"].includes(
      status,
    ),
    `Deployment ${id}: ${status}`,
  );
  return "pending";
}

const readJSON = (path) => JSON.parse(readFileSync(path, "utf8"));
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const [command, locale, directory] = process.argv.slice(2);
    validateLocaleID(locale);
    if (command === "gate") {
      const site = readJSON(`locales/${locale}/site.json`);
      assert.equal(site.id, locale);
      const exit = readFileSync(
        `${directory}/refresh-exit-code.txt`,
        "utf8",
      ).trim();
      assert.match(exit, /^[0-9]+$/);
      const report = assessDeployment(
        readJSON(`${directory}/report.json`),
        Number(exit),
        locale,
        Object.keys(site.sources),
      );
      console.log(
        `All sources published. Individual record rejections remain in the report (${report.status}).`,
      );
    } else if (command === "verify") {
      const target = readJSON(`.github/railway-${locale}.json`);
      const upload = readJSON(`${directory}/upload.json`);
      assert.match(upload.deploymentId, /^[0-9a-f-]{36}$/);
      const deadline = Date.now() + 20 * 60 * 1000;
      let complete = false;
      while (Date.now() < deadline) {
        const result = spawnSync(
          "railway",
          [
            "deployment",
            "list",
            "--project",
            target.project,
            "--environment",
            target.environment,
            "--service",
            target.service,
            "--limit",
            "20",
            "--json",
          ],
          { encoding: "utf8", timeout: 30000 },
        );
        assert.equal(
          result.status,
          0,
          "Railway deployment status request failed",
        );
        const status = deploymentState(
          JSON.parse(result.stdout),
          upload.deploymentId,
        );
        console.log(`Deployment ${upload.deploymentId}: ${status}`);
        if (status === "success") {
          complete = true;
          break;
        }
        await new Promise((accept) => setTimeout(accept, 10000));
      }
      assert(complete, "Timed out waiting for Railway deployment");
      const result = await checkCalendar(target.url, locale);
      const expected = readJSON(`${directory}/http-check.json`);
      assert.equal(
        result.calendar_sha256,
        expected.calendar_sha256,
        "Live calendar differs from verified snapshot",
      );
      writeFileSync(
        `${directory}/live-check.json`,
        JSON.stringify(
          { deployment_id: upload.deploymentId, ...result },
          null,
          2,
        ) + "\n",
      );
      console.log(`Verified ${target.url}`);
    } else
      throw Error("Usage: deploy-locale.mjs gate|verify LOCALE RESULTS_DIR");
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
