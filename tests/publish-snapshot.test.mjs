import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawnSync } from "node:child_process";
import { publishSnapshot } from "../scripts/publish-snapshot.mjs";

function fixture() {
  const root = mkdtempSync(join(tmpdir(), "publish-snapshot-"));
  const remote = join(root, "remote.git");
  const checkout = join(root, "checkout");
  const git = (args, cwd = root) => {
    const result = spawnSync("git", args, { cwd, encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    return result.stdout.trim();
  };
  git(["init", "--bare", "--initial-branch=main", remote]);
  git(["clone", remote, checkout]);
  git(["config", "user.name", "Fixture"], checkout);
  git(["config", "user.email", "fixture@example.invalid"], checkout);
  mkdirSync(join(checkout, "locales/denver/catalog"), { recursive: true });
  writeFileSync(join(checkout, "locales/denver/catalog/catalog.json"), "seed");
  mkdirSync(join(checkout, "XYZlocales/denver/catalog"), { recursive: true });
  writeFileSync(
    join(checkout, "XYZlocales/denver/catalog/outside.json"),
    "outside",
  );
  git(["add", "."], checkout);
  git(["commit", "-m", "seed"], checkout);
  git(["push", "origin", "HEAD:refs/heads/main"], checkout);
  const sha = git(["rev-parse", "HEAD"], checkout);
  return { root, remote, checkout, git, sha };
}

test("publication commits only the selected catalog and supports no-op snapshots", () => {
  const f = fixture();
  assert.equal(publishSnapshot(f.checkout, "denver", f.sha), f.sha);
  writeFileSync(
    join(f.checkout, "locales/denver/catalog/catalog.json"),
    "refreshed",
  );
  const sha = publishSnapshot(f.checkout, "denver", f.sha);
  assert.notEqual(sha, f.sha);
  assert.equal(f.git(["rev-parse", "main"], f.remote), sha);
  assert.equal(
    f.git(["show", `${sha}:locales/denver/catalog/catalog.json`], f.remote),
    "refreshed",
  );
});

test("publication refuses stale runs and unrelated changes without pushing", () => {
  for (const mode of [
    "stale",
    "dirty",
    "wrong-head",
    "unsafe-locale",
    "staged-rename",
  ]) {
    const f = fixture();
    if (mode === "stale") {
      const other = join(f.root, "other");
      f.git(["clone", f.remote, other]);
      f.git(
        [
          "-c",
          "user.name=Fixture",
          "-c",
          "user.email=fixture@example.invalid",
          "commit",
          "--allow-empty",
          "-m",
          "newer",
        ],
        other,
      );
      f.git(["push", "origin", "main"], other);
    }
    if (mode === "dirty")
      writeFileSync(join(f.checkout, "unrelated.txt"), "preserve");
    if (mode === "staged-rename")
      f.git(
        [
          "mv",
          "XYZlocales/denver/catalog/outside.json",
          "locales/denver/catalog/moved.json",
        ],
        f.checkout,
      );
    const before = f.git(["rev-parse", "main"], f.remote);
    writeFileSync(
      join(f.checkout, "locales/denver/catalog/catalog.json"),
      "refreshed",
    );
    assert.throws(() =>
      publishSnapshot(
        f.checkout,
        mode === "unsafe-locale" ? "../denver" : "denver",
        mode === "wrong-head" ? "a".repeat(40) : f.sha,
      ),
    );
    assert.equal(f.git(["rev-parse", "main"], f.remote), before);
  }
});
