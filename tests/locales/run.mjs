// Local-only isolation check. All synthetic files are generated in a new temp dir.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import {
  mkdtempSync,
  mkdirSync,
  readFileSync,
  writeFileSync,
  readdirSync,
  cpSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import { deflateSync } from "node:zlib";
import { packageLocale } from "../../scripts/package-locale.mjs";

const root = mkdtempSync(join(tmpdir(), "withadult-isolation-"));
const denver = join(root, "denver");
const coastal = join(root, "coastal");
const started = new Set();
function command(bin, args, options = {}) {
  const result = spawnSync(bin, args, {
    encoding: "utf8",
    stdio: options.capture ? "pipe" : "inherit",
    ...options,
  });
  assert.equal(
    result.status,
    0,
    `${bin} ${args.join(" ")}: ${result.stderr || result.error || "failed"}`,
  );
  return result.stdout;
}
function save(path, data) {
  mkdirSync(join(path, ".."), { recursive: true });
  writeFileSync(
    path,
    typeof data === "string" || Buffer.isBuffer(data)
      ? data
      : JSON.stringify(data),
  );
}
function checksum(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}
// A generated 1px test PNG, not artwork for a real locale.
function chunk(kind, data) {
  const payload = Buffer.concat([Buffer.from(kind), data]);
  let crc = 0xffffffff;
  for (const byte of payload) {
    crc ^= byte;
    for (let i = 0; i < 8; i++) crc = (crc >>> 1) ^ (crc & 1 ? 0xedb88320 : 0);
  }
  const length = Buffer.alloc(4),
    tail = Buffer.alloc(4);
  length.writeUInt32BE(data.length);
  tail.writeUInt32BE((crc ^ 0xffffffff) >>> 0);
  return Buffer.concat([length, payload, tail]);
}
function testPNG() {
  const header = Buffer.alloc(13);
  header.writeUInt32BE(1);
  header.writeUInt32BE(1, 4);
  header[8] = 8;
  header[9] = 6;
  return Buffer.concat([
    Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk("IHDR", header),
    chunk("IDAT", deflateSync(Buffer.from([0, 18, 52, 86, 255]))),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}
async function ready(port) {
  for (let i = 0; i < 40; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${port}/healthz`);
      if (r.ok) return;
    } catch {
      /* starting */
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  throw Error("Container did not become ready");
}
function start(name, image, port) {
  command("docker", [
    "run",
    "--rm",
    "-d",
    "--name",
    name,
    "--read-only",
    "--cap-drop",
    "ALL",
    "--security-opt",
    "no-new-privileges:true",
    "-p",
    `127.0.0.1:${port}:8080`,
    image,
  ]);
  started.add(name);
}

try {
  packageLocale("denver", denver);
  mkdirSync(coastal);
  for (const name of readdirSync(denver))
    if (name !== "locales")
      cpSync(join(denver, name), join(coastal, name), { recursive: true });
  const site = JSON.parse(
    readFileSync(join(denver, "locales/denver/site.json")),
  );
  Object.assign(site, {
    id: "coastal",
    city: "coastal",
    name: "withAdult(coastal)",
    tagline: "Coastal test.",
    description: "Synthetic coastal calendar used only for isolation tests.",
    origin: "https://coastal.example",
    timezone: "Pacific/Auckland",
    language: "en-NZ",
    default_view: "month",
    storage_namespace: "withadult.coastal",
    ics_namespace: "coastal.example",
    image_alt: "Synthetic test pixel",
    image_width: 1,
    image_height: 1,
    venue_colors: { Harbor: "#123456" },
    sources: { harbor: { adapter: "aeg-json", venue_key: "harbor" } },
  });
  save(join(coastal, "locales/coastal/site.json"), site);
  save(join(coastal, "locales/coastal/assets/favicon.png"), testPNG());
  save(join(coastal, "locales/coastal/capture.json"), {});
  const source = JSON.parse(readFileSync("tests/contracts/source.json"));
  source.source.id = "harbor";
  source.venue = {
    key: "harbor",
    name: "Harbor",
    timezone: "Pacific/Auckland",
    website: "https://harbor.example",
  };
  const event = source.events[0];
  Object.assign(event, {
    id: "harbor-012345abcdef",
    public_path: "/events/harbor/2026-09-15-harbor-fixture-012345abcdef",
    title: "Harbor fixture concert",
    date: "2026-09-15",
    expires_on: "2026-12-14",
    doors_at: "2026-09-15T19:00:00+12:00",
    show_at: "2026-09-15T20:00:00+12:00",
    event_url: "https://harbor.example/event",
    ticket_url: "https://harbor.example/tickets",
  });
  delete event.price;
  save(join(coastal, "locales/coastal/sources/harbor.yaml"), {
    schema_version: 1,
    source: source.source,
    venue: source.venue,
    state: "established",
    adapter_options: { feed_id: "987", venue_id: "654" },
  });
  const bytes = Buffer.from(JSON.stringify(source));
  save(
    join(coastal, "locales/coastal/catalog/sources/harbor/fixture.json"),
    bytes,
  );
  save(join(coastal, "locales/coastal/catalog/catalog.json"), {
    schema_version: 1,
    generation: "fixture",
    generated_at: source.generated_at,
    sources: [
      {
        source_id: "harbor",
        artifact: "sources/harbor/fixture.json",
        sha256: checksum(bytes),
      },
    ],
  });
  const dockerfile = join(coastal, "Dockerfile.railway");
  save(
    dockerfile,
    readFileSync(dockerfile, "utf8").replaceAll(
      "ARG LOCALE=denver",
      "ARG LOCALE=coastal",
    ),
  );
  assert.deepEqual(readdirSync(join(coastal, "locales")), ["coastal"]);
  assert.ok(!readdirSync(coastal).includes(".artifacts"));
  assert.ok(!readdirSync(denver).includes(".artifacts"));
  const denImage = `withadult-denver-check:${process.pid}`,
    coastImage = `withadult-coastal-check:${process.pid}`;
  command("docker", [
    "build",
    "-f",
    join(denver, "Dockerfile.railway"),
    "-t",
    denImage,
    denver,
  ]);
  command("docker", ["build", "-f", dockerfile, "-t", coastImage, coastal]);
  const denName = `withadult-denver-check-${process.pid}`,
    coastName = `withadult-coastal-check-${process.pid}`;
  start(denName, denImage, 8097);
  start(coastName, coastImage, 8098);
  await ready(8097);
  await ready(8098);
  const denBytes = Buffer.from(
    await (await fetch("http://127.0.0.1:8097/api/calendar")).arrayBuffer(),
  );
  const denData = JSON.parse(denBytes);
  assert(denData.events.every((e) => e.venue !== "Harbor"));
  const coastData = await (
    await fetch("http://127.0.0.1:8098/api/calendar")
  ).json();
  assert.equal(coastData.events.length, 1);
  assert.equal(coastData.events[0].venue, "Harbor");
  assert.equal(coastData.events[0].title, event.title);
  assert.equal(
    (await fetch("http://127.0.0.1:8098" + denData.events[0].public_path))
      .status,
    404,
  );
  assert.equal(
    (await fetch("http://127.0.0.1:8097" + event.public_path)).status,
    404,
  );
  assert(
    (
      await (
        await fetch("http://127.0.0.1:8098/api" + event.public_path + ".ics")
      ).text()
    ).includes("UID:harbor-012345abcdef@coastal.example"),
  );
  const denIcon = Buffer.from(
    await (
      await fetch("http://127.0.0.1:8097/assets/favicon.png")
    ).arrayBuffer(),
  );
  const coastIcon = Buffer.from(
    await (
      await fetch("http://127.0.0.1:8098/assets/favicon.png")
    ).arrayBuffer(),
  );
  assert.notDeepEqual(denIcon, coastIcon);
  const beforeImage = command(
    "docker",
    ["inspect", denName, "--format", "{{.Image}}"],
    { capture: true },
  );
  site.tagline = "Coastal test refresh.";
  save(join(coastal, "locales/coastal/site.json"), site);
  command("docker", ["build", "-f", dockerfile, "-t", coastImage, coastal]);
  command("docker", ["stop", coastName]);
  started.delete(coastName);
  // Docker can release the stopped process before --rm releases its name.
  start(coastName + "-rebuilt", coastImage, 8098);
  await ready(8098);
  assert.equal(
    command("docker", ["inspect", denName, "--format", "{{.Image}}"], {
      capture: true,
    }),
    beforeImage,
  );
  assert.deepEqual(
    Buffer.from(
      await (await fetch("http://127.0.0.1:8097/api/calendar")).arrayBuffer(),
    ),
    denBytes,
  );
  assert.equal(
    (await (await fetch("http://127.0.0.1:8098/api/site")).json()).tagline,
    "Coastal test refresh.",
  );
  command("npm", ["run", "test:e2e", "--", "tests/browser/locales.spec.ts"], {
    env: {
      ...process.env,
      LOCALE_DENVER_URL: "http://127.0.0.1:8097",
      LOCALE_COASTAL_URL: "http://127.0.0.1:8098",
    },
  });
  console.log(
    "Independent contexts, images, data, assets, event URLs, ICS namespaces and single-locale rebuild verified.",
  );
} finally {
  for (const name of started) command("docker", ["stop", name]);
  console.log(`Retained isolation build contexts: ${root}`);
}
