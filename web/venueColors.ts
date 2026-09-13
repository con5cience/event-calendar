import { site } from "./site";

export function venueColor(venue: string): string {
  return Object.hasOwn(site.venue_colors, venue)
    ? site.venue_colors[venue]
    : "#6C5CE7";
}
