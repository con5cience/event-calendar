import { test } from "node:test";
import assert from "node:assert/strict";
import { validateLocaleID, snapshotPath } from "../scripts/package-locale.mjs";
test("packaging requires one safe locale identifier", () => {
  assert.equal(validateLocaleID("denver"), "denver");
  for (const id of ["", "../denver", "denver,coastal", "Denver", "a/b"])
    assert.throws(() => validateLocaleID(id));
});
test("packaging selects the tracked locale catalog, not local working artifacts", () => {
  assert.equal(
    snapshotPath("/repo", "denver"),
    "/repo/locales/denver/catalog/catalog.json",
  );
  assert.throws(() => snapshotPath("/repo", "../denver"));
});
