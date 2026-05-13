import fs from "fs";
import path from "path";
import { buildSchemaBundleJSON } from "./schema-bundle-builder";
import { resolveAtlasProtocolPackageRoot } from "./path-utils";

function main(): void {
  try {
    const atlasProtocolRoot = resolveAtlasProtocolPackageRoot();
    const outFile = path.join(atlasProtocolRoot, "generated", "schema-bundle.json");
    const text = buildSchemaBundleJSON(atlasProtocolRoot);

    fs.mkdirSync(path.dirname(outFile), { recursive: true });
    fs.writeFileSync(outFile, text);
    console.log(`[atlas-protocol] wrote ${outFile}`);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    console.error(`[atlas-protocol] export-schema-bundle failed: ${message}`);
    process.exit(1);
  }
}

main();
