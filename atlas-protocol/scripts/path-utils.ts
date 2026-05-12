import fs from "fs";
import path from "path";

function findAncestorWithPaths(start: string, requiredPaths: string[]): string | undefined {
  let current = path.resolve(start);
  while (true) {
    if (requiredPaths.every((requiredPath) => fs.existsSync(path.join(current, requiredPath)))) {
      return current;
    }
    const parent = path.dirname(current);
    if (parent === current) {
      return undefined;
    }
    current = parent;
  }
}

export function resolveAtlasProtocolPackageRoot(): string {
  for (const start of [__dirname, process.cwd()]) {
    const found = findAncestorWithPaths(start, ["package.json", path.join("source", "schemas")]);
    if (found !== undefined) {
      return found;
    }
  }
  throw new Error("unable to locate atlas-protocol package root");
}

export function resolveAtlasProtocolRuntimeRoot(): string {
  if (process.env.ATLAS_PROTOCOL_ROOT) {
    return path.resolve(process.env.ATLAS_PROTOCOL_ROOT);
  }
  return resolveAtlasProtocolPackageRoot();
}

export function resolveRepoRoot(atlasProtocolRoot: string): string {
  for (const start of [atlasProtocolRoot, path.dirname(atlasProtocolRoot), process.cwd()]) {
    const found = findAncestorWithPaths(start, ["atlas.py", "atlas-core"]);
    if (found !== undefined) {
      return found;
    }
  }
  return path.resolve(atlasProtocolRoot, "..");
}
