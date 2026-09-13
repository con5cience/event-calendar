// Version-one publication types. These are separate from the prototype UI model.
// Both language bindings are checked against the bundled schemas and shared corpus.
export type AgeCategory = "All ages" | "13+" | "16+" | "18+" | "21+";
export interface AdmissionPolicy {
  with_adult?: AdultAdmission;
  text: string;
  url?: string;
  category?: AgeCategory;
}
export interface AdultAdmission {
  url: string;
  reviewed_on: string;
  ranges: { min_age: number; max_age: number; condition: string }[];
}
export interface Venue {
  key: string;
  name: string;
  timezone?: string;
  address?: string;
  phone?: string;
  website?: string;
  admission_policy?: AdmissionPolicy;
}
export interface Source {
  id: string;
  adapter: string;
}
export interface Price {
  text: string;
  amount_minor?: number;
  min_minor?: number;
  max_minor?: number;
  currency?: string;
}
export interface EventData {
  status?: string;
  date: string;
  title: string;
  venue?: Venue;
  performers?: string[];
  doors_at?: string;
  show_at?: string;
  price?: Price;
  admission_policy?: AdmissionPolicy;
  event_url?: string;
  ticket_url?: string;
}
export interface Event extends EventData {
  id: string;
  upstream_id: string;
  public_path: string;
  off_site: boolean;
  listed: boolean;
  expires_on: string;
}
export interface Coverage {
  from: string;
  through: string;
  complete: true;
}
export interface Artifact {
  schema_version: 1;
  source: Source;
  venue: Venue & { timezone: string };
  generated_at: string;
  coverage: Coverage;
  events: Event[];
}
export interface ArtifactRef {
  source_id: string;
  artifact: string;
  sha256: string;
}
export interface Catalog {
  schema_version: 1;
  generation: string;
  generated_at: string;
  sources: ArtifactRef[];
}
export interface Match {
  upstream_id: string;
  date: string;
  venue_key: string;
}
export type OverrideFields = Omit<EventData, "date" | "venue">;
export interface Override {
  match: Match;
  set?: Partial<OverrideFields>;
  remove?: Exclude<keyof OverrideFields, "title">[];
}
export interface SourceConfig {
  admission_rules?: Partial<Record<AgeCategory, AdultAdmission>>;
  schema_version: 1;
  source: Source;
  venue: Venue & { timezone: string };
  state: "new" | "established";
  adapter_options?: Record<string, string>;
  overrides?: Override[];
}
