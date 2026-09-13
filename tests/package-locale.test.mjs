import { test } from "node:test";
import assert from "node:assert/strict";
import { validateLocaleID } from "../scripts/package-locale.mjs";
test("packaging requires one safe locale identifier", () => {
  assert.equal(validateLocaleID("denver"), "denver");
  for (const id of ["", "../denver", "denver,coastal", "Denver", "a/b"])
    assert.throws(() => validateLocaleID(id));
});
