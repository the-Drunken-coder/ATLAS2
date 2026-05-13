import fs from "fs";
import path from "path";
import { buildSchemaBundleJSON } from "./schema-bundle-builder";
import { resolveAtlasProtocolPackageRoot } from "./path-utils";

function main(): void {
  const args = process.argv.slice(2);
  const checkOnly = args.includes("--check");

  try {
    const atlasProtocolRoot = resolveAtlasProtocolPackageRoot();
    const outFile = path.join(atlasProtocolRoot, "generated", "schema-bundle.json");
    const text = buildSchemaBundleJSON(atlasProtocolRoot);

    if (checkOnly) {
      let diskText: string;
      try {
        diskText = fs.readFileSync(outFile, "utf8");
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        console.error(`[atlas-protocol] export-schema-bundle --check failed: ${message}`);
        process.exit(1);
      }
      if (diskText !== text) {
        console.error(
          `[atlas-protocol] export-schema-bundle --check: ${outFile} is stale (run npm run bundle to regenerate)`,
        );
        process.exit(1);
      }
      console.log("[atlas-protocol] export-schema-bundle --check passed");
      return;
    }

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
