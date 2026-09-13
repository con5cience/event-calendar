import type { EventInput } from "@fullcalendar/react";
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
  return new Intl.DateTimeFormat("en-US", {
    timeZone: event.timezone,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
    timeZoneName: "short",
  }).format(new Date(instant));
}

const collator = new Intl.Collator("en", {
  numeric: true,
  sensitivity: "base",
});
export function compareEvents(a: CalendarEvent, b: CalendarEvent): number {
  return (
    a.date.localeCompare(b.date) ||
    displayTime(a).slice(0, 5).localeCompare(displayTime(b).slice(0, 5)) ||
    collator.compare(a.venue, b.venue) ||
    collator.compare(a.artist || a.title, b.artist || b.title) ||
    collator.compare(a.title, b.title) ||
    a.id.localeCompare(b.id)
  );
}

export type ProjectedEvent = EventInput & {
  extendedProps: {
    kind: "event" | "more";
    record?: CalendarEvent;
    date: string;
    hiddenCount?: number;
    sortIndex: number;
  };
};

export function projectEvents(
  events: CalendarEvent[],
  limit: number | null,
  capWithMoreRow: boolean,
): ProjectedEvent[] {
  const days = new Map<string, CalendarEvent[]>();
  for (const event of [...events].sort(compareEvents)) {
    const day = days.get(event.date) || [];
    day.push(event);
    days.set(event.date, day);
  }
  return [...days].flatMap(([date, day]) => {
    const visible = capWithMoreRow && limit ? day.slice(0, limit) : day;
    // These are date-card placements, not an assertion of all-day duration.
    // The source instant remains on record and is formatted in its venue timezone.
    const rows: ProjectedEvent[] = visible.map((record, sortIndex) => ({
      id: record.id,
      title: record.title,
      start: date,
      allDay: true,
      extendedProps: { kind: "event", record, date, sortIndex },
    }));
    if (visible.length < day.length)
      rows.push({
        id: `more:${date}`,
        title: "Show All",
        start: date,
        allDay: true,
        extendedProps: {
          kind: "more",
          date,
          hiddenCount: day.length - visible.length,
          sortIndex: day.length,
        },
      });
    return rows;
  });
}
