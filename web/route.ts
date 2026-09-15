import type { View } from "./view";

// The calendar position is addressable on the root URL so Back/Forward and
// reloads return to the exact viewed range. Event entries carry the same
// position in history state because their path is fixed by the record.
export interface CalendarContext {
  view: View;
  date: string;
}

const datePattern = /^\d{4}-\d{2}-\d{2}$/;

function isView(value: unknown): value is View {
  return value === "day" || value === "week" || value === "month";
}

export function calendarUrl(context: CalendarContext): string {
  return `/?view=${context.view}&date=${context.date}`;
}

export function readCalendarContext(search: string): CalendarContext | null {
  const params = new URLSearchParams(search);
  const view = params.get("view");
  const date = params.get("date");
  if (!isView(view) || date === null || !datePattern.test(date)) return null;
  return { view, date };
}

export function readEntryContext(state: unknown): CalendarContext | null {
  if (typeof state !== "object" || state === null) return null;
  const { view, date } = state as Record<string, unknown>;
  return isView(view) && typeof date === "string" && datePattern.test(date)
    ? { view, date }
    : null;
}

export function readEntryScroll(state: unknown): number | null {
  if (typeof state !== "object" || state === null) return null;
  const { scrollY } = state as Record<string, unknown>;
  return typeof scrollY === "number" && Number.isFinite(scrollY)
    ? scrollY
    : null;
}

export function sameContext(a: CalendarContext, b: CalendarContext): boolean {
  return a.view === b.view && a.date === b.date;
}

// Maps a FullCalendar view type ("dayGridWeek", "listMonth", ...) to the
// addressable view name shared by the desktop and phone layouts.
export function semanticView(type: string): View {
  const name = type.replace(/^(?:dayGrid|list)/, "").toLowerCase();
  return isView(name) ? name : "week";
}
