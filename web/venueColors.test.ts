import { describe, expect, it } from "vitest";
import { venueColor } from "./venueColors";
import { site } from "./site";

describe("fixed venue palette", () => {
  it("assigns a unique approved color to every integrated venue", () => {
    expect(Object.keys(site.venue_colors)).toHaveLength(26);
    expect(new Set(Object.values(site.venue_colors)).size).toBe(26);
    expect(venueColor("Gothic Theatre")).toBe("#FFAD52");
    expect(venueColor("Mission Ballroom")).toBe("#56F0E5");
    expect(venueColor("Fiddler's Green Amphitheatre")).toBe("#228F60");
    expect(venueColor("Red Rocks Amphitheatre")).toBe("#DD7027");
    expect(venueColor("Herb's")).toBe("#F6D8B0");
    expect(venueColor("Ophelia's Electric Soapbox")).toBe("#247F7B");
  });
  it("uses the existing accent for unknown names, including object property names", () => {
    for (const name of ["New venue", "", "constructor", "__proto__"])
      expect(venueColor(name)).toBe("#6C5CE7");
  });
});
