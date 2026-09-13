import Ajv2020 from "ajv/dist/2020";
import addFormats from "ajv-formats";
import { Temporal } from "temporal-polyfill";
import common from "../../internal/artifact/schema/v1/common.schema.json";
import artifactSchema from "../../internal/artifact/schema/v1/source-artifact.schema.json";
import catalogSchema from "../../internal/artifact/schema/v1/catalog.schema.json";
import configSchema from "../../internal/artifact/schema/v1/source-config.schema.json";
import type {
  AdmissionPolicy,
  AdultAdmission,
  Artifact,
  Catalog,
  Event,
  Price,
  SourceConfig,
  Venue,
} from "./types";
export type * from "./types";

const ajv = new Ajv2020({
  strict: true,
  strictRequired: false,
  allErrors: true,
});
addFormats(ajv, { mode: "full" });
ajv.addFormat("iana-time-zone", (value) => {
  if (!/^(UTC|[A-Za-z_]+(\/[A-Za-z0-9_+.-]+)+)$/.test(value)) return false;
  try {
    new Intl.DateTimeFormat("en", { timeZone: value });
    return true;
  } catch {
    return false;
  }
});
ajv.addFormat("safe-http-url", (value) => {
  if (
    !/^https?:\/\//.test(value) ||
    value.includes("\\") ||
    [...value].some((char) => char.charCodeAt(0) <= 32)
  )
    return false;
  try {
    const u = new URL(value);
    return !!u.hostname && !u.username && !u.password;
  } catch {
    return false;
  }
});
ajv.addSchema(common);
const checkArtifact = ajv.compile<Artifact>(artifactSchema);
const checkCatalog = ajv.compile<Catalog>(catalogSchema);
const checkConfig = ajv.compile<SourceConfig>(configSchema);
const maxDocumentBytes = 4 << 20;
function parse(text: string): unknown {
  if (new TextEncoder().encode(text).length > maxDocumentBytes)
    throw new Error("Document too large");
  return JSON.parse(text) as unknown;
}
function requireCondition(
  condition: unknown,
  message: string,
): asserts condition {
  if (!condition) throw new Error(message);
}
function checkPrice(price?: Price) {
  requireCondition(
    price?.min_minor === undefined ||
      price.max_minor === undefined ||
      price.min_minor <= price.max_minor,
    "Reversed price range",
  );
}
function checkAdmission(admission?: AdultAdmission) {
  let last = -1;
  for (const range of admission?.ranges ?? []) {
    requireCondition(
      range.min_age <= range.max_age && range.min_age > last,
      "Admission ranges must be ordered and disjoint",
    );
    last = range.max_age;
  }
}
export function expirationDate(date: string): string {
  return Temporal.PlainDate.from(date).add({ days: 90 }).toString();
}
export function expired(event: Event, today: string): boolean {
  return today >= event.expires_on;
}
export function effectiveVenue(defaultVenue: Venue, event: Event): Venue {
  return event.venue ?? defaultVenue;
}
export function effectivePolicy(
  defaultVenue: Venue,
  event: Event,
): AdmissionPolicy | undefined {
  return (
    event.admission_policy ??
    effectiveVenue(defaultVenue, event).admission_policy
  );
}

export function decodeArtifact(text: string): Artifact {
  const a = parse(text);
  requireCondition(checkArtifact(a), ajv.errorsText(checkArtifact.errors));
  requireCondition(a.coverage.from <= a.coverage.through, "Reversed coverage");
  checkAdmission(a.venue.admission_policy?.with_adult);
  const ids = new Set<string>(),
    paths = new Set<string>(),
    occurrences = new Set<string>();
  for (const e of a.events) {
    const venue = effectiveVenue(a.venue, e);
    checkAdmission(venue.admission_policy?.with_adult);
    checkAdmission(e.admission_policy?.with_adult);
    requireCondition(
      e.off_site === !!e.venue,
      "Off-site flag differs from venue attribution",
    );
    requireCondition(
      !e.venue || e.venue.key !== a.venue.key,
      "Off-site venue has default venue key",
    );
    requireCondition(
      e.expires_on === expirationDate(e.date),
      "Invalid expiration date",
    );
    const path = e.public_path.split("/");
    const prefix = a.source.id + "-";
    const suffix = e.id.slice(prefix.length);
    requireCondition(
      e.id.startsWith(prefix) && /^[a-f0-9]{12}$/.test(suffix),
      "Event ID must belong to source and contain its assigned suffix",
    );
    requireCondition(
      path[2] === venue.key &&
        path[3]?.startsWith(e.date + "-") &&
        path[3]?.endsWith("-" + suffix),
      "Public path differs from occurrence identity",
    );
    for (const instant of [e.doors_at, e.show_at])
      if (instant) {
        requireCondition(
          venue.timezone,
          "Timed event requires actual venue timezone",
        );
        requireCondition(
          Temporal.Instant.from(instant)
            .toZonedDateTimeISO(venue.timezone)
            .toPlainDate()
            .toString() === e.date,
          "Instant differs from actual venue date",
        );
      }
    checkPrice(e.price);
    const occurrence = JSON.stringify([e.upstream_id, e.date, venue.key]);
    requireCondition(
      !ids.has(e.id) &&
        !paths.has(e.public_path) &&
        !occurrences.has(occurrence),
      "Duplicate event identity, path, or occurrence",
    );
    ids.add(e.id);
    paths.add(e.public_path);
    occurrences.add(occurrence);
  }
  return a;
}
export function decodeCatalog(text: string): Catalog {
  const c = parse(text);
  requireCondition(checkCatalog(c), ajv.errorsText(checkCatalog.errors));
  const seen = new Set<string>();
  for (const ref of c.sources) {
    requireCondition(!seen.has(ref.source_id), "Duplicate catalog source");
    seen.add(ref.source_id);
    requireCondition(
      ref.artifact.startsWith(`sources/${ref.source_id}/`),
      "Artifact belongs to another source",
    );
  }
  return c;
}
export function validateConfig(value: unknown): SourceConfig {
  requireCondition(checkConfig(value), ajv.errorsText(checkConfig.errors));
  checkAdmission(value.venue.admission_policy?.with_adult);
  for (const rule of Object.values(value.admission_rules ?? {}))
    checkAdmission(rule);
  const seen = new Set<string>();
  for (const o of value.overrides ?? []) {
    const key = JSON.stringify([
      o.match.upstream_id,
      o.match.date,
      o.match.venue_key,
    ]);
    requireCondition(!seen.has(key), "Duplicate override selector");
    seen.add(key);
    for (const name of o.remove ?? [])
      requireCondition(
        !(name in (o.set ?? {})),
        `Override both sets and removes ${name}`,
      );
    checkPrice(o.set?.price);
    checkAdmission(o.set?.admission_policy?.with_adult);
  }
  return value;
}
