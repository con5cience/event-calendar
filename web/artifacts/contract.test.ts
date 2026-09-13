import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import cases from "../../tests/contracts/cases.json";
import {
  decodeArtifact,
  decodeCatalog,
  validateConfig,
  effectivePolicy,
  effectiveVenue,
  expired,
} from "./contract";

describe("shared version-one contract", () => {
  it("preserves literal status and rejects invalid status values", () => {
    const a = JSON.parse(readFileSync("tests/contracts/source.json", "utf8"));
    a.events[0].status = "Cancelled";
    expect(
      JSON.parse(JSON.stringify(decodeArtifact(JSON.stringify(a)))).events[0]
        .status,
    ).toBe("Cancelled");
    for (const value of [null, "", " ", 2, "a".repeat(2049)]) {
      a.events[0].status = value;
      expect(() => decodeArtifact(JSON.stringify(a))).toThrow();
    }
  });
  it("validates the multi-source browser fixture and its exact checksums", () => {
    const c = decodeCatalog(
      readFileSync("tests/fixtures/catalog.json", "utf8"),
    );
    for (const ref of c.sources) {
      const bytes = readFileSync(join("tests/fixtures", ref.artifact));
      expect(createHash("sha256").update(bytes).digest("hex")).toBe(ref.sha256);
      expect(decodeArtifact(bytes.toString("utf8")).source.id).toBe(
        ref.source_id,
      );
    }
  });
  it("accepts an event identity under the longest allowed source key", () => {
    const a = JSON.parse(readFileSync("tests/contracts/source.json", "utf8"));
    a.source.id = "a".repeat(100);
    a.events[0].id = `${a.source.id}-012345abcdef`;
    expect(() => decodeArtifact(JSON.stringify(a))).not.toThrow();
  });
  for (const fixture of cases)
    it(fixture.name, () => {
      const check = () => {
        if (fixture.kind === "source-artifact")
          return decodeArtifact(JSON.stringify(fixture.document));
        if (fixture.kind === "catalog")
          return decodeCatalog(JSON.stringify(fixture.document));
        return validateConfig(fixture.document);
      };
      if (fixture.valid) expect(check).not.toThrow();
      else expect(check).toThrow();
    });
  it("applies replacement policy inheritance without leaking off-site defaults", () => {
    const a = decodeArtifact(
      readFileSync("tests/contracts/source.json", "utf8"),
    );
    const e = a.events[0];
    expect(effectivePolicy(a.venue, e)?.text).toBe("16+");
    e.admission_policy = { text: "21+" };
    expect(effectivePolicy(a.venue, e)).toEqual({ text: "21+" });
    delete e.admission_policy;
    e.venue = { key: "warehouse", name: "Warehouse" };
    e.off_site = true;
    expect(effectivePolicy(a.venue, e)).toBeUndefined();
    expect(effectiveVenue(a.venue, e).timezone).toBeUndefined();
    expect(expired(e, "2026-12-08")).toBe(false);
    expect(expired(e, "2026-12-09")).toBe(true);
  });
  it.skipIf(!process.env.CONTRACT_HANDOFF_DIR)(
    "consumes bytes produced by Go reconciliation",
    () => {
      const directory = process.env.CONTRACT_HANDOFF_DIR!;
      const catalog = decodeCatalog(
        readFileSync(join(directory, "catalog.json"), "utf8"),
      );
      expect(catalog.sources).toHaveLength(1);
      const ref = catalog.sources[0];
      const bytes = readFileSync(join(directory, ref.artifact));
      expect(createHash("sha256").update(bytes).digest("hex")).toBe(ref.sha256);
      const a = decodeArtifact(bytes.toString("utf8"));
      expect(a.source.id).toBe(ref.source_id);
      expect(a.events).toHaveLength(1);
      expect(a.events[0].title).toBe("Configured title");
      expect(a.events[0].public_path).toBe(
        "/events/gothic/2026-09-10-configured-title-000000000001",
      );
      expect(effectivePolicy(a.venue, a.events[0])).toEqual({
        text: "18+",
        category: "18+",
      });
    },
  );
});
