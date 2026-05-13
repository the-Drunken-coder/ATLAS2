import fs from "fs";
import path from "path";

function readJsonFile<T>(filePath: string, label: string): T {
  let text: string;
  try {
    text = fs.readFileSync(filePath, "utf8");
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`failed to read ${label} at ${filePath}: ${message}`);
  }
  try {
    return JSON.parse(text) as T;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`failed to parse ${label} at ${filePath}: ${message}`);
  }
}

/**
 * Deterministic JSON for `generated/schema-bundle.json` and the Go copy.
 * Sorted directory order, stable stringify (2-space indent + trailing newline).
 */
export function buildSchemaBundleJSON(atlasProtocolRoot: string): string {
  const schemaDir = path.join(atlasProtocolRoot, "source", "schemas");
  const pkg = readJsonFile<{ version: string }>(
    path.join(atlasProtocolRoot, "package.json"),
    "package.json",
  );

  let schemaNames: string[];
  try {
    schemaNames = fs.readdirSync(schemaDir).sort();
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    throw new Error(`failed to read schema directory ${schemaDir}: ${message}`);
  }

  const schemas: Record<string, unknown> = {};
  for (const name of schemaNames) {
    if (!name.endsWith(".schema.json")) {
      continue;
    }
    const key = name.replace(/\.schema\.json$/, "");
    schemas[key] = readJsonFile(path.join(schemaDir, name), `schema ${name}`);
  }

  const bundle = {
    protocolVersion: pkg.version,
    schemas,
  };

  return `${JSON.stringify(bundle, null, 2)}\n`;
}
