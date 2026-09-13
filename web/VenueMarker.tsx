import { venueColor } from "./venueColors";

export function VenueMarker({ venue }: { venue: string }) {
  return (
    <span
      className="venue-marker"
      aria-hidden="true"
      style={{ backgroundColor: venueColor(venue) }}
    />
  );
}
