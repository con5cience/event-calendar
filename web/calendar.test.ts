import { describe, expect, it } from "vitest";
import {
  compareEvents,
  displayTime,
  displayStatus,
  displayAgePolicy,
  projectEvents,
  type CalendarEvent,
} from "./calendar";
const event = (
  id: string,
  extra: Partial<CalendarEvent> = {},
): CalendarEvent => ({
  id,
  title: id,
  venue: "Mission Ballroom",
  date: "2026-09-08",
  timezone: "America/Denver",
  ...extra,
});
describe("calendar projection", () => {
  it("normalizes only the reviewed uppercase policy phrases for display", () => {
    expect(
      displayAgePolicy(
        "All ages, ticketed guests under 16 ONLY ADMITTED WITH TICKETED GUARDIAN 21+",
      ),
    ).toBe(
      "All ages, ticketed guests under 16 only admitted with ticketed guardian 21+",
    );
    expect(displayAgePolicy("RECOMMENDED AGE 18+")).toBe("Recommended age 18+");
    for (const text of [
      "All Ages / Bar with ID",
      "16 & Over",
      "VIP ONLY",
      "RECOMMENDED AGE 21+",
      "",
    ])
      expect(displayAgePolicy(text)).toBe(text);
  });
  it("uses artist after time and venue, with title as fallback", () => {
    const rows = [
      event("Alpha", { artist: "Zulu" }),
      event("Bravo"),
      event("Zulu", { artist: "Alpha" }),
    ];
    expect(rows.sort(compareEvents).map((e) => e.id)).toEqual([
      "Zulu",
      "Bravo",
      "Alpha",
    ]);
  });
  it("maps explicit cancellation to Cancelled and everything else to Scheduled without changing raw data", () => {
    for (const status of ["Cancelled", " CANCELLED ", "canceled"]) {
      const record = event("cancelled", { status });
      expect(displayStatus(record)).toBe("Cancelled");
      expect(record.status).toBe(status);
    }
    for (const status of [
      undefined,
      "",
      "Buy Tickets",
      "Get Tickets",
      "Scheduled",
      "Not cancelled",
      "Cancelled <notice>",
    ]) {
      expect(displayStatus(event("scheduled", { status }))).toBe("Scheduled");
    }
  });
  it("orders untimed first, then venue/title, and prefers doors over show", () => {
    const events = [
      event("Zulu", { doors_at: "2026-09-08T19:00:00-06:00" }),
      event("Untimed Z", { venue: "Z room" }),
      event("Alpha", { doors_at: "2026-09-08T19:00:00-06:00" }),
      event("First doors", {
        doors_at: "2026-09-08T18:00:00-06:00",
        show_at: "2026-09-08T22:00:00-06:00",
      }),
      event("Untimed A", { venue: "A room" }),
      event("Earlier venue", {
        venue: "Gothic",
        show_at: "2026-09-08T19:00:00-06:00",
      }),
    ];
    expect(events.sort(compareEvents).map((e) => e.id)).toEqual([
      "Untimed A",
      "Untimed Z",
      "First doors",
      "Earlier venue",
      "Alpha",
      "Zulu",
    ]);
  });
  it("formats the venue's time, including daylight saving, independently of browser timezone", () => {
    expect(
      displayTime(event("summer", { doors_at: "2026-09-09T01:00:00Z" })),
    ).toBe("19:00 MDT");
    expect(
      displayTime(
        event("winter", {
          date: "2026-12-08",
          doors_at: "2026-12-09T02:00:00Z",
        }),
      ),
    ).toBe("19:00 MST");
    expect(displayTime(event("unknown"))).toBe("");
  });
  it("caps each day separately and adds a presentation-only drill-in row", () => {
    const events = Array.from({ length: 14 }, (_, i) =>
      event(String(i).padStart(2, "0")),
    );
    events.push(event("next day", { date: "2026-09-09" }));
    const before = JSON.stringify(events);
    const rows = projectEvents(events, 10, true);
    expect(rows.filter((r) => r.extendedProps.kind === "event")).toHaveLength(
      11,
    );
    expect(rows.filter((r) => r.extendedProps.kind === "more")).toHaveLength(1);
    expect(
      rows.find((r) => r.extendedProps.kind === "more")?.extendedProps
        .hiddenCount,
    ).toBe(4);
    expect(JSON.stringify(events)).toBe(before);
  });
  it("formats both sides of Denver DST transitions without inventing times", () => {
    for (const [instant, expected] of [
      ["2026-03-08T08:30:00Z", "01:30 MST"],
      ["2026-03-08T09:30:00Z", "03:30 MDT"],
      ["2026-11-01T07:30:00Z", "01:30 MDT"],
      ["2026-11-01T08:30:00Z", "01:30 MST"],
    ])
      expect(displayTime(event("transition", { doors_at: instant }))).toBe(
        expected,
      );
  });
  it("leaves the domain dataset complete and day view unlimited by default", () => {
    const events = Array.from({ length: 14 }, (_, i) => event(String(i)));
    expect(projectEvents(events, null, true)).toHaveLength(14);
    expect(projectEvents(events, 5, false)).toHaveLength(14);
  });
});
