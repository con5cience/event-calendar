import { readFileSync } from "node:fs";
import { join } from "node:path";

export function captureProfiles() {
  const directory = process.env.SITE_DIR;
  if (!directory) throw Error("SITE_DIR is required for capture");
  const site = JSON.parse(readFileSync(join(directory, "site.json"), "utf8"));
  const profiles = JSON.parse(
    readFileSync(join(directory, "capture.json"), "utf8"),
  );
  for (const [id, profile] of Object.entries(profiles)) {
    if (!Object.hasOwn(site.sources, id))
      throw Error("Capture source is not assigned to this locale");
    if (!profile || typeof profile !== "object")
      throw Error("Invalid capture profile");
    profile.adapter = site.sources[id].adapter;
    profile.source_id = id;
    profile.locale_id = site.id;
    if (profile.city && !/^[a-z0-9-]+$/.test(profile.city))
      throw Error("Invalid capture city slug");
  }
  return profiles;
}
export function captureProfile(id) {
  const profiles = captureProfiles();
  if (!Object.hasOwn(profiles, id)) throw Error("Unsupported source");
  return profiles[id];
}
