import { test } from "node:test";
import assert from "node:assert/strict";
import { captureProfile } from "./capture-profile.mjs";
test("capture rejects missing locale and unassigned sources", () => {
  const previous = process.env.SITE_DIR;
  delete process.env.SITE_DIR;
  assert.throws(() => captureProfile("marquis"), /SITE_DIR/);
  process.env.SITE_DIR = previous;
  assert.throws(() => captureProfile("other-city"), /Unsupported/);
});
