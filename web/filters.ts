import { displayStatus, displayTime, type CalendarEvent } from "./calendar";
import { site } from "./site";

export interface Preferences {
  withAdult: boolean;
  childAge: number;
  venues: string[]; // Empty means All, including newly added venues.
  query: string;
}
export const defaultPreferences = (): Preferences => ({
  withAdult: false,
  childAge: 14,
  venues: [],
  query: "",
});
const storageKey = () => `${site.storage_namespace}.filters.v1`;

export function readPreferences(
  storage: () => Pick<Storage, "getItem"> = () => localStorage,
): Preferences {
  try {
    const value: unknown = JSON.parse(
      storage().getItem(storageKey()) || "null",
    );
    if (
      value &&
      typeof value === "object" &&
      "venues" in value &&
      "query" in value &&
      Array.isArray(value.venues) &&
      value.venues.every((v) => typeof v === "string") &&
      typeof value.query === "string"
    ) {
      return {
        withAdult: "withAdult" in value && value.withAdult === true,
        childAge:
          "childAge" in value && validChildAge(value.childAge)
            ? value.childAge
            : 14,
        venues: [...new Set(value.venues)],
        query: value.query,
      };
    }
  } catch {
    /* Storage is optional, including in private browsing. */
  }
  return defaultPreferences();
}

export function savePreferences(
  value: Preferences,
  storage: () => Pick<Storage, "setItem"> = () => localStorage,
): void {
  try {
    storage().setItem(storageKey(), JSON.stringify(value));
  } catch {
    /* Keep working with in-memory preferences when storage is blocked/full. */
  }
}

export function filterEvents(
  events: CalendarEvent[],
  prefs: Preferences,
): CalendarEvent[] {
  const words = prefs.query.toLowerCase().trim().split(/\s+/).filter(Boolean);
  return events.filter((event) => {
    if (prefs.withAdult && !adultCondition(event, prefs.childAge)) return false;
    if (prefs.venues.length && !prefs.venues.includes(event.venue))
      return false;
    if (!words.length) return true;
    // Search public metadata, never internal IDs or source configuration.
    const text = [
      event.title,
      event.venue,
      event.date,
      event.timezone,
      event.artist,
      event.doors_at,
      event.show_at,
      displayTime(event),
      event.age_policy,
      event.age_category,
      event.age_policy_url,
      ...(event.with_adult?.ranges.map((range) => range.condition) ?? []),
      event.status,
      displayStatus(event),
      event.event_url,
      event.ticket_url,
      event.off_site ? "off-site" : "",
    ]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();
    return words.every((word) => text.includes(word));
  });
}
export function validChildAge(age: unknown): age is number {
  return (
    typeof age === "number" && Number.isInteger(age) && age >= 0 && age <= 17
  );
}
export function adultCondition(
  event: CalendarEvent,
  age: number,
): string | undefined {
  if (!validChildAge(age)) return undefined;
  return event.with_adult?.ranges.find(
    (range) => age >= range.min_age && age <= range.max_age,
  )?.condition;
}
