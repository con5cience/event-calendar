import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";
import { spawn } from "node:child_process";
import { once } from "node:events";
import { createServer } from "node:http";
import { checkCalendar } from "../scripts/refresh-dry-run.mjs";
import {
  assessDeployment,
  deploymentState,
} from "../scripts/deploy-locale.mjs";

const source = { source: "gothic", status: "published", rejected: [] };
const report = { locale: "denver", status: "success", sources: [source] };

test("deployment requires every registered source, but permits rejected records", () => {
  assert.equal(assessDeployment(report, 0, "denver", ["gothic"]), report);
  const partial = {
    ...report,
    status: "partial",
    sources: [{ ...source, rejected: [{ reason: "invalid date" }] }],
  };
  assert.equal(assessDeployment(partial, 2, "denver", ["gothic"]), partial);
  for (const [data, code, registry] of [
    [
      { ...partial, sources: [source, { source: "roxy", status: "failed" }] },
      2,
      ["gothic", "roxy"],
    ],
    [report, 0, ["gothic", "roxy"]],
    [{ ...report, locale: "other" }, 0, ["gothic"]],
    [{ ...report, sources: [source, source] }, 0, ["gothic"]],
    [partial, 0, ["gothic"]],
    [report, 137, ["gothic"]],
  ])
    assert.throws(() => assessDeployment(data, code, "denver", registry));
});

test("deployment polling checks the exact upload ID, never another successful deployment", () => {
  const id = "new";
  assert.equal(
    deploymentState([{ id: "old", status: "SUCCESS" }], id),
    "pending",
  );
  for (const status of [
    "BUILDING",
    "QUEUED",
    "DEPLOYING",
    "INITIALIZING",
    "WAITING",
  ])
    assert.equal(deploymentState([{ id, status }], id), "pending");
  assert.equal(deploymentState([{ id, status: "SUCCESS" }], id), "success");
  for (const status of [
    "FAILED",
    "CRASHED",
    "REMOVED",
    "SKIPPED",
    "CANCELED",
    "UNKNOWN",
  ])
    assert.throws(() => deploymentState([{ id, status }], id));
  assert.throws(() => deploymentState({}, id));
  assert.throws(() =>
    deploymentState(
      [
        { id, status: "SUCCESS" },
        { id, status: "SUCCESS" },
      ],
      id,
    ),
  );
});

test("manual workflow defaults to deployment, is serialized, main-only, and isolates credentials", () => {
  const yaml = readFileSync(
    new URL("../.github/workflows/refresh-dry-run.yml", import.meta.url),
    "utf8",
  );
  assert.match(yaml, /deploy:[\s\S]*type: boolean\n\s+default: true/);
  assert.match(yaml, /group: refresh-dry-run-/);
  const deploy = yaml.split("\n  deploy:\n")[1];
  assert(deploy);
  assert.match(deploy, /needs: dry-run/);
  assert.match(deploy, /github.ref == 'refs\/heads\/main'/);
  assert.match(deploy, /contents: write/);
  assert.match(deploy, /ref: \$\{\{ github.sha \}\}/);
  assert.match(
    deploy,
    /snapshot-\$\{\{ inputs.locale \}\}-\$\{\{ github.run_id \}\}-\$\{\{ github.run_attempt \}\}/,
  );
  const upload = deploy
    .split("      - name: Deploy and verify Railway\n")[1]
    .split("      - name:")[0];
  assert.match(
    upload,
    /RAILWAY_TOKEN: \$\{\{ secrets.WITHADULT_RAILWAY_API_TOKEN \}\}/,
  );
  assert.match(upload, /--path-as-root/);
  assert.match(upload, /--detach --json/);
  assert.match(upload, /deploy-locale.mjs verify/);
  assert.equal(yaml.split("secrets.WITHADULT_RAILWAY_API_TOKEN").length, 3);
  assert.match(deploy, /publish-snapshot.mjs/);
  assert.match(deploy, /deploy-locale.mjs gate/);
});

test("verification CLI checks the uploaded deployment and real HTTP response digest", async () => {
  const root = mkdtempSync(join(tmpdir(), "deployment-cli-"));
  const id = "11111111-1111-4111-8111-111111111111";
  for (const dir of [".github", "results", "bin"]) mkdirSync(join(root, dir));
  const data = {
    events: [
      {
        id: "fixture",
        title: "Fixture",
        date: "2026-09-14",
        venue: "Fixture venue",
        public_path: "/events/fixture",
      },
    ],
  };
  const server = createServer((req, res) => {
    res.writeHead(200, { "content-type": "application/json" });
    res.end(JSON.stringify(req.url === "/api/site" ? { id: "denver" } : data));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const url = `http://127.0.0.1:${server.address().port}`;
  const json = (path, value) =>
    writeFileSync(join(root, path), JSON.stringify(value));
  const run = async () => {
    const child = spawn(
      process.execPath,
      [
        fileURLToPath(new URL("../scripts/deploy-locale.mjs", import.meta.url)),
        "verify",
        "denver",
        "results",
      ],
      {
        cwd: root,
        env: {
          ...process.env,
          PATH: `${join(root, "bin")}:${process.env.PATH}`,
        },
        stdio: "ignore",
      },
    );
    return (await once(child, "exit"))[0];
  };
  try {
    json(".github/railway-denver.json", {
      project: "p",
      environment: "e",
      service: "s",
      url,
    });
    json("results/upload.json", { deploymentId: id });
    json("results/http-check.json", await checkCalendar(url, "denver"));
    writeFileSync(
      join(root, "bin/railway"),
      `#!/bin/sh\nprintf '%s\\n' '[{"id":"${id}","status":"SUCCESS"}]'\n`,
      { mode: 0o755 },
    );
    assert.equal(await run(), 0);
    assert.equal(
      JSON.parse(readFileSync(join(root, "results/live-check.json"), "utf8"))
        .deployment_id,
      id,
    );
    data.events[0].title = "Different snapshot";
    assert.equal(await run(), 1);
    json("results/upload.json", {});
    assert.equal(await run(), 1);
  } finally {
    server.close();
  }
});
