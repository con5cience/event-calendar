// Operator-only refresh. No Git commands, deployment, or application-store writes.
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  writeFileSync,
  renameSync,
  rmdirSync,
  existsSync,
  lstatSync,
} from "node:fs";
import { resolve, join } from "node:path";
import { fileURLToPath } from "node:url";
import { spawn } from "node:child_process";
import { validateLocaleID } from "./package-locale.mjs";

const codeRoot = fileURLToPath(new URL("../", import.meta.url));
const runners = {
  "nocturne-ical": {
    command: "replay-nocturne",
    script: "nocturne/capture.mjs",
  },
  "aeg-json": { command: "replay-aeg", feed: "aeg" },
  "holdmyticket-ical": { command: "replay-hmt", feed: "hmt" },
  "rhp-calendar": {
    command: "replay-rhp",
    script: "rhp/capture.mjs",
    keyed: true,
  },
  "livenation-venue-events": {
    command: "replay-livenation",
    script: "marquis/capture.mjs",
    keyed: true,
  },
  "kse-calendar": {
    command: "replay-kse-calendar",
    script: "kse/ball-capture.mjs",
    fn: "captureBall",
  },
  "kse-venue-events": { command: "replay-kse", script: "kse/capture.mjs" },
  "supabase-events": {
    command: "replay-blackbox",
    script: "blackbox/capture.mjs",
  },
  "html-buzzard": { command: "replay-buzzard", script: "buzzard/capture.mjs" },
  "html-herbs": { command: "replay-herbs", script: "herbs/capture.mjs" },
  "html-seventh-circle": {
    command: "replay-seventh-circle",
    script: "seventh-circle/capture.mjs",
  },
  "html-ophelias": {
    command: "replay-ophelias",
    script: "ophelias/capture.mjs",
  },
  "plot-listings": { command: "replay-plot", script: "plot/capture.mjs" },
  venuepilot: {
    command: "replay-venuepilot",
    script: "venuepilot/capture.mjs",
  },
  "embedded-nextjs": {
    command: "replay-meowwolf",
    script: "meowwolf/capture.mjs",
  },
  "clique-calendar": { command: "replay-clique", script: "clique/capture.mjs" },
  afton: { command: "replay-afton", script: "afton/capture.mjs" },
};
export function runnerFor(adapter) {
  if (!Object.hasOwn(runners, adapter))
    throw Error(`Unsupported adapter: ${adapter}`);
  return runners[adapter];
}

export function execute(bin, args, options = {}) {
  return new Promise((accept, reject) => {
    const started = Date.now();
    const label = options.label || bin;
    const progress = (message) => options.progress?.(`${label}: ${message}`);
    progress("started");
    const inherited = { ...process.env };
    delete inherited.ROXY_PROXY_URL;
    delete inherited.ROXY_PROXY_REQUIRED;
    const child = spawn(bin, args, {
      env: { ...inherited, ...options.env },
      detached: true,
      stdio: ["ignore", "pipe", "pipe"],
    });
    let stdout = "",
      stderr = "",
      failure;
    let lastOutput = started;
    const heartbeatMs = options.heartbeatMs ?? 30000;
    const heartbeat = options.progress
      ? setInterval(() => {
          if (Date.now() - lastOutput >= heartbeatMs)
            progress(
              `still running (${Math.floor((Date.now() - started) / 1000)}s elapsed)`,
            );
        }, heartbeatMs)
      : undefined;
    const stop = (message) => {
      failure = Error(message);
      try {
        process.kill(-child.pid, "SIGKILL");
      } catch {
        /* already exited */
      }
    };
    const timer = setTimeout(
      () => stop(`Process timeout: ${bin}`),
      options.timeout ?? 15 * 60 * 1000,
    );
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stdout.on("data", (data) => {
      lastOutput = Date.now();
      stdout += data;
      if (stdout.length > 1024 * 1024) stop("Process output limit exceeded");
      else options.onStdout?.(data);
    });
    child.stderr.on("data", (data) => {
      lastOutput = Date.now();
      stderr += data;
      if (stderr.length > 1024 * 1024)
        stop("Process error output limit exceeded");
      else options.onStderr?.(data);
    });
    child.on("error", (error) => {
      clearTimeout(timer);
      clearInterval(heartbeat);
      reject(error);
    });
    child.on("close", (status) => {
      clearTimeout(timer);
      clearInterval(heartbeat);
      progress(
        `${failure || status !== 0 ? "failed" : "completed"} (${Math.floor((Date.now() - started) / 1000)}s elapsed)`,
      );
      if (failure || status !== 0)
        reject(
          failure ?? Error(`${bin} exited ${status}: ${stderr || stdout}`),
        );
      else accept(stdout);
    });
  });
}
const packStore = (site, input, output, logging = {}) =>
  execute(
    process.env.SNAPSHOT_BIN || "package-snapshot",
    ["--site", site, input, output],
    {
      ...logging,
      label: `${logging.locale || "snapshot"} validate ${output.split("/").at(-1)}`,
    },
  );

async function captureSource({ site, sourceID, directory, logging }) {
  await execute(
    process.execPath,
    [
      join(codeRoot, "scripts/capture-source.mjs"),
      sourceID,
      join(directory, "capture"),
    ],
    {
      ...logging,
      label: `${sourceID} capture`,
      env: {
        SITE_DIR: site,
        CAPTURE_SOURCE: sourceID,
        ...(sourceID === "roxy"
          ? {
              ROXY_PROXY_URL: process.env.ROXY_PROXY_URL || "",
              ROXY_PROXY_REQUIRED: process.env.ROXY_PROXY_REQUIRED || "",
            }
          : {}),
      },
    },
  );
}

// Conservative provider groups: one active capture per adapter family. Distinct
// KSE and Wix adapters also share infrastructure, so share a slot within it.
function providerGroup(adapter) {
  if (["kse-calendar", "kse-venue-events"].includes(adapter)) return "kse";
  if (["html-buzzard", "html-herbs"].includes(adapter)) return "wix";
  return adapter;
}

async function capturePool(jobs, limit, capture) {
  const pending = [...jobs],
    active = new Map(),
    outcomes = new Map();
  while (pending.length || active.size) {
    while (active.size < limit) {
      const index = pending.findIndex((job) => !active.has(job.group));
      if (index < 0) break;
      const [job] = pending.splice(index, 1);
      const task = Promise.resolve()
        .then(() => capture(job))
        .then(
          () => outcomes.set(job.sourceID, { ok: true }),
          (error) => outcomes.set(job.sourceID, { ok: false, error }),
        )
        .finally(() => active.delete(job.group));
      active.set(job.group, task);
    }
    if (active.size) await Promise.race(active.values());
  }
  return outcomes;
}

async function refreshSource({
  site,
  sourceID,
  runner,
  store,
  directory,
  logging,
}) {
  const capture = join(directory, "capture");
  const report = JSON.parse(readFileSync(join(capture, "report.json"), "utf8"));
  const raw = await execute(
    process.env.INGEST_BIN || "ingest",
    [
      runner.command,
      "--store",
      store,
      "--config",
      join(site, "sources", `${sourceID}.yaml`),
      "--snapshot",
      join(capture, "snapshot.json"),
      "--now",
      report.captured_at,
    ],
    { ...logging, label: `${sourceID} ingest` },
  );
  writeFileSync(join(directory, "ingestion.json"), raw, { flag: "wx" });
  const result = JSON.parse(raw);
  if (!result.published || !result.durable || result.error)
    throw Error("Ingestion did not confirm durable publication");
  return result;
}

function context(id, options) {
  validateLocaleID(id);
  const root = resolve(options.root || process.env.CALENDAR_REPO || codeRoot);
  const site = join(root, "locales", id);
  if (!lstatSync(site).isDirectory() || lstatSync(site).isSymbolicLink())
    throw Error("Locale must be a real directory");
  const config = JSON.parse(readFileSync(join(site, "site.json"), "utf8"));
  if (config.id !== id) throw Error("Locale ID mismatch");
  const target = join(site, "catalog");
  if (
    existsSync(target) &&
    (!lstatSync(target).isDirectory() || lstatSync(target).isSymbolicLink())
  )
    throw Error("Catalog must be a real directory");
  const lock = join(site, ".refresh-lock");
  try {
    mkdirSync(lock);
  } catch {
    throw Error(`Refresh lock unavailable: ${lock}`);
  }
  try {
    const parent = join(root, ".artifacts", "refresh", id);
    mkdirSync(parent, { recursive: true });
    const workspace = mkdtempSync(join(parent, "run-"));
    return {
      root,
      site,
      config,
      target,
      lock,
      workspace,
      pack:
        options.pack ||
        ((site, input, output) =>
          packStore(site, input, output, { ...options, locale: id })),
    };
  } catch (error) {
    rmdirSync(lock);
    throw error;
  }
}
function replaceSnapshot(ctx, output) {
  const backup = join(ctx.workspace, "previous");
  const hadPrior = existsSync(ctx.target);
  if (hadPrior) renameSync(ctx.target, backup);
  try {
    renameSync(output, ctx.target);
  } catch (error) {
    if (hadPrior) renameSync(backup, ctx.target);
    throw error;
  }
  // Keep the backup and diagnostics for operator recovery. This is not a live store.
  return hadPrior ? backup : null;
}
export async function snapshotLocale(id, input, options = {}) {
  const ctx = context(id, options);
  try {
    const output = join(ctx.workspace, "export");
    await ctx.pack(ctx.site, resolve(input), output);
    const backup = replaceSnapshot(ctx, output);
    return { snapshot: ctx.target, backup };
  } finally {
    rmdirSync(ctx.lock);
  }
}
export async function refreshLocale(id, options = {}) {
  const captureConcurrency =
    options.captureConcurrency ?? Number(process.env.CAPTURE_CONCURRENCY ?? 2);
  if (
    !Number.isInteger(captureConcurrency) ||
    captureConcurrency < 1 ||
    captureConcurrency > 4
  )
    throw Error("Capture concurrency must be an integer from 1 to 4");
  const ctx = context(id, options);
  const progress = (message) => options.progress?.(message);
  progress(`${id}: refresh started`);
  const report = {
    locale: id,
    status: "failed",
    sources: [],
    report: join(ctx.workspace, "report.json"),
  };
  try {
    const sources = Object.entries(ctx.config.sources);
    for (const [id, source] of sources) {
      validateLocaleID(id);
      runnerFor(source.adapter);
    }
    let store = join(ctx.workspace, "seed");
    await ctx.pack(ctx.site, ctx.target, store);
    const jobs = sources.map(([sourceID, source]) => {
      const directory = join(ctx.workspace, sourceID);
      mkdirSync(directory);
      return {
        site: ctx.site,
        sourceID,
        runner: runnerFor(source.adapter),
        directory,
        logging: options,
        group: providerGroup(source.adapter),
      };
    });
    progress(`${id}: capturing with concurrency ${captureConcurrency}`);
    // Finish all captures before publishing. No background writer or child is
    // left running if a later savepoint/export fails and releases the locale lock.
    const captured = await capturePool(
      jobs,
      captureConcurrency,
      async (job) => {
        progress(`${job.sourceID}: source started`);
        await (options.capture || captureSource)(job);
      },
    );
    for (const job of jobs) {
      const { sourceID, directory } = job;
      const candidate = join(directory, "store");
      const outcome = captured.get(sourceID);
      if (outcome.ok) await ctx.pack(ctx.site, store, candidate);
      try {
        if (!outcome.ok) throw outcome.error;
        const result = await (options.refresh || refreshSource)({
          ...job,
          store: candidate,
        });
        if (!result.published || !result.durable || result.error)
          throw Error("Ingestion did not confirm durable publication");
        // Validate each successful savepoint before allowing later jobs to consume it.
        const validated = join(directory, "validated");
        await ctx.pack(ctx.site, candidate, validated);
        store = validated;
        report.sources.push({
          source: sourceID,
          status: "published",
          rejected: result.rejected ?? [],
        });
        progress(
          `${sourceID}: published; ${(result.rejected ?? []).length} rejected records`,
        );
      } catch (error) {
        progress(
          `${sourceID}: failed; retaining last valid data: ${error.message}`,
        );
        report.sources.push({
          source: sourceID,
          status: "failed",
          error: String(error),
        });
      }
    }
    if (!report.sources.some((source) => source.status === "published"))
      throw Error("No source refreshed; tracked snapshot unchanged");
    const output = join(ctx.workspace, "export");
    await ctx.pack(ctx.site, store, output);
    report.backup = replaceSnapshot(ctx, output);
    report.status = report.sources.some(
      (source) => source.status === "failed" || source.rejected.length,
    )
      ? "partial"
      : "success";
    progress(`${id}: refresh ${report.status}`);
    return report;
  } catch (error) {
    progress(`${id}: refresh failed: ${error.message}`);
    report.error = String(error);
    throw Error(`${error.message}; report: ${report.report}`, { cause: error });
  } finally {
    try {
      writeFileSync(report.report, JSON.stringify(report, null, 2) + "\n");
    } finally {
      rmdirSync(ctx.lock);
    }
  }
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const args = process.argv.slice(2);
    // Only the final result belongs on stdout. Stream child output separately.
    const logging = {
      onStdout: (chunk) => process.stderr.write(chunk),
      onStderr: (chunk) => process.stderr.write(chunk),
      progress: (message) =>
        process.stderr.write(`[${new Date().toISOString()}] ${message}\n`),
    };
    let result;
    if (args.length === 3 && args[0] === "--snapshot")
      result = await snapshotLocale(args[1], args[2], logging);
    else if (args.length === 1) result = await refreshLocale(args[0], logging);
    else
      throw Error(
        "Usage: node scripts/refresh-locale.mjs LOCALE | --snapshot LOCALE INPUT_STORE",
      );
    console.log(JSON.stringify(result));
    if (result.status === "partial") process.exitCode = 2;
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
