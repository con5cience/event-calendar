import { test } from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import {
  mkdtempSync,
  mkdirSync,
  writeFileSync,
  readFileSync,
  existsSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import { assessRefresh, checkCalendar } from "../scripts/refresh-dry-run.mjs";

test("workflow streams and retains output, preserves status, and exports reports inside Docker", () => {
  const yaml = readFileSync(
    new URL("../.github/workflows/refresh-dry-run.yml", import.meta.url),
    "utf8",
  );
  const step = yaml
    .split("      - name: Refresh sources\n")[1]
    .split("      - name: Build refreshed application\n")[0];
  const shell = step
    .split("        run: |\n")[1]
    .split("\n")
    .map((line) => line.slice(10))
    .join("\n");
  assert.match(shell, /tee dry-run-results\/refresh.log/);
  assert.match(shell, /PIPESTATUS/);
  assert.match(
    shell,
    /docker run.*--entrypoint node.*refresh-dry-run.mjs report/,
  );
  assert.match(shell, /stop-commands/);
  for (const exitCode of [0, 1, 2, 137]) {
    const root = mkdtempSync(join(tmpdir(), "refresh-shell-"));
    mkdirSync(join(root, "dry-run-results"));
    const result = spawnSync(
      "bash",
      [
        "-e",
        "-c",
        `docker() { if [[ "$*" == *"--entrypoint node"* ]]; then printf 'report exported\\n'; return 0; fi; printf 'live stdout\\n'; printf 'live stderr\\n' >&2; return ${exitCode}; }\n${shell}`,
      ],
      {
        cwd: root,
        encoding: "utf8",
        env: {
          ...process.env,
          LOCALE: "denver",
          RUNNER_TEMP: root,
          GITHUB_WORKSPACE: root,
          GITHUB_STEP_SUMMARY: join(root, "summary.md"),
        },
      },
    );
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /live stdout/);
    assert.match(result.stdout, /live stderr/);
    assert.equal(
      readFileSync(join(root, "dry-run-results/refresh.log"), "utf8"),
      "live stdout\nlive stderr\n",
    );
    assert.equal(
      readFileSync(
        join(root, "dry-run-results/refresh-exit-code.txt"),
        "utf8",
      ).trim(),
      String(exitCode),
    );
  }
});

const report = (status, sources) => ({ locale: "denver", status, sources });
const published = { source: "gothic", status: "published", rejected: [] };
test("report CLI writes diagnostics, rejects failures, and preserves input", () => {
  for (const mode of [
    "success",
    "partial",
    "failed",
    "missing",
    "blank-exit",
  ]) {
    const root = mkdtempSync(join(tmpdir(), "dry-run-report-"));
    const source = join(root, ".artifacts/refresh/denver/run-test");
    const output = join(root, "out");
    mkdirSync(source, { recursive: true });
    mkdirSync(output);
    mkdirSync(join(root, "locales/denver"), { recursive: true });
    writeFileSync(
      join(root, "locales/denver/site.json"),
      JSON.stringify({ sources: { gothic: {} } }),
    );
    const data = report(
      mode === "blank-exit" ? "success" : mode,
      mode === "partial" ? [{ ...published, rejected: [{}] }] : [published],
    );
    const bytes = JSON.stringify(data);
    if (mode !== "missing") writeFileSync(join(source, "report.json"), bytes);
    writeFileSync(
      join(output, "refresh-exit-code.txt"),
      mode === "blank-exit"
        ? ""
        : mode === "partial"
          ? "2"
          : mode === "failed"
            ? "1"
            : "0",
    );
    const result = spawnSync(
      process.execPath,
      ["scripts/refresh-dry-run.mjs", "report", "denver", root, output],
      {
        encoding: "utf8",
        env: {
          ...process.env,
          GITHUB_STEP_SUMMARY: join(output, "github-summary.md"),
        },
      },
    );
    const usable = ["success", "partial"].includes(mode);
    assert.equal(result.status, usable ? 0 : 1, `${mode}: ${result.stderr}`);
    assert.equal(existsSync(join(output, "summary.md")), usable);
    if (mode !== "missing") {
      assert.deepEqual(
        JSON.parse(readFileSync(join(output, "report.json"))),
        data,
      );
      assert.equal(readFileSync(join(source, "report.json"), "utf8"), bytes);
    }
  }
});
test("accepts complete refresh and usable partial results", () => {
  assert.equal(
    assessRefresh(report("success", [published]), 0, "denver", ["gothic"])
      .status,
    "success",
  );
  const partial = report("partial", [
    published,
    { source: "mission", status: "failed", error: "unavailable" },
  ]);
  assert.equal(
    assessRefresh(partial, 2, "denver", ["gothic", "mission"]).status,
    "partial",
  );
  assert.equal(
    assessRefresh(
      report("partial", [
        { ...published, rejected: [{ reason: "invalid date" }] },
      ]),
      2,
      "denver",
      ["gothic"],
    ).status,
    "partial",
  );
});
test("rejects failed, missing, foreign, duplicate and inconsistent results", () => {
  for (const [data, exit] of [
    [report("failed", []), 1],
    [report("success", [published]), 137],
    [report("success", []), 0],
    [{ ...report("success", [published]), locale: "other" }, 0],
    [report("success", [published, published]), 0],
    [report("partial", [{ source: "gothic", status: "failed" }]), 2],
    [report("success", [{ ...published, rejected: [{}] }]), 0],
    [report("partial", [published]), 2],
    [report("success", [{ ...published, status: "unknown" }]), 0],
  ])
    assert.throws(() => assessRefresh(data, exit, "denver", ["gothic"]));
});
test("HTTP checks reject unhealthy, empty and wrong-locale calendars", async () => {
  let health = 200;
  let data = {
    events: [
      {
        id: "a",
        title: "Test",
        venue: "Gothic",
        date: "2026-09-13",
        public_path: "/events/gothic/test",
      },
    ],
  };
  const server = createServer((req, res) => {
    res.writeHead(req.url === "/healthz" ? health : 200, {
      "content-type": "application/json",
    });
    res.end(JSON.stringify(req.url === "/api/site" ? { id: "denver" } : data));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const base = `http://127.0.0.1:${server.address().port}`;
  try {
    await checkCalendar(base, "denver");
    await assert.rejects(checkCalendar(base, "other"));
    health = 503;
    await assert.rejects(checkCalendar(base, "denver"));
    health = 200;
    data = { events: [] };
    await assert.rejects(checkCalendar(base, "denver"));
    data = { events: [{ title: "Invalid" }] };
    await assert.rejects(checkCalendar(base, "denver"));
  } finally {
    server.close();
  }
});
