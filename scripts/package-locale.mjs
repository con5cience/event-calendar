// Export one self-contained build context. Never deploy or overwrite a context.
import { mkdirSync, readFileSync, writeFileSync, existsSync } from "node:fs";
import { resolve, join, relative, isAbsolute } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

export function validateLocaleID(id) {
  if (typeof id !== "string" || !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(id))
    throw Error("Select exactly one locale identifier");
  return id;
}
export function packageLocale(id, destination) {
  validateLocaleID(id);
  const root = fileURLToPath(new URL("../", import.meta.url));
  const site = JSON.parse(
    readFileSync(join(root, "locales", id, "site.json"), "utf8"),
  );
  if (site.id !== id)
    throw Error("Locale directory and configuration ID differ");
  if (!existsSync(join(root, ".artifacts", id, "catalog.json")))
    throw Error("Locale catalog is missing");
  const output = resolve(destination);
  const within = relative(root, output);
  if (
    !within ||
    (!within.startsWith("..") &&
      !isAbsolute(within) &&
      !within.startsWith(".builds/"))
  )
    throw Error("Export outside the repository or under .builds/");
  mkdirSync(output); // Refuse existing destinations, including symlinks.
  const result = spawnSync(
    "docker",
    [
      "build",
      "-f",
      "Dockerfile.railway",
      "--build-arg",
      `LOCALE=${id}`,
      "--target",
      "upload-context",
      "--output",
      `type=local,dest=${output}`,
      ".",
    ],
    { cwd: root, stdio: "inherit" },
  );
  if (result.error || result.status !== 0)
    throw Error(`Export failed; incomplete context retained at ${output}`);
  const dockerfile = join(output, "Dockerfile.railway");
  writeFileSync(
    dockerfile,
    readFileSync(dockerfile, "utf8").replaceAll(
      "ARG LOCALE\n",
      `ARG LOCALE=${id}\n`,
    ),
  );
  console.log(`Standalone ${id} build context: ${output}`);
}
if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  if (process.argv.length !== 4)
    throw Error(
      "Usage: node scripts/package-locale.mjs LOCALE NEW_OUTPUT_DIRECTORY",
    );
  packageLocale(process.argv[2], process.argv[3]);
}
