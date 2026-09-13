// Approved presentation palette. Keep assignments fixed as the catalog changes.
// Keys are canonical venue names from the calendar projection, not source names.
export const venueColors: Readonly<Record<string, string>> = {
  "Ball Arena": "#00B8D4",
  "The Black Box": "#AC4FE8",
  "The Black Buzzard": "#B8C0CC",
  "Bluebird Theater": "#90D8FF",
  Cervantes: "#F4DD55",
  "The Federal Theatre": "#899633",
  "Fiddler's Green Amphitheatre": "#228F60",
  "Fillmore Auditorium": "#D9B6FF",
  "Globe Hall": "#D58A16",
  "Gothic Theatre": "#FFAD52",
  "Herb's": "#F6D8B0",
  "Hi-Dive": "#BCED79",
  HQ: "#427BE8",
  "Larimer Lounge": "#FF82C5",
  "Levitt Pavilion Denver": "#A85583",
  "Lost Lake": "#286C9B",
  Marquis: "#F1F1EC",
  "Meow Wolf Denver": "#E335D1",
  "Mission Ballroom": "#56F0E5",
  "Ogden Theatre": "#B78059",
  "Ophelia's Electric Soapbox": "#247F7B",
  "The Oriental Theater": "#D0B7D0",
  "Paramount Theatre": "#9F792D",
  "Red Rocks Amphitheatre": "#DD7027",
  "The Roxy Theatre": "#71828E",
  Summit: "#7379BD",
};

export function venueColor(venue: string): string {
  return Object.hasOwn(venueColors, venue) ? venueColors[venue] : "#6C5CE7";
}
