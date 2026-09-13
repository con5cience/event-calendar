export type View = "day" | "week" | "month";
import { site } from "./site";
const storageKey = () => `${site.storage_namespace}.view.v1`;

export function readView(): View {
  try {
    const value = localStorage.getItem(storageKey());
    if (value === "day" || value === "week" || value === "month") return value;
  } catch {
    // Storage is optional, as it is for filters.
  }
  return site.default_view;
}

export function saveView(view: View): void {
  try {
    localStorage.setItem(storageKey(), view);
  } catch {
    // Keep the selected view in memory when storage is unavailable.
  }
}
