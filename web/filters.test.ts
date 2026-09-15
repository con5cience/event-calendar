import { describe, expect, it } from "vitest";
import {
  filterEvents,
  readPreferences,
  savePreferences,
  defaultPreferences,
} from "./filters";
import { projectEvents, type CalendarEvent } from "./calendar";

const record = (
  id: string,
  extra: Partial<CalendarEvent> = {},
): CalendarEvent => ({
  id,
  title: "Evening Echo",
  venue: "Gothic Theatre",
  date: "2026-09-08",
  timezone: "America/Denver",
  ...extra,
});
describe("discovery filters", () => {
  it("uses reviewed adult admission instead of restriction categories and respects age boundaries", () => {
    const admission = {
      url: "https://example.com/policy",
      reviewed_on: "2026-09-09",
      ranges: [
        { min_age: 11, max_age: 15, condition: "Ticketed adult required" },
      ],
    };
    const rows = [
      record("allowed", { age_category: "16+", with_adult: admission }),
      record("unknown", { age_category: "All ages" }),
      record("denied", { with_adult: { ...admission, ranges: [] } }),
    ];
    for (const age of [10, 11, 14, 15, 16]) {
      expect(
        filterEvents(rows, {
          ...defaultPreferences(),
          withAdult: true,
          childAge: age,
        }).map((e) => e.id),
      ).toEqual(age >= 11 && age <= 15 ? ["allowed"] : []);
    }
    expect(
      readPreferences(() => ({
        getItem: () =>
          JSON.stringify({
            venues: [],
            query: "",
            withAdult: true,
            childAge: 14,
          }),
      })),
    ).toMatchObject({ withAdult: true, childAge: 14 });
    expect(
      readPreferences(() => ({
        getItem: () =>
          JSON.stringify({
            venues: [],
            query: "",
            withAdult: true,
            childAge: -1,
          }),
      })),
    ).toMatchObject({ withAdult: true, childAge: 14 });
  });
  it("ignores retired age selections without hiding events", () => {
    const rows = [
      record("all", { age_category: "All ages" }),
      record("adult", { age_category: "21+" }),
      record("unknown"),
    ];
    const prefs = readPreferences(() => ({
      getItem: () => JSON.stringify({ ...defaultPreferences(), ages: ["21+"] }),
    }));
    expect(prefs).not.toHaveProperty("ages");
    expect(filterEvents(rows, prefs)).toEqual(rows);
  });
  it("shows every supplied retained date by default regardless of timezone", () => {
    const rows = [
      record("past", { date: "2026-09-07" }),
      record("untimed"),
      record("early", { doors_at: "2026-09-08T09:00:00-06:00" }),
      record("future", { date: "2027-01-01" }),
      record("tokyo", { timezone: "Asia/Tokyo" }),
    ];
    expect(filterEvents(rows, defaultPreferences())).toEqual(rows);
  });
  it("combines any selected venue with every query word across public metadata", () => {
    const rows = [
      record("yes", { artist: "Alice", age_policy: "16+" }),
      record("other", { venue: "Mission Ballroom", artist: "Alice" }),
      record("missing"),
    ];
    const prefs = {
      ...defaultPreferences(),
      venues: ["Gothic Theatre", "HQ"],
      query: " ALIce  16+ ",
      includePast: false,
    };
    expect(filterEvents(rows, prefs).map((e) => e.id)).toEqual(["yes"]);
    expect(filterEvents(rows, { ...prefs, query: "alice missing" })).toEqual(
      [],
    );
    expect(filterEvents(rows, defaultPreferences())).toEqual(rows);
    expect(filterEvents(rows, { ...prefs, query: "yes" })).toEqual([]); // IDs are internal.
  });
  it("searches dates outside the current view and filters without mutating records", () => {
    const rows = [
      record("hidden"),
      record("match", { title: "Needle", date: "2027-01-01" }),
    ];
    const before = JSON.stringify(rows);
    const matches = filterEvents(rows, {
      ...defaultPreferences(),
      query: "NEED",
    });
    expect(projectEvents(matches).map((e) => e.id)).toEqual(["match"]);
    expect(JSON.stringify(rows)).toBe(before);
  });
  it("searches readable times, status and optional links, not absent values", () => {
    const row = record("id", {
      status: "Canceled",
      doors_at: "2026-09-09T01:00:00Z",
      ticket_url: "https://tickets.example.test/echo",
      off_site: true,
    });
    for (const query of [
      "cancelled",
      "19:00 MDT",
      "tickets.example",
      "off-site",
      "2026-09-08",
    ]) {
      expect(filterEvents([row], { ...defaultPreferences(), query })).toEqual([
        row,
      ]);
    }
    expect(
      filterEvents([record("sparse")], {
        ...defaultPreferences(),
        query: "undefined",
      }),
    ).toEqual([]);
  });
});
describe("browser preferences", () => {
  it("ignores retired past visibility while preserving the other saved filters", () => {
    for (const includePast of [false, true, undefined]) {
      const saved = {
        venues: ["HQ"],
        query: "Alice",
        includePast,
      };
      const prefs = readPreferences(() => ({
        getItem: () => JSON.stringify(saved),
      }));
      expect(prefs).toEqual({
        ...defaultPreferences(),
        venues: ["HQ"],
        query: "Alice",
      });
      expect(
        filterEvents(
          [
            record("past", {
              date: "2026-09-01",
              venue: "HQ",
              age_category: "16+",
              artist: "Alice",
            }),
          ],
          prefs,
        ),
      ).toHaveLength(1);
    }
  });
  it("ignores saved cost selections so unpriced events remain visible", () => {
    const prefs = readPreferences(() => ({
      getItem: () =>
        JSON.stringify({ ...defaultPreferences(), costs: ["$$$$"] }),
    }));
    expect(prefs).not.toHaveProperty("costs");
    const rows = [record("unknown"), { ...record("priced"), price: "$20" }];
    expect(filterEvents(rows, prefs)).toEqual(rows);
  });
  it("never searches legacy price or cost metadata", () => {
    const legacy = { ...record("legacy"), price: "$20", cost_category: "$$$$" };
    for (const query of ["$20", "$$$$"]) {
      expect(
        filterEvents([legacy], { ...defaultPreferences(), query }),
      ).toEqual([]);
    }
  });
  it("round-trips preferences with a versioned storage key", () => {
    const values = new Map<string, string>();
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => {
        values.set(key, value);
      },
    };
    const prefs = {
      ...defaultPreferences(),
      venues: ["HQ"],
      query: "Alice",
    };
    savePreferences(prefs, () => storage);
    expect(readPreferences(() => storage)).toEqual(prefs);
    expect([...values.keys()]).toEqual(["event-calendar.filters.v1"]);
  });
  it("migrates old preferences without losing selections and rejects unknown categories independently", () => {
    const old = { venues: ["HQ"], query: "Alice", includePast: true };
    expect(
      readPreferences(() => ({ getItem: () => JSON.stringify(old) })),
    ).toEqual({
      ...defaultPreferences(),
      venues: ["HQ"],
      query: "Alice",
    });
    expect(
      readPreferences(() => ({
        getItem: () =>
          JSON.stringify({
            ...old,
            ages: ["16+", "16+", "unknown"],
            costs: ["$$", "bogus"],
          }),
      })),
    ).toEqual({
      ...defaultPreferences(),
      venues: ["HQ"],
      query: "Alice",
    });
  });
  it("uses defaults for malformed or unavailable storage, and ignores write failures", () => {
    for (const value of [
      null,
      "broken",
      "null",
      "{}",
      '{"venues":[3],"query":"","includePast":false}',
      '{"venues":[],"query":3,"includePast":false}',
    ]) {
      expect(readPreferences(() => ({ getItem: () => value }))).toEqual(
        defaultPreferences(),
      );
    }
    const unavailable = () => {
      throw new Error("storage denied");
    };
    expect(readPreferences(unavailable)).toEqual(defaultPreferences());
    expect(() =>
      savePreferences(defaultPreferences(), unavailable),
    ).not.toThrow();
    expect(() =>
      savePreferences(defaultPreferences(), () => ({
        setItem: () => {
          throw new Error("quota");
        },
      })),
    ).not.toThrow();
  });
});
