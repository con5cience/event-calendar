import { afterEach, expect, it } from "vitest";
import denver from "../tests/contracts/site.json";
import { configureSite, site, type Site } from "./site";
import { venueColor } from "./venueColors";
import { readView } from "./view";

afterEach(() => configureSite(denver as Site));
it("uses only the selected locale palette and default view", () => {
  configureSite({
    ...denver,
    id: "coastal",
    city: "coastal",
    timezone: "Pacific/Auckland",
    default_view: "month",
    storage_namespace: "withadult.coastal",
    venue_colors: { Harbor: "#112233" },
  } as Site);
  expect(site.city).toBe("coastal");
  expect(venueColor("Harbor")).toBe("#112233");
  expect(venueColor("Gothic Theatre")).toBe("#6C5CE7");
  expect(readView()).toBe("month");
});
