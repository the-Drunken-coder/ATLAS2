import fs from "fs";
import path from "path";
import { resolveAtlasProtocolPackageRoot } from "./path-utils";

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

function main(): void {
  try {
    const atlasProtocolRoot = resolveAtlasProtocolPackageRoot();
    const schemaDir = path.join(atlasProtocolRoot, "source", "schemas");
    const outFile = path.join(atlasProtocolRoot, "generated", "schema-bundle.json");
    const pkg = readJsonFile<{ version: string }>(
      path.join(atlasProtocolRoot, "package.json"),
      "package.json",
    );

    let schemaNames: string[];
    try {
      schemaNames = fs.readdirSync(schemaDir);
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

    fs.mkdirSync(path.dirname(outFile), { recursive: true });
    fs.writeFileSync(outFile, `${JSON.stringify(bundle, null, 2)}\n`);
    console.log(`[atlas-protocol] wrote ${outFile}`);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(`[atlas-protocol] export-schema-bundle failed: ${message}`);
    process.exit(1);
  }
}

main();
