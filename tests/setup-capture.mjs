// Explicit locale for deterministic offline capture tests, never for jobs.
import { fileURLToPath } from "node:url";
process.env.SITE_DIR = fileURLToPath(
  new URL("../locales/denver/", import.meta.url),
);
