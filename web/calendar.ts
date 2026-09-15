import type { EventInput } from "@fullcalendar/react";
import { site, siteCollator } from "./site";
import type { AgeCategory, AdultAdmission } from "./artifacts/types";

export const ageCategories: readonly AgeCategory[] = [
  "All ages",
  "13+",
  "16+",
  "18+",
  "21+",
];

// Display projection from the server-validated catalog, not the stored artifact shape.
export interface CalendarEvent {
  with_adult?: AdultAdmission;
  id: string;
  public_path?: string;
  listed?: boolean;
  title: string;
  venue: string;
  date: string;
  timezone: string;
  artist?: string;
  doors_at?: string;
  show_at?: string;
  age_policy?: string;
  age_policy_url?: string;
  age_category?: AgeCategory;
  off_site?: boolean;
  status?: string;
  event_url?: string;
  ticket_url?: string;
}

export interface CalendarData {
  events: CalendarEvent[];
  // Week and month limits stay in the payload for compatibility, but nothing
  // applies them: phones list every event for a date and scroll, and desktop
  // Week/Month fit their available cell height. Only the desktop Day cap is
  // read.
  limits: { day: number | null; week: number; month: number };
  initial_date?: string;
}

// Display-only corrections for reviewed upstream wording. Preserve unfamiliar
// text and abbreviations rather than lowercasing policies indiscriminately.
export function displayAgePolicy(text: string): string {
  switch (text) {
    case "All ages, ticketed guests under 16 ONLY ADMITTED WITH TICKETED GUARDIAN 21+":
      return "All ages, ticketed guests under 16 only admitted with ticketed guardian 21+";
    case "RECOMMENDED AGE 18+":
      return "Recommended age 18+";
    default:
      return text;
  }
}

// Presentation only: preserve the provider's raw status on the event.
export function displayStatus(event: CalendarEvent): "Scheduled" | "Cancelled" {
  const status = event.status?.trim().toLowerCase();
  return status === "cancelled" || status === "canceled"
    ? "Cancelled"
    : "Scheduled";
}

export function displayTime(event: CalendarEvent): string {
  const instant = event.doors_at || event.show_at;
  if (!instant) return "";
  return new Intl.DateTimeFormat(site.language, {
    timeZone: event.timezone,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
    timeZoneName: "short",
  }).format(new Date(instant));
}

export function compareEvents(a: CalendarEvent, b: CalendarEvent): number {
  return (
    a.date.localeCompare(b.date) ||
    displayTime(a).slice(0, 5).localeCompare(displayTime(b).slice(0, 5)) ||
    siteCollator.compare(a.venue, b.venue) ||
    siteCollator.compare(a.artist || a.title, b.artist || b.title) ||
    siteCollator.compare(a.title, b.title) ||
    a.id.localeCompare(b.id)
  );
}

// Every event is projected; FullCalendar's own dayMaxEvents option performs
// any visible slicing, so the feed never hides domain records.
export type ProjectedEvent = EventInput & {
  extendedProps: {
    record: CalendarEvent;
    sortIndex: number;
  };
};

export function projectEvents(events: CalendarEvent[]): ProjectedEvent[] {
  const days = new Map<string, CalendarEvent[]>();
  for (const event of [...events].sort(compareEvents)) {
    const day = days.get(event.date) || [];
    day.push(event);
    days.set(event.date, day);
  }
  return [...days].flatMap(([date, day]) =>
    // These are date-card placements, not an assertion of all-day duration.
    // The source instant remains on record and is formatted in its venue timezone.
    day.map((record, sortIndex) => ({
      id: record.id,
      title: record.title,
      start: date,
      allDay: true,
      extendedProps: { record, sortIndex },
    })),
  );
}
