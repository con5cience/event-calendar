export interface Site {
  id: string;
  city: string;
  name: string;
  tagline: string;
  timezone: string;
  language: string;
  week_start: number;
  default_view: "day" | "week" | "month";
  storage_namespace: string;
  venue_colors: Readonly<Record<string, string>>;
}

// One server-validated locale per document. Never choose a locale from Host,
// localStorage, query parameters, or a hidden Denver fallback.
export let site: Site;
export let siteCollator: Intl.Collator;
export function configureSite(value: Site): void {
  if (!value?.id || !value.timezone || !value.venue_colors)
    throw new Error("Site configuration is unavailable");
  site = value;
  siteCollator = new Intl.Collator(value.language, {
    numeric: true,
    sensitivity: "base",
  });
}
