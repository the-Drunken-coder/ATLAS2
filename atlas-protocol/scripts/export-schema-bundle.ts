import fs from "fs";
import path from "path";

function main(): void {
  const atlasProtocolRoot = path.resolve(__dirname, "..", "..");
  const schemaDir = path.join(atlasProtocolRoot, "source", "schemas");
  const outFile = path.join(atlasProtocolRoot, "generated", "schema-bundle.json");
  const pkg = JSON.parse(
    fs.readFileSync(path.join(atlasProtocolRoot, "package.json"), "utf8"),
  ) as { version: string };

  const schemas: Record<string, unknown> = {};
  for (const name of fs.readdirSync(schemaDir)) {
    if (!name.endsWith(".schema.json")) {
      continue;
    }
    const key = name.replace(/\.schema\.json$/, "");
    schemas[key] = JSON.parse(fs.readFileSync(path.join(schemaDir, name), "utf8"));
  }

  const bundle = {
    protocolVersion: pkg.version,
    schemas,
  };

  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  fs.writeFileSync(outFile, `${JSON.stringify(bundle, null, 2)}\n`);
  console.log(`[atlas-protocol] wrote ${outFile}`);
}

main();
