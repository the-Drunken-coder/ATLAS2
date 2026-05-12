import fs from "fs";
import path from "path";
import { AtlasProtocolValidator, type ResourceKind } from "../packages/typescript/src";

function usage(): never {
  console.error(
    "Usage: node dist/scripts/validate.js --resource <entity|task|observation|object|commandCatalog|customSection> --file <path.json> [--variant <name>] [--example <key>]",
  );
  process.exit(2);
}

function main(): void {
  const argv = process.argv.slice(2);
  let resource: ResourceKind | undefined;
  let filePath: string | undefined;
  let variant: string | undefined;
  let example: string | undefined;

  for (let i = 0; i < argv.length; i += 1) {
    const a = argv[i];
    if (a === "--resource" && argv[i + 1]) {
      resource = argv[i + 1] as ResourceKind;
      i += 1;
    } else if (a === "--file" && argv[i + 1]) {
      filePath = argv[i + 1];
      i += 1;
    } else if (a === "--variant" && argv[i + 1]) {
      variant = argv[i + 1];
      i += 1;
    } else if (a === "--example" && argv[i + 1]) {
      example = argv[i + 1];
      i += 1;
    }
  }

  if (!resource || !filePath) {
    usage();
  }

  const atlasProtocolRoot = process.env.ATLAS_PROTOCOL_ROOT
    ? path.resolve(process.env.ATLAS_PROTOCOL_ROOT)
    : path.resolve(__dirname, "..", "..");
  const repoRoot = path.resolve(atlasProtocolRoot, "..");
  const validator = new AtlasProtocolValidator(repoRoot, atlasProtocolRoot);

  const absoluteFile = resolveValidateFilePath(repoRoot, filePath);
  const text = fs.readFileSync(absoluteFile, "utf8");
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    console.log(
      JSON.stringify([
        { field: "json", code: "invalid_json", message: "json must be valid JSON" },
      ]),
    );
    process.exit(1);
  }
  if (example !== undefined) {
    if (!isJsonObject(value) || !(example in value)) {
      console.error(`example key not found: ${example}`);
      process.exit(2);
    }
    value = value[example];
  }
  const issues = validator.validateValue(resource, value, {
    variant,
    rawText: example === undefined ? text : JSON.stringify(value),
  });

  if (issues.length > 0) {
    console.log(JSON.stringify(issues, null, 2));
    process.exit(1);
  }
  console.log("[]");
}

main();

function resolveValidateFilePath(repoRoot: string, filePath: string): string {
  if (path.isAbsolute(filePath)) {
    return filePath;
  }
  const fromCwd = path.resolve(process.cwd(), filePath);
  if (fs.existsSync(fromCwd)) {
    return fromCwd;
  }
  const fromRepo = path.resolve(repoRoot, filePath);
  if (fs.existsSync(fromRepo)) {
    return fromRepo;
  }
  return fromCwd;
}

function isJsonObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
