import { describe, expect, it } from "vitest";
import {
  calendarUrl,
  readCalendarContext,
  readEntryContext,
  readEntryScroll,
  sameContext,
  semanticView,
} from "./route";

describe("calendar route context", () => {
  it("round-trips the root URL for every view", () => {
    for (const view of ["day", "week", "month"] as const) {
      const url = calendarUrl({ view, date: "2026-09-08" });
      expect(url).toBe(`/?view=${view}&date=2026-09-08`);
      expect(readCalendarContext("?" + url.split("?")[1])).toEqual({
        view,
        date: "2026-09-08",
      });
    }
  });
  it("rejects unknown views, malformed dates, and missing parameters", () => {
    expect(readCalendarContext("")).toBeNull();
    expect(readCalendarContext("?view=week")).toBeNull();
    expect(readCalendarContext("?date=2026-09-08")).toBeNull();
    expect(readCalendarContext("?view=dayGridWeek&date=2026-09-08")).toBeNull();
    expect(readCalendarContext("?view=week&date=2026-9-8")).toBeNull();
    expect(readCalendarContext("?view=week&date=not-a-date")).toBeNull();
    expect(readCalendarContext("?view=week&date=2026-09-08&extra=1")).toEqual({
      view: "week",
      date: "2026-09-08",
    });
  });
  it("reads stored entry context and rejects foreign state", () => {
    expect(
      readEntryContext({ view: "month", date: "2026-10-01", scrollY: 480 }),
    ).toEqual({ view: "month", date: "2026-10-01" });
    expect(readEntryContext(null)).toBeNull();
    expect(readEntryContext("week")).toBeNull();
    expect(readEntryContext({ view: "dayGridWeek" })).toBeNull();
    expect(readEntryContext({ view: "week", date: "09/08/2026" })).toBeNull();
    expect(readEntryContext({ view: "week", date: 8 })).toBeNull();
  });
  it("reads finite saved scroll offsets only", () => {
    expect(readEntryScroll({ scrollY: 320 })).toBe(320);
    expect(readEntryScroll({})).toBeNull();
    expect(readEntryScroll(null)).toBeNull();
    expect(readEntryScroll({ scrollY: "320" })).toBeNull();
    expect(readEntryScroll({ scrollY: Number.NaN })).toBeNull();
  });
  it("compares positions by view and date", () => {
    expect(
      sameContext(
        { view: "week", date: "2026-09-06" },
        { view: "week", date: "2026-09-06" },
      ),
    ).toBe(true);
    expect(
      sameContext(
        { view: "week", date: "2026-09-06" },
        { view: "week", date: "2026-09-08" },
      ),
    ).toBe(false);
    expect(
      sameContext(
        { view: "day", date: "2026-09-08" },
        { view: "week", date: "2026-09-08" },
      ),
    ).toBe(false);
  });
  it("maps FullCalendar view types to addressable names", () => {
    expect(semanticView("dayGridWeek")).toBe("week");
    expect(semanticView("listWeek")).toBe("week");
    expect(semanticView("dayGridMonth")).toBe("month");
    expect(semanticView("listMonth")).toBe("month");
    expect(semanticView("dayGridDay")).toBe("day");
    expect(semanticView("listDay")).toBe("day");
    expect(semanticView("somethingElse")).toBe("week");
  });
});
