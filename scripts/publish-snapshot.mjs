// Fast-forward only. Never merge stale captures or stage unrelated changes.
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { validateLocaleID } from "./package-locale.mjs";

export function publishSnapshot(root, locale, expectedHead) {
  validateLocaleID(locale);
  assert.match(expectedHead, /^[a-f0-9]{40}$/);
  const git = (...args) => {
    const result = spawnSync("git", args, {
      cwd: root,
      encoding: "utf8",
      timeout: 60000,
    });
    assert.equal(result.status, 0, `git ${args[0]} failed`);
    return result.stdout.replace(/\n$/, "");
  };
  assert.equal(
    git("rev-parse", "HEAD"),
    expectedHead,
    "Checkout differs from workflow commit",
  );
  git("fetch", "origin", "refs/heads/main");
  assert.equal(
    git("rev-parse", "FETCH_HEAD"),
    expectedHead,
    "main changed during refresh; start a new run",
  );
  const prefix = `locales/${locale}/catalog/`;
  const changed = git(
    "status",
    "--porcelain=v1",
    "--no-renames",
    "-z",
    "--untracked-files=all",
  );
  const paths = changed
    .split("\0")
    .filter(Boolean)
    .map((line) => line.slice(3));
  assert(
    paths.every((path) => path.startsWith(prefix)),
    "Unrelated changes block snapshot publication",
  );
  git("add", "--all", "--", prefix);
  if (git("diff", "--cached", "--name-only")) {
    git(
      "-c",
      "user.name=github-actions[bot]",
      "-c",
      "user.email=41898282+github-actions[bot]@users.noreply.github.com",
      "commit",
      "-m",
      `data: refresh ${locale} event snapshot`,
    );
    git("push", "origin", "HEAD:refs/heads/main");
  }
  return git("rev-parse", "HEAD");
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  try {
    const [locale, sha] = process.argv.slice(2);
    console.log(publishSnapshot(process.cwd(), locale, sha));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
